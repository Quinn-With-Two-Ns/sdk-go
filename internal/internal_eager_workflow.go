// The MIT License
//
// Copyright (c) 2022 Temporal Technologies Inc.  All rights reserved.
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
	"math/rand"
	"sync"
	"sync/atomic"

	"go.temporal.io/api/workflowservice/v1"
)

// eagerWorkflowDispatcher is responsible for finding an available worker for an eager workflow task.
type eagerWorkflowDispatcher struct {
	lock               sync.Mutex
	workersByTaskQueue map[string][]*workflowWorker
}

func (e *eagerWorkflowDispatcher) registerWorker(worker *workflowWorker) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.workersByTaskQueue[worker.executionParameters.TaskQueue] = append(e.workersByTaskQueue[worker.executionParameters.TaskQueue], worker)
}

func (e *eagerWorkflowDispatcher) tryGetEagerWorkflowExecutor(options *StartWorkflowOptions) *eagerWorkflowExecutor {
	e.lock.Lock()
	defer e.lock.Unlock()
	// Try every worker that is assigned to the desired task queue.
	workers := e.workersByTaskQueue[options.TaskQueue]
	rand.Shuffle(len(workers), func(i, j int) { workers[i], workers[j] = workers[j], workers[i] })
	for _, worker := range workers {
		executor := worker.reserveWorkflowExecutor()
		if executor != nil {
			return executor
		}
	}
	return nil
}

// eagerWorkflowExecutor is a worker-scoped executor for an eager workflow task.
type eagerWorkflowExecutor struct {
	handledResponse atomic.Bool
	worker          *workflowWorker
}

// handleResponse of an eager workflow task from a StartWorkflowExecution request.
func (e *eagerWorkflowExecutor) handleResponse(response *workflowservice.PollWorkflowTaskQueueResponse) {
	if !e.handledResponse.CompareAndSwap(false, true) {
		panic("eagerWorkflowExecutor trying to handle multiple responses")
	}
	// Before starting the goroutine we have to increase the wait group counter
	// that the poller would have otherwise increased
	e.worker.worker.stopWG.Add(1)
	// Asynchronously execute
	task := &eagerWorkflowTask{
		task: response,
	}
	go func() {
		// Mark completed when complete
		defer func() {
			e.release()
		}()

		// Process the task synchronously. We call the processor on the base
		// worker instead of a higher level so we can get the benefits of metrics,
		// stop wait group update, etc.
		e.worker.worker.processTask(task)
	}()
}

// release the executor task slot this eagerWorkflowExecutor was holding.
// If it is currently handling a responses or has already released the task slot
// then do nothing.
func (e *eagerWorkflowExecutor) release() {
	if e.handledResponse.CompareAndSwap(false, true) {
		// Assume there is room because it is reserved on creation, so we make a blocking send.
		// The processTask does not do this itself because our task is not *polledTask.
		e.worker.worker.pollerRequestCh <- struct{}{}
	}
}
