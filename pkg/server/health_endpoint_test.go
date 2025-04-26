package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockHealthChecker создает тестовую версию DatabaseHealthChecker
type mockHealthChecker struct {
	pgHealthy    bool
	redisHealthy bool
}

// IsDatabaseHealthy возвращает заранее определенное состояние PostgreSQL
func (m *mockHealthChecker) IsDatabaseHealthy(ctx context.Context) bool {
	return m.pgHealthy
}

// IsRedisHealthy возвращает заранее определенное состояние Redis
func (m *mockHealthChecker) IsRedisHealthy(ctx context.Context) bool {
	return m.redisHealthy
}

// WithDatabaseResilience выполняет операцию немедленно
func (m *mockHealthChecker) WithDatabaseResilience(ctx context.Context, operation string, fn func(context.Context) error) error {
	return fn(ctx)
}

// WithRedisResilience выполняет операцию немедленно
func (m *mockHealthChecker) WithRedisResilience(ctx context.Context, operation string, fn func(context.Context) error) error {
	return fn(ctx)
}

// TestHealthCheck_LivenessHandler тестирует обработчик проверки жизнеспособности
func TestHealthCheck_LivenessHandler(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем тестовый checker с разными состояниями
	healthyChecker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}
	unhealthyChecker := &mockHealthChecker{pgHealthy: false, redisHealthy: false}

	// Создаем HealthCheck с разными состояниями
	healthyHealth := NewHealthCheck(healthyChecker, logger, "1.0.0")
	unhealthyHealth := NewHealthCheck(unhealthyChecker, logger, "1.0.0")

	// Тест 1: Проверка liveness всегда должна возвращать положительный результат
	t.Run("AlwaysHealthy", func(t *testing.T) {
		// Проверка liveness с рабочим сервисом
		req := httptest.NewRequest("GET", "/health/live", nil)
		w := httptest.NewRecorder()

		healthyHealth.livenessHandler(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем заголовок Content-Type
		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}

		// Проверяем содержимое ответа
		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if status, exists := response["status"]; !exists || status != "up" {
			t.Errorf("Expected status 'up', got '%s'", status)
		}

		// Проверка liveness с нерабочими базами данных
		req = httptest.NewRequest("GET", "/health/live", nil)
		w = httptest.NewRecorder()

		unhealthyHealth.livenessHandler(w, req)

		// Проверяем код ответа - должен быть OK, так как liveness не зависит от состояния баз данных
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d for unhealthy service, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestHealthCheck_ReadinessHandler тестирует обработчик проверки готовности
func TestHealthCheck_ReadinessHandler(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Тест 1: Сервис готов к работе
	t.Run("ServiceReady", func(t *testing.T) {
		// Создаем тестовый checker с рабочей PostgreSQL
		checker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Вручную устанавливаем статус PostgreSQL в "up"
		health.statusMutex.Lock()
		health.serviceStatus["postgres"] = "up"
		health.statusMutex.Unlock()

		// Запрос проверки готовности
		req := httptest.NewRequest("GET", "/health/ready", nil)
		w := httptest.NewRecorder()

		health.readinessHandler(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем содержимое ответа
		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if status, exists := response["status"]; !exists || status != "up" {
			t.Errorf("Expected status 'up', got '%s'", status)
		}
	})

	// Тест 2: Сервис не готов к работе
	t.Run("ServiceNotReady", func(t *testing.T) {
		// Создаем тестовый checker с нерабочей PostgreSQL
		checker := &mockHealthChecker{pgHealthy: false, redisHealthy: false}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Вручную устанавливаем статус PostgreSQL в "down"
		health.statusMutex.Lock()
		health.serviceStatus["postgres"] = "down"
		health.statusMutex.Unlock()

		// Запрос проверки готовности
		req := httptest.NewRequest("GET", "/health/ready", nil)
		w := httptest.NewRecorder()

		health.readinessHandler(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
		}

		// Проверяем содержимое ответа
		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if status, exists := response["status"]; !exists || status != "down" {
			t.Errorf("Expected status 'down', got '%s'", status)
		}

		if message, exists := response["message"]; !exists || message != "PostgreSQL is not available" {
			t.Errorf("Expected message about PostgreSQL, got '%s'", message)
		}
	})
}

// TestHealthCheck_HealthHandler тестирует обработчик полной информации о здоровье
func TestHealthCheck_HealthHandler(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Тест 1: Все сервисы работают
	t.Run("AllServicesUp", func(t *testing.T) {
		// Создаем тестовый checker с рабочими сервисами
		checker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Вручную устанавливаем статусы сервисов
		health.statusMutex.Lock()
		health.serviceStatus["postgres"] = "up"
		health.serviceStatus["redis"] = "up"
		health.statusMutex.Unlock()

		// Запрос полной информации о здоровье
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		health.healthHandler(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем содержимое ответа
		var response HealthResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Status != "up" {
			t.Errorf("Expected status 'up', got '%s'", response.Status)
		}

		if pgStatus, exists := response.Services["postgres"]; !exists || pgStatus != "up" {
			t.Errorf("Expected PostgreSQL status 'up', got '%s'", pgStatus)
		}

		if redisStatus, exists := response.Services["redis"]; !exists || redisStatus != "up" {
			t.Errorf("Expected Redis status 'up', got '%s'", redisStatus)
		}

		if response.Version != "1.0.0" {
			t.Errorf("Expected version '1.0.0', got '%s'", response.Version)
		}
	})

	// Тест 2: PostgreSQL не работает
	t.Run("PostgreSQLDown", func(t *testing.T) {
		// Создаем тестовый checker с нерабочей PostgreSQL
		checker := &mockHealthChecker{pgHealthy: false, redisHealthy: true}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Вручную устанавливаем статусы сервисов
		health.statusMutex.Lock()
		health.serviceStatus["postgres"] = "down"
		health.serviceStatus["redis"] = "up"
		health.statusMutex.Unlock()

		// Запрос полной информации о здоровье
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		health.healthHandler(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
		}

		// Проверяем содержимое ответа
		var response HealthResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Status != "down" {
			t.Errorf("Expected status 'down', got '%s'", response.Status)
		}

		if pgStatus, exists := response.Services["postgres"]; !exists || pgStatus != "down" {
			t.Errorf("Expected PostgreSQL status 'down', got '%s'", pgStatus)
		}
	})

	// Тест 3: Redis не работает (деградированное состояние)
	t.Run("RedisDown", func(t *testing.T) {
		// Создаем тестовый checker с нерабочим Redis
		checker := &mockHealthChecker{pgHealthy: true, redisHealthy: false}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Вручную устанавливаем статусы сервисов
		health.statusMutex.Lock()
		health.serviceStatus["postgres"] = "up"
		health.serviceStatus["redis"] = "degraded"
		health.statusMutex.Unlock()

		// Запрос полной информации о здоровье
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		health.healthHandler(w, req)

		// Проверяем код ответа - должен быть OK, так как Redis не критичен
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем содержимое ответа
		var response HealthResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Status != "up" {
			t.Errorf("Expected status 'up', got '%s'", response.Status)
		}

		if redisStatus, exists := response.Services["redis"]; !exists || redisStatus != "degraded" {
			t.Errorf("Expected Redis status 'degraded', got '%s'", redisStatus)
		}
	})
}

// TestHealthCheck_CheckServicesHealth тестирует метод проверки здоровья сервисов
func TestHealthCheck_CheckServicesHealth(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Тест: Обновление статусов сервисов
	t.Run("UpdateServiceStatuses", func(t *testing.T) {
		// Начинаем с checker, где все сервисы работают
		checker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}
		health := NewHealthCheck(checker, logger, "1.0.0")

		// Запускаем проверку сервисов
		health.checkServicesHealth()

		// Проверяем статусы
		health.statusMutex.RLock()
		pgStatus := health.serviceStatus["postgres"]
		redisStatus := health.serviceStatus["redis"]
		health.statusMutex.RUnlock()

		if pgStatus != "up" {
			t.Errorf("Expected PostgreSQL status 'up', got '%s'", pgStatus)
		}

		if redisStatus != "up" {
			t.Errorf("Expected Redis status 'up', got '%s'", redisStatus)
		}

		// Изменяем состояние сервисов
		checker.pgHealthy = false
		checker.redisHealthy = false

		// Запускаем проверку сервисов снова
		health.checkServicesHealth()

		// Проверяем обновленные статусы
		health.statusMutex.RLock()
		pgStatus = health.serviceStatus["postgres"]
		redisStatus = health.serviceStatus["redis"]
		health.statusMutex.RUnlock()

		if pgStatus != "down" {
			t.Errorf("Expected PostgreSQL status 'down', got '%s'", pgStatus)
		}

		if redisStatus != "degraded" {
			t.Errorf("Expected Redis status 'degraded', got '%s'", redisStatus)
		}
	})
}

// TestHealthCheck_ServerLifecycle тестирует жизненный цикл сервера проверки здоровья
func TestHealthCheck_ServerLifecycle(t *testing.T) {
	// Пропускаем этот тест, если задана переменная окружения SKIP_SERVER_TESTS
	// (полезно для CI/CD, где запуск серверов может быть проблематичным)
	/*
		if os.Getenv("SKIP_SERVER_TESTS") != "" {
			t.Skip("Skipping server lifecycle test")
		}
	*/

	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем тестовый checker
	checker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}

	// Создаем HealthCheck
	health := NewHealthCheck(checker, logger, "1.0.0")

	// Запускаем сервер на случайном порту
	health.StartServer(0)

	// Ждем немного, чтобы сервер успел запуститься
	time.Sleep(100 * time.Millisecond)

	// Останавливаем сервер
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := health.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop health check server: %v", err)
	}
}

// Дополнительно: тесты для создания сервера с реальным checker
// (требуется mockng для DatabaseHealthChecker)
func TestHealthCheck_WithRealChecker(t *testing.T) {
	// Создаем реальный logger
	logger := zap.NewNop()

	// Создаем мок для DatabaseHealthChecker
	checker := &mockHealthChecker{pgHealthy: true, redisHealthy: true}

	// Создаем HealthCheck с мок checker
	health := NewHealthCheck(checker, logger, "1.0.0")

	// Проверяем, что HealthCheck был создан
	if health == nil {
		t.Fatal("Expected health check to be created")
	}

	// Инициализируем внутренние структуры
	if health.serviceStatus == nil {
		t.Fatal("Expected serviceStatus map to be initialized")
	}

	// Проверяем версию
	health.statusMutex.RLock()
	version := health.serviceStatus["version"]
	health.statusMutex.RUnlock()

	if version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", version)
	}
}
