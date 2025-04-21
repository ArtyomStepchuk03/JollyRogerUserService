package postgres

import (
	"JollyRogerUserService/internal/models"
	"errors"
	"gorm.io/gorm"
	"time"
)

// UserRepository представляет репозиторий для работы с пользователями
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository создает новый экземпляр UserRepository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// Create создает нового пользователя
func (r *UserRepository) Create(user *models.User) error {
	// Создаем пользователя в транзакции вместе со всеми связанными сущностями
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Проверяем, существует ли пользователь с данным telegram_id
		var existingUser models.User
		result := tx.Where("telegram_id = ?", user.TelegramID).First(&existingUser)
		if result.Error == nil {
			return errors.New("user with this telegram_id already exists")
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		// Создаем пользователя
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Создаем статистику пользователя
		stats := models.UserStats{
			UserID:    user.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := tx.Create(&stats).Error; err != nil {
			return err
		}

		// Создаем настройки уведомлений по умолчанию
		notificationSettings := models.UserNotificationSetting{
			UserID:               user.ID,
			NewEventNotification: true,
		}
		if err := tx.Create(&notificationSettings).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetByID получает пользователя по ID
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByTelegramID получает пользователя по Telegram ID
func (r *UserRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	var user models.User
	if err := r.db.Where("telegram_id = ?", telegramID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update обновляет пользователя
func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// GetUserWithPreferences получает пользователя с предпочтениями
func (r *UserRepository) GetUserWithPreferences(userID uint) (*models.User, error) {
	var user models.User
	if err := r.db.Preload("Preferences").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// AddPreference добавляет предпочтение пользователю
func (r *UserRepository) AddPreference(userPreference *models.UserPreference) error {
	// Проверяем, существует ли уже такое предпочтение
	var existing models.UserPreference
	result := r.db.Where("user_id = ? AND tag_id = ?", userPreference.UserID, userPreference.TagID).First(&existing)
	if result.Error == nil {
		return errors.New("preference already exists")
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	return r.db.Create(userPreference).Error
}

// RemovePreference удаляет предпочтение пользователя
func (r *UserRepository) RemovePreference(userID uint, tagID uint) error {
	return r.db.Where("user_id = ? AND tag_id = ?", userID, tagID).Delete(&models.UserPreference{}).Error
}

// GetPreferences получает все предпочтения пользователя
func (r *UserRepository) GetPreferences(userID uint) ([]models.UserPreference, error) {
	var preferences []models.UserPreference
	if err := r.db.Where("user_id = ?", userID).Find(&preferences).Error; err != nil {
		return nil, err
	}
	return preferences, nil
}

// UpdateLocation обновляет местоположение пользователя
func (r *UserRepository) UpdateLocation(location *models.UserLocation) error {
	// Проверяем, существует ли уже местоположение для этого пользователя
	var existing models.UserLocation
	result := r.db.Where("user_id = ?", location.UserID).First(&existing)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Если местоположения нет, создаем новое
		return r.db.Create(location).Error
	} else if result.Error != nil {
		return result.Error
	}

	// Если местоположение существует, обновляем его
	existing.Latitude = location.Latitude
	existing.Longitude = location.Longitude
	existing.City = location.City
	existing.Region = location.Region
	existing.Country = location.Country
	existing.UpdatedAt = time.Now()

	return r.db.Save(&existing).Error
}

// GetLocation получает местоположение пользователя
func (r *UserRepository) GetLocation(userID uint) (*models.UserLocation, error) {
	var location models.UserLocation
	if err := r.db.Where("user_id = ?", userID).First(&location).Error; err != nil {
		return nil, err
	}
	return &location, nil
}

// FindNearbyUsers находит пользователей рядом с заданными координатами
func (r *UserRepository) FindNearbyUsers(lat, lon float64, radiusKm float64, limit int) ([]models.User, error) {
	// Для использования функций PostGIS можно использовать Raw SQL запрос
	// Это более эффективно, чем делать расчеты внутри Go
	var users []models.User

	// Запрос для поиска пользователей в заданном радиусе
	// Используем формулу гаверсинуса для расчета расстояния на сфере
	query := `
		SELECT u.* FROM users u
		JOIN user_locations l ON u.id = l.user_id
		WHERE (
			6371 * acos(
				cos(radians(?)) * cos(radians(l.latitude)) * 
				cos(radians(l.longitude) - radians(?)) + 
				sin(radians(?)) * sin(radians(l.latitude))
			)
		) <= ?
		LIMIT ?
	`

	if err := r.db.Raw(query, lat, lon, lat, radiusKm, limit).Scan(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

// GetStats получает статистику пользователя
func (r *UserRepository) GetStats(userID uint) (*models.UserStats, error) {
	var stats models.UserStats
	if err := r.db.Where("user_id = ?", userID).First(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// UpdateStats обновляет статистику пользователя
func (r *UserRepository) UpdateStats(stats *models.UserStats) error {
	return r.db.Save(stats).Error
}

// UpdateUserRating обновляет рейтинг пользователя
func (r *UserRepository) UpdateUserRating(userID uint, ratingChange float32) error {
	var user models.User
	if err := r.db.First(&user, userID).Error; err != nil {
		return err
	}

	user.Rating += ratingChange
	return r.db.Save(&user).Error
}

// UpdateLastActive обновляет время последней активности пользователя
func (r *UserRepository) UpdateLastActive(userID uint) error {
	var stats models.UserStats
	if err := r.db.Where("user_id = ?", userID).First(&stats).Error; err != nil {
		return err
	}

	now := time.Now()
	stats.LastActiveAt = &now
	stats.UpdatedAt = now

	return r.db.Save(&stats).Error
}

// GetNotificationSettings получает настройки уведомлений пользователя
func (r *UserRepository) GetNotificationSettings(userID uint) (*models.UserNotificationSetting, error) {
	var settings models.UserNotificationSetting
	if err := r.db.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

// UpdateNotificationSettings обновляет настройки уведомлений пользователя
func (r *UserRepository) UpdateNotificationSettings(settings *models.UserNotificationSetting) error {
	var existing models.UserNotificationSetting
	result := r.db.Where("user_id = ?", settings.UserID).First(&existing)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return r.db.Create(settings).Error
	} else if result.Error != nil {
		return result.Error
	}

	existing.NewEventNotification = settings.NewEventNotification
	return r.db.Save(&existing).Error
}
