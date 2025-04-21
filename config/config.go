package config

import (
	"github.com/spf13/viper"
	"os"
	"strconv"
)

// Config содержит все настройки приложения
type Config struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
}

// PostgresConfig содержит настройки для PostgreSQL
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// RedisConfig содержит настройки для Redis
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// GRPCConfig содержит настройки для gRPC сервера
type GRPCConfig struct {
	Port int `mapstructure:"port"`
}

// LoadConfig загружает настройки из файла или переменных окружения
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Значения по умолчанию
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		// Если файл конфигурации не найден, используем переменные окружения
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Проверяем наличие переменных окружения и переопределяем значения конфигурации
	loadFromEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	// PostgreSQL defaults
	viper.SetDefault("postgres.host", "localhost")
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.username", "postgres")
	viper.SetDefault("postgres.password", "postgres")
	viper.SetDefault("postgres.dbname", "teamfinder")
	viper.SetDefault("postgres.sslmode", "disable")

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// gRPC defaults
	viper.SetDefault("grpc.port", 50051)
}

func loadFromEnv() {
	// PostgreSQL from env
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		viper.Set("postgres.host", dbHost)
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			viper.Set("postgres.port", port)
		}
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		viper.Set("postgres.username", dbUser)
	}
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		viper.Set("postgres.password", dbPassword)
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		viper.Set("postgres.dbname", dbName)
	}

	// Redis from env
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		redisPort := "6379" // Default Redis port
		if port := os.Getenv("REDIS_PORT"); port != "" {
			redisPort = port
		}
		viper.Set("redis.addr", redisHost+":"+redisPort)
	}

	// gRPC from env
	if grpcPort := os.Getenv("GRPC_PORT"); grpcPort != "" {
		if port, err := strconv.Atoi(grpcPort); err == nil {
			viper.Set("grpc.port", port)
		}
	}
}
