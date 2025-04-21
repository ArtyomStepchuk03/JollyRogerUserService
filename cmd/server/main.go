package main

import (
	"JollyRogerUserService/config"
	"JollyRogerUserService/internal/delivery/grpc"
	"JollyRogerUserService/internal/repository/postgres"
	"JollyRogerUserService/internal/repository/redis"
	"JollyRogerUserService/internal/service"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/logger"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	userProto "JollyRogerUserService/pkg/proto/user"
	"go.uber.org/zap"
	grpcServer "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Инициализация логгера
	log := logger.NewLogger()
	log.Info("Starting user service")

	// Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config", zap.Error(err))
	}

	// Подключение к PostgreSQL
	db, err := database.NewPostgresDB(cfg.Postgres)
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	log.Info("Connected to PostgreSQL")

	// Подключение к Redis
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	log.Info("Connected to Redis")

	// Инициализация репозиториев
	userRepo := postgres.NewUserRepository(db)
	cacheRepo := redis.NewCacheRepository(redisClient)

	// Инициализация сервиса
	userService := service.NewUserService(userRepo, cacheRepo, log)

	// Инициализация gRPC сервера
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}

	s := grpcServer.NewServer()
	userHandler := grpc.NewUserHandler(userService, log)
	userProto.RegisterUserServiceServer(s, userHandler)

	// Включение reflection для удобства отладки (в production можно отключить)
	reflection.Register(s)

	// Запуск gRPC сервера в отдельной горутине
	go func() {
		log.Info("Starting gRPC server", zap.Int("port", cfg.GRPC.Port))
		if err := s.Serve(lis); err != nil {
			log.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Обработка сигналов для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	s.GracefulStop()
	log.Info("Server stopped")
}
