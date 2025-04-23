package server

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const (
	// RequestIDKey ключ для request ID в контексте
	RequestIDKey contextKey = "request_id"

	// StartTimeKey ключ для времени начала запроса в контексте
	StartTimeKey contextKey = "start_time"
)

// TracingUnaryInterceptor создает перехватчик для трассировки запросов
func TracingUnaryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Создаем или получаем request ID
		requestID := getRequestIDFromMetadata(ctx)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Добавляем request ID в контекст
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		// Добавляем время начала запроса в контекст
		startTime := time.Now()
		ctx = context.WithValue(ctx, StartTimeKey, startTime)

		// Логируем начало запроса
		logger.Info("Start processing request",
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID))

		// Выполняем исходный обработчик
		resp, err := handler(ctx, req)

		// Рассчитываем длительность запроса
		duration := time.Since(startTime)

		// Логируем завершение запроса
		if err != nil {
			logger.Error("Request failed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.Duration("duration", duration),
				zap.Error(err))
		} else {
			logger.Info("Request completed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.Duration("duration", duration))
		}

		return resp, err
	}
}

// LoggingMiddleware создает middleware для HTTP запросов
func LoggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Создаем или получаем request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Создаем контекст с request ID
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Логируем начало запроса
		startTime := time.Now()
		logger.Info("Start processing HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("request_id", requestID))

		// Создаем wrapper для ResponseWriter для отслеживания кода состояния
		ww := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Выполняем исходный обработчик
		next.ServeHTTP(ww, r)

		// Рассчитываем длительность запроса
		duration := time.Since(startTime)

		// Логируем завершение запроса
		logger.Info("HTTP request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("request_id", requestID),
			zap.Int("status", ww.statusCode),
			zap.Duration("duration", duration))
	})
}

// responseWriterWrapper обертка для http.ResponseWriter для отслеживания кода состояния
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader перехватывает WriteHeader для сохранения кода состояния
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// getRequestIDFromMetadata извлекает request ID из metadata
func getRequestIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("x-request-id")
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// GetRequestID извлекает request ID из контекста
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithRequestID добавляет request ID в логгер
func WithRequestID(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if requestID := GetRequestID(ctx); requestID != "" {
		return logger.With(zap.String("request_id", requestID))
	}
	return logger
}
