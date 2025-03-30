package user

import (
	"time"
)

type Cache interface {
	Get(chatID int64) (*User, error)
	Set(user *User, ttl time.Duration) error
	Delete(chatID int64) error
}
