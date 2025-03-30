package main

import (
	"JollyRogerUserService/infrastructure"
	"JollyRogerUserService/internal/database"
	"JollyRogerUserService/internal/user"
	"JollyRogerUserService/pb"
	"os"

	"context"
	"fmt"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	// 🚀 Лог старта
	log.Println("🛡️  Starting Jolly Roger UserService...")

	// 📦 Подключаемся к PostgreSQL
	dbDsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "jolly_roger"),
		getEnv("DB_PORT", "5432"),
	)
	database.ConnectDB(dbDsn)

	// 🧱 Автомиграция (если надо)
	database.AutoMigrate(&user.User{}, &user.Details{}, &user.Settings{})

	// 🧠 Подключаем Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s",
			getEnv("REDIS_HOST", "localhost"),
			getEnv("REDIS_PORT", "6379"),
		),
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Redis connection failed: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// 🛠 Инициализация зависимостей
	userRepo := infrastructure.NewUserRepository(database.DB)
	userService := user.NewService(userRepo, rdb)
	handler := infrastructure.NewGRPCHandler(userService)

	// 🛰 Запуск gRPC-сервера
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen on port 50051: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, handler)

	log.Println("✅ gRPC server is running on port :50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("❌ Failed to serve gRPC server: %v", err)
	}
}

// getEnv получает переменные окружения с дефолтами
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
