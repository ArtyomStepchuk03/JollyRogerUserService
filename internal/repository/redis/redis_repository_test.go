package redis

import (
	"JollyRogerUserService/internal/models"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis создает мини-Redis сервер для тестирования
func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		client.Close()
		mr.Close()
	}
}

// TestSetAndGetUser тестирует методы SetUser и GetUser
func TestSetAndGetUser(t *testing.T) {
	// Настраиваем тестовый Redis
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	// Создаем репозиторий
	repo := NewCacheRepository(client)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тестовый пользователь
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     4.5,
	}

	// Сохраняем пользователя в кэше
	err := repo.SetUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to set user in cache: %v", err)
	}

	// Получаем пользователя из кэша
	cachedUser, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user from cache: %v", err)
	}

	// Проверяем, что данные соответствуют
	if cachedUser.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, cachedUser.ID)
	}
	if cachedUser.TelegramID != user.TelegramID {
		t.Errorf("Expected TelegramID %d, got %d", user.TelegramID, cachedUser.TelegramID)
	}
	if cachedUser.Username != user.Username {
		t.Errorf("Expected Username %s, got %s", user.Username, cachedUser.Username)
	}
	if cachedUser.Bio != user.Bio {
		t.Errorf("Expected Bio %s, got %s", user.Bio, cachedUser.Bio)
	}
	if cachedUser.Rating != user.Rating {
		t.Errorf("Expected Rating %f, got %f", user.Rating, cachedUser.Rating)
	}
}

// TestDeleteUser тестирует метод DeleteUser
func TestDeleteUser(t *testing.T) {
	// Настраиваем тестовый Redis
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	// Создаем репозиторий
	repo := NewCacheRepository(client)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тестовый пользователь
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     4.5,
	}

	// Сохраняем пользователя в кэше
	err := repo.SetUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to set user in cache: %v", err)
	}

	// Удаляем пользователя из кэша
	err = repo.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to delete user from cache: %v", err)
	}

	// Пытаемся получить пользователя из кэша
	_, err = repo.GetUser(ctx, user.ID)
	if err == nil {
		t.Fatalf("Expected error when getting deleted user, got nil")
	}
}

// TestSetAndGetUserLocation тестирует методы SetUserLocation и GetUserLocation
func TestSetAndGetUserLocation(t *testing.T) {
	// Настраиваем тестовый Redis
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	// Создаем репозиторий
	repo := NewCacheRepository(client)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тестовое местоположение
	location := &models.UserLocation{
		ID:        1,
		UserID:    1,
		Latitude:  55.7558,
		Longitude: 37.6173,
		City:      "Moscow",
		Region:    "Moscow",
		Country:   "Russia",
		UpdatedAt: time.Now().Truncate(time.Second), // Обрезаем до секунд для корректного сравнения
	}

	// Сохраняем местоположение в кэше
	err := repo.SetUserLocation(ctx, location)
	if err != nil {
		t.Fatalf("Failed to set user location in cache: %v", err)
	}

	// Получаем местоположение из кэша
	cachedLocation, err := repo.GetUserLocation(ctx, location.UserID)
	if err != nil {
		t.Fatalf("Failed to get user location from cache: %v", err)
	}

	// Проверяем, что данные соответствуют (исключая UpdatedAt, который может отличаться из-за сериализации)
	if cachedLocation.ID != location.ID {
		t.Errorf("Expected location ID %d, got %d", location.ID, cachedLocation.ID)
	}
	if cachedLocation.UserID != location.UserID {
		t.Errorf("Expected UserID %d, got %d", location.UserID, cachedLocation.UserID)
	}
	if cachedLocation.Latitude != location.Latitude {
		t.Errorf("Expected Latitude %f, got %f", location.Latitude, cachedLocation.Latitude)
	}
	if cachedLocation.Longitude != location.Longitude {
		t.Errorf("Expected Longitude %f, got %f", location.Longitude, cachedLocation.Longitude)
	}
	if cachedLocation.City != location.City {
		t.Errorf("Expected City %s, got %s", location.City, cachedLocation.City)
	}
}

// TestSetAndGetUserPreferences тестирует методы SetUserPreferences и GetUserPreferences
func TestSetAndGetUserPreferences(t *testing.T) {
	// Настраиваем тестовый Redis
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	// Создаем репозиторий
	repo := NewCacheRepository(client)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тестовые предпочтения
	userID := uint(1)
	preferences := []models.UserPreference{
		{
			ID:        1,
			UserID:    userID,
			TagID:     10,
			CreatedAt: time.Now().Truncate(time.Second),
		},
		{
			ID:        2,
			UserID:    userID,
			TagID:     20,
			CreatedAt: time.Now().Truncate(time.Second),
		},
	}

	// Сохраняем предпочтения в кэше
	err := repo.SetUserPreferences(ctx, userID, preferences)
	if err != nil {
		t.Fatalf("Failed to set user preferences in cache: %v", err)
	}

	// Получаем предпочтения из кэша
	cachedPreferences, err := repo.GetUserPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user preferences from cache: %v", err)
	}

	// Проверяем, что данные соответствуют
	if len(cachedPreferences) != len(preferences) {
		t.Errorf("Expected %d preferences, got %d", len(preferences), len(cachedPreferences))
	}

	for i, pref := range preferences {
		if cachedPreferences[i].ID != pref.ID {
			t.Errorf("Expected preference ID %d, got %d", pref.ID, cachedPreferences[i].ID)
		}
		if cachedPreferences[i].TagID != pref.TagID {
			t.Errorf("Expected TagID %d, got %d", pref.TagID, cachedPreferences[i].TagID)
		}
	}
}

// TestClearUserCache тестирует метод ClearUserCache
func TestClearUserCache(t *testing.T) {
	// Настраиваем тестовый Redis
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	// Создаем репозиторий
	repo := NewCacheRepository(client)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тестовый пользователь
	userID := uint(1)
	user := &models.User{
		ID:         userID,
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     4.5,
	}

	// Тестовое местоположение
	location := &models.UserLocation{
		UserID:    userID,
		Latitude:  55.7558,
		Longitude: 37.6173,
		City:      "Moscow",
	}

	// Тестовые настройки уведомлений
	settings := &models.UserNotificationSetting{
		UserID:               userID,
		NewEventNotification: true,
	}

	// Сохраняем данные в кэше
	err := repo.SetUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to set user in cache: %v", err)
	}

	err = repo.SetUserLocation(ctx, location)
	if err != nil {
		t.Fatalf("Failed to set user location in cache: %v", err)
	}

	err = repo.SetNotificationSettings(ctx, settings)
	if err != nil {
		t.Fatalf("Failed to set notification settings in cache: %v", err)
	}

	// Очищаем кэш пользователя
	err = repo.ClearUserCache(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to clear user cache: %v", err)
	}

	// Проверяем, что данные удалены
	_, err = repo.GetUser(ctx, userID)
	if err == nil {
		t.Errorf("Expected error when getting user after cache clear, got nil")
	}

	_, err = repo.GetUserLocation(ctx, userID)
	if err == nil {
		t.Errorf("Expected error when getting location after cache clear, got nil")
	}

	_, err = repo.GetNotificationSettings(ctx, userID)
	if err == nil {
		t.Errorf("Expected error when getting notification settings after cache clear, got nil")
	}
}
