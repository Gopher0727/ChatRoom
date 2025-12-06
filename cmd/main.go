package main

import (
	"log"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/cache"
	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/internal/routers"
	"github.com/Gopher0727/ChatRoom/internal/services"
	"github.com/Gopher0727/ChatRoom/pkg/middlewares"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
	"github.com/Gopher0727/ChatRoom/pkg/ws"
)

func main() {
	cfg, err := config.LoadConfig("./config.toml")
	// fmt.Printf("%+v\n", cfg)
	if err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	// 初始化全局限流器
	middlewares.InitGlobalLimiter(cfg.RateLimit.Burst, cfg.RateLimit.QPS)

	// 初始化全局 Worker Pool (协程池)
	// 用于异步处理请求，防止高并发下 Goroutine 暴涨
	utils.InitGlobalWorkerPool(cfg.WorkerPool.Size, cfg.WorkerPool.QueueSize)

	// 初始化 PostgreSQL
	dsn := cache.BuildDSN(cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName)
	postgres, err := cache.InitPostgres(dsn, cfg.Postgres.MaxIdleConns, cfg.Postgres.MaxOpenConns)
	if err != nil {
		log.Fatalf("postgres 初始化失败: %v", err)
	}

	// 初始化 Redis
	redisClient, err := cache.InitRedis(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.PoolSize, cfg.Redis.MinIdleConns)
	if err != nil {
		log.Fatalf("redis 初始化失败: %v", err)
	}

	// 初始化仓储层
	userRepo := repositories.NewUserRepository(postgres, redisClient)
	guildRepo := repositories.NewGuildRepository(postgres, redisClient)

	// 初始化服务层
	userService := services.NewUserService(userRepo)
	guildService := services.NewGuildService(guildRepo, userRepo)

	// 初始化一致性哈希环（用于分布式路由）
	ring := utils.NewHashRing(128)
	for node, weight := range cfg.Gateway.Nodes {
		ring.Add(node, weight)
	}

	// 初始化 WebSocket Hub（注入哈希环与当前节点ID）
	hub := ws.NewHub(guildRepo, redisClient, ring, cfg.Gateway.NodeID)
	go hub.Run()

	// 初始化处理器
	userHandler := handlers.NewUserHandler(userService)
	guildHandler := handlers.NewGuildHandler(guildService, hub)

	// 配置并创建 Gin 引擎
	gin.SetMode(cfg.Server.Mode)

	r := gin.Default()

	// 设置路由
	routers.SetupRoutes(r,
		cfg, // 传入配置对象，用于中间件设置
		userHandler,
		guildHandler,
		hub,
		guildService,
	)

	// 启动服务器
	log.Printf("Starting server on :%d\n", cfg.Server.Port)
	if err := r.Run(":" + strconv.FormatInt(int64(cfg.Server.Port), 10)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
