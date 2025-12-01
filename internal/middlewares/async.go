package middlewares

import (
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/utils"
)

// AsyncMiddleware 异步处理中间件
// 将请求的处理逻辑提交到 Worker Pool 中执行，而不是在 Gin 分配的 Goroutine 中直接执行。
// 这样可以严格控制并发处理的请求数量（CPU/DB 密集型操作），防止系统过载。
// 同时，由于 Worker Pool 有巨大的缓冲队列，请求不会被立即拒绝，而是排队等待处理。
func AsyncMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果没有初始化 Worker Pool，直接降级为同步执行
		if utils.GlobalWorkerPool == nil {
			c.Next()
			return
		}

		// 创建一个信号通道，用于等待 Worker 处理完成
		// 这是一个无缓冲通道，确保同步等待
		done := make(chan struct{})

		// 封装任务
		// 注意：这里我们利用了闭包特性捕获了 gin.Context
		// 虽然 gin.Context 不是线程安全的，但由于我们在主 Goroutine 中
		// 阻塞等待 (<-done)，所以同一时间只有一个 Goroutine (Worker) 在操作 c
		// 因此是安全的。
		task := func() {
			defer close(done) // 任务完成（无论成功还是 panic）后关闭通道
			c.Next()          // 执行后续的处理链（业务逻辑）
		}

		// 提交任务到 Worker Pool
		// 如果队列满了，这里会阻塞，直到有空位，从而实现"不拒绝但变慢"的效果
		utils.GlobalWorkerPool.Submit(task)

		// 主 Goroutine 挂起，等待任务完成
		// 这样对于 HTTP 客户端来说，依然是同步的 Request-Response 模型
		// 但服务端内部实现了排队和并发控制
		<-done
	}
}
