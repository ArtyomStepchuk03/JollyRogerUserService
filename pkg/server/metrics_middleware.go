package server

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"time"
)

var (
	// grpcRequestDuration измеряет длительность gRPC запросов
	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "status"},
	)
	
	// grpcRequestsTotal подсчитывает общее количество gRPC запросов
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)
	
	// dbOperationDuration измеряет длительность операций с базой данных
	dbOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)
	
	// dbOperationsTotal подсчитывает общее количество операций с базой данных
	dbOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "status"},
	)
	
	// cacheOperationDuration измеряет длительность операций с кэшем
	cacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Duration of cache operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)
	
	// cacheOperationsTotal подсчитывает общее количество операций с кэшем
	cacheOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total number of cache operations",
		},
		[]string{"operation", "status"},
	)
	
	// circuitBreakerState отслеживает состояние circuit breaker
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "State of circuit breaker (0: closed, 1: half-open, 2: open)",
		},
		[]string{"name"},
	)
)

// MetricsServer запускает HTTP сервер для Prometheus
func MetricsServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Логируем ошибку, но не паникуем
			// Если метрики недоступны, это не должно останавливать основной сервис
		}
	}()
	
	return server
}

// MetricsUnaryInterceptor создает gRPC перехватчик для сбора метрик
func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		
		// Выполняем исходный обработчик
		resp, err := handler(ctx, req)
		
		// Рассчитываем длительность
		duration := time.Since(startTime).Seconds()
		
		// Определяем статус
		var statusCode codes.Code
		if err != nil {
			statusCode = status.Code(err)
		} else {
			statusCode = codes.OK
		}
		
		// Обновляем метрики
		grpcRequestDuration.WithLabelValues(info.FullMethod, statusCode.String()).Observe(duration)
		grpcRequestsTotal.WithLabelValues(info.FullMethod, statusCode.String()).Inc()
		
		return resp, err
	}
}

// RecordDBOperation записывает метрики операции с базой данных
func RecordDBOperation(operation string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	
	dbOperationDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
	dbOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordCacheOperation записывает метрики операции с кэшем
func RecordCacheOperation(operation string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	
	cacheOperationDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
	cacheOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordCircuitBreakerStateChange записывает изменение состояния circuit breaker
func RecordCircuitBreakerStateChange(name string, state int) {
	circuitBreakerState.WithLabelValues(name).Set(float64(state))
}
