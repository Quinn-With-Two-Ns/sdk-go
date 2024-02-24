// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package internal

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/internal/common/metrics"
)

var (
	_ SlotSupplier          = (*fixedSlotSupplier)(nil)
	_ SlotSupplier          = (*memoryBoundSlotSupplier)(nil)
	_ PauseableSlotSupplier = (*pauseableSlotSupplier)(nil)
	_ SlotInfo              = (*WorkflowSlotInfo)(nil)
	_ SlotInfo              = (*ActivitySlotInfo)(nil)
	_ SlotInfo              = (*LocalActivitySlotInfo)(nil)
)

type (
	// SlotInfo is an interface that can be used to store information about a task slot
	SlotInfo interface {
		isSlotInfo()
	}

	// SlotSupplier is used to reserve and release task slots
	SlotSupplier interface {
		// ReserveSlot tries to reserver a task slot on the worker, blocks until a slot is available or stopCh is closed.
		// Returns false if no slot is reserved.
		ReserveSlot(stopCh chan struct{}) bool
		// TryReserveSlot tries to reserver a task slot on the worker without blocking
		TryReserveSlot() bool
		// MarkSlotInUse marks a slot as being used.
		MarkSlotInUse(slotInfo SlotInfo)
		// MarkSlotNotInUse marks a slot as not being used
		// TODO: This is only needed for eager tasks because they don't follow the normal task lifecycle.
		// not clear why they can't be cleaned up in the same way as normal tasks.
		MarkSlotNotInUse(slotInfo SlotInfo)
		// ReleaseSlot release a task slot acquired by the supplier
		// TODO: May be useful to supply info if processing the task failed.
		ReleaseSlot(err error)
	}

	// SlotSupplierAvailableSlots is a SlotSupplier that can report the number of available slots
	SlotSupplierAvailableSlots interface {
		AvailableSlots() int
	}

	// PauseableSlotSupplier is a SlotSupplier that can be paused and unpaused
	PauseableSlotSupplier interface {
		SlotSupplier
		// Pause pauses the slot supplier so no new slots can be reserved
		Pause()
		// Unpause unpauses the slot supplier so new slots can be reserved
		Unpause()
	}

	fixedSlotSupplier struct {
		slots chan struct{}
		// Must be atomically accessed
		taskSlotsAvailable      atomic.Int32
		taskSlotsAvailableGauge metrics.Gauge
	}

	memoryBoundSlotSupplier struct {
		memoryUsageLimitBytes int
		taskSlots             atomic.Int32
		taskSlotsInUse        atomic.Int32
		taskSlotsInUseGauge   metrics.Gauge
	}

	pauseableSlotSupplier struct {
		// TODO track the number of slots in use to tell how "paused" we are
		slotSupplier SlotSupplier
		mutex        sync.Mutex
		paused       bool
		pauseWaitCh  chan struct{}
	}

	// WorkflowSlotInfo contains information about a workflow task using a slot
	WorkflowSlotInfo struct {
		WorkflowType string
		WorkflowID   string
		RunID        string
		Eager        bool
	}

	// ActivitySlotInfo contains information about an activity task using a slot
	ActivitySlotInfo struct {
		ActivityType string
		ActivityID   string
		Eager        bool
	}

	// LocalActivitySlotInfo contains information about a local activity task using a slot
	LocalActivitySlotInfo struct {
		ActivityType string
	}
)

// isSlotInfo implements SlotInfo.
func (*LocalActivitySlotInfo) isSlotInfo() {
}

// isSlotInfo implements SlotInfo.
func (*ActivitySlotInfo) isSlotInfo() {
}

// isSlotInfo implements SlotInfo.
func (*WorkflowSlotInfo) isSlotInfo() {
}

// MarkSlotInUse implements SlotSupplier.
func (ps *pauseableSlotSupplier) MarkSlotInUse(slotInfo SlotInfo) {
	ps.slotSupplier.MarkSlotInUse(slotInfo)
}

// MarkSlotNotInUse implements SlotSupplier.
func (ps *pauseableSlotSupplier) MarkSlotNotInUse(slotInfo SlotInfo) {
	ps.slotSupplier.MarkSlotNotInUse(slotInfo)
}

// ReleaseSlot implements SlotSupplier.
func (ps *pauseableSlotSupplier) ReleaseSlot(err error) {
	ps.slotSupplier.ReleaseSlot(err)
}

// ReserveSlot implements SlotSupplier.
func (ps *pauseableSlotSupplier) ReserveSlot(stopCh chan struct{}) bool {
	ps.mutex.Lock()
	if ps.paused {
		ch := ps.pauseWaitCh
		ps.mutex.Unlock()
		select {
		case <-stopCh:
			return false
		case <-ch:
		}
	} else {
		ps.mutex.Unlock()
	}
	slot := ps.slotSupplier.ReserveSlot(stopCh)
	if !slot {
		return false
	}
	// If we are paused, we need to release the slot we just acquired
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	if ps.paused {
		ps.slotSupplier.ReleaseSlot(nil)
		return false
	}
	return slot
}

// TryReserveSlot implements SlotSupplier.
func (ps *pauseableSlotSupplier) TryReserveSlot() bool {
	if func() bool {
		ps.mutex.Lock()
		defer ps.mutex.Unlock()
		return ps.paused
	}() {
		return false
	}
	// TryReserveSlot should not block so we don't need to check again after acquiring the slot
	return ps.slotSupplier.TryReserveSlot()
}

func (ps *pauseableSlotSupplier) Pause() {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if ps.paused {
		return
	}

	ps.paused = true
}

func (ps *pauseableSlotSupplier) Unpause() {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if !ps.paused {
		return
	}
	close(ps.pauseWaitCh)
	ps.pauseWaitCh = make(chan struct{})
	ps.paused = false

}

func NewPauseableSlotSupplier(slotSupplier SlotSupplier) *pauseableSlotSupplier {
	return &pauseableSlotSupplier{
		slotSupplier: slotSupplier,
		pauseWaitCh:  make(chan struct{}),
	}
}

func NewMemoryBoundSlotSupplier(memoryUsageLimitBytes int, taskSlotsInUseGauge metrics.Gauge) *memoryBoundSlotSupplier {
	return &memoryBoundSlotSupplier{
		memoryUsageLimitBytes: memoryUsageLimitBytes,
		taskSlotsInUseGauge:   taskSlotsInUseGauge,
	}
}

// MarkSlotInUse implements SlotSupplier.
func (ms *memoryBoundSlotSupplier) MarkSlotInUse(slotInfo SlotInfo) {
	ms.taskSlotsInUseGauge.Update(float64(ms.taskSlotsInUse.Add(1)))
}

// MarkSlotNotInUse implements SlotSupplier.
func (ms *memoryBoundSlotSupplier) MarkSlotNotInUse(slotInfo SlotInfo) {
	ms.taskSlotsInUseGauge.Update(float64(ms.taskSlotsInUse.Add(-1)))
}

// ReleaseSlot implements SlotSupplier.
func (ms *memoryBoundSlotSupplier) ReleaseSlot(err error) {
	ms.taskSlots.Add(-1)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func (ms *memoryBoundSlotSupplier) canReserveSlot() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	inUseMem := m.StackInuse + m.HeapInuse + m.MSpanInuse + m.MCacheInuse
	projectedMemoryUsage := float64(inUseMem)
	fmt.Printf("Projected Memory Usage: %d Mb\n", bToMb(inUseMem))
	fmt.Printf("Allow new slot: %t\n", projectedMemoryUsage/float64(ms.memoryUsageLimitBytes) < 0.8)
	return projectedMemoryUsage/float64(ms.memoryUsageLimitBytes) < 0.8
}

// ReserveSlot implements SlotSupplier.
func (ms *memoryBoundSlotSupplier) ReserveSlot(stopCh chan struct{}) bool {
	if ms.canReserveSlot() {
		ms.taskSlots.Add(1)
		return true
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if ms.canReserveSlot() {
				ms.taskSlots.Add(1)
				return true
			}
		case <-stopCh:
			return false
		}
	}
}

// TryReserveSlot implements SlotSupplier.
func (ms *memoryBoundSlotSupplier) TryReserveSlot() bool {
	if ms.canReserveSlot() {
		ms.taskSlots.Add(1)
		return true
	} else {
		return false
	}
}

func NewFixedSlotSupplier(maxSlots int, taskSlotsAvailableGauge metrics.Gauge) *fixedSlotSupplier {
	s := &fixedSlotSupplier{
		slots:                   make(chan struct{}, maxSlots),
		taskSlotsAvailableGauge: taskSlotsAvailableGauge,
	}
	s.taskSlotsAvailable.Store(int32(maxSlots))
	for i := 0; i < maxSlots; i++ {
		s.slots <- struct{}{}
	}
	return s
}

// ReserveSlot implements SlotSupplier.
func (ss *fixedSlotSupplier) ReserveSlot(stopCh chan struct{}) bool {
	select {
	case <-ss.slots:
		return true
	case <-stopCh:
		return false
	}
}

// TryReserveSlot implements SlotSupplier.
func (ss *fixedSlotSupplier) TryReserveSlot() bool {
	select {
	case <-ss.slots:
		return true
	default:
		return false
	}
}

// MarkSlotInUse implements SlotSupplier.
func (ss *fixedSlotSupplier) MarkSlotInUse(slotInfo SlotInfo) {
	ss.taskSlotsAvailableGauge.Update(float64(ss.taskSlotsAvailable.Add(-1)))
}

// MarkSlotNotInUse implements SlotSupplier.
func (ss *fixedSlotSupplier) MarkSlotNotInUse(slotInfo SlotInfo) {
	ss.taskSlotsAvailableGauge.Update(float64(ss.taskSlotsAvailable.Add(1)))
}

// ReleaseSlot implements SlotSupplier.
func (ss *fixedSlotSupplier) ReleaseSlot(err error) {
	ss.slots <- struct{}{}
}

func taskToSlotInfo(task interface{}) SlotInfo {
	// TODO fill in more information
	switch task := task.(type) {
	case *workflowTask:
		return &WorkflowSlotInfo{
			WorkflowType: task.task.GetWorkflowType().GetName(),
			WorkflowID:   task.task.GetWorkflowExecution().GetWorkflowId(),
			RunID:        task.task.GetWorkflowExecution().GetRunId(),
		}
	case *eagerWorkflowTask:
		return &WorkflowSlotInfo{
			WorkflowType: task.task.GetWorkflowType().GetName(),
			WorkflowID:   task.task.GetWorkflowExecution().GetWorkflowId(),
			RunID:        task.task.GetWorkflowExecution().GetRunId(),
			Eager:        true,
		}
	case *activityTask:
		return &ActivitySlotInfo{
			ActivityType: task.task.GetActivityType().GetName(),
			ActivityID:   task.task.GetActivityId(),
			Eager:        task.eager,
		}
	case *localActivityTask:
		return &LocalActivitySlotInfo{
			ActivityType: task.params.ActivityType,
		}
	default:
		panic("unknown task type.")
	}
}
