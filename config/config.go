package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// AppConfig содержит конфигурацию для всего приложения
type AppConfig struct {
	Version  string // Версия приложения
	LogLevel string // Уровень логирования
}

// Config содержит все настройки приложения
type Config struct {
	App      AppConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	GRPC     GRPCConfig
}

// PostgresConfig содержит настройки для PostgreSQL
type PostgresConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig содержит настройки для Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// GRPCConfig содержит настройки для gRPC сервера
type GRPCConfig struct {
	Port int
}

// LoadConfig загружает настройки из .env файла и переменных окружения
func LoadConfig() (*Config, error) {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		// Если файл не найден, это не критическая ошибка
		// Продолжаем работать с переменными окружения
	}

	version := getEnv("APP_VERSION", "1.0.0") // По умолчанию версия 1.0.0
	logLevel := getEnv("LOG_LEVEL", "info")   // По умолчанию info

	config := &Config{
		App: AppConfig{
			Version:  version,
			LogLevel: logLevel,
		},
		Postgres: loadPostgresConfig(),
		Redis:    loadRedisConfig(),
		GRPC:     loadGRPCConfig(),
	}

	return config, nil
}

// loadPostgresConfig загружает конфигурацию PostgreSQL
func loadPostgresConfig() PostgresConfig {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))

	return PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     port,
		Username: getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "jollyroger"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// loadRedisConfig загружает конфигурацию Redis
func loadRedisConfig() RedisConfig {
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	return RedisConfig{
		Addr:     redisHost + ":" + redisPort,
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       redisDB,
	}
}

// loadGRPCConfig загружает конфигурацию gRPC
func loadGRPCConfig() GRPCConfig {
	port, _ := strconv.Atoi(getEnv("GRPC_PORT", "50051"))

	return GRPCConfig{
		Port: port,
	}
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
