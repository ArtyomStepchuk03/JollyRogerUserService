package config

import (
	"time"
)

// ResilienceConfig содержит настройки для механизмов отказоустойчивости
type ResilienceConfig struct {
	// CircuitBreaker содержит настройки для circuit breaker
	CircuitBreaker struct {
		// FailureThreshold количество ошибок, после которого circuit breaker откроется
		FailureThreshold int
		// ResetTimeout время, через которое circuit breaker перейдет в полуоткрытое состояние
		ResetTimeout time.Duration
	}

	// Retry содержит настройки для механизма повторных попыток
	Retry struct {
		// MaxRetries максимальное количество повторных попыток
		MaxRetries int
		// InitialBackoff начальная задержка между попытками
		InitialBackoff time.Duration
		// MaxBackoff максимальная задержка между попытками
		MaxBackoff time.Duration
		// BackoffFactor коэффициент увеличения задержки
		BackoffFactor float64
		// Jitter процент случайного отклонения от задержки
		Jitter float64
	}

	// Database содержит настройки механизмов отказоустойчивости для базы данных
	Database struct {
		// CommandTimeout таймаут для выполнения команд
		CommandTimeout time.Duration
	}

	// Redis содержит настройки механизмов отказоустойчивости для Redis
	Redis struct {
		// CommandTimeout таймаут для выполнения команд
		CommandTimeout time.Duration
	}
}

// DefaultResilienceConfig возвращает конфигурацию отказоустойчивости по умолчанию
func DefaultResilienceConfig() ResilienceConfig {
	config := ResilienceConfig{}

	// Настройки circuit breaker
	config.CircuitBreaker.FailureThreshold = 5
	config.CircuitBreaker.ResetTimeout = 30 * time.Second

	// Настройки retry
	config.Retry.MaxRetries = 3
	config.Retry.InitialBackoff = 100 * time.Millisecond
	config.Retry.MaxBackoff = 2 * time.Second
	config.Retry.BackoffFactor = 2.0
	config.Retry.Jitter = 0.2

	// Настройки для базы данных
	config.Database.CommandTimeout = 3 * time.Second

	// Настройки для Redis
	config.Redis.CommandTimeout = 1 * time.Second

	return config
}
