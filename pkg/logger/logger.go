package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

// NewLogger создает новый логгер
func NewLogger() *zap.Logger {
	// Определение уровня логирования на основе переменной окружения
	logLevel := getLogLevel()

	// Настройка кодировщика для структурированного логирования
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Создание ядра логгера
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	// Создание логгера с добавлением информации о вызывающем коде
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return logger
}

// getLogLevel определяет уровень логирования на основе переменной окружения
func getLogLevel() zapcore.Level {
	// По умолчанию используем информационный уровень
	logLevel := zapcore.InfoLevel

	// Проверяем переменную окружения
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	}

	return logLevel
}
