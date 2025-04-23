package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resetMetrics сбрасывает все метрики для тестов
func resetMetrics() {
	// Сбрасываем все коллекторы
	grpcRequestDuration.Reset()
	grpcRequestsTotal.Reset()
	dbOperationDuration.Reset()
	dbOperationsTotal.Reset()
	cacheOperationDuration.Reset()
	cacheOperationsTotal.Reset()
	circuitBreakerState.Reset()
}

// TestMetricsUnaryInterceptor тестирует перехватчик метрик для gRPC
func TestMetricsUnaryInterceptor(t *testing.T) {
	// Сбрасываем метрики перед тестом
	resetMetrics()

	// Создаем перехватчик
	interceptor := MetricsUnaryInterceptor()

	// Создаем контекст
	ctx := context.Background()

	// Тест 1: Успешное выполнение метода
	t.Run("SuccessfulMethod", func(t *testing.T) {
		// Создаем тестовый обработчик
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			// Добавляем небольшую задержку для тестирования длительности
			time.Sleep(5 * time.Millisecond)
			return "success", nil
		}

		// Создаем информацию о методе
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.TestService/SuccessMethod",
		}

		// Выполняем перехватчик
		resp, err := interceptor(ctx, "test_request", info, handler)

		// Проверяем результат
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if resp != "success" {
			t.Errorf("Expected 'success', got: %v", resp)
		}

		// Проверяем метрики
		// В реальном тесте нужно использовать testutil.CollectAndCount или testutil.GatherAndCount,
		// но для упрощения мы только проверим, что метрики были созданы
		if testutil.CollectAndCount(grpcRequestsTotal) == 0 {
			t.Error("Expected grpcRequestsTotal metric to be incremented")
		}

		if testutil.CollectAndCount(grpcRequestDuration) == 0 {
			t.Error("Expected grpcRequestDuration metric to be observed")
		}
	})

	// Тест 2: Метод с ошибкой
	t.Run("MethodWithError", func(t *testing.T) {
		// Сбрасываем метрики перед тестом
		resetMetrics()

		// Создаем тестовый обработчик с ошибкой
		testErr := status.Error(codes.NotFound, "not found")
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, testErr
		}

		// Создаем информацию о методе
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.TestService/ErrorMethod",
		}

		// Выполняем перехватчик
		resp, err := interceptor(ctx, "error_request", info, handler)

		// Проверяем результат
		if err != testErr {
			t.Errorf("Expected specific error, got: %v", err)
		}

		if resp != nil {
			t.Errorf("Expected nil response, got: %v", resp)
		}

		// Проверяем метрики
		if testutil.CollectAndCount(grpcRequestsTotal) == 0 {
			t.Error("Expected grpcRequestsTotal metric to be incremented for error")
		}

		if testutil.CollectAndCount(grpcRequestDuration) == 0 {
			t.Error("Expected grpcRequestDuration metric to be observed for error")
		}
	})
}

// TestMetricsServer тестирует HTTP сервер для Prometheus
func TestMetricsServer(t *testing.T) {
	// Тест: Запуск и проверка метрик сервера
	t.Run("ServerStartup", func(t *testing.T) {
		// Запускаем сервер на случайном порту
		server := MetricsServer("0")
		defer server.Close()

		// Создаем запрос к /metrics
		req, err := http.NewRequest("GET", "http://localhost"+server.Addr+"/metrics", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Выполняем запрос
		client := &http.Client{
			Timeout: 1 * time.Second,
		}

		var resp *http.Response
		// Пробуем несколько раз, так как серверу может потребоваться время для запуска
		for i := 0; i < 5; i++ {
			resp, err = client.Do(req)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer resp.Body.Close()

		// Проверяем код ответа
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
		}

		// Проверяем заголовок Content-Type
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/plain; version=0.0.4; charset=utf-8" {
			t.Errorf("Expected Content-Type 'text/plain; version=0.0.4; charset=utf-8', got '%s'", contentType)
		}
	})
}

// TestRecordDBOperation тестирует запись метрик для операций с базой данных
func TestRecordDBOperation(t *testing.T) {
	// Сбрасываем метрики перед тестом
	resetMetrics()

	// Тест 1: Успешная операция
	t.Run("SuccessfulDBOperation", func(t *testing.T) {
		// Записываем метрику
		RecordDBOperation("test_db_operation", 50*time.Millisecond, nil)

		// Проверяем метрики
		if testutil.CollectAndCount(dbOperationsTotal) == 0 {
			t.Error("Expected dbOperationsTotal metric to be incremented")
		}

		if testutil.CollectAndCount(dbOperationDuration) == 0 {
			t.Error("Expected dbOperationDuration metric to be observed")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("DBOperationWithError", func(t *testing.T) {
		// Сбрасываем метрики перед тестом
		resetMetrics()

		// Записываем метрику с ошибкой
		testErr := errors.New("database error")
		RecordDBOperation("error_db_operation", 100*time.Millisecond, testErr)

		// Проверяем метрики
		if testutil.CollectAndCount(dbOperationsTotal) == 0 {
			t.Error("Expected dbOperationsTotal metric to be incremented for error")
		}

		if testutil.CollectAndCount(dbOperationDuration) == 0 {
			t.Error("Expected dbOperationDuration metric to be observed for error")
		}
	})
}

// TestRecordCacheOperation тестирует запись метрик для операций с кэшем
func TestRecordCacheOperation(t *testing.T) {
	// Сбрасываем метрики перед тестом
	resetMetrics()

	// Тест 1: Успешная операция
	t.Run("SuccessfulCacheOperation", func(t *testing.T) {
		// Записываем метрику
		RecordCacheOperation("test_cache_operation", 20*time.Millisecond, nil)

		// Проверяем метрики
		if testutil.CollectAndCount(cacheOperationsTotal) == 0 {
			t.Error("Expected cacheOperationsTotal metric to be incremented")
		}

		if testutil.CollectAndCount(cacheOperationDuration) == 0 {
			t.Error("Expected cacheOperationDuration metric to be observed")
		}
	})

	// Тест 2: Операция с ошибкой
	t.Run("CacheOperationWithError", func(t *testing.T) {
		// Сбрасываем метрики перед тестом
		resetMetrics()

		// Записываем метрику с ошибкой
		testErr := errors.New("cache error")
		RecordCacheOperation("error_cache_operation", 30*time.Millisecond, testErr)

		// Проверяем метрики
		if testutil.CollectAndCount(cacheOperationsTotal) == 0 {
			t.Error("Expected cacheOperationsTotal metric to be incremented for error")
		}

		if testutil.CollectAndCount(cacheOperationDuration) == 0 {
			t.Error("Expected cacheOperationDuration metric to be observed for error")
		}
	})
}

// TestRecordCircuitBreakerStateChange тестирует запись метрик для состояния circuit breaker
func TestRecordCircuitBreakerStateChange(t *testing.T) {
	// Сбрасываем метрики перед тестом
	resetMetrics()

	// Тестируем различные состояния circuit breaker
	states := []struct {
		name  string
		state int
	}{
		{"db_circuit", 0},    // Closed
		{"redis_circuit", 1}, // Half-Open
		{"user_circuit", 2},  // Open
	}

	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			// Записываем метрику
			RecordCircuitBreakerStateChange(s.name, s.state)

			// Проверяем метрики
			if testutil.CollectAndCount(circuitBreakerState) == 0 {
				t.Error("Expected circuitBreakerState metric to be set")
			}
		})
	}
}
