package main

import (
	"log"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/api"
	"github.com/Gopher0727/ChatRoom/internal/configs"
	"github.com/Gopher0727/ChatRoom/internal/db"
	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/internal/services"
)

func main() {
	cfg, err := configs.LoadConfig("./config.toml")
	// fmt.Printf("%+v\n", cfg)
	if err != nil {
		log.Fatalf("Init config failed: %v", err)
	}

	// 初始化 PostgreSQL
	dsn := db.BuildDSN(cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName)
	postgres, err := db.InitPostgres(dsn)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 初始化 Redis
	// redisClient, err := db.InitRedis(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	// if err != nil {
	// 	log.Fatalf("Failed to initialize redis: %v", err)
	// }

	// 初始化仓储层
	userRepo := repositories.NewUserRepository(postgres)

	// 初始化服务层
	userService := services.NewUserService(userRepo)

	// 初始化处理器
	userHandler := handlers.NewUserHandler(userService)

	// 配置并创建 Gin 引擎
	gin.SetMode(cfg.Server.Mode)

	r := gin.Default()

	// 设置路由
	api.SetupRoutes(r,
		userHandler,
	)

	// 启动服务器
	log.Printf("Starting server on :%d\n", cfg.Server.Port)
	if err := r.Run(":" + strconv.FormatInt(int64(cfg.Server.Port), 10)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
