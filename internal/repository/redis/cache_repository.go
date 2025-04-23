package redis

import (
	"JollyRogerUserService/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

const (
	// TTL для разных типов кэша
	userProfileTTL     = 30 * time.Minute
	userLocationTTL    = 24 * time.Hour
	userPreferencesTTL = 1 * time.Hour
	userStatsTTL       = 12 * time.Hour
	geoSearchTTL       = 5 * time.Minute
)

// CacheRepository представляет репозиторий для работы с кэшем в Redis
type CacheRepository struct {
	client *redis.Client
}

// NewCacheRepository создает новый экземпляр CacheRepository
func NewCacheRepository(client *redis.Client) *CacheRepository {
	return &CacheRepository{
		client: client,
	}
}

// SetUser кэширует пользователя
func (r *CacheRepository) SetUser(ctx context.Context, user *models.User) error {
	key := fmt.Sprintf("user:%d:profile", user.ID)

	// Преобразуем пользователя в JSON
	userData, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, userData, userProfileTTL).Err()
}

// GetUser получает пользователя из кэша
func (r *CacheRepository) GetUser(ctx context.Context, id uint) (*models.User, error) {
	key := fmt.Sprintf("user:%d:profile", id)

	// Получаем данные из Redis
	userData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в пользователя
	var user models.User
	if err := json.Unmarshal(userData, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// DeleteUser удаляет пользователя из кэша
func (r *CacheRepository) DeleteUser(ctx context.Context, id uint) error {
	key := fmt.Sprintf("user:%d:profile", id)
	return r.client.Del(ctx, key).Err()
}

// SetUserLocation кэширует местоположение пользователя
func (r *CacheRepository) SetUserLocation(ctx context.Context, location *models.UserLocation) error {
	key := fmt.Sprintf("user:%d:location", location.UserID)

	// Преобразуем местоположение в JSON
	locationData, err := json.Marshal(location)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, locationData, userLocationTTL).Err()
}

// GetUserLocation получает местоположение пользователя из кэша
func (r *CacheRepository) GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error) {
	key := fmt.Sprintf("user:%d:location", userID)

	// Получаем данные из Redis
	locationData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в местоположение
	var location models.UserLocation
	if err := json.Unmarshal(locationData, &location); err != nil {
		return nil, err
	}

	return &location, nil
}

// SetUserPreferences кэширует предпочтения пользователя
func (r *CacheRepository) SetUserPreferences(ctx context.Context, userID uint, preferences []models.UserPreference) error {
	key := fmt.Sprintf("user:%d:preferences", userID)

	// Преобразуем предпочтения в JSON
	preferencesData, err := json.Marshal(preferences)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, preferencesData, userPreferencesTTL).Err()
}

// GetUserPreferences получает предпочтения пользователя из кэша
func (r *CacheRepository) GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error) {
	key := fmt.Sprintf("user:%d:preferences", userID)

	// Получаем данные из Redis
	preferencesData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в предпочтения
	var preferences []models.UserPreference
	if err := json.Unmarshal(preferencesData, &preferences); err != nil {
		return nil, err
	}

	return preferences, nil
}

// SetUserStats кэширует статистику пользователя
func (r *CacheRepository) SetUserStats(ctx context.Context, stats *models.UserStats) error {
	key := fmt.Sprintf("user:%d:stats", stats.UserID)

	// Преобразуем статистику в JSON
	statsData, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, statsData, userStatsTTL).Err()
}

// GetUserStats получает статистику пользователя из кэша
func (r *CacheRepository) GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error) {
	key := fmt.Sprintf("user:%d:stats", userID)

	// Получаем данные из Redis
	statsData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в статистику
	var stats models.UserStats
	if err := json.Unmarshal(statsData, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// SetGeoSearchResults кэширует результаты геопоиска
func (r *CacheRepository) SetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64, users []models.User) error {
	key := fmt.Sprintf("search:geo:%f:%f:%f", lat, lon, radiusKm)

	// Преобразуем пользователей в JSON
	usersData, err := json.Marshal(users)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, usersData, geoSearchTTL).Err()
}

// GetGeoSearchResults получает результаты геопоиска из кэша
func (r *CacheRepository) GetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64) ([]models.User, error) {
	key := fmt.Sprintf("search:geo:%f:%f:%f", lat, lon, radiusKm)

	// Получаем данные из Redis
	usersData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в пользователей
	var users []models.User
	if err := json.Unmarshal(usersData, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// SetNotificationSettings кэширует настройки уведомлений
func (r *CacheRepository) SetNotificationSettings(ctx context.Context, settings *models.UserNotificationSetting) error {
	key := fmt.Sprintf("user:%d:notifications", settings.UserID)

	// Преобразуем настройки в JSON
	settingsData, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	// Сохраняем в Redis с TTL
	return r.client.Set(ctx, key, settingsData, userProfileTTL).Err()
}

// GetNotificationSettings получает настройки уведомлений из кэша
func (r *CacheRepository) GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error) {
	key := fmt.Sprintf("user:%d:notifications", userID)

	// Получаем данные из Redis
	settingsData, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Преобразуем JSON в настройки
	var settings models.UserNotificationSetting
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// ClearCache очищает весь кэш пользователя
func (r *CacheRepository) ClearUserCache(ctx context.Context, userID uint) error {
	// Паттерн для поиска всех ключей, связанных с пользователем
	pattern := fmt.Sprintf("user:%d:*", userID)

	// Находим все ключи по паттерну
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	// Если ключи найдены, удаляем их
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}

	return nil
}
