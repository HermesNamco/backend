package pool

import (
	"context"
	"sync"
	"sync/atomic"
)

// 协程池
type WorkerPool struct {
	workers    int32
	maxWorkers int32
	minWorkers int32

	taskQueue  chan func()
	workerChan chan struct{}

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// 监控指标
	submitted int64
	completed int64
}

func NewWorkerPool(min, max int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPool{
		minWorkers: int32(min),
		maxWorkers: int32(max),
		taskQueue:  make(chan func(), max*2), // 缓冲队列
		workerChan: make(chan struct{}, max),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动最小数量的工作协程
	for i := 0; i < min; i++ {
		wp.addWorker()
	}

	return wp
}

func (wp *WorkerPool) Submit(task func()) bool {
	select {
	case wp.taskQueue <- task:
		atomic.AddInt64(&wp.submitted, 1)

		// 动态扩容检查
		if len(wp.taskQueue) > cap(wp.taskQueue)/2 {
			wp.tryAddWorker()
		}
		return true
	case <-wp.ctx.Done():
		return false
	default:
		return false
	}
}

func (wp *WorkerPool) tryAddWorker() {
	if atomic.LoadInt32(&wp.workers) < wp.maxWorkers {
		select {
		case wp.workerChan <- struct{}{}:
			wp.addWorker()
		default:
		}
	}
}

func (wp *WorkerPool) addWorker() {
	atomic.AddInt32(&wp.workers, 1)
	wp.wg.Add(1)

	go func() {
		defer func() {
			wp.wg.Done()
			atomic.AddInt32(&wp.workers, -1)
			<-wp.workerChan
		}()

		for {
			select {
			case task := <-wp.taskQueue:
				task()
				atomic.AddInt64(&wp.completed, 1)
			case <-wp.ctx.Done():
				return
			}
		}
	}()
}

func (wp *WorkerPool) Close() {
	wp.cancel()
	wp.wg.Wait()
}

func (wp *WorkerPool) Stats() (submitted, completed int64, workers int32) {
	return atomic.LoadInt64(&wp.submitted),
		atomic.LoadInt64(&wp.completed),
		atomic.LoadInt32(&wp.workers)
}
