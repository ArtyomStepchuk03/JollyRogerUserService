package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRetryMechanism_BasicRetry(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаем настройки для тестирования
	options := RetryOptions{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         0.1,
	}

	// Создаем счетчик вызовов
	callCount := 0
	testErr := errors.New("test error")

	// Функция, которая будет вызываться несколько раз до успеха
	err := WithRetry(ctx, logger, "test_operation", options, func(ctx context.Context) error {
		callCount++
		if callCount <= options.MaxRetries {
			return testErr
		}
		return nil
	})

	// Проверяем, что функция вызывалась нужное количество раз
	if err != nil {
		t.Errorf("Expected success after %d retries, got error: %v", options.MaxRetries, err)
	}

	expectedCalls := options.MaxRetries + 1 // Изначальный вызов + повторы
	if callCount != expectedCalls {
		t.Errorf("Expected %d calls, got %d", expectedCalls, callCount)
	}
}

func TestRetryMechanism_MaxRetriesExceeded(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаем настройки для тестирования
	options := RetryOptions{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		BackoffFactor:  1.5,
		Jitter:         0.1,
	}

	// Создаем счетчик вызовов
	callCount := 0
	testErr := errors.New("persistent error")

	// Функция, которая всегда возвращает ошибку
	err := WithRetry(ctx, logger, "test_operation", options, func(ctx context.Context) error {
		callCount++
		return testErr
	})

	// Проверяем, что функция вызывалась нужное количество раз
	if err != testErr {
		t.Errorf("Expected the original error after max retries, got: %v", err)
	}

	expectedCalls := options.MaxRetries + 1 // Изначальный вызов + повторы
	if callCount != expectedCalls {
		t.Errorf("Expected %d calls, got %d", expectedCalls, callCount)
	}
}

func TestRetryMechanism_BackoffCalculation(t *testing.T) {
	// Тестируем вычисление задержки
	options := RetryOptions{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.0, // Без случайности для предсказуемого тестирования
	}

	// Проверяем последовательные попытки
	backoff0 := calculateBackoff(0, options)
	backoff1 := calculateBackoff(1, options)
	backoff2 := calculateBackoff(2, options)

	// Первая попытка должна использовать начальную задержку
	if backoff0 != options.InitialBackoff {
		t.Errorf("Expected initial backoff to be %v, got %v", options.InitialBackoff, backoff0)
	}

	// Вторая попытка должна удвоить задержку
	expectedBackoff1 := time.Duration(float64(options.InitialBackoff) * options.BackoffFactor)
	if backoff1 != expectedBackoff1 {
		t.Errorf("Expected second backoff to be %v, got %v", expectedBackoff1, backoff1)
	}

	// Третья попытка должна снова удвоить задержку
	expectedBackoff2 := time.Duration(float64(options.InitialBackoff) * options.BackoffFactor * options.BackoffFactor)
	if backoff2 != expectedBackoff2 {
		t.Errorf("Expected third backoff to be %v, got %v", expectedBackoff2, backoff2)
	}

	// Проверяем, что задержка не превышает максимальную
	largeAttempt := calculateBackoff(10, options)
	if largeAttempt > options.MaxBackoff {
		t.Errorf("Backoff %v exceeded max backoff %v", largeAttempt, options.MaxBackoff)
	}
}

func TestRetryMechanism_RetryableErrors(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаем тестовые ошибки
	retryableErr := errors.New("retryable error")
	nonRetryableErr := errors.New("non-retryable error")

	// Создаем настройки с указанием повторяемых ошибок
	options := RetryOptions{
		MaxRetries:      2,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      50 * time.Millisecond,
		BackoffFactor:   1.5,
		Jitter:          0.1,
		RetryableErrors: []error{retryableErr},
	}

	// Тест 1: Повторяемая ошибка
	t.Run("RetryableError", func(t *testing.T) {
		callCount := 0

		err := WithRetry(ctx, logger, "retryable_test", options, func(ctx context.Context) error {
			callCount++
			if callCount <= options.MaxRetries {
				return retryableErr
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected success after retries with retryable error, got: %v", err)
		}

		expectedCalls := options.MaxRetries + 1
		if callCount != expectedCalls {
			t.Errorf("Expected %d calls, got %d", expectedCalls, callCount)
		}
	})

	// Тест 2: Неповторяемая ошибка
	t.Run("NonRetryableError", func(t *testing.T) {
		callCount := 0

		err := WithRetry(ctx, logger, "non_retryable_test", options, func(ctx context.Context) error {
			callCount++
			return nonRetryableErr
		})

		if err != nonRetryableErr {
			t.Errorf("Expected non-retryable error to be returned immediately, got: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Expected only 1 call for non-retryable error, got %d", callCount)
		}
	})

	// Тест 3: Пустой список повторяемых ошибок (все ошибки считаются повторяемыми)
	t.Run("EmptyRetryableErrorsList", func(t *testing.T) {
		callCount := 0
		emptyOptions := options
		emptyOptions.RetryableErrors = nil

		err := WithRetry(ctx, logger, "empty_list_test", emptyOptions, func(ctx context.Context) error {
			callCount++
			if callCount <= emptyOptions.MaxRetries {
				return nonRetryableErr // Теперь должна повторяться
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected success with empty retryable errors list, got: %v", err)
		}

		expectedCalls := emptyOptions.MaxRetries + 1
		if callCount != expectedCalls {
			t.Errorf("Expected %d calls, got %d", expectedCalls, callCount)
		}
	})
}

func TestRetryMechanism_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())

	// Создаем настройки для тестирования
	options := RetryOptions{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
	}

	callCount := 0
	testErr := errors.New("test error")

	// Запускаем асинхронное выполнение с повторами
	resultChan := make(chan error)
	go func() {
		resultChan <- WithRetry(ctx, logger, "cancel_test", options, func(ctx context.Context) error {
			callCount++
			// Отменяем контекст после первого вызова
			if callCount == 1 {
				cancel()
				time.Sleep(50 * time.Millisecond) // Даем время для обработки отмены
			}
			return testErr
		})
	}()

	// Ожидаем результат с таймаутом
	select {
	case err := <-resultChan:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Test timed out - retry mechanism did not respect context cancellation")
	}

	// Проверяем, что функция была вызвана только один раз
	if callCount != 1 {
		t.Errorf("Expected only 1 call before context cancellation, got %d", callCount)
	}
}

func TestRetryMechanism_Jitter(t *testing.T) {
	// Проверяем, что jitter действительно изменяет задержку
	options := RetryOptions{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.5, // 50% случайное отклонение
	}

	// Получаем несколько значений задержки для одной и той же попытки
	samples := 10
	backoffs := make([]time.Duration, samples)

	for i := 0; i < samples; i++ {
		backoffs[i] = calculateBackoff(1, options)
	}

	// Проверяем, что были получены разные значения
	allSame := true
	firstValue := backoffs[0]

	for i := 1; i < samples; i++ {
		if backoffs[i] != firstValue {
			allSame = false
			break
		}
	}

	if allSame {
		t.Errorf("Expected different backoff values with jitter, but all values were %v", firstValue)
	}

	// Проверяем, что значения находятся в ожидаемом диапазоне
	baseValue := time.Duration(float64(options.InitialBackoff) * options.BackoffFactor)
	minExpected := time.Duration(float64(baseValue) * (1 - options.Jitter))
	maxExpected := time.Duration(float64(baseValue) * (1 + options.Jitter))

	for i, backoff := range backoffs {
		if backoff < minExpected || backoff > maxExpected {
			t.Errorf("Backoff[%d] = %v is outside expected range [%v, %v]",
				i, backoff, minExpected, maxExpected)
		}
	}
}
