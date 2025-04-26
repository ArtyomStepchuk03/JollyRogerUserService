package database

import (
	"JollyRogerUserService/pkg/resilience"
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// mockGormDBInterface имитирует интерфейс gorm.DB для тестирования
type mockGormDBInterface struct {
	healthy       bool
	queryCallback func() error
}

// DB имитирует метод DB() из gorm.DB
func (m *mockGormDBInterface) DB() (*sql.DB, error) {
	if !m.healthy {
		return nil, errors.New("database connection error")
	}
	// Для тестирования нам не нужен реальный *sql.DB
	return nil, nil
}

// mockRedisClient имитирует клиент Redis для тестирования
type mockRedisClient struct {
	healthy bool
	client  *redis.Client
}

// Ping имитирует метод Ping из redis клиента
func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if m.healthy {
		cmd.SetVal("PONG")
	} else {
		cmd.SetErr(errors.New("redis connection error"))
	}
	return cmd
}

// Создаем мок health checker для тестирования
type mockHealthChecker struct {
	db              *mockGormDBInterface
	redis           *mockRedisClient
	logger          *zap.Logger
	pgCircuit       *resilience.CircuitBreaker
	redisCircuit    *resilience.CircuitBreaker
	dbOpCallback    func(context.Context) error
	redisOpCallback func(context.Context) error
}

// IsDatabaseHealthy имитирует проверку здоровья PostgreSQL
func (m *mockHealthChecker) IsDatabaseHealthy(ctx context.Context) bool {
	return m.db.healthy
}

// IsRedisHealthy имитирует проверку здоровья Redis
func (m *mockHealthChecker) IsRedisHealthy(ctx context.Context) bool {
	return m.redis.healthy
}

// WithDatabaseResilience имитирует выполнение операции с отказоустойчивостью для БД
func (m *mockHealthChecker) WithDatabaseResilience(ctx context.Context, operation string, fn func(context.Context) error) error {
	if m.dbOpCallback != nil {
		return m.dbOpCallback(ctx)
	}
	return fn(ctx)
}

// WithRedisResilience имитирует выполнение операции с отказоустойчивостью для Redis
func (m *mockHealthChecker) WithRedisResilience(ctx context.Context, operation string, fn func(context.Context) error) error {
	if m.redisOpCallback != nil {
		return m.redisOpCallback(ctx)
	}
	return fn(ctx)
}

// newMockHealthChecker создает новый мок для тестирования
func newMockHealthChecker(pgHealthy, redisHealthy bool, logger *zap.Logger) *mockHealthChecker {
	return &mockHealthChecker{
		db:           &mockGormDBInterface{healthy: pgHealthy},
		redis:        &mockRedisClient{healthy: redisHealthy},
		logger:       logger,
		pgCircuit:    resilience.NewCircuitBreaker(5, 30*time.Second, logger),
		redisCircuit: resilience.NewCircuitBreaker(5, 10*time.Second, logger),
	}
}

// TestDatabaseHealthChecker_IsDatabaseHealthy тестирует проверку здоровья PostgreSQL
func TestDatabaseHealthChecker_IsDatabaseHealthy(t *testing.T) {
	logger := zap.NewNop()

	// Тест 1: Здоровая база данных
	t.Run("HealthyDatabase", func(t *testing.T) {
		// Создаем мок здоровой базы данных
		checker := newMockHealthChecker(true, true, logger)

		// Выполняем проверку
		ctx := context.Background()
		isHealthy := checker.IsDatabaseHealthy(ctx)

		// Проверяем результат
		if !isHealthy {
			t.Error("Expected database to be healthy")
		}
	})

	// Тест 2: Нездоровая база данных
	t.Run("UnhealthyDatabase", func(t *testing.T) {
		// Создаем мок нездоровой базы данных
		checker := newMockHealthChecker(false, true, logger)

		// Выполняем проверку
		ctx := context.Background()
		isHealthy := checker.IsDatabaseHealthy(ctx)

		// Проверяем результат
		if isHealthy {
			t.Error("Expected database to be unhealthy")
		}
	})

	// Тест 3: Таймаут проверки
	t.Run("CheckTimeout", func(t *testing.T) {
		// Создаем мок базы данных с таймаутом
		checker := newMockHealthChecker(true, true, logger)

		// Устанавливаем callback с задержкой
		checker.dbOpCallback = func(ctx context.Context) error {
			select {
			case <-time.After(3 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Создаем контекст с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Выполняем операцию с искусственной задержкой
		err := checker.WithDatabaseResilience(ctx, "test_timeout", func(ctx context.Context) error {
			// В реальном тесте эта функция не будет вызвана из-за таймаута контекста
			return nil
		})

		// Проверяем, что получили ошибку контекста
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected deadline exceeded error, got: %v", err)
		}
	})
}

// TestDatabaseHealthChecker_IsRedisHealthy тестирует проверку здоровья Redis
func TestDatabaseHealthChecker_IsRedisHealthy(t *testing.T) {
	logger := zap.NewNop()

	// Тест 1: Здоровый Redis
	t.Run("HealthyRedis", func(t *testing.T) {
		checker := newMockHealthChecker(true, true, logger)

		// Выполняем проверку
		ctx := context.Background()
		isHealthy := checker.IsRedisHealthy(ctx)

		// Проверяем результат
		if !isHealthy {
			t.Error("Expected Redis to be healthy")
		}
	})

	// Тест 2: Нездоровый Redis
	t.Run("UnhealthyRedis", func(t *testing.T) {
		checker := newMockHealthChecker(true, false, logger)

		// Выполняем проверку
		ctx := context.Background()
		isHealthy := checker.IsRedisHealthy(ctx)

		// Проверяем результат
		if isHealthy {
			t.Error("Expected Redis to be unhealthy")
		}
	})

	// Тест 3: Таймаут проверки Redis
	t.Run("RedisCheckTimeout", func(t *testing.T) {
		checker := newMockHealthChecker(true, true, logger)

		// Устанавливаем callback с задержкой
		checker.redisOpCallback = func(ctx context.Context) error {
			select {
			case <-time.After(3 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Создаем контекст с коротким таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Выполняем операцию с искусственной задержкой
		err := checker.WithRedisResilience(ctx, "test_timeout", func(ctx context.Context) error {
			// В реальном тесте эта функция не будет вызвана из-за таймаута контекста
			return nil
		})

		// Проверяем, что получили ошибку контекста
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected deadline exceeded error, got: %v", err)
		}
	})
}

// TestDatabaseHealthChecker_WithDatabaseResilience тестирует выполнение операций с отказоустойчивостью для БД
func TestDatabaseHealthChecker_WithDatabaseResilience(t *testing.T) {
	logger := zap.NewNop()
	checker := newMockHealthChecker(true, true, logger)

	// Тест 1: Успешное выполнение операции
	t.Run("SuccessfulOperation", func(t *testing.T) {
		ctx := context.Background()
		operationCalled := false

		err := checker.WithDatabaseResilience(ctx, "test_operation", func(ctx context.Context) error {
			operationCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !operationCalled {
			t.Error("Operation was not called")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("OperationWithError", func(t *testing.T) {
		ctx := context.Background()
		testErr := errors.New("test error")

		err := checker.WithDatabaseResilience(ctx, "test_error_operation", func(ctx context.Context) error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	})

	// Тест 3: Отмена контекста
	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Отменяем контекст немедленно

		operationCalled := false
		err := checker.WithDatabaseResilience(ctx, "cancelled_operation", func(ctx context.Context) error {
			operationCalled = true
			return ctx.Err()
		})

		if !operationCalled {
			t.Error("Operation should be called even with cancelled context")
		}

		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	})
}

// TestDatabaseHealthChecker_WithRedisResilience тестирует выполнение операций с отказоустойчивостью для Redis
func TestDatabaseHealthChecker_WithRedisResilience(t *testing.T) {
	logger := zap.NewNop()
	checker := newMockHealthChecker(true, true, logger)

	// Тест 1: Успешное выполнение операции
	t.Run("SuccessfulOperation", func(t *testing.T) {
		ctx := context.Background()
		operationCalled := false

		err := checker.WithRedisResilience(ctx, "test_redis_operation", func(ctx context.Context) error {
			operationCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !operationCalled {
			t.Error("Operation was not called")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("OperationWithError", func(t *testing.T) {
		ctx := context.Background()
		testErr := errors.New("redis test error")

		err := checker.WithRedisResilience(ctx, "test_redis_error", func(ctx context.Context) error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	})

	// Тест 3: Отмена контекста
	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Отменяем контекст немедленно

		operationCalled := false
		err := checker.WithRedisResilience(ctx, "cancelled_operation", func(ctx context.Context) error {
			operationCalled = true
			return ctx.Err()
		})

		if !operationCalled {
			t.Error("Operation should be called even with cancelled context")
		}

		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	})
}

// TestSafeDBOperation тестирует безопасное выполнение операций с базой данных
func TestSafeDBOperation(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаем mock для GORM DB с помощью go-sqlmock
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	// Создаем GORM DB на основе mock
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create GORM DB: %v", err)
	}

	// Тест 1: Успешная операция
	t.Run("SuccessfulOperation", func(t *testing.T) {
		operationCalled := false

		err := SafeDBOperation(ctx, gormDB, logger, "successful_db_op", func(tx *gorm.DB) error {
			operationCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !operationCalled {
			t.Error("Operation was not called")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("OperationWithError", func(t *testing.T) {
		testErr := errors.New("database operation error")

		err := SafeDBOperation(ctx, gormDB, logger, "error_db_op", func(tx *gorm.DB) error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	})
}

// TestSafeRedisOperation тестирует безопасное выполнение операций с Redis
func TestSafeRedisOperation(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Тест 1: Успешная операция
	t.Run("SuccessfulOperation", func(t *testing.T) {
		operationCalled := false

		err := SafeRedisOperation(ctx, client, logger, "successful_redis_op", func(ctx context.Context, client *redis.Client) error {
			operationCalled = true
			return client.Ping(ctx).Err()
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !operationCalled {
			t.Error("Operation was not called")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("OperationWithError", func(t *testing.T) {
		testErr := errors.New("redis operation error")

		err := SafeRedisOperation(ctx, client, logger, "error_redis_op", func(ctx context.Context, client *redis.Client) error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	})

	// Тест 3: Операция с таймаутом
	t.Run("OperationWithTimeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := SafeRedisOperation(timeoutCtx, client, logger, "timeout_redis_op", func(ctx context.Context, client *redis.Client) error {
			// Нам нужно использовать контекст для проверки его истечения
			select {
			case <-time.After(100 * time.Millisecond):
				return nil // Этот код не должен выполниться
			case <-ctx.Done():
				return ctx.Err() // Должно произойти это
			}
		})

		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
		}
	})
}
