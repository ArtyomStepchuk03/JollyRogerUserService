package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// RetryOptions настройки для механизма повторных попыток
type RetryOptions struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	Jitter          float64
	RetryableErrors []error
}

// DefaultRetryOptions возвращает настройки по умолчанию
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.2,
	}
}

// WithRetry выполняет функцию с повторными попытками при ошибках
func WithRetry(ctx context.Context, logger *zap.Logger, operation string, options RetryOptions, fn func(context.Context) error) error {
	var lastErr error
	
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// Выполняем функцию
		err := fn(ctx)
		
		// Если ошибки нет, возвращаем успех
		if err == nil {
			if attempt > 0 {
				logger.Info("Operation succeeded after retries",
					zap.String("operation", operation),
					zap.Int("attempt", attempt+1))
			}
			return nil
		}
		
		// Если это последняя попытка, возвращаем ошибку
		if attempt == options.MaxRetries {
			logger.Warn("All retry attempts failed",
				zap.String("operation", operation),
				zap.Int("attempts", attempt+1),
				zap.Error(err))
			lastErr = err
			break
		}
		
		// Проверяем, можно ли повторять для этой ошибки
		if !isRetryable(err, options.RetryableErrors) {
			logger.Warn("Non-retryable error occurred",
				zap.String("operation", operation),
				zap.Error(err))
			return err
		}
		
		// Вычисляем время ожидания с экспоненциальной задержкой и случайным отклонением
		backoff := calculateBackoff(attempt, options)
		
		logger.Info("Retrying operation after error",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Duration("backoff", backoff),
			zap.Error(err))
		
		// Проверяем контекст перед ожиданием
		select {
		case <-time.After(backoff):
			// Продолжаем с новой попыткой
		case <-ctx.Done():
			// Контекст отменен, выходим
			logger.Warn("Context cancelled during retry",
				zap.String("operation", operation),
				zap.Error(ctx.Err()))
			return ctx.Err()
		}
		
		lastErr = err
	}
	
	return lastErr
}

// isRetryable проверяет, нужно ли повторять операцию для данной ошибки
func isRetryable(err error, retryableErrors []error) bool {
	// Если список ошибок пуст, считаем все ошибки повторяемыми
	if len(retryableErrors) == 0 {
		return true
	}
	
	// Проверяем, содержится ли ошибка в списке повторяемых
	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}
	
	return false
}

// calculateBackoff вычисляет время ожидания с экспоненциальной задержкой
func calculateBackoff(attempt int, options RetryOptions) time.Duration {
	// Применяем экспоненциальную задержку
	backoff := float64(options.InitialBackoff) * math.Pow(options.BackoffFactor, float64(attempt))
	
	// Применяем случайное отклонение (jitter)
	if options.Jitter > 0 {
		jitter := (rand.Float64() * 2 - 1) * options.Jitter
		backoff = backoff * (1 + jitter)
	}
	
	// Ограничиваем максимальным значением
	if backoff > float64(options.MaxBackoff) {
		backoff = float64(options.MaxBackoff)
	}
	
	return time.Duration(backoff)
}
