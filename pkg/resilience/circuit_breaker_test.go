package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCircuitBreaker_States(t *testing.T) {
	logger := zap.NewNop()

	// Создаем circuit breaker с низким порогом для более быстрого тестирования
	failureThreshold := 3
	resetTimeout := 500 * time.Millisecond
	cb := NewCircuitBreaker(failureThreshold, resetTimeout, logger)

	// Проверяем начальное состояние
	if state := cb.GetState(); state != CircuitClosed {
		t.Errorf("Expected initial state to be CLOSED, got %v", cb.stateString())
	}

	// Тестовая ошибка
	testErr := errors.New("test error")
	ctx := context.Background()

	// Шаг 1: Проверяем, что circuit breaker открывается после нескольких ошибок
	for i := 0; i < failureThreshold; i++ {
		err := cb.Execute(ctx, "test_operation", func(ctx context.Context) error {
			return testErr
		})
		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	}

	// Проверяем, что circuit breaker открыт
	if state := cb.GetState(); state != CircuitOpen {
		t.Errorf("Expected circuit to be OPEN after %d failures, got %v",
			failureThreshold, cb.stateString())
	}

	// Шаг 2: При открытом circuit breaker функция не должна выполняться
	operationCalled := false
	err := cb.Execute(ctx, "test_operation", func(ctx context.Context) error {
		operationCalled = true
		return nil
	})

	if operationCalled {
		t.Error("Operation was called when circuit is open")
	}

	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("Expected 'circuit breaker is open' error, got: %v", err)
	}

	// Шаг 3: Ждем, пока circuit breaker перейдет в полуоткрытое состояние
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// Следующий запрос должен выполниться в полуоткрытом состоянии
	successOp := false
	err = cb.Execute(ctx, "test_operation", func(ctx context.Context) error {
		successOp = true
		return nil
	})

	if !successOp {
		t.Error("Operation was not called in half-open state")
	}

	if err != nil {
		t.Errorf("Expected no error in half-open state for successful operation, got: %v", err)
	}

	// Проверяем, что после успешного запроса circuit breaker закрыт
	if state := cb.GetState(); state != CircuitClosed {
		t.Errorf("Expected circuit to be CLOSED after successful operation, got %v",
			cb.stateString())
	}

	// Шаг 4: Проверяем переход из полуоткрытого состояния обратно в открытое при ошибке
	// Сначала снова открываем circuit breaker
	for i := 0; i < failureThreshold; i++ {
		_ = cb.Execute(ctx, "test_operation", func(ctx context.Context) error {
			return testErr
		})
	}

	// Ждем, пока circuit breaker перейдет в полуоткрытое состояние
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// Ошибка в полуоткрытом состоянии должна снова открыть circuit breaker
	_ = cb.Execute(ctx, "test_operation", func(ctx context.Context) error {
		return testErr
	})

	// Проверяем, что circuit breaker снова открыт
	if state := cb.GetState(); state != CircuitOpen {
		t.Errorf("Expected circuit to be OPEN after failure in half-open state, got %v",
			cb.stateString())
	}
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	logger := zap.NewNop()
	cb := NewCircuitBreaker(5, 1*time.Second, logger)
	ctx := context.Background()

	// Запускаем несколько одновременных горутин для проверки потокобезопасности
	const numGoroutines = 10
	const numRequests = 20

	errChan := make(chan error, numGoroutines*numRequests)
	done := make(chan struct{})

	// Создаем барьер для одновременного запуска всех горутин
	ready := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(goID int) {
			// Ждем сигнала для запуска
			<-ready

			for j := 0; j < numRequests; j++ {
				err := cb.Execute(ctx, "concurrent_test", func(ctx context.Context) error {
					// Возвращаем ошибку для некоторых запросов, чтобы открыть circuit breaker
					if j > numRequests/2 {
						return errors.New("deliberate test error")
					}
					return nil
				})
				errChan <- err
			}

			// Сигнализируем о завершении
			done <- struct{}{}
		}(i)
	}

	// Запускаем все горутины одновременно
	close(ready)

	// Ждем завершения всех горутин
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Проверяем, что circuit breaker оказался в ожидаемом состоянии
	// Так как мы вызвали больше ошибок, чем порог, ожидаем, что circuit breaker открыт
	if state := cb.GetState(); state != CircuitOpen {
		t.Errorf("Expected circuit to be OPEN after concurrent tests, got %v", cb.stateString())
	}

	// Закрываем канал ошибок и проверяем, что все операции завершились ожидаемым образом
	close(errChan)

	successCount := 0
	failureCount := 0
	circuitOpenCount := 0

	for err := range errChan {
		if err == nil {
			successCount++
		} else if err.Error() == "circuit breaker is open" {
			circuitOpenCount++
		} else {
			failureCount++
		}
	}

	t.Logf("Success: %d, Failures: %d, Circuit Open: %d",
		successCount, failureCount, circuitOpenCount)

	// Проверяем, что были все три типа результатов
	if successCount == 0 {
		t.Error("Expected some successful operations")
	}
	if failureCount == 0 {
		t.Error("Expected some failed operations")
	}
	if circuitOpenCount == 0 {
		t.Error("Expected some operations to be rejected by open circuit")
	}
}

func TestCircuitBreaker_EdgeCases(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Тест 1: Circuit breaker с нулевым порогом
	t.Run("ZeroThreshold", func(t *testing.T) {
		cb := NewCircuitBreaker(0, 1*time.Second, logger)

		// Проверяем, что circuit breaker работает и при нулевом пороге
		err := cb.Execute(ctx, "zero_threshold", func(ctx context.Context) error {
			return errors.New("test error")
		})
		if err == nil {
			t.Errorf("Expected error, got nil")
		}

		// Проверяем, что circuit breaker был открыт
		if state := cb.GetState(); state != CircuitOpen {
			t.Errorf("Expected circuit to be OPEN even with zero threshold, got %v", cb.stateString())
		}
	})

	// Тест 2: Circuit breaker с очень коротким таймаутом
	t.Run("VeryShortTimeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 1*time.Millisecond, logger)

		// Открываем circuit breaker
		_ = cb.Execute(ctx, "short_timeout", func(ctx context.Context) error {
			return errors.New("test error")
		})

		// Ждем, пока circuit breaker перейдет в полуоткрытое состояние
		time.Sleep(5 * time.Millisecond)

		// Успешный запрос должен закрыть circuit breaker
		_ = cb.Execute(ctx, "short_timeout", func(ctx context.Context) error {
			return nil
		})

		// Проверяем, что circuit breaker закрыт
		if state := cb.GetState(); state != CircuitClosed {
			t.Errorf("Expected circuit to be CLOSED after timeout, got %v", cb.stateString())
		}
	})

	// Тест 3: Отмена контекста
	t.Run("CancelledContext", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 1*time.Second, logger)

		// Создаем отмененный контекст
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		executed := false
		err := cb.Execute(cancelCtx, "cancelled_context", func(ctx context.Context) error {
			executed = true
			return nil
		})

		// Операция всё равно должна выполниться, так как отмена контекста
		// проверяется в коде операции, а не в circuit breaker
		if !executed {
			t.Error("Operation should be executed even with cancelled context")
		}

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}
