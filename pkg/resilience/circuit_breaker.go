package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitState представляет состояние circuit breaker
type CircuitState int

const (
	// CircuitClosed означает, что circuit breaker закрыт (нормальное состояние)
	CircuitClosed CircuitState = iota
	// CircuitOpen означает, что circuit breaker открыт (состояние ошибки)
	CircuitOpen
	// CircuitHalfOpen означает, что circuit breaker полуоткрыт (пробное состояние)
	CircuitHalfOpen
)

// CircuitBreaker реализует паттерн circuit breaker для повышения отказоустойчивости
type CircuitBreaker struct {
	state            CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastStateChange  time.Time
	mutex            sync.RWMutex
	logger           *zap.Logger
	ignoredErrors    []error // Добавлено: список игнорируемых ошибок
}

// NewCircuitBreaker создает новый экземпляр CircuitBreaker
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration, logger *zap.Logger, ignoredErrors ...error) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureCount:     0,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		lastStateChange:  time.Now(),
		logger:           logger,
		ignoredErrors:    ignoredErrors,
	}
}

// DefaultCircuitBreakerOptions возвращает рекомендуемые настройки Circuit Breaker
func DefaultCircuitBreakerOptions() (int, time.Duration) {
	return 5, 30 * time.Second // 5 ошибок для срабатывания, сброс через 30 секунд
}

// Execute выполняет функцию с учетом состояния circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation string, fn func(context.Context) error) error {
	// Проверяем состояние circuit breaker
	if !cb.allowRequest() {
		cb.logger.Warn("Circuit breaker preventing operation execution",
			zap.String("operation", operation),
			zap.String("state", cb.stateString()))
		return errors.New("circuit breaker is open")
	}

	// Выполняем функцию
	err := fn(ctx)

	// Обрабатываем результат
	cb.handleResult(operation, err)

	return err
}

// allowRequest проверяет, можно ли выполнить запрос в текущем состоянии
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Проверяем, не пора ли перейти в полуоткрытое состояние
		if time.Since(cb.lastStateChange) > cb.resetTimeout {
			// Не меняем состояние здесь, так как у нас Read Lock
			// Это будет сделано при следующем вызове Execute
			return true
		}
		return false
	case CircuitHalfOpen:
		// В полуоткрытом состоянии разрешаем только один запрос
		return true
	default:
		return false
	}
}

// handleResult обрабатывает результат выполнения функции
func (cb *CircuitBreaker) handleResult(operation string, err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Проверяем, нужно ли перейти из открытого в полуоткрытое состояние
	if cb.state == CircuitOpen && time.Since(cb.lastStateChange) > cb.resetTimeout {
		cb.transitionToHalfOpen(operation)
	}

	// Проверяем, является ли ошибка игнорируемой
	if err != nil && cb.isIgnoredError(err) {
		cb.logger.Debug("Игнорируем ошибку для circuit breaker",
			zap.String("operation", operation),
			zap.Error(err))
		return
	}

	// Обрабатываем результат в зависимости от текущего состояния
	if err != nil {
		switch cb.state {
		case CircuitClosed:
			// Увеличиваем счетчик ошибок
			cb.failureCount++
			// Если превышен порог, открываем circuit breaker
			if cb.failureCount >= cb.failureThreshold {
				cb.transitionToOpen(operation)
			}
		case CircuitHalfOpen:
			// При ошибке в полуоткрытом состоянии, возвращаемся в открытое
			cb.transitionToOpen(operation)
		}
	} else {
		// Успешный запрос
		switch cb.state {
		case CircuitClosed:
			// Сбрасываем счетчик ошибок
			cb.failureCount = 0
		case CircuitHalfOpen:
			// Успешный запрос в полуоткрытом состоянии - возвращаемся в закрытое
			cb.transitionToClosed(operation)
		}
	}
}

// isIgnoredError проверяет, является ли ошибка игнорируемой
func (cb *CircuitBreaker) isIgnoredError(err error) bool {
	for _, ignoredErr := range cb.ignoredErrors {
		if errors.Is(err, ignoredErr) {
			return true
		}
	}
	return false
}

// transitionToOpen переводит circuit breaker в открытое состояние
func (cb *CircuitBreaker) transitionToOpen(operation string) {
	cb.state = CircuitOpen
	cb.lastStateChange = time.Now()
	cb.logger.Warn("Circuit breaker opened",
		zap.String("operation", operation),
		zap.Int("failures", cb.failureCount),
		zap.Duration("reset_timeout", cb.resetTimeout))
}

// transitionToHalfOpen переводит circuit breaker в полуоткрытое состояние
func (cb *CircuitBreaker) transitionToHalfOpen(operation string) {
	cb.state = CircuitHalfOpen
	cb.lastStateChange = time.Now()
	cb.logger.Info("Circuit breaker half-opened",
		zap.String("operation", operation),
		zap.Time("since", cb.lastStateChange))
}

// transitionToClosed переводит circuit breaker в закрытое состояние
func (cb *CircuitBreaker) transitionToClosed(operation string) {
	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.lastStateChange = time.Now()
	cb.logger.Info("Circuit breaker closed",
		zap.String("operation", operation))
}

// GetState возвращает текущее состояние circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// stateString возвращает строковое представление состояния
func (cb *CircuitBreaker) stateString() string {
	switch cb.state {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}
