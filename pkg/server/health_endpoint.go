package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthCheckerInterface определяет интерфейс для проверки здоровья сервисов
// Этот интерфейс должен соответствовать тому, что ожидает структура HealthCheck
type HealthCheckerInterface interface {
	// IsDatabaseHealthy проверяет здоровье PostgreSQL
	IsDatabaseHealthy(ctx context.Context) bool

	// IsRedisHealthy проверяет здоровье Redis
	IsRedisHealthy(ctx context.Context) bool

	// WithDatabaseResilience выполняет операцию с механизмами отказоустойчивости для базы данных
	WithDatabaseResilience(ctx context.Context, operation string, fn func(context.Context) error) error

	// WithRedisResilience выполняет операцию с механизмами отказоустойчивости для Redis
	WithRedisResilience(ctx context.Context, operation string, fn func(context.Context) error) error
}

// HealthCheck представляет сервис проверки здоровья
type HealthCheck struct {
	checker       HealthCheckerInterface
	logger        *zap.Logger
	server        *http.Server
	statusMutex   sync.RWMutex
	serviceStatus map[string]string
}

// HealthResponse представляет ответ эндпоинта проверки здоровья
type HealthResponse struct {
	Status    string            `json:"status"`
	Services  map[string]string `json:"services"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
}

// NewHealthCheck создает новый сервис проверки здоровья
func NewHealthCheck(checker HealthCheckerInterface, logger *zap.Logger, version string) *HealthCheck {
	health := &HealthCheck{
		checker:       checker,
		logger:        logger,
		serviceStatus: make(map[string]string),
	}

	// Инициализация статусов сервисов
	health.serviceStatus["service"] = "up"
	health.serviceStatus["postgres"] = "unknown"
	health.serviceStatus["redis"] = "unknown"
	health.serviceStatus["version"] = version

	return health
}

// StartServer запускает HTTP сервер для проверки здоровья
func (h *HealthCheck) StartServer(port int) {
	mux := http.NewServeMux()

	// Обработчик для проверки жизнеспособности (liveness)
	mux.HandleFunc("/health/live", h.livenessHandler)

	// Обработчик для проверки готовности (readiness)
	mux.HandleFunc("/health/ready", h.readinessHandler)

	// Обработчик для полной информации о здоровье
	mux.HandleFunc("/health", h.healthHandler)

	// Создаем и запускаем HTTP сервер
	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		h.logger.Info("Starting health check server", zap.Int("port", port))
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("Health check server failed", zap.Error(err))
		}
	}()

	// Запускаем фоновую проверку здоровья сервисов
	go h.monitorHealth()
}

// Stop останавливает HTTP сервер
func (h *HealthCheck) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// livenessHandler обрабатывает запросы проверки жизнеспособности
func (h *HealthCheck) livenessHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка жизнеспособности проверяет только, работает ли сам сервис
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "up"})
}

// readinessHandler обрабатывает запросы проверки готовности
func (h *HealthCheck) readinessHandler(w http.ResponseWriter, r *http.Request) {
	h.statusMutex.RLock()
	pgStatus := h.serviceStatus["postgres"]
	h.statusMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	// Если PostgreSQL недоступен, сервис не готов к работе
	if pgStatus != "up" {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "down",
			"message": "PostgreSQL is not available",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "up"})
}

// healthHandler обрабатывает запросы полной информации о здоровье
func (h *HealthCheck) healthHandler(w http.ResponseWriter, r *http.Request) {
	h.statusMutex.RLock()
	status := "up"
	services := make(map[string]string)

	// Копируем текущие статусы сервисов
	for k, v := range h.serviceStatus {
		services[k] = v
	}

	// Если PostgreSQL недоступен, статус всего сервиса - down
	if services["postgres"] != "up" {
		status = "down"
	}

	h.statusMutex.RUnlock()

	response := HealthResponse{
		Status:    status,
		Services:  services,
		Timestamp: time.Now(),
		Version:   services["version"],
	}

	w.Header().Set("Content-Type", "application/json")
	if status != "up" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(response)
}

// monitorHealth регулярно проверяет состояние зависимостей
func (h *HealthCheck) monitorHealth() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkServicesHealth()
		}
	}
}

// checkServicesHealth проверяет здоровье всех зависимостей
func (h *HealthCheck) checkServicesHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Проверяем PostgreSQL
	pgHealthy := h.checker.IsDatabaseHealthy(ctx)
	pgStatus := "up"
	if !pgHealthy {
		pgStatus = "down"
		h.logger.Warn("PostgreSQL health check failed")
	}

	// Проверяем Redis
	redisHealthy := h.checker.IsRedisHealthy(ctx)
	redisStatus := "up"
	if !redisHealthy {
		redisStatus = "degraded"
		h.logger.Warn("Redis health check failed")
	}

	// Обновляем статусы
	h.statusMutex.Lock()
	h.serviceStatus["postgres"] = pgStatus
	h.serviceStatus["redis"] = redisStatus
	h.statusMutex.Unlock()
}
