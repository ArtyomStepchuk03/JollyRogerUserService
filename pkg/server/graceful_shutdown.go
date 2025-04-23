package server

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// GracefulShutdown обеспечивает корректное завершение работы сервера
type GracefulShutdown struct {
	logger         *zap.Logger
	timeout        time.Duration
	shutdownFuncs  []func(context.Context) error
	shutdownSignal chan os.Signal
	done           chan struct{}
	once           sync.Once
}

// NewGracefulShutdown создает новый экземпляр GracefulShutdown
func NewGracefulShutdown(logger *zap.Logger, timeout time.Duration) *GracefulShutdown {
	gs := &GracefulShutdown{
		logger:         logger,
		timeout:        timeout,
		shutdownFuncs:  make([]func(context.Context) error, 0),
		shutdownSignal: make(chan os.Signal, 1),
		done:           make(chan struct{}),
	}

	signal.Notify(gs.shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	return gs
}

// AddShutdownFunc добавляет функцию для выполнения при завершении работы
func (gs *GracefulShutdown) AddShutdownFunc(f func(context.Context) error) {
	gs.shutdownFuncs = append(gs.shutdownFuncs, f)
}

// Wait блокирует выполнение до получения сигнала завершения
func (gs *GracefulShutdown) Wait() {
	<-gs.shutdownSignal
	gs.logger.Info("Shutdown signal received")

	gs.once.Do(func() {
		gs.shutdown()
		close(gs.done)
	})
}

// WaitWithContext блокирует выполнение до получения сигнала завершения или отмены контекста
func (gs *GracefulShutdown) WaitWithContext(ctx context.Context) {
	select {
	case <-gs.shutdownSignal:
		gs.logger.Info("Shutdown signal received")
	case <-ctx.Done():
		gs.logger.Info("Context cancelled, initiating shutdown")
	}

	gs.once.Do(func() {
		gs.shutdown()
		close(gs.done)
	})
}

// Done возвращает канал, который закрывается после завершения всех операций
func (gs *GracefulShutdown) Done() <-chan struct{} {
	return gs.done
}

// Shutdown инициирует процесс завершения работы
func (gs *GracefulShutdown) Shutdown() {
	gs.shutdownSignal <- syscall.SIGTERM
	<-gs.done
}

// shutdown выполняет все зарегистрированные функции завершения
func (gs *GracefulShutdown) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
	defer cancel()

	// Выполняем функции завершения в обратном порядке (LIFO)
	for i := len(gs.shutdownFuncs) - 1; i >= 0; i-- {
		shutdownFunc := gs.shutdownFuncs[i]
		if err := shutdownFunc(ctx); err != nil {
			gs.logger.Error("Error during shutdown", zap.Error(err), zap.Int("func_index", i))
		}
	}

	gs.logger.Info("Graceful shutdown completed")
}
