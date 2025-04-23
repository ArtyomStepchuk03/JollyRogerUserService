package server

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
)

// TestTracingUnaryInterceptor тестирует работу перехватчика трассировки для gRPC
func TestTracingUnaryInterceptor(t *testing.T) {
	// Создаем тестовый логгер, который будет записывать логи в буфер
	logger := zaptest.NewLogger(t)

	// Создаем контекст
	ctx := context.Background()

	// Создаем перехватчик
	interceptor := TracingUnaryInterceptor(logger)

	// Тест 1: Базовое выполнение без ошибок
	t.Run("BasicExecution", func(t *testing.T) {
		// Создаем тестовый обработчик, который ничего не делает
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return "test_response", nil
		}

		// Создаем информацию о методе
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.TestService/TestMethod",
		}

		// Выполняем перехватчик
		resp, err := interceptor(ctx, "test_request", info, handler)

		// Проверяем результат
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if resp != "test_response" {
			t.Errorf("Expected 'test_response', got: %v", resp)
		}

		// Проверяем, что запрос получил ID
		requestID := GetRequestID(ctx)
		if requestID == "" {
			t.Error("Expected request ID to be set, got empty string")
		}
	})

	// Тест 2: Контекст содержит request ID из метаданных
	t.Run("RequestIDFromMetadata", func(t *testing.T) {
		// Добавляем request ID в контекст через метаданные
		md := map[string][]string{
			"x-request-id": {"test-request-id-123"},
		}

		// В реальном случае используется grpc.IncomingContext, но для тестов мы добавим key/value напрямую
		ctxWithMD := context.WithValue(ctx, requestIDMetadataKey{}, md)

		// Создаем тестовый обработчик
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			// Проверяем ID запроса внутри обработчика
			requestID := GetRequestID(ctx)
			if requestID != "test-request-id-123" {
				t.Errorf("Expected request ID 'test-request-id-123', got: %s", requestID)
			}
			return "metadata_response", nil
		}

		// Создаем информацию о методе
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.TestService/TestMethod",
		}

		// Выполняем перехватчик
		resp, err := interceptor(ctxWithMD, "test_request", info, handler)

		// Проверяем результат
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if resp != "metadata_response" {
			t.Errorf("Expected 'metadata_response', got: %v", resp)
		}
	})

	// Тест 3: Обработка ошибки в обработчике
	t.Run("ErrorHandling", func(t *testing.T) {
		testErr := status.Error(codes.NotFound, "test error")
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, testErr
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.TestService/ErrorMethod",
		}

		resp, err := interceptor(ctx, "error_request", info, handler)

		if resp != nil {
			t.Errorf("Expected nil response, got: %v", resp)
		}

		if status.Code(err) != codes.NotFound || err.Error() != testErr.Error() {
			t.Errorf("Expected error code %v and message %q, got code %v and message %q",
				codes.NotFound, testErr.Error(),
				status.Code(err), err.Error(),
			)
		}
	})
}

// Вспомогательный тип для хранения ключа метаданных
type requestIDMetadataKey struct{}

// MockResponseWriter имитирует http.ResponseWriter для тестов
type MockResponseWriter struct {
	Headers    http.Header
	StatusCode int
	Body       strings.Builder
}

// Header возвращает заголовки ответа
func (m *MockResponseWriter) Header() http.Header {
	return m.Headers
}

// Write записывает данные в ответ
func (m *MockResponseWriter) Write(data []byte) (int, error) {
	return m.Body.Write(data)
}

// WriteHeader устанавливает код состояния HTTP
func (m *MockResponseWriter) WriteHeader(statusCode int) {
	m.StatusCode = statusCode
}

// TestLoggingMiddleware тестирует middleware для HTTP
func TestLoggingMiddleware(t *testing.T) {
	// Создаем тестовый логгер
	logger := zaptest.NewLogger(t)

	// Тест 1: Базовое выполнение HTTP запроса
	t.Run("BasicHttpRequest", func(t *testing.T) {
		// Создаем тестовый обработчик
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем ID запроса внутри обработчика
			requestID := GetRequestID(r.Context())
			if requestID == "" {
				t.Error("Expected request ID to be set, got empty string")
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Оборачиваем обработчик в middleware
		handler := LoggingMiddleware(logger, nextHandler)

		// Создаем тестовый HTTP запрос
		req := httptest.NewRequest("GET", "http://example.com/test", nil)

		// Создаем ответ
		w := httptest.NewRecorder()

		// Выполняем обработчик
		handler.ServeHTTP(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем тело ответа
		if w.Body.String() != "OK" {
			t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
		}
	})

	// Тест 2: HTTP запрос с существующим request ID
	t.Run("RequestWithID", func(t *testing.T) {
		// Создаем тестовый обработчик
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем ID запроса внутри обработчика
			requestID := GetRequestID(r.Context())
			if requestID != "existing-request-id" {
				t.Errorf("Expected request ID 'existing-request-id', got: %s", requestID)
			}

			w.WriteHeader(http.StatusOK)
		})

		// Оборачиваем обработчик в middleware
		handler := LoggingMiddleware(logger, nextHandler)

		// Создаем тестовый HTTP запрос с заголовком X-Request-ID
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set("X-Request-ID", "existing-request-id")

		// Создаем ответ
		w := httptest.NewRecorder()

		// Выполняем обработчик
		handler.ServeHTTP(w, req)

		// Проверяем код ответа
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestGetRequestID тестирует функцию получения ID запроса из контекста
func TestGetRequestID(t *testing.T) {
	// Создаем базовый контекст
	ctx := context.Background()

	// Проверяем пустой контекст
	requestID := GetRequestID(ctx)
	if requestID != "" {
		t.Errorf("Expected empty request ID for empty context, got: %s", requestID)
	}

	// Добавляем ID запроса в контекст
	ctxWithID := context.WithValue(ctx, RequestIDKey, "test-id-456")

	// Проверяем контекст с ID
	requestID = GetRequestID(ctxWithID)
	if requestID != "test-id-456" {
		t.Errorf("Expected request ID 'test-id-456', got: %s", requestID)
	}
}

// TestWithRequestID тестирует функцию добавления ID запроса в логгер
func TestWithRequestID(t *testing.T) {
	// Создаем базовый контекст и логгер
	ctx := context.Background()
	baseLogger := zap.NewNop()

	// Проверяем с пустым контекстом
	logger := WithRequestID(ctx, baseLogger)
	if logger != baseLogger {
		t.Error("Expected logger to remain unchanged for empty context")
	}

	// Добавляем ID запроса в контекст
	ctxWithID := context.WithValue(ctx, RequestIDKey, "test-id-789")

	// Проверяем логгер с ID запроса
	// Здесь мы не можем напрямую проверить содержимое логгера,
	// но мы можем убедиться, что возвращается новый экземпляр
	logger = WithRequestID(ctxWithID, baseLogger)
	if logger == baseLogger {
		t.Error("Expected a new logger instance with request ID")
	}
}
