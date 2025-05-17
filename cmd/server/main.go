package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"JollyRogerUserService/config"
	"JollyRogerUserService/internal/delivery/grpc"
	"JollyRogerUserService/internal/repository/postgres"
	"JollyRogerUserService/internal/repository/redis"
	"JollyRogerUserService/internal/service"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/logger"
	userProto "JollyRogerUserService/pkg/proto/user"
	"JollyRogerUserService/pkg/server"

	"go.uber.org/zap"
	grpcServer "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Версия сервиса
const (
	ServiceVersion = "1.0.0"
)

func main() {
	// Инициализация логгера
	log := logger.NewLogger()
	log.Info("Запуск сервиса пользователей", zap.String("version", ServiceVersion))

	// Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Не удалось загрузить конфигурацию", zap.Error(err))
	}

	// Определение номеров портов
	grpcPort := cfg.GRPC.Port
	healthPort := grpcPort + 100
	metricsPort := grpcPort + 200

	// Создаем механизм graceful shutdown
	gracefulShutdown := server.NewGracefulShutdown(log, 30*time.Second)

	// Подключение к PostgreSQL
	db, err := database.NewPostgresDB(cfg.Postgres)
	if err != nil {
		log.Fatal("Не удалось подключиться к PostgreSQL", zap.Error(err))
	}
	log.Info("Подключение к PostgreSQL установлено")

	// Получаем базовое подключение к PostgreSQL для закрытия
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Не удалось получить экземпляр SQL DB", zap.Error(err))
	}

	// Добавляем закрытие соединения с PostgreSQL при завершении
	gracefulShutdown.AddShutdownFunc(func(ctx context.Context) error {
		log.Info("Закрытие соединения с PostgreSQL")
		return sqlDB.Close()
	})

	// Подключение к Redis
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatal("Не удалось подключиться к Redis", zap.Error(err))
	}
	log.Info("Подключение к Redis установлено")

	// Добавляем закрытие соединения с Redis при завершении
	gracefulShutdown.AddShutdownFunc(func(ctx context.Context) error {
		log.Info("Закрытие соединения с Redis")
		return redisClient.Close()
	})

	// Создаем проверку здоровья баз данных
	healthChecker := database.NewDatabaseHealthChecker(db, redisClient, log)

	// Запускаем сервер для метрик Prometheus
	metricsServer := server.MetricsServer(strconv.Itoa(metricsPort))

	// Добавляем остановку сервера метрик при завершении
	gracefulShutdown.AddShutdownFunc(func(ctx context.Context) error {
		log.Info("Остановка сервера метрик")
		return metricsServer.Shutdown(ctx)
	})

	// Инициализация отказоустойчивых репозиториев
	userRepo := postgres.NewResilientUserRepository(db, redisClient, log)
	cacheRepo := redis.NewResilientCacheRepository(redisClient, db, log)

	// Инициализация сервиса
	userService := service.NewUserService(userRepo, cacheRepo, log)

	// Инициализация gRPC сервера
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatal("Не удалось запустить прослушивание порта", zap.Error(err))
	}

	// Создаем gRPC сервер с перехватчиками для метрик и трассировки
	s := grpcServer.NewServer(
		grpcServer.ChainUnaryInterceptor(
			server.TracingUnaryInterceptor(log),
			server.MetricsUnaryInterceptor(),
		),
	)

	userHandler := grpc.NewUserHandler(userService, log)
	userProto.RegisterJollyRogerUserServiceServer(s, userHandler)

	// Регистрация сервиса проверки здоровья для gRPC
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s, healthServer)

	// Устанавливаем начальное состояние сервиса
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Включение reflection для удобства отладки
	reflection.Register(s)

	// Создаем и запускаем HTTP сервер для проверки здоровья
	healthCheck := server.NewHealthCheck(healthChecker, log, ServiceVersion)
	healthCheck.StartServer(healthPort)

	// Добавляем остановку HTTP сервера для проверки здоровья при завершении
	gracefulShutdown.AddShutdownFunc(func(ctx context.Context) error {
		log.Info("Остановка сервера проверки здоровья")
		return healthCheck.Stop(ctx)
	})

	// Добавляем остановку gRPC сервера при завершении
	gracefulShutdown.AddShutdownFunc(func(ctx context.Context) error {
		log.Info("Остановка gRPC сервера")
		s.GracefulStop()
		return nil
	})

	// Запуск gRPC сервера в отдельной горутине
	go func() {
		log.Info("Запуск gRPC сервера", zap.Int("port", grpcPort))
		if err := s.Serve(lis); err != nil {
			log.Fatal("Не удалось запустить сервер", zap.Error(err))
		}
	}()

	// Логируем информацию о версии и PID
	hostname, _ := os.Hostname()
	log.Info("Сервис успешно запущен",
		zap.Int("grpc_port", grpcPort),
		zap.Int("health_port", healthPort),
		zap.Int("metrics_port", metricsPort),
		zap.String("version", ServiceVersion),
		zap.Int("pid", os.Getpid()),
		zap.String("hostname", hostname))

	// Ожидаем сигнала остановки
	gracefulShutdown.Wait()
	log.Info("Завершение работы сервиса выполнено")
}
