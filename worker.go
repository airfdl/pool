package pool

import (
	"runtime/debug"
	"sync/atomic"
)

type worker struct {
	exit chan struct{}
	id   int64
	cnt  int64
}

func newWorker() *worker {
	return &worker{
		exit: make(chan struct{}),
		id:   atomic.AddInt64(&_WORK_ID, 1),
	}
}

func (w *worker) working(taskQueue <-chan Handler) {
	defer func() {
		if r := recover(); r != nil {
			//fmt.Printf("unexpect error in worker running: %v\n", r)
			debug.PrintStack()
		}
	}()
	for {
		select {
		case task := <-taskQueue:
			w.cnt++
			task()
			//fmt.Printf("WORK %v \t\t%d\n", w.id, w.cnt)
		case <-w.exit:
			return
		}
	}
}

func (w *worker) close() {
	close(w.exit)
}
