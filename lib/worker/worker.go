package worker

import (
	"context"
	"fmt"
	"sync"
)

type Job interface {
	Run(ctx context.Context) error
}

type Worker struct {
	id         int
	workerPool chan chan Job // used to communicate between dispatcher and workers
	jobQueue   chan Job
	quit       chan bool
	job        *Job
	lock       sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewWorker(id int, workerPool chan chan Job) Worker {
	return Worker{
		id:         id,
		workerPool: workerPool,
		jobQueue:   make(chan Job),
		quit:       make(chan bool),
	}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		for {
			w.workerPool <- w.jobQueue

			select {
			case job := <-w.jobQueue:
				var jobCtx context.Context
				func() {
					w.lock.Lock()
					defer w.lock.Unlock()
					w.job = &job
					jobCtx, w.cancel = context.WithCancel(ctx)
				}()

				if err := job.Run(jobCtx); err != nil {
					fmt.Println(err)
				}

				func() {
					w.lock.Lock()
					defer w.lock.Unlock()
					w.job = nil
					w.ctx = nil
					w.cancel = nil
				}()
			case <-w.quit:
				return
			}
		}
	}()
}

func (w *Worker) Stop(ctx context.Context) {
	func() {
		w.lock.Lock()
		defer w.lock.Unlock()
		if w.cancel != nil {
			w.cancel()
		}
	}()

	w.quit <- true
}
