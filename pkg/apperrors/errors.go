package apperrors

import (
	"errors"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Список игнорируемых ошибок для механизмов отказоустойчивости
var (
	// ErrNotFound возвращается, когда запись не найдена (обобщенная ошибка)
	ErrNotFound = errors.New("запись не найдена")

	// ErrCacheMiss возвращается, когда запись не найдена в кэше
	ErrCacheMiss = redis.Nil

	// ErrRecordNotFound возвращается, когда запись не найдена в базе данных
	ErrRecordNotFound = gorm.ErrRecordNotFound

	// IgnoredErrors содержит список всех игнорируемых ошибок для circuit breaker
	IgnoredErrors = []error{
		ErrNotFound,
		ErrCacheMiss,
		ErrRecordNotFound,
	}
)

// IsNotFound проверяет, является ли ошибка ошибкой "запись не найдена"
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrCacheMiss) ||
		errors.Is(err, ErrRecordNotFound)
}
