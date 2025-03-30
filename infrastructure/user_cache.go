package infrastructure

import (
	"JollyRogerUserService/internal/user"
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type UserCache struct {
	client *redis.Client
}

func NewUserCache(client *redis.Client) user.Cache {
	return &UserCache{client}
}

func (c *UserCache) Get(chatID int64) (*user.User, error) {
	key := fmt.Sprintf("user:%d", chatID)
	val, err := c.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	var u user.User
	json.Unmarshal([]byte(val), &u)
	return &u, nil
}

func (c *UserCache) Set(user *user.User, ttl time.Duration) error {
	key := fmt.Sprintf("user:%d", user.ChatID)
	data, _ := json.Marshal(user)
	return c.client.Set(context.Background(), key, data, ttl).Err()
}

func (c *UserCache) Delete(chatID int64) error {
	key := fmt.Sprintf("user:%d", chatID)
	return c.client.Del(context.Background(), key).Err()
}
