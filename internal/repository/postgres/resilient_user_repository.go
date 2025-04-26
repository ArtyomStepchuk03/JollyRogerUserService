package postgres

import (
	"JollyRogerUserService/pkg/server"
	"context"
	"github.com/redis/go-redis/v9"
	"time"

	"JollyRogerUserService/internal/models"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/resilience"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ResilientUserRepository добавляет механизмы отказоустойчивости к репозиторию пользователей
type ResilientUserRepository struct {
	db            *gorm.DB
	repo          *UserRepository
	logger        *zap.Logger
	healthChecker *database.HealthChecker
}

// NewResilientUserRepository создает новый экземпляр отказоустойчивого репозитория
func NewResilientUserRepository(db *gorm.DB, redisClient *redis.Client, logger *zap.Logger) *ResilientUserRepository {
	baseRepo := NewUserRepository(db)
	healthChecker := database.NewDatabaseHealthChecker(db, redisClient, logger)

	return &ResilientUserRepository{
		db:            db,
		repo:          baseRepo,
		logger:        logger,
		healthChecker: healthChecker,
	}
}

// Create создает нового пользователя с отказоустойчивостью
func (r *ResilientUserRepository) Create(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "create_user", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "create_user", func(tx *gorm.DB) error {
			return r.repo.Create(user)
		})
	})
}

// GetByID получает пользователя по ID с отказоустойчивостью
func (r *ResilientUserRepository) GetByID(id uint) (*models.User, error) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user *models.User
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_user_by_id", func(ctx context.Context) error {
		// Используем механизм повторных попыток для этой операции
		retryOptions := resilience.DefaultRetryOptions()
		retryOptions.MaxRetries = 2

		return resilience.WithRetry(ctx, r.logger, "get_user_by_id", retryOptions, func(ctx context.Context) error {
			user, err = r.repo.GetByID(id)
			return err
		})
	})

	// Записываем метрику
	server.RecordDBOperation("get_user_by_id", time.Since(startTime), err)

	return user, err
}

// GetByTelegramID получает пользователя по Telegram ID с отказоустойчивостью
func (r *ResilientUserRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user *models.User
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_user_by_telegram_id", func(ctx context.Context) error {
		user, err = r.repo.GetByTelegramID(telegramID)
		return err
	})

	return user, err
}

// Update обновляет пользователя с отказоустойчивостью
func (r *ResilientUserRepository) Update(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_user", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_user", func(tx *gorm.DB) error {
			return r.repo.Update(user)
		})
	})
}

// GetUserWithPreferences получает пользователя с предпочтениями с отказоустойчивостью
func (r *ResilientUserRepository) GetUserWithPreferences(userID uint) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user *models.User
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_user_with_preferences", func(ctx context.Context) error {
		user, err = r.repo.GetUserWithPreferences(userID)
		return err
	})

	return user, err
}

// AddPreference добавляет предпочтение с отказоустойчивостью
func (r *ResilientUserRepository) AddPreference(preference *models.UserPreference) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "add_preference", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "add_preference", func(tx *gorm.DB) error {
			return r.repo.AddPreference(preference)
		})
	})
}

// RemovePreference удаляет предпочтение с отказоустойчивостью
func (r *ResilientUserRepository) RemovePreference(userID uint, tagID uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "remove_preference", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "remove_preference", func(tx *gorm.DB) error {
			return r.repo.RemovePreference(userID, tagID)
		})
	})
}

// GetPreferences получает предпочтения с отказоустойчивостью
func (r *ResilientUserRepository) GetPreferences(userID uint) ([]models.UserPreference, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var preferences []models.UserPreference
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_preferences", func(ctx context.Context) error {
		preferences, err = r.repo.GetPreferences(userID)
		return err
	})

	return preferences, err
}

// UpdateLocation обновляет местоположение с отказоустойчивостью
func (r *ResilientUserRepository) UpdateLocation(location *models.UserLocation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_location", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_location", func(tx *gorm.DB) error {
			return r.repo.UpdateLocation(location)
		})
	})
}

// GetLocation получает местоположение с отказоустойчивостью
func (r *ResilientUserRepository) GetLocation(userID uint) (*models.UserLocation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var location *models.UserLocation
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_location", func(ctx context.Context) error {
		location, err = r.repo.GetLocation(userID)
		return err
	})

	return location, err
}

// FindNearbyUsers находит пользователей поблизости с отказоустойчивостью
func (r *ResilientUserRepository) FindNearbyUsers(lat, lon float64, radiusKm float64, limit int) ([]models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var users []models.User
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "find_nearby_users", func(ctx context.Context) error {
		users, err = r.repo.FindNearbyUsers(lat, lon, radiusKm, limit)
		return err
	})

	return users, err
}

// GetStats получает статистику пользователя с отказоустойчивостью
func (r *ResilientUserRepository) GetStats(userID uint) (*models.UserStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var stats *models.UserStats
	var err error

	err = r.healthChecker.WithDatabaseResilience(ctx, "get_stats", func(ctx context.Context) error {
		stats, err = r.repo.GetStats(userID)
		return err
	})

	return stats, err
}

// UpdateStats обновляет статистику пользователя с отказоустойчивостью
func (r *ResilientUserRepository) UpdateStats(stats *models.UserStats) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_stats", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_stats", func(tx *gorm.DB) error {
			return r.repo.UpdateStats(stats)
		})
	})
}

// UpdateUserRating обновляет рейтинг пользователя с отказоустойчивостью
func (r *ResilientUserRepository) UpdateUserRating(userID uint, ratingChange float32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_user_rating", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_user_rating", func(tx *gorm.DB) error {
			return r.repo.UpdateUserRating(userID, ratingChange)
		})
	})
}

// UpdateLastActive обновляет время последней активности пользователя с отказоустойчивостью
func (r *ResilientUserRepository) UpdateLastActive(userID uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_last_active", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_last_active", func(tx *gorm.DB) error {
			return r.repo.UpdateLastActive(userID)
		})
	})
}

// GetNotificationSettings получает настройки уведомлений с отказоустойчивостью
func (r *ResilientUserRepository) GetNotificationSettings(userID uint) (*models.UserNotificationSetting, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var settings *models.UserNotificationSetting
	var err error

	// Add retry logic here using WithRetry to handle temporary errors
	retryOptions := resilience.RetryOptions{
		MaxRetries:     2,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  1.5,
		Jitter:         0.1,
	}

	err = resilience.WithRetry(ctx, r.logger, "get_notification_settings", retryOptions, func(ctx context.Context) error {
		var opErr error
		settings, opErr = r.repo.GetNotificationSettings(userID)
		return opErr
	})

	return settings, err
}

// UpdateNotificationSettings обновляет настройки уведомлений с отказоустойчивостью
func (r *ResilientUserRepository) UpdateNotificationSettings(settings *models.UserNotificationSetting) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.healthChecker.WithDatabaseResilience(ctx, "update_notification_settings", func(ctx context.Context) error {
		return database.SafeDBOperation(ctx, r.db, r.logger, "update_notification_settings", func(tx *gorm.DB) error {
			return r.repo.UpdateNotificationSettings(settings)
		})
	})
}
