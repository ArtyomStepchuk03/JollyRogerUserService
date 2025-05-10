package redis

import (
	"JollyRogerUserService/pkg/server"
	"context"
	"errors"
	"gorm.io/gorm"
	"time"

	"JollyRogerUserService/internal/models"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/resilience"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ResilientCacheRepository добавляет механизмы отказоустойчивости к кэш-репозиторию
type ResilientCacheRepository struct {
	client        *redis.Client
	repo          *CacheRepository
	logger        *zap.Logger
	healthChecker *database.HealthChecker
}

// NewResilientCacheRepository создает новый экземпляр отказоустойчивого кэш-репозитория
func NewResilientCacheRepository(client *redis.Client, db *gorm.DB, logger *zap.Logger) *ResilientCacheRepository {
	baseRepo := NewCacheRepository(client)
	healthChecker := database.NewDatabaseHealthChecker(db, client, logger)

	return &ResilientCacheRepository{
		client:        client,
		repo:          baseRepo,
		logger:        logger,
		healthChecker: healthChecker,
	}
}

// SetUser кэширует пользователя с отказоустойчивостью
func (r *ResilientCacheRepository) SetUser(ctx context.Context, user *models.User) error {
	// Если контекст не имеет таймаута, добавляем его
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	// Используем механизм circuit breaker для Redis
	err := r.healthChecker.WithRedisResilience(ctx, "set_user_cache", func(ctx context.Context) error {
		// Выполняем операцию Redis с безопасной обработкой ошибок
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_user_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetUser(ctx, user)
		})
	})

	// Игнорируем ошибки кэша, приложение должно продолжать работать
	if err != nil {
		r.logger.Warn("Failed to cache user, continuing without caching",
			zap.Error(err),
			zap.Uint("user_id", user.ID))
		return nil
	}

	return nil
}

// GetUser получает пользователя из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetUser(ctx context.Context, id uint) (*models.User, error) {
	startTime := time.Now()
	// Если контекст не имеет таймаута, добавляем его
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var user *models.User

	// Используем механизм повторных попыток с коротким таймаутом
	retryOptions := resilience.DefaultRetryOptions()
	retryOptions.MaxRetries = 1
	retryOptions.InitialBackoff = 50 * time.Millisecond
	retryOptions.MaxBackoff = 100 * time.Millisecond

	err := resilience.WithRetry(ctx, r.logger, "get_user_cache", retryOptions, func(ctx context.Context) error {
		var opErr error
		user, opErr = r.repo.GetUser(ctx, id)

		// Важное изменение: если ключ не найден, это не ошибка для circuit breaker
		if errors.Is(opErr, redis.Nil) {
			r.logger.Debug("Пользователь не найден в кэше", zap.Uint("user_id", id))
			return redis.Nil // Возвращаем ошибку для обработки вне retry
		}
		return opErr
	})

	// Записываем метрику только для реальных ошибок, а не для отсутствия ключа
	if err != nil && !errors.Is(err, redis.Nil) {
		server.RecordCacheOperation("get_user", time.Since(startTime), err)
	} else {
		// Для случая отсутствия ключа или успешной операции записываем успешную метрику
		server.RecordCacheOperation("get_user", time.Since(startTime), nil)
	}

	return user, err
}

// DeleteUser удаляет пользователя из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) DeleteUser(ctx context.Context, id uint) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "delete_user_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "delete_user_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.DeleteUser(ctx, id)
		})
	})

	// Игнорируем ошибки операций удаления кэша
	if err != nil {
		r.logger.Warn("Failed to delete user from cache",
			zap.Error(err),
			zap.Uint("user_id", id))
		return nil
	}

	return nil
}

// SetUserLocation кэширует местоположение пользователя с отказоустойчивостью
func (r *ResilientCacheRepository) SetUserLocation(ctx context.Context, location *models.UserLocation) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "set_location_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_location_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetUserLocation(ctx, location)
		})
	})

	// Игнорируем ошибки кэша
	if err != nil {
		r.logger.Warn("Failed to cache user location",
			zap.Error(err),
			zap.Uint("user_id", location.UserID))
		return nil
	}

	return nil
}

// GetUserLocation получает местоположение пользователя из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error) {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var location *models.UserLocation
	var err error

	// Пытаемся получить данные из кэша с быстрым таймаутом
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = r.healthChecker.WithRedisResilience(ctx, "get_location_cache", func(ctx context.Context) error {
		// Непосредственное обращение к кэшу без повторов
		location, err = r.repo.GetUserLocation(ctx, userID)
		return err
	})

	return location, err
}

// ClearUserCache очищает весь кэш пользователя с отказоустойчивостью
func (r *ResilientCacheRepository) ClearUserCache(ctx context.Context, userID uint) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}
	err := r.healthChecker.WithRedisResilience(ctx, "clear_user_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "clear_user_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.ClearUserCache(ctx, userID)
		})
	})

	// Игнорируем ошибки операций очистки кэша
	if err != nil {
		r.logger.Warn("Failed to clear user cache",
			zap.Error(err),
			zap.Uint("user_id", userID))
		return nil
	}

	return nil
}

// SetUserPreferences кэширует предпочтения пользователя с отказоустойчивостью
func (r *ResilientCacheRepository) SetUserPreferences(ctx context.Context, userID uint, preferences []models.UserPreference) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "set_preferences_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_preferences_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetUserPreferences(ctx, userID, preferences)
		})
	})

	// Игнорируем ошибки кэша
	if err != nil {
		r.logger.Warn("Failed to cache user preferences",
			zap.Error(err),
			zap.Uint("user_id", userID))
		return nil
	}

	return nil
}

// GetUserPreferences получает предпочтения пользователя из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error) {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var preferences []models.UserPreference
	var err error

	// Пытаемся получить данные из кэша с быстрым таймаутом
	fastCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = r.healthChecker.WithRedisResilience(fastCtx, "get_preferences_cache", func(ctx context.Context) error {
		preferences, err = r.repo.GetUserPreferences(ctx, userID)
		return err
	})

	return preferences, err
}

// SetUserStats кэширует статистику пользователя с отказоустойчивостью
func (r *ResilientCacheRepository) SetUserStats(ctx context.Context, stats *models.UserStats) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "set_stats_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_stats_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetUserStats(ctx, stats)
		})
	})

	// Игнорируем ошибки кэша
	if err != nil {
		r.logger.Warn("Failed to cache user stats",
			zap.Error(err),
			zap.Uint("user_id", stats.UserID))
		return nil
	}

	return nil
}

// GetUserStats получает статистику пользователя из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error) {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var stats *models.UserStats
	var err error

	// Пытаемся получить данные из кэша с быстрым таймаутом
	fastCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = r.healthChecker.WithRedisResilience(fastCtx, "get_stats_cache", func(ctx context.Context) error {
		stats, err = r.repo.GetUserStats(ctx, userID)
		return err
	})

	return stats, err
}

// SetGeoSearchResults кэширует результаты геопоиска с отказоустойчивостью
func (r *ResilientCacheRepository) SetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64, users []models.User) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "set_geo_search_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_geo_search_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetGeoSearchResults(ctx, lat, lon, radiusKm, users)
		})
	})

	// Игнорируем ошибки кэша
	if err != nil {
		r.logger.Warn("Failed to cache geo search results",
			zap.Error(err),
			zap.Float64("lat", lat),
			zap.Float64("lon", lon),
			zap.Float64("radius", radiusKm))
		return nil
	}

	return nil
}

// GetGeoSearchResults получает результаты геопоиска из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64) ([]models.User, error) {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var users []models.User
	var err error

	// Пытаемся получить данные из кэша с быстрым таймаутом
	fastCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = r.healthChecker.WithRedisResilience(fastCtx, "get_geo_search_cache", func(ctx context.Context) error {
		users, err = r.repo.GetGeoSearchResults(ctx, lat, lon, radiusKm)
		return err
	})

	return users, err
}

// SetNotificationSettings кэширует настройки уведомлений с отказоустойчивостью
func (r *ResilientCacheRepository) SetNotificationSettings(ctx context.Context, settings *models.UserNotificationSetting) error {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	err := r.healthChecker.WithRedisResilience(ctx, "set_notification_settings_cache", func(ctx context.Context) error {
		return database.SafeRedisOperation(ctx, r.client, r.logger, "set_notification_settings_cache", func(ctx context.Context, client *redis.Client) error {
			return r.repo.SetNotificationSettings(ctx, settings)
		})
	})

	// Игнорируем ошибки кэша
	if err != nil {
		r.logger.Warn("Failed to cache notification settings",
			zap.Error(err),
			zap.Uint("user_id", settings.UserID))
		return nil
	}

	return nil
}

// GetNotificationSettings получает настройки уведомлений из кэша с отказоустойчивостью
func (r *ResilientCacheRepository) GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error) {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
	}

	var settings *models.UserNotificationSetting
	var err error

	// Пытаемся получить данные из кэша с быстрым таймаутом
	fastCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = r.healthChecker.WithRedisResilience(fastCtx, "get_notification_settings_cache", func(ctx context.Context) error {
		settings, err = r.repo.GetNotificationSettings(ctx, userID)
		return err
	})

	return settings, err
}
