package utils

import (
	"log"
	"sync"
)

// WorkerPool 通用协程池
type WorkerPool struct {
	JobQueue  chan func()
	WorkerNum int
	wg        sync.WaitGroup
	quit      chan bool
}

var (
	GlobalWorkerPool *WorkerPool
	poolOnce         sync.Once
)

// InitGlobalWorkerPool 初始化全局协程池
func InitGlobalWorkerPool(workerNum int, queueSize int) {
	poolOnce.Do(func() {
		GlobalWorkerPool = NewWorkerPool(workerNum, queueSize)
		GlobalWorkerPool.Start()
	})
}

// NewWorkerPool 创建一个新的协程池
func NewWorkerPool(workerNum int, queueSize int) *WorkerPool {
	return &WorkerPool{
		JobQueue:  make(chan func(), queueSize),
		WorkerNum: workerNum,
		quit:      make(chan bool),
	}
}

// Start 启动协程池
func (p *WorkerPool) Start() {
	for i := 0; i < p.WorkerNum; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			for {
				select {
				case job := <-p.JobQueue:
					// 执行任务
					// 使用 defer recover 防止单个任务 panic 导致 worker 挂掉
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("Worker %d panic: %v", workerID, r)
							}
						}()
						job()
					}()
				case <-p.quit:
					return
				}
			}
		}(i)
	}
	log.Printf("WorkerPool started with %d workers", p.WorkerNum)
}

// Submit 提交任务到协程池
// 如果队列已满，此方法会阻塞，直到有空位
// 这实现了"不拒绝请求，而是排队等待"的需求
func (p *WorkerPool) Submit(job func()) {
	p.JobQueue <- job
}

// Stop 停止协程池
func (p *WorkerPool) Stop() {
	close(p.quit)
	p.wg.Wait()
}
