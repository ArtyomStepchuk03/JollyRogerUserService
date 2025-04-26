package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// mockDB представляет мок для базы данных
type mockDB struct {
	db            *gorm.DB
	healthy       bool
	queryCallback func() error
}

// DB возвращает подключение к базе данных
func (m *mockDB) DB() (*gorm.DB, error) {
	if !m.healthy {
		return nil, errors.New("database connection error")
	}
	return m.db, nil
}

// Exec выполняет SQL-запрос
func (m *mockDB) Exec(query string, args ...interface{}) error {
	if m.queryCallback != nil {
		return m.queryCallback()
	}
	if !m.healthy {
		return errors.New("database query error")
	}
	return nil
}

// TestDatabaseHealthChecker_IsDatabaseHealthy тестирует проверку здоровья PostgreSQL
func TestDatabaseHealthChecker_IsDatabaseHealthy(t *testing.T) {
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Тест 1: Здоровая база данных
	t.Run("HealthyDatabase", func(t *testing.T) {
		// Создаем мок здоровой базы данных
		mockHealthyDB := &mockDB{
			db:      &gorm.DB{},
			healthy: true,
		}

		// Создаем проверку здоровья
		checker := NewDatabaseHealthChecker(mockHealthyDB.db, redisClient, logger)

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
		mockUnhealthyDB := &mockDB{
			db:      &gorm.DB{},
			healthy: false,
		}

		// Создаем проверку здоровья
		checker := NewDatabaseHealthChecker(mockUnhealthyDB.db, redisClient, logger)

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
		// Создаем мок базы данных с медленным ответом
		mockSlowDB := &mockDB{
			db:      &gorm.DB{},
			healthy: true,
			queryCallback: func() error {
				time.Sleep(3 * time.Second)
				return nil
			},
		}

		// Создаем проверку здоровья
		checker := NewDatabaseHealthChecker(mockSlowDB.db, redisClient, logger)

		// Создаем контекст с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Выполняем проверку
		isHealthy := checker.IsDatabaseHealthy(ctx)

		// Проверяем результат
		if isHealthy {
			t.Error("Expected health check to fail due to timeout")
		}
	})
}

// TestDatabaseHealthChecker_IsRedisHealthy тестирует проверку здоровья Redis
func TestDatabaseHealthChecker_IsRedisHealthy(t *testing.T) {
	logger := zap.NewNop()
	db := &mockDB{db: &gorm.DB{}, healthy: true}

	// Тест 1: Здоровый Redis
	t.Run("HealthyRedis", func(t *testing.T) {
		// Создаем тестовый Redis
		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to create mini redis: %v", err)
		}
		defer mr.Close()

		redisClient := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		// Создаем проверку здоровья
		checker := NewDatabaseHealthChecker(db.db, redisClient, logger)

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
		// Создаем клиент для несуществующего Redis
		badRedisClient := redis.NewClient(&redis.Options{
			Addr: "non.existent.host:6379",
		})

		// Создаем проверку здоровья
		checker := NewDatabaseHealthChecker(db.db, badRedisClient, logger)

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
		// Создаем тестовый Redis
		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to create mini redis: %v", err)
		}
		defer mr.Close()

		redisClient := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		db := &mockDB{db: &gorm.DB{}, healthy: true}
		checker := NewDatabaseHealthChecker(db.db, redisClient, zap.NewNop())

		// Создаем контекст с коротким таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Выполняем операцию с искусственной задержкой
		isHealthy := checker.WithRedisResilience(ctx, "test_timeout", func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond) // Искусственная задержка, чтобы вызвать таймаут
			return nil
		}) == nil

		if isHealthy {
			t.Error("Expected Redis health check to fail due to timeout")
		}
	})

}

// TestDatabaseHealthChecker_WithDatabaseResilience тестирует выполнение операций с отказоустойчивостью
func TestDatabaseHealthChecker_WithDatabaseResilience(t *testing.T) {
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	db := &mockDB{db: &gorm.DB{}, healthy: true}
	checker := NewDatabaseHealthChecker(db.db, redisClient, logger)

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
			return nil
		})

		// Операция должна быть вызвана, так как отмена контекста
		// обрабатывается внутри операции, а не circuit breaker
		if !operationCalled {
			t.Error("Operation should be called even with cancelled context")
		}

		if err != nil {
			t.Errorf("Expected no error despite cancelled context, got: %v", err)
		}
	})
}

// TestDatabaseHealthChecker_WithRedisResilience тестирует выполнение операций Redis с отказоустойчивостью
func TestDatabaseHealthChecker_WithRedisResilience(t *testing.T) {
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	db := &mockDB{db: &gorm.DB{}, healthy: true}
	checker := NewDatabaseHealthChecker(db.db, redisClient, logger)

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

	// Тест 3: Circuit breaker для Redis
	t.Run("RedisCircuitBreaker", func(t *testing.T) {
		ctx := context.Background()
		redisErr := errors.New("redis connection error")

		// Выполняем несколько операций с ошибками, чтобы открыть circuit breaker
		for i := 0; i < 6; i++ {
			checker.WithRedisResilience(ctx, "redis_circuit_open", func(ctx context.Context) error {
				return redisErr
			})
		}

		// Теперь circuit breaker должен быть открыт
		// Следующая операция не должна быть вызвана
		operationCalled := false
		err := checker.WithRedisResilience(ctx, "redis_circuit_open", func(ctx context.Context) error {
			operationCalled = true
			return nil
		})

		if err == nil {
			t.Error("Expected circuit breaker error, got nil")
		}

		if operationCalled {
			t.Error("Operation was called despite open circuit")
		}
	})
}

// TestSafeDBOperation тестирует безопасное выполнение операций с базой данных
func TestSafeDBOperation(t *testing.T) {
	logger := zap.NewNop()
	db := &gorm.DB{}
	ctx := context.Background()

	// Тест 1: Успешная операция
	t.Run("SuccessfulOperation", func(t *testing.T) {
		operationCalled := false

		err := SafeDBOperation(ctx, db, logger, "successful_db_op", func(tx *gorm.DB) error {
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

		err := SafeDBOperation(ctx, db, logger, "error_db_op", func(tx *gorm.DB) error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	})

	// Тест 3: Обработка panic
	t.Run("PanicHandling", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Expected SafeDBOperation to handle panic, but it didn't: %v", r)
			}
		}()

		err := SafeDBOperation(ctx, db, logger, "panic_db_op", func(tx *gorm.DB) error {
			panic("unexpected panic")
			return nil // Никогда не будет выполнено
		})

		// Если мы дошли до этой точки, значит panic был перехвачен
		if err == nil {
			t.Error("Expected error after panic, got nil")
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
		ctx := context.Background()
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := SafeRedisOperation(timeoutCtx, client, logger, "timeout_redis_op", func(ctx context.Context, client *redis.Client) error {
			time.Sleep(100 * time.Millisecond)
			_, err := client.Ping(ctx).Result()
			return err
		})

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})

	// Тест 4: Обработка panic
	t.Run("PanicHandling", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Expected SafeRedisOperation to handle panic, but it didn't: %v", r)
			}
		}()

		err := SafeRedisOperation(ctx, client, logger, "panic_redis_op", func(ctx context.Context, client *redis.Client) error {
			panic("unexpected redis panic")
			return nil // Никогда не будет выполнено
		})

		// Если мы дошли до этой точки, значит panic был перехвачен
		if err == nil {
			t.Error("Expected error after panic, got nil")
		}
	})
}
