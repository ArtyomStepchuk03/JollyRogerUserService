package server

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestGracefulShutdown_AddShutdownFunc тестирует добавление функций завершения
func TestGracefulShutdown_AddShutdownFunc(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown с коротким таймаутом для тестов
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Проверяем, что изначально нет функций завершения
	if len(gs.shutdownFuncs) != 0 {
		t.Errorf("Expected 0 shutdown functions, got %d", len(gs.shutdownFuncs))
	}

	// Добавляем функцию завершения
	functionCalled := false
	gs.AddShutdownFunc(func(ctx context.Context) error {
		functionCalled = true
		return nil
	})

	// Проверяем, что функция была добавлена
	if len(gs.shutdownFuncs) != 1 {
		t.Errorf("Expected 1 shutdown function, got %d", len(gs.shutdownFuncs))
	}

	// Вызываем метод shutdown для проверки выполнения функции
	gs.shutdown()

	// Проверяем, что функция была вызвана
	if !functionCalled {
		t.Error("Shutdown function was not called")
	}
}

// TestGracefulShutdown_FunctionOrder тестирует порядок выполнения функций завершения
func TestGracefulShutdown_FunctionOrder(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Добавляем несколько функций и отслеживаем порядок их выполнения
	order := make([]int, 0, 3)

	gs.AddShutdownFunc(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})

	gs.AddShutdownFunc(func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})

	gs.AddShutdownFunc(func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	// Вызываем shutdown
	gs.shutdown()

	// Проверяем порядок выполнения (LIFO - последним вошел, первым вышел)
	expectedOrder := []int{3, 2, 1}
	for i, v := range expectedOrder {
		if i >= len(order) || order[i] != v {
			t.Errorf("Expected shutdown order %v, got %v", expectedOrder, order)
			break
		}
	}
}

// TestGracefulShutdown_ErrorHandling тестирует обработку ошибок в функциях завершения
func TestGracefulShutdown_ErrorHandling(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Добавляем функции, некоторые из которых возвращают ошибки
	firstCalled := false
	secondCalled := false
	thirdCalled := false

	gs.AddShutdownFunc(func(ctx context.Context) error {
		firstCalled = true
		return nil // Успешное завершение
	})

	gs.AddShutdownFunc(func(ctx context.Context) error {
		secondCalled = true
		return context.DeadlineExceeded // Ошибка таймаута
	})

	gs.AddShutdownFunc(func(ctx context.Context) error {
		thirdCalled = true
		return nil // Успешное завершение
	})

	// Вызываем shutdown
	gs.shutdown()

	// Проверяем, что все функции были вызваны, несмотря на ошибку во второй
	if !firstCalled {
		t.Error("First shutdown function was not called")
	}

	if !secondCalled {
		t.Error("Second shutdown function was not called")
	}

	if !thirdCalled {
		t.Error("Third shutdown function was not called")
	}
}

// TestGracefulShutdown_Timeout тестирует обработку таймаута
func TestGracefulShutdown_Timeout(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown с очень коротким таймаутом
	gs := NewGracefulShutdown(logger, 50*time.Millisecond)

	// Добавляем функцию, которая выполняется дольше таймаута
	functionCompleted := false
	gs.AddShutdownFunc(func(ctx context.Context) error {
		// Проверяем, есть ли таймаут в контексте
		deadline, hasDeadline := ctx.Deadline()
		if !hasDeadline {
			t.Error("Expected context with deadline")
		} else {
			// Проверяем, что таймаут примерно соответствует ожидаемому
			now := time.Now()
			if deadline.Sub(now) > 100*time.Millisecond {
				t.Errorf("Expected deadline to be close to 50ms, got %v", deadline.Sub(now))
			}
		}

		// Пытаемся выполнить действие, которое превышает таймаут
		select {
		case <-time.After(200 * time.Millisecond):
			functionCompleted = true
		case <-ctx.Done():
			// Контекст должен быть отменен по таймауту
			if ctx.Err() != context.DeadlineExceeded {
				t.Errorf("Expected deadline exceeded error, got: %v", ctx.Err())
			}
		}

		return nil
	})

	// Вызываем shutdown
	gs.shutdown()

	// Функция не должна была завершиться полностью из-за таймаута
	if functionCompleted {
		t.Error("Expected function to be interrupted by timeout, but it completed")
	}
}

// TestGracefulShutdown_WaitWithContext тестирует ожидание с контекстом
func TestGracefulShutdown_WaitWithContext(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Отслеживаем вызов shutdown
	shutdownCalled := false
	gs.AddShutdownFunc(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	// Создаем контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())

	// Запускаем WaitWithContext в отдельной горутине
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel() // Отменяем контекст после небольшой задержки
	}()

	// Вызываем WaitWithContext, который должен завершиться после отмены контекста
	gs.WaitWithContext(ctx)

	// Проверяем, что shutdown был вызван
	if !shutdownCalled {
		t.Error("Expected shutdown to be called after context cancellation")
	}

	// Проверяем, что канал done закрыт
	select {
	case <-gs.done:
		// Канал закрыт, как и ожидалось
	default:
		t.Error("Expected done channel to be closed")
	}
}

// TestGracefulShutdown_ShutdownSignal тестирует обработку сигнала завершения
func TestGracefulShutdown_ShutdownSignal(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Отслеживаем вызов shutdown
	shutdownCalled := false
	gs.AddShutdownFunc(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	// Запускаем Wait в отдельной горутине
	waitDone := make(chan struct{})
	go func() {
		gs.Wait()
		close(waitDone)
	}()

	// Отправляем сигнал завершения через небольшую задержку
	go func() {
		time.Sleep(50 * time.Millisecond)
		gs.shutdownSignal <- syscall.SIGTERM
	}()

	// Ожидаем завершения Wait
	select {
	case <-waitDone:
		// Wait завершился, как и ожидалось
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for graceful shutdown")
	}

	// Проверяем, что shutdown был вызван
	if !shutdownCalled {
		t.Error("Expected shutdown to be called after signal")
	}
}

// TestGracefulShutdown_Done тестирует метод Done
func TestGracefulShutdown_Done(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Канал done должен быть изначально открыт
	select {
	case <-gs.Done():
		t.Error("Expected done channel to be open initially")
	default:
		// Канал открыт, как и ожидалось
	}

	// Вызываем Shutdown для инициирования завершения
	go gs.Shutdown()

	// Ожидаем закрытия канала done
	select {
	case <-gs.Done():
		// Канал закрыт, как и ожидалось
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for done channel to close")
	}
}

// TestGracefulShutdown_Shutdown тестирует метод Shutdown
func TestGracefulShutdown_Shutdown(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Отслеживаем вызов shutdown
	shutdownCalled := false
	gs.AddShutdownFunc(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	// Вызываем Shutdown
	gs.Shutdown()

	// Проверяем, что shutdown был вызван
	if !shutdownCalled {
		t.Error("Expected shutdown to be called")
	}

	// Проверяем, что канал done закрыт
	select {
	case <-gs.done:
		// Канал закрыт, как и ожидалось
	default:
		t.Error("Expected done channel to be closed")
	}
}

// TestGracefulShutdown_RealSignalHandling тестирует обработку реальных сигналов системы
func TestGracefulShutdown_RealSignalHandling(t *testing.T) {
	// Пропускаем этот тест, если запущены все тесты пакета
	// (так как отправка сигналов может повлиять на другие тесты)
	if testing.Short() {
		t.Skip("Skipping real signal test in short mode")
	}

	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Отслеживаем вызов shutdown
	shutdownCalled := false
	shutdownDone := make(chan struct{})
	gs.AddShutdownFunc(func(ctx context.Context) error {
		shutdownCalled = true
		close(shutdownDone)
		return nil
	})

	// Запускаем Wait в отдельной горутине
	waitDone := make(chan struct{})
	go func() {
		gs.Wait()
		close(waitDone)
	}()

	// Получаем PID текущего процесса
	pid := os.Getpid()

	// Отправляем сигнал SIGTERM процессу
	// Делаем это в отдельной горутине с задержкой,
	// чтобы успела выполниться Wait
	go func() {
		time.Sleep(100 * time.Millisecond)
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Logf("Failed to find process: %v", err)
			return
		}
		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			t.Logf("Failed to send signal: %v", err)
		}
	}()

	// Ожидаем выполнения shutdown функции с таймаутом
	select {
	case <-shutdownDone:
		// Функция завершения вызвана, как и ожидалось
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for shutdown function")
	}

	// Проверяем, что shutdown был вызван
	if !shutdownCalled {
		t.Error("Expected shutdown to be called after signal")
	}

	// Ожидаем завершения Wait
	select {
	case <-waitDone:
		// Wait завершился, как и ожидалось
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for Wait to complete")
	}
}

// TestGracefulShutdown_MultipleShutdowns тестирует, что shutdown вызывается только один раз
func TestGracefulShutdown_MultipleShutdowns(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Счетчик вызовов shutdown функции
	shutdownCount := 0
	gs.AddShutdownFunc(func(ctx context.Context) error {
		shutdownCount++
		return nil
	})

	// Вызываем Shutdown несколько раз
	for i := 0; i < 5; i++ {
		gs.Shutdown()
	}

	// Проверяем, что shutdown был вызван только один раз
	if shutdownCount != 1 {
		t.Errorf("Expected shutdown to be called once, got %d calls", shutdownCount)
	}
}

// TestGracefulShutdown_ConcurrentShutdowns тестирует конкурентные вызовы Shutdown
func TestGracefulShutdown_ConcurrentShutdowns(t *testing.T) {
	// Создаем тестовый логгер
	logger := zap.NewNop()

	// Создаем GracefulShutdown
	gs := NewGracefulShutdown(logger, 100*time.Millisecond)

	// Счетчик вызовов shutdown функции и мьютекс для безопасного доступа
	var shutdownCount int
	var mu sync.Mutex

	gs.AddShutdownFunc(func(ctx context.Context) error {
		mu.Lock()
		shutdownCount++
		mu.Unlock()
		return nil
	})

	// Запускаем несколько горутин, которые конкурентно вызывают Shutdown
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			gs.Shutdown()
		}()
	}

	// Ожидаем завершения всех горутин
	wg.Wait()

	// Проверяем, что shutdown был вызван только один раз
	if shutdownCount != 1 {
		t.Errorf("Expected shutdown to be called once, got %d calls", shutdownCount)
	}
}
