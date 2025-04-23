package redis

import (
	"JollyRogerUserService/internal/models"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockDB struct {
	gormDB *gorm.DB
}

// TestResilientCacheRepository_WithRedisFailures тестирует отказоустойчивость при сбоях Redis
func TestResilientCacheRepository_WithRedisFailures(t *testing.T) {
	// Настройка тестового Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := zap.NewNop()

	// Создаем мок для базы данных
	db := &mockDB{gormDB: &gorm.DB{}}

	// Создаем отказоустойчивый репозиторий кэша
	repo := NewResilientCacheRepository(client, db.gormDB, logger)

	ctx := context.Background()

	// Тестовый пользователь
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     4.5,
	}

	// Тест 1: Успешное сохранение в кэш при работающем Redis
	t.Run("SetUserWithWorkingRedis", func(t *testing.T) {
		err := repo.SetUser(ctx, user)
		if err != nil {
			t.Fatalf("Failed to set user in cache: %v", err)
		}

		// Проверяем, что данные сохранены в Redis
		cachedUser, err := repo.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user from cache: %v", err)
		}

		if cachedUser.ID != user.ID || cachedUser.Username != user.Username {
			t.Errorf("Cached user data mismatch: expected %v, got %v", user, cachedUser)
		}
	})

	// Тест 2: Сохранение при отказе Redis
	t.Run("SetUserWithFailingRedis", func(t *testing.T) {
		// Останавливаем Redis
		mr.Close()

		// Операция должна завершиться без ошибки благодаря механизму отказоустойчивости
		err := repo.SetUser(ctx, user)
		if err != nil {
			t.Fatalf("Expected resilient behavior on Redis failure, got: %v", err)
		}
	})

	// Перезапускаем Redis для следующих тестов
	mr, err = miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to recreate mini redis: %v", err)
	}

	// Обновляем клиента Redis в репозитории
	newClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	repo = NewResilientCacheRepository(newClient, db.gormDB, logger)

	// Тест 3: Сохранение и восстановление после сбоя
	t.Run("RecoveryAfterFailure", func(t *testing.T) {
		// Останавливаем Redis
		mr.Close()

		// Попытка получить данные должна завершиться ошибкой
		_, err := repo.GetUser(ctx, user.ID)
		if err == nil {
			t.Fatal("Expected error when Redis is down, got nil")
		}

		// Перезапускаем Redis
		mr, err = miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to recreate mini redis: %v", err)
		}

		// Обновляем клиента Redis в репозитории
		newClient = redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})
		repo = NewResilientCacheRepository(newClient, db.gormDB, logger)

		// Сохраняем данные в восстановленный Redis
		err = repo.SetUser(ctx, user)
		if err != nil {
			t.Fatalf("Failed to set user after recovery: %v", err)
		}

		// Проверяем, что данные можно получить
		_, err = repo.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user after recovery: %v", err)
		}
	})
}

// TestResilientCacheRepository_CircuitBreaker тестирует работу circuit breaker в репозитории
func TestResilientCacheRepository_CircuitBreaker(t *testing.T) {
	// Создаем мок для базы данных
	db := &mockDB{gormDB: &gorm.DB{}}
	logger := zap.NewNop()

	// Создаем Redis-клиент, который будет всегда возвращать ошибку
	badClient := redis.NewClient(&redis.Options{
		Addr: "non.existent.host:6379", // Несуществующий хост
	})

	// Создаем репозиторий с неработающим Redis
	repo := NewResilientCacheRepository(badClient, db.gormDB, logger)

	ctx := context.Background()
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "testuser",
	}

	// Выполняем несколько запросов, чтобы сработал circuit breaker
	failureCount := 0
	maxAttempts := 10

	for i := 0; i < maxAttempts; i++ {
		err := repo.SetUser(ctx, user)
		if err != nil {
			failureCount++
		}
	}

	// Проверяем, что все операции завершились успешно (без ошибок) благодаря
	// отказоустойчивому дизайну, даже если Redis недоступен
	if failureCount > 0 {
		t.Errorf("Expected all operations to succeed with circuit breaker, got %d failures", failureCount)
	}

	// Для операций чтения должны быть ошибки, так как они не могут быть выполнены без Redis
	readFailures := 0
	for i := 0; i < 5; i++ {
		_, err := repo.GetUser(ctx, user.ID)
		if err != nil {
			readFailures++
		}
	}

	if readFailures != 5 {
		t.Errorf("Expected read operations to fail with circuit breaker, got %d failures out of 5", readFailures)
	}
}

// TestResilientCacheRepository_DataConsistency проверяет сохранение согласованности данных
func TestResilientCacheRepository_DataConsistency(t *testing.T) {
	// Настройка тестового Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := zap.NewNop()
	db := &mockDB{gormDB: &gorm.DB{}}
	repo := NewResilientCacheRepository(client, db.gormDB, logger)

	ctx := context.Background()

	// Тестовый пользователь
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     4.5,
	}

	// Сохраняем пользователя в кэш
	err = repo.SetUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to set user in cache: %v", err)
	}

	// Проверяем, что данные корректно сохранены
	cachedUser, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user from cache: %v", err)
	}

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

	// Изменяем пользователя и обновляем кэш
	updatedUser := *user
	updatedUser.Username = "updated_username"
	updatedUser.Bio = "Updated bio"

	err = repo.SetUser(ctx, &updatedUser)
	if err != nil {
		t.Fatalf("Failed to update user in cache: %v", err)
	}

	// Получаем обновленного пользователя из кэша
	cachedUpdatedUser, err := repo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user from cache: %v", err)
	}

	// Проверяем, что данные были обновлены
	if cachedUpdatedUser.Username != updatedUser.Username {
		t.Errorf("Expected updated Username %s, got %s", updatedUser.Username, cachedUpdatedUser.Username)
	}
	if cachedUpdatedUser.Bio != updatedUser.Bio {
		t.Errorf("Expected updated Bio %s, got %s", updatedUser.Bio, cachedUpdatedUser.Bio)
	}

	// Очищаем кэш пользователя
	err = repo.ClearUserCache(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to clear user cache: %v", err)
	}

	// Проверяем, что данные были удалены из кэша
	_, err = repo.GetUser(ctx, user.ID)
	if err == nil {
		t.Error("Expected error when getting user after cache clear, got nil")
	}
}

// TestResilientCacheRepository_CacheOperations тестирует различные операции с кэшем
func TestResilientCacheRepository_CacheOperations(t *testing.T) {
	// Настройка тестового Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := zap.NewNop()
	db := &mockDB{gormDB: &gorm.DB{}}
	repo := NewResilientCacheRepository(client, db.gormDB, logger)

	ctx := context.Background()
	userID := uint(1)

	// Тест 1: Работа с UserLocation
	t.Run("UserLocation", func(t *testing.T) {
		// Создаем тестовое местоположение
		location := &models.UserLocation{
			UserID:    userID,
			Latitude:  55.7558,
			Longitude: 37.6173,
			City:      "Moscow",
			Region:    "Moscow",
			Country:   "Russia",
			UpdatedAt: time.Now().Truncate(time.Second), // Обрезаем до секунд для корректного сравнения
		}

		// Сохраняем местоположение в кэш
		err := repo.SetUserLocation(ctx, location)
		if err != nil {
			t.Fatalf("Failed to set user location in cache: %v", err)
		}

		// Получаем местоположение из кэша
		cachedLocation, err := repo.GetUserLocation(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get user location from cache: %v", err)
		}

		// Проверяем, что данные корректно сохранены
		if cachedLocation.Latitude != location.Latitude {
			t.Errorf("Expected Latitude %f, got %f", location.Latitude, cachedLocation.Latitude)
		}
		if cachedLocation.City != location.City {
			t.Errorf("Expected City %s, got %s", location.City, cachedLocation.City)
		}
	})

	// Тест 2: Работа с UserPreferences
	t.Run("UserPreferences", func(t *testing.T) {
		// Создаем тестовые предпочтения
		preferences := []models.UserPreference{
			{
				UserID: userID,
				TagID:  10,
			},
			{
				UserID: userID,
				TagID:  20,
			},
		}

		// Сохраняем предпочтения в кэш
		err := repo.SetUserPreferences(ctx, userID, preferences)
		if err != nil {
			t.Fatalf("Failed to set user preferences in cache: %v", err)
		}

		// Получаем предпочтения из кэша
		cachedPreferences, err := repo.GetUserPreferences(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get user preferences from cache: %v", err)
		}

		// Проверяем, что данные корректно сохранены
		if len(cachedPreferences) != len(preferences) {
			t.Errorf("Expected %d preferences, got %d", len(preferences), len(cachedPreferences))
		} else {
			for i, pref := range preferences {
				if cachedPreferences[i].TagID != pref.TagID {
					t.Errorf("Expected TagID %d, got %d", pref.TagID, cachedPreferences[i].TagID)
				}
			}
		}
	})

	// Тест 3: Работа с UserStats
	t.Run("UserStats", func(t *testing.T) {
		// Создаем тестовую статистику
		now := time.Now().Truncate(time.Second)
		stats := &models.UserStats{
			UserID:             userID,
			EventsCreated:      5,
			EventsParticipated: 10,
			CreatedAt:          now,
			UpdatedAt:          now,
			LastActiveAt:       &now,
			IsActive:           true,
		}

		// Сохраняем статистику в кэш
		err := repo.SetUserStats(ctx, stats)
		if err != nil {
			t.Fatalf("Failed to set user stats in cache: %v", err)
		}

		// Получаем статистику из кэша
		cachedStats, err := repo.GetUserStats(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get user stats from cache: %v", err)
		}

		// Проверяем, что данные корректно сохранены
		if cachedStats.EventsCreated != stats.EventsCreated {
			t.Errorf("Expected EventsCreated %d, got %d", stats.EventsCreated, cachedStats.EventsCreated)
		}
		if cachedStats.EventsParticipated != stats.EventsParticipated {
			t.Errorf("Expected EventsParticipated %d, got %d", stats.EventsParticipated, cachedStats.EventsParticipated)
		}
		if !cachedStats.IsActive {
			t.Errorf("Expected IsActive to be true")
		}
	})

	// Тест 4: Работа с NotificationSettings
	t.Run("NotificationSettings", func(t *testing.T) {
		// Создаем тестовые настройки уведомлений
		settings := &models.UserNotificationSetting{
			UserID:               userID,
			NewEventNotification: true,
		}

		// Сохраняем настройки в кэш
		err := repo.SetNotificationSettings(ctx, settings)
		if err != nil {
			t.Fatalf("Failed to set notification settings in cache: %v", err)
		}

		// Получаем настройки из кэша
		cachedSettings, err := repo.GetNotificationSettings(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get notification settings from cache: %v", err)
		}

		// Проверяем, что данные корректно сохранены
		if cachedSettings.NewEventNotification != settings.NewEventNotification {
			t.Errorf("Expected NewEventNotification %v, got %v",
				settings.NewEventNotification, cachedSettings.NewEventNotification)
		}
	})

	// Тест 5: Работа с GeoSearchResults
	t.Run("GeoSearchResults", func(t *testing.T) {
		// Тестовые координаты
		lat := 55.7558
		lon := 37.6173
		radiusKm := 5.0

		// Создаем тестовых пользователей
		users := []models.User{
			{
				ID:         1,
				TelegramID: 123456789,
				Username:   "user1",
				Rating:     4.5,
			},
			{
				ID:         2,
				TelegramID: 987654321,
				Username:   "user2",
				Rating:     3.8,
			},
		}

		// Сохраняем результаты поиска в кэш
		err := repo.SetGeoSearchResults(ctx, lat, lon, radiusKm, users)
		if err != nil {
			t.Fatalf("Failed to set geo search results in cache: %v", err)
		}

		// Получаем результаты поиска из кэша
		cachedUsers, err := repo.GetGeoSearchResults(ctx, lat, lon, radiusKm)
		if err != nil {
			t.Fatalf("Failed to get geo search results from cache: %v", err)
		}

		// Проверяем, что данные корректно сохранены
		if len(cachedUsers) != len(users) {
			t.Errorf("Expected %d users, got %d", len(users), len(cachedUsers))
		} else {
			for i, user := range users {
				if cachedUsers[i].ID != user.ID {
					t.Errorf("Expected user ID %d, got %d", user.ID, cachedUsers[i].ID)
				}
				if cachedUsers[i].Username != user.Username {
					t.Errorf("Expected Username %s, got %s", user.Username, cachedUsers[i].Username)
				}
			}
		}
	})
}
