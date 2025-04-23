package service

import (
	"JollyRogerUserService/internal/models"
	"context"
	"errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

// UserServiceInterface определяет интерфейс для сервиса пользователей
type UserServiceInterface interface {
	CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error)
	GetUser(ctx context.Context, id uint) (*models.User, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
	UpdateUser(ctx context.Context, id uint, username, bio string) (*models.User, error)
	AddUserPreference(ctx context.Context, userID, tagID uint) error
	RemoveUserPreference(ctx context.Context, userID, tagID uint) error
	GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error)
	UpdateUserLocation(ctx context.Context, req *models.UserLocationRequest) error
	GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error)
	FindNearbyUsers(ctx context.Context, lat, lon float64, radiusKm float64, limit int) ([]models.User, error)
	GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error)
	UpdateUserRating(ctx context.Context, userID uint, ratingChange float32) (*models.User, error)
	GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error)
	UpdateNotificationSettings(ctx context.Context, req *models.UpdateNotificationSettingRequest) error
}

// UserRepositoryInterface описывает интерфейс для работы с репозиторием пользователей
type UserRepositoryInterface interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByTelegramID(telegramID int64) (*models.User, error)
	Update(user *models.User) error
	GetUserWithPreferences(userID uint) (*models.User, error)
	AddPreference(preference *models.UserPreference) error
	RemovePreference(userID uint, tagID uint) error
	GetPreferences(userID uint) ([]models.UserPreference, error)
	UpdateLocation(location *models.UserLocation) error
	GetLocation(userID uint) (*models.UserLocation, error)
	FindNearbyUsers(lat, lon float64, radiusKm float64, limit int) ([]models.User, error)
	GetStats(userID uint) (*models.UserStats, error)
	UpdateStats(stats *models.UserStats) error
	UpdateUserRating(userID uint, ratingChange float32) error
	UpdateLastActive(userID uint) error
	GetNotificationSettings(userID uint) (*models.UserNotificationSetting, error)
	UpdateNotificationSettings(settings *models.UserNotificationSetting) error
}

// CacheRepositoryInterface описывает интерфейс для работы с кэшем
type CacheRepositoryInterface interface {
	SetUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, id uint) (*models.User, error)
	DeleteUser(ctx context.Context, id uint) error
	SetUserLocation(ctx context.Context, location *models.UserLocation) error
	GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error)
	SetUserPreferences(ctx context.Context, userID uint, preferences []models.UserPreference) error
	GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error)
	SetUserStats(ctx context.Context, stats *models.UserStats) error
	GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error)
	SetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64, users []models.User) error
	GetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64) ([]models.User, error)
	SetNotificationSettings(ctx context.Context, settings *models.UserNotificationSetting) error
	GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error)
	ClearUserCache(ctx context.Context, userID uint) error
}

// UserService представляет сервис для работы с пользователями
type UserService struct {
	userRepo  UserRepositoryInterface
	cacheRepo CacheRepositoryInterface
	logger    *zap.Logger
}

// NewUserService создает новый экземпляр UserService
func NewUserService(userRepo UserRepositoryInterface, cacheRepo CacheRepositoryInterface, logger *zap.Logger) *UserService {
	return &UserService{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		logger:    logger,
	}
}

// CreateUser создает нового пользователя
func (s *UserService) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	// Создаем модель пользователя
	user := &models.User{
		TelegramID: req.TelegramID,
		Username:   req.Username,
		Bio:        req.Bio,
		Rating:     0,
	}

	// Сохраняем пользователя в БД
	if err := s.userRepo.Create(user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err), zap.Int64("telegram_id", req.TelegramID))
		return nil, err
	}

	// Кэшируем пользователя
	if err := s.cacheRepo.SetUser(ctx, user); err != nil {
		s.logger.Warn("Failed to cache user", zap.Error(err), zap.Uint("user_id", user.ID))
	}

	s.logger.Info("User created", zap.Uint("user_id", user.ID), zap.Int64("telegram_id", user.TelegramID))
	return user, nil
}

// GetUser получает пользователя по ID
func (s *UserService) GetUser(ctx context.Context, id uint) (*models.User, error) {
	// Пытаемся получить пользователя из кэша
	user, err := s.cacheRepo.GetUser(ctx, id)
	if err == nil {
		s.logger.Debug("User retrieved from cache", zap.Uint("user_id", id))
		return user, nil
	}

	// Если не удалось, получаем из БД
	user, err = s.userRepo.GetByID(id)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err), zap.Uint("user_id", id))
		return nil, err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(id); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", id))
	}

	// Кэшируем пользователя
	if err := s.cacheRepo.SetUser(ctx, user); err != nil {
		s.logger.Warn("Failed to cache user", zap.Error(err), zap.Uint("user_id", id))
	}

	return user, nil
}

// GetUserByTelegramID получает пользователя по Telegram ID
func (s *UserService) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	// Получаем пользователя из БД по telegram_id
	user, err := s.userRepo.GetByTelegramID(telegramID)
	if err != nil {
		s.logger.Error("Failed to get user by telegram_id", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(user.ID); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", user.ID))
	}

	// Кэшируем пользователя
	if err := s.cacheRepo.SetUser(ctx, user); err != nil {
		s.logger.Warn("Failed to cache user", zap.Error(err), zap.Uint("user_id", user.ID))
	}

	return user, nil
}

// UpdateUser обновляет пользователя
func (s *UserService) UpdateUser(ctx context.Context, id uint, username, bio string) (*models.User, error) {
	// Получаем пользователя из БД
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		s.logger.Error("Failed to get user for update", zap.Error(err), zap.Uint("user_id", id))
		return nil, err
	}

	// Обновляем поля
	user.Username = username
	user.Bio = bio

	// Сохраняем изменения в БД
	if err := s.userRepo.Update(user); err != nil {
		s.logger.Error("Failed to update user", zap.Error(err), zap.Uint("user_id", id))
		return nil, err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(id); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", id))
	}

	// Очищаем кэш пользователя
	if err := s.cacheRepo.ClearUserCache(ctx, id); err != nil {
		s.logger.Warn("Failed to clear user cache", zap.Error(err), zap.Uint("user_id", id))
	}

	// Кэшируем обновленного пользователя
	if err := s.cacheRepo.SetUser(ctx, user); err != nil {
		s.logger.Warn("Failed to cache updated user", zap.Error(err), zap.Uint("user_id", id))
	}

	s.logger.Info("User updated", zap.Uint("user_id", id))
	return user, nil
}

// AddUserPreference добавляет предпочтение пользователю
func (s *UserService) AddUserPreference(ctx context.Context, userID, tagID uint) error {
	// Создаем объект предпочтения
	preference := &models.UserPreference{
		UserID:    userID,
		TagID:     tagID,
		CreatedAt: time.Now(),
	}

	// Добавляем предпочтение в БД
	if err := s.userRepo.AddPreference(preference); err != nil {
		s.logger.Error("Failed to add preference",
			zap.Error(err),
			zap.Uint("user_id", userID),
			zap.Uint("tag_id", tagID))
		return err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(userID); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", userID))
	}

	// Очищаем кэш предпочтений пользователя
	// Полностью очищаем кэш пользователя, так как предпочтения могут быть частью профиля
	if err := s.cacheRepo.ClearUserCache(ctx, userID); err != nil {
		s.logger.Warn("Failed to clear user cache", zap.Error(err), zap.Uint("user_id", userID))
	}

	// Получаем обновленный список предпочтений
	preferences, err := s.userRepo.GetPreferences(userID)
	if err != nil {
		s.logger.Warn("Failed to get updated preferences", zap.Error(err), zap.Uint("user_id", userID))
	} else {
		// Кэшируем обновленный список предпочтений
		if err := s.cacheRepo.SetUserPreferences(ctx, userID, preferences); err != nil {
			s.logger.Warn("Failed to cache preferences", zap.Error(err), zap.Uint("user_id", userID))
		}
	}

	s.logger.Info("Preference added", zap.Uint("user_id", userID), zap.Uint("tag_id", tagID))
	return nil
}

// RemoveUserPreference удаляет предпочтение пользователя
func (s *UserService) RemoveUserPreference(ctx context.Context, userID, tagID uint) error {
	// Удаляем предпочтение из БД
	if err := s.userRepo.RemovePreference(userID, tagID); err != nil {
		s.logger.Error("Failed to remove preference",
			zap.Error(err),
			zap.Uint("user_id", userID),
			zap.Uint("tag_id", tagID))
		return err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(userID); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", userID))
	}

	// Очищаем кэш предпочтений пользователя
	if err := s.cacheRepo.ClearUserCache(ctx, userID); err != nil {
		s.logger.Warn("Failed to clear user cache", zap.Error(err), zap.Uint("user_id", userID))
	}

	// Получаем обновленный список предпочтений
	preferences, err := s.userRepo.GetPreferences(userID)
	if err != nil {
		s.logger.Warn("Failed to get updated preferences", zap.Error(err), zap.Uint("user_id", userID))
	} else {
		// Кэшируем обновленный список предпочтений
		if err := s.cacheRepo.SetUserPreferences(ctx, userID, preferences); err != nil {
			s.logger.Warn("Failed to cache preferences", zap.Error(err), zap.Uint("user_id", userID))
		}
	}

	s.logger.Info("Preference removed", zap.Uint("user_id", userID), zap.Uint("tag_id", tagID))
	return nil
}

// GetUserPreferences получает все предпочтения пользователя
func (s *UserService) GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error) {
	// Пытаемся получить предпочтения из кэша
	preferences, err := s.cacheRepo.GetUserPreferences(ctx, userID)
	if err == nil {
		s.logger.Debug("Preferences retrieved from cache", zap.Uint("user_id", userID))
		return preferences, nil
	}

	// Если не удалось, получаем из БД
	preferences, err = s.userRepo.GetPreferences(userID)
	if err != nil {
		s.logger.Error("Failed to get preferences", zap.Error(err), zap.Uint("user_id", userID))
		return nil, err
	}

	// Кэшируем предпочтения
	if err := s.cacheRepo.SetUserPreferences(ctx, userID, preferences); err != nil {
		s.logger.Warn("Failed to cache preferences", zap.Error(err), zap.Uint("user_id", userID))
	}

	return preferences, nil
}

// UpdateUserLocation обновляет местоположение пользователя
func (s *UserService) UpdateUserLocation(ctx context.Context, req *models.UserLocationRequest) error {
	// Создаем объект местоположения
	location := &models.UserLocation{
		UserID:    req.UserID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		City:      req.City,
		Region:    req.Region,
		Country:   req.Country,
		UpdatedAt: time.Now(),
	}

	// Обновляем местоположение в БД
	if err := s.userRepo.UpdateLocation(location); err != nil {
		s.logger.Error("Failed to update location", zap.Error(err), zap.Uint("user_id", req.UserID))
		return err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(req.UserID); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", req.UserID))
	}

	// Кэшируем местоположение
	if err := s.cacheRepo.SetUserLocation(ctx, location); err != nil {
		s.logger.Warn("Failed to cache location", zap.Error(err), zap.Uint("user_id", req.UserID))
	}

	s.logger.Info("Location updated",
		zap.Uint("user_id", req.UserID),
		zap.Float64("lat", req.Latitude),
		zap.Float64("lon", req.Longitude))
	return nil
}

// GetUserLocation получает местоположение пользователя
func (s *UserService) GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error) {
	// Пытаемся получить местоположение из кэша
	location, err := s.cacheRepo.GetUserLocation(ctx, userID)
	if err == nil {
		s.logger.Debug("Location retrieved from cache", zap.Uint("user_id", userID))
		return location, nil
	}

	// Если не удалось, получаем из БД
	location, err = s.userRepo.GetLocation(userID)
	if err != nil {
		s.logger.Error("Failed to get location", zap.Error(err), zap.Uint("user_id", userID))
		return nil, err
	}

	// Кэшируем местоположение
	if err := s.cacheRepo.SetUserLocation(ctx, location); err != nil {
		s.logger.Warn("Failed to cache location", zap.Error(err), zap.Uint("user_id", userID))
	}

	return location, nil
}

// FindNearbyUsers находит пользователей рядом с заданными координатами
func (s *UserService) FindNearbyUsers(ctx context.Context, lat, lon float64, radiusKm float64, limit int) ([]models.User, error) {
	// Пытаемся получить результаты поиска из кэша
	users, err := s.cacheRepo.GetGeoSearchResults(ctx, lat, lon, radiusKm)
	if err == nil && len(users) > 0 {
		s.logger.Debug("Geo search results retrieved from cache",
			zap.Float64("lat", lat),
			zap.Float64("lon", lon),
			zap.Float64("radius", radiusKm))

		// Если в кэше больше результатов, чем запрошено, ограничиваем
		if len(users) > limit {
			return users[:limit], nil
		}
		return users, nil
	}

	// Если не удалось, выполняем поиск в БД
	users, err = s.userRepo.FindNearbyUsers(lat, lon, radiusKm, limit)
	if err != nil {
		s.logger.Error("Failed to find nearby users",
			zap.Error(err),
			zap.Float64("lat", lat),
			zap.Float64("lon", lon),
			zap.Float64("radius", radiusKm))
		return nil, err
	}

	// Кэшируем результаты поиска
	if err := s.cacheRepo.SetGeoSearchResults(ctx, lat, lon, radiusKm, users); err != nil {
		s.logger.Warn("Failed to cache geo search results", zap.Error(err))
	}

	s.logger.Info("Found nearby users",
		zap.Int("count", len(users)),
		zap.Float64("lat", lat),
		zap.Float64("lon", lon),
		zap.Float64("radius", radiusKm))
	return users, nil
}

// GetUserStats получает статистику пользователя
func (s *UserService) GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error) {
	// Пытаемся получить статистику из кэша
	stats, err := s.cacheRepo.GetUserStats(ctx, userID)
	if err == nil {
		s.logger.Debug("Stats retrieved from cache", zap.Uint("user_id", userID))
		return stats, nil
	}

	// Если не удалось, получаем из БД
	stats, err = s.userRepo.GetStats(userID)
	if err != nil {
		s.logger.Error("Failed to get user stats", zap.Error(err), zap.Uint("user_id", userID))
		return nil, err
	}

	// Кэшируем статистику
	if err := s.cacheRepo.SetUserStats(ctx, stats); err != nil {
		s.logger.Warn("Failed to cache user stats", zap.Error(err), zap.Uint("user_id", userID))
	}

	return stats, nil
}

// UpdateUserRating обновляет рейтинг пользователя
func (s *UserService) UpdateUserRating(ctx context.Context, userID uint, ratingChange float32) (*models.User, error) {
	// Проверяем существование пользователя
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.logger.Error("Failed to get user for rating update", zap.Error(err), zap.Uint("user_id", userID))
		return nil, err
	}

	// Обновляем рейтинг в БД
	if err := s.userRepo.UpdateUserRating(userID, ratingChange); err != nil {
		s.logger.Error("Failed to update user rating",
			zap.Error(err),
			zap.Uint("user_id", userID),
			zap.Float32("rating_change", ratingChange))
		return nil, err
	}

	// Обновляем пользователя в переменной
	user.Rating += ratingChange

	// Очищаем кэш пользователя
	if err := s.cacheRepo.ClearUserCache(ctx, userID); err != nil {
		s.logger.Warn("Failed to clear user cache", zap.Error(err), zap.Uint("user_id", userID))
	}

	// Кэшируем обновленного пользователя
	if err := s.cacheRepo.SetUser(ctx, user); err != nil {
		s.logger.Warn("Failed to cache updated user", zap.Error(err), zap.Uint("user_id", userID))
	}

	s.logger.Info("User rating updated",
		zap.Uint("user_id", userID),
		zap.Float32("rating_change", ratingChange),
		zap.Float32("new_rating", user.Rating))
	return user, nil
}

// GetNotificationSettings получает настройки уведомлений пользователя
func (s *UserService) GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error) {
	// Пытаемся получить настройки из кэша
	settings, err := s.cacheRepo.GetNotificationSettings(ctx, userID)
	if err == nil {
		s.logger.Debug("Notification settings retrieved from cache", zap.Uint("user_id", userID))
		return settings, nil
	}

	// Если не удалось, получаем из БД
	settings, err = s.userRepo.GetNotificationSettings(userID)
	if err != nil {
		// Если настройки не найдены, возвращаем настройки по умолчанию
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = &models.UserNotificationSetting{
				UserID:               userID,
				NewEventNotification: true,
			}
			// Создаем настройки по умолчанию в БД
			if err := s.userRepo.UpdateNotificationSettings(settings); err != nil {
				s.logger.Error("Failed to create default notification settings", zap.Error(err), zap.Uint("user_id", userID))
				return nil, err
			}
		} else {
			s.logger.Error("Failed to get notification settings", zap.Error(err), zap.Uint("user_id", userID))
			return nil, err
		}
	}

	// Кэшируем настройки
	if err := s.cacheRepo.SetNotificationSettings(ctx, settings); err != nil {
		s.logger.Warn("Failed to cache notification settings", zap.Error(err), zap.Uint("user_id", userID))
	}

	return settings, nil
}

// UpdateNotificationSettings обновляет настройки уведомлений пользователя
func (s *UserService) UpdateNotificationSettings(ctx context.Context, req *models.UpdateNotificationSettingRequest) error {
	// Создаем объект настроек
	settings := &models.UserNotificationSetting{
		UserID:               req.UserID,
		NewEventNotification: req.NewEventNotification,
	}

	// Обновляем настройки в БД
	if err := s.userRepo.UpdateNotificationSettings(settings); err != nil {
		s.logger.Error("Failed to update notification settings", zap.Error(err), zap.Uint("user_id", req.UserID))
		return err
	}

	// Обновляем время последней активности
	if err := s.userRepo.UpdateLastActive(req.UserID); err != nil {
		s.logger.Warn("Failed to update last active time", zap.Error(err), zap.Uint("user_id", req.UserID))
	}

	// Кэшируем настройки
	if err := s.cacheRepo.SetNotificationSettings(ctx, settings); err != nil {
		s.logger.Warn("Failed to cache notification settings", zap.Error(err), zap.Uint("user_id", req.UserID))
	}

	s.logger.Info("Notification settings updated",
		zap.Uint("user_id", req.UserID),
		zap.Bool("new_event_notification", req.NewEventNotification))
	return nil
}
