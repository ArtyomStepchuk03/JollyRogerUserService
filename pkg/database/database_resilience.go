package database

import (
	"JollyRogerUserService/pkg/apperrors"
	"context"
	"errors"
	"time"

	"JollyRogerUserService/pkg/resilience"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// HealthChecker предоставляет функции для проверки состояния баз данных
type HealthChecker struct {
	db           *gorm.DB
	redisClient  *redis.Client
	logger       *zap.Logger
	pgCircuit    *resilience.CircuitBreaker
	redisCircuit *resilience.CircuitBreaker
}

// NewDatabaseHealthChecker создает новый экземпляр проверки состояния баз данных
func NewDatabaseHealthChecker(db *gorm.DB, redisClient *redis.Client, logger *zap.Logger) *HealthChecker {
	failureThreshold, resetTimeout := resilience.DefaultCircuitBreakerOptions()

	return &HealthChecker{
		db:           db,
		redisClient:  redisClient,
		logger:       logger,
		pgCircuit:    resilience.NewCircuitBreaker(failureThreshold, resetTimeout, logger, apperrors.IgnoredErrors...),
		redisCircuit: resilience.NewCircuitBreaker(failureThreshold, resetTimeout, logger, apperrors.IgnoredErrors...),
	}
}

// IsDatabaseHealthy проверяет здоровье PostgreSQL
func (c *HealthChecker) IsDatabaseHealthy(ctx context.Context) bool {
	var result int
	err := c.pgCircuit.Execute(ctx, "postgres_health_check", func(ctx context.Context) error {
		// Проверяем подключение к PostgreSQL с таймаутом
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		sqlDB, err := c.db.DB()
		if err != nil {
			return err
		}

		// Простой запрос для проверки
		return sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	})

	return err == nil && result == 1
}

// IsRedisHealthy проверяет здоровье Redis
func (c *HealthChecker) IsRedisHealthy(ctx context.Context) bool {
	var err error
	err = c.redisCircuit.Execute(ctx, "redis_health_check", func(ctx context.Context) error {
		// Проверяем подключение к Redis с таймаутом
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		// Используем PING для проверки подключения
		_, err := c.redisClient.Ping(ctx).Result()
		return err
	})

	return err == nil
}

// WithDatabaseResilience выполняет операцию в базе данных с механизмами отказоустойчивости
func (c *HealthChecker) WithDatabaseResilience(ctx context.Context, operation string, fn func(ctx context.Context) error) error {
	err := c.pgCircuit.Execute(ctx, operation, fn)

	// Если получили ошибку redis.Nil или gorm.ErrRecordNotFound, не считаем её ошибкой для circuit breaker
	if errors.Is(err, redis.Nil) || errors.Is(err, gorm.ErrRecordNotFound) {
		logLevel := c.logger.Debug
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logLevel = c.logger.Debug
		}

		logLevel("Запись не найдена, это не ошибка для circuit breaker",
			zap.String("operation", operation),
			zap.Error(err))

		// Все равно возвращаем ошибку для обработки на уровне бизнес-логики
		return err
	}

	return err
}

// WithRedisResilience выполняет операцию в Redis с механизмами отказоустойчивости
func (c *HealthChecker) WithRedisResilience(ctx context.Context, operation string, fn func(ctx context.Context) error) error {
	err := c.redisCircuit.Execute(ctx, operation, fn)

	// Если получили ошибку redis.Nil, не считаем её ошибкой для circuit breaker
	if errors.Is(err, redis.Nil) {
		c.logger.Debug("Ключ не найден в Redis, это не ошибка для circuit breaker",
			zap.String("operation", operation))

		// Все равно возвращаем ошибку для обработки на уровне бизнес-логики
		return err
	}

	return err
}

// SafeDBOperation выполняет операцию в базе данных, логируя ошибки и добавляя контекст
func SafeDBOperation(ctx context.Context, db *gorm.DB, logger *zap.Logger, operation string, fn func(tx *gorm.DB) error) error {
	// Начинаем транзакцию с опцией повтора
	tx := db.WithContext(ctx)

	// Выполняем функцию
	err := fn(tx)

	// Обрабатываем ошибки
	if err != nil {
		logger.Error("Database operation failed",
			zap.String("operation", operation),
			zap.Error(err))

		// Проверяем тип ошибки для более подробного логирования
		if errors.Is(err, gorm.ErrInvalidTransaction) {
			logger.Error("Database transaction failed due to invalid transaction",
				zap.String("operation", operation))
		}

		return err
	}

	return nil
}

// SafeRedisOperation выполняет операцию в Redis, логируя ошибки и добавляя контекст
func SafeRedisOperation(ctx context.Context, client *redis.Client, logger *zap.Logger, operation string, fn func(ctx context.Context, client *redis.Client) error) error {
	// Устанавливаем таймаут для контекста, если его еще нет
	_, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}

	// Выполняем функцию
	err := fn(ctx, client)

	// Обрабатываем ошибки
	if err != nil {
		logger.Error("Redis operation failed",
			zap.String("operation", operation),
			zap.Error(err))

		// Проверяем тип ошибки для более подробного логирования
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error("Redis operation timed out", zap.String("operation", operation))
		} else if errors.Is(err, redis.ErrClosed) {
			logger.Error("Redis connection closed", zap.String("operation", operation))
		}

		return err
	}

	return nil
}
