package pool

import (
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

//task handler
type Handler func()

// runtime worker num config when u need
type RuntimeWorkerNumFunc func() int

var (
	ERR_FULL    = errors.New("queue full")
	ERR_CLOSED  = errors.New("queue closed")
	ERR_TIMEOUT = errors.New("timeout")

	_WORK_ID int64
)

type WorkerPool struct {
	taskQueue            chan Handler
	exit                 chan struct{}
	capCnt               int
	lock                 sync.Mutex
	workers              []*worker
	runtimeWorkerNumFunc RuntimeWorkerNumFunc
}

func NewWorkerPool(capCnt, startWorkerNum int) *WorkerPool {
	return NewWorkerPoolWithRuntimeWorkerNum(capCnt, startWorkerNum, nil)
}

func NewWorkerPoolWithRuntimeWorkerNum(capCnt, startWorkerNum int, runtimeWorkerNumFunc RuntimeWorkerNumFunc) *WorkerPool {
	workerPool := &WorkerPool{
		exit:      make(chan struct{}),
		capCnt:    capCnt,
		taskQueue: make(chan Handler, capCnt),
	}
	for i := 0; i < startWorkerNum; i++ {
		workerPool.addWorker()
	}
	workerPool.runtimeWorkerNumFunc = runtimeWorkerNumFunc
	if workerPool.runtimeWorkerNumFunc != nil {
		go workerPool.workerSentinel()
	}
	return workerPool
}

func (p *WorkerPool) workerSentinel() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		runtimeWorkerNum := p.runtimeWorkerNumFunc()
		if runtimeWorkerNum == len(p.workers) {
			continue
		}
		p.lock.Lock()
		diffNum := runtimeWorkerNum - len(p.workers)
		adjustNum := int(math.Abs(float64(diffNum)))
		p.adjustWorkers(adjustNum, diffNum > 0)
		p.lock.Unlock()
		fmt.Printf("WOKER NUM %d diffNum %d \n", len(p.workers), diffNum)
	}
}

func (p *WorkerPool) adjustWorkers(cnt int, isAdd bool) {
	for i := 0; i < cnt; i++ {
		if isAdd {
			p.addWorker()
		} else {
			p.deleteWorker()
		}
	}
}

func (p *WorkerPool) AddTaskSync(h Handler) error {
	return p.AddTaskWithTimout(h, 0)
}

func (p *WorkerPool) AddTaskWithTimout(h Handler, timeout time.Duration) error {
	if timeout == 0 {
		select {
		case <-p.exit:
			return ERR_CLOSED
		case p.taskQueue <- h:
			return nil
		}
	} else {
		select {
		case <-p.exit:
			return ERR_CLOSED
		case p.taskQueue <- h:
			return nil
		case <-time.After(timeout):
			return ERR_TIMEOUT
		}
	}

}

func (p *WorkerPool) addWorker() {
	worker := newWorker()
	go worker.working(p.taskQueue)
	p.workers = append(p.workers, worker)
}

func (p *WorkerPool) Close() {
	p.lock.Lock()
	defer p.lock.Unlock()
	close(p.exit)
	for _, worker := range p.workers {
		worker.close()
	}
	//TODO taskQueue task should return faields or continue process all?
}

func (p *WorkerPool) deleteWorker() {
	if len(p.workers) > 0 {
		worker0 := p.workers[0]
		worker0.close()
		p.workers = p.workers[1:]
	}
}

func (p *WorkerPool) Name() string {
	return "WorkerPool"
}

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
			fmt.Printf("unexpect error in worker running: %v\n", r)
			debug.PrintStack()
		}
	}()
	for {
		select {
		case task := <-taskQueue:
			w.cnt++
			task()
			fmt.Printf("WORK %v \t\t%d\n", w.id, w.cnt)
		case <-w.exit:
			return
		}
	}
}

func (w *worker) close() {
	close(w.exit)
}
