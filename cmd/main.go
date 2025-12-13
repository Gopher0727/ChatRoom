package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/api"
	"github.com/Gopher0727/ChatRoom/internal/handler"
	"github.com/Gopher0727/ChatRoom/internal/model"
	"github.com/Gopher0727/ChatRoom/internal/pkg/gateway"
	"github.com/Gopher0727/ChatRoom/internal/pkg/kafka"
	"github.com/Gopher0727/ChatRoom/internal/pkg/redis"
	"github.com/Gopher0727/ChatRoom/internal/repository"
	"github.com/Gopher0727/ChatRoom/internal/service"
	"github.com/Gopher0727/ChatRoom/middleware/jwt"
	"github.com/Gopher0727/ChatRoom/utils"
	"github.com/Gopher0727/ChatRoom/utils/snowflake"
)

func main() {
	cfg, err := config.LoadConfig("./config.toml")
	// fmt.Printf("%+v\n", cfg)
	if err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	// 初始化全局限流器
	// middlewares.InitGlobalLimiter(cfg.RateLimit.Burst, cfg.RateLimit.QPS)

	// 初始化全局 Worker Pool (协程池)
	// 用于异步处理请求，防止高并发下 Goroutine 暴涨
	utils.InitGlobalWorkerPool(cfg.WorkerPool.Size, cfg.WorkerPool.QueueSize)

	// 初始化 PostgreSQL
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Postgres.Host, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName, cfg.Postgres.Port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto Migrate
	if err := db.AutoMigrate(
		&model.User{},
		&model.Guild{},
		&model.GuildMember{},
		&model.Message{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化 Redis
	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Fatalf("redis 初始化失败: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("关闭 Redis 连接失败: %v", err)
		}
	}()

	// 初始化 Snowflake
	sfConfig := snowflake.Config{
		WorkerID:     cfg.Snowflake.WorkerID,
		DatacenterID: cfg.Snowflake.DatacenterID,
	}
	sfGen, err := snowflake.NewGenerator(sfConfig)
	if err != nil {
		log.Fatalf("Failed to init snowflake: %v", err)
	}

	// 初始化 Kafka Producer
	kafkaProducer, err := kafka.NewProducer(&cfg.Kafka)
	if err != nil {
		log.Printf("Failed to init kafka producer: %v", err)
	}
	defer func() {
		if kafkaProducer != nil {
			kafkaProducer.Close()
		}
	}()

	// 初始化仓储层
	userRepo := repository.NewUserRepository(db)
	guildRepo := repository.NewGuildRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// 初始化 Token Manager
	tokenManager := jwt.NewTokenManager(cfg.JWT.Secret, cfg.JWT.ExpireHours, cfg.JWT.RefreshHours)

	// 初始化服务层
	authService := service.NewAuthService(userRepo, tokenManager)
	guildService := service.NewGuildService(guildRepo, userRepo)
	messageService := service.NewMessageService(messageRepo, guildService, sfGen, redisClient)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(authService)
	guildHandler := handler.NewGuildHandler(guildService)
	messageHandler := handler.NewMessageHandler(messageService)

	// 初始化 Gateway (WebSocket)
	ctx := context.Background()
	connManager := gateway.NewConnectionManager(ctx, &cfg.Websocket, redisClient)
	gwMessageHandler := gateway.NewMessageHandler(ctx, connManager, kafkaProducer, redisClient, cfg)

	// Start Gateway Subscriber (Subscribe to all guilds using pattern)
	if err := gwMessageHandler.StartSubscriber("guild:*"); err != nil {
		log.Printf("Failed to start gateway subscriber: %v", err)
	}

	// 配置并创建 Gin 引擎
	gin.SetMode(cfg.Server.Mode)

	r := gin.Default()

	// 设置 API 路由
	api.RegisterRoutes(r, authHandler, guildHandler, messageHandler)

	// 设置 WebSocket 路由
	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.Websocket.ReadBufferSize,
		WriteBufferSize: cfg.Websocket.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for demo
		},
	}

	r.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
			return
		}

		// Validate token
		claims, err := tokenManager.ParseToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Upgrade connection
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade websocket: %v", err)
			return
		}

		guildID := c.Query("guild_id") // Optional

		// Add connection to manager
		connection, err := connManager.AddConnection(claims.UserID, guildID, conn)
		if err != nil {
			log.Printf("Failed to add connection: %v", err)
			conn.Close()
			return
		}

		// Handle connection
		go gwMessageHandler.HandleConnection(connection)
	})

	// 启动服务器
	log.Printf("正在启动服务器，监听端口 :%d\n", cfg.Server.Port)
	if err := r.Run(":" + strconv.FormatInt(int64(cfg.Server.Port), 10)); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
