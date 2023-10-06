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

	"github.com/shirou/gopsutil/cpu"
	"go.temporal.io/sdk/log"
)

type workerTuner interface {
	Record(name string, t time.Duration)
	Update(name string, v float64)
	Start()
	Stop()
	setWorker(*baseWorker)
	ReportSyncMatchPoll()
	ReportMatchPoll()
	ReportEmptyPoll()
}

type workerTunerImpl struct {
	bw            *baseWorker
	wg            sync.WaitGroup
	logger        log.Logger
	update        chan struct{}
	stopCh        chan struct{}
	mu            sync.Mutex
	maxTaskSlots  int
	taskSlots     int
	pollers       int
	percent       float64
	memoryUsageMb uint64

	taskSlotsAvailable     atomic.Int64
	scheduleToStartLatency atomic.Int64
	syncMatches            atomic.Int64
	matches                atomic.Int64
	emptyPolls             atomic.Int64
}

func newWorkerTuner(logger log.Logger) workerTuner {
	return &workerTunerImpl{
		logger: logger,
		update: make(chan struct{}),
		stopCh: make(chan struct{}),
		mu:     sync.Mutex{},
	}
}

func (wt *workerTunerImpl) Record(name string, t time.Duration) {
	wt.scheduleToStartLatency.Store(t.Milliseconds())
	select {
	case wt.update <- struct{}{}:
	default:
	}
}

func (wt *workerTunerImpl) Update(name string, v float64) {
	wt.taskSlotsAvailable.Store(int64(v))
	select {
	case wt.update <- struct{}{}:
	default:
	}
}

func (wt *workerTunerImpl) Start() {
	wt.wg.Add(1)
	go func() {
		defer wt.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-wt.stopCh:
				ticker.Stop()
				return
			case <-ticker.C:
				percent, err := cpu.Percent(time.Second, false)
				if err != nil {
					fmt.Println(err)
				}
				wt.mu.Lock()
				wt.percent = percent[0]
				wt.memoryUsageMb = getSystemMemoryUsage()
				wt.mu.Unlock()
				wt.bw.metricsHandler.Gauge("temporal_memory_usage").Update(float64(wt.memoryUsageMb))
			}
		}
	}()
	wt.wg.Add(1)
	go func() {
		defer wt.wg.Done()
		for {
			select {
			case <-wt.stopCh:
				return
			case <-wt.update:
				wt.tune()
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (wt *workerTunerImpl) Stop() {
	close(wt.stopCh)
	wt.wg.Wait()
}

func (wt *workerTunerImpl) setWorker(bw *baseWorker) {
	wt.bw = bw
	wt.maxTaskSlots = 10 * wt.bw.options.maxConcurrentTask
	wt.taskSlots = wt.bw.options.maxConcurrentTask
	wt.pollers = wt.bw.options.pollerCount
}

func (wt *workerTunerImpl) ReportSyncMatchPoll() {
	wt.logger.Debug("sync match")
	wt.syncMatches.Add(1)
	select {
	case wt.update <- struct{}{}:
	default:
	}
}

func (wt *workerTunerImpl) ReportMatchPoll() {
	wt.logger.Debug("match")
	wt.matches.Add(1)
	select {
	case wt.update <- struct{}{}:
	default:
	}
}

func (wt *workerTunerImpl) ReportEmptyPoll() {
	wt.logger.Debug("empty poll")
	wt.emptyPolls.Add(1)
	select {
	case wt.update <- struct{}{}:
	default:
	}
}

func (wt *workerTunerImpl) tune() {
	if wt.bw == nil {
		return
	}
	wt.mu.Lock()
	defer wt.mu.Unlock()

	// if int(wt.taskSlotsAvailable.Load()) <= wt.pollers && wt.taskSlots < wt.maxTaskSlots && wt.memoryUsageMb < 1024 {
	// 	wt.logger.Debug("Adding task slot")
	// 	wt.taskSlots += 1
	// 	wt.bw.addTaskSlot(1)
	// }
	// if wt.scheduleToStartLatency.Load() > 1000 && wt.pollers < 100 {
	// 	wt.logger.Debug("Adding poller")
	// 	wt.pollers += 1
	// 	wt.scheduleToStartLatency.Store(0.0)
	// 	wt.bw.startPollerCh <- struct{}{}
	// }
	wt.logger.Debug("Calling tunner")

	if wt.syncMatches.Load() > 0 && wt.syncMatches.Load()/10 > wt.matches.Load() && wt.emptyPolls.Load() == 0 && wt.pollers < 150 {
		wt.logger.Debug("Adding poller")
		wt.pollers += 1
		wt.bw.startPollerCh <- struct{}{}
	} else if wt.syncMatches.Load() == 0 && wt.matches.Load() == 0 && wt.emptyPolls.Load() > 0 && wt.pollers > 2 {
		wt.logger.Debug("Removing poller")
		wt.pollers -= 1
		wt.bw.stopPollerCh <- struct{}{}
	}
	wt.syncMatches.Store(0)
	wt.emptyPolls.Store(0)
	wt.matches.Store(0)

	// if wt.memoryUsageMb > 1024 {
	// 	if wt.taskSlots > wt.pollers {
	// 		wt.logger.Debug("Removing task slot")
	// 		wt.taskSlots -= 1
	// 		wt.bw.addTaskSlot(-1)
	// 	}
	// }

	//time.Sleep(time.Second)
}

func getSystemMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return bToMb(m.Sys)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
