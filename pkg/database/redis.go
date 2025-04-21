package database

import (
	"JollyRogerUserService/config"
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

// NewRedisClient создает новое подключение к Redis
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10, // Максимальное количество соединений в пуле
		MinIdleConns: 5,  // Минимальное количество соединений в пуле
	})

	// Проверка подключения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return client, nil
}
