package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"JollyRogerUserService/internal/models"
	"go.uber.org/zap"
)

// Мок для репозитория пользователей
type MockUserRepository struct {
	users         map[uint]*models.User
	usersByTgID   map[int64]*models.User
	preferences   map[uint][]models.UserPreference
	locations     map[uint]*models.UserLocation
	stats         map[uint]*models.UserStats
	notifications map[uint]*models.UserNotificationSetting
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:         make(map[uint]*models.User),
		usersByTgID:   make(map[int64]*models.User),
		preferences:   make(map[uint][]models.UserPreference),
		locations:     make(map[uint]*models.UserLocation),
		stats:         make(map[uint]*models.UserStats),
		notifications: make(map[uint]*models.UserNotificationSetting),
	}
}

func (m *MockUserRepository) Create(user *models.User) error {
	if _, exists := m.usersByTgID[user.TelegramID]; exists {
		return errors.New("user with this telegram_id already exists")
	}

	user.ID = uint(len(m.users) + 1)
	m.users[user.ID] = user
	m.usersByTgID[user.TelegramID] = user

	// Создаем статистику по умолчанию
	m.stats[user.ID] = &models.UserStats{
		UserID:    user.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Создаем настройки уведомлений по умолчанию
	m.notifications[user.ID] = &models.UserNotificationSetting{
		UserID:               user.ID,
		NewEventNotification: true,
	}

	return nil
}

func (m *MockUserRepository) GetByID(id uint) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *MockUserRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	user, exists := m.usersByTgID[telegramID]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *MockUserRepository) Update(user *models.User) error {
	if _, exists := m.users[user.ID]; !exists {
		return errors.New("user not found")
	}
	m.users[user.ID] = user
	m.usersByTgID[user.TelegramID] = user
	return nil
}

func (m *MockUserRepository) GetUserWithPreferences(userID uint) (*models.User, error) {
	user, err := m.GetByID(userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (m *MockUserRepository) AddPreference(preference *models.UserPreference) error {
	if _, exists := m.users[preference.UserID]; !exists {
		return errors.New("user not found")
	}

	// Проверяем, существует ли такое предпочтение
	prefs := m.preferences[preference.UserID]
	for _, p := range prefs {
		if p.TagID == preference.TagID {
			return errors.New("preference already exists")
		}
	}

	m.preferences[preference.UserID] = append(m.preferences[preference.UserID], *preference)
	return nil
}

func (m *MockUserRepository) RemovePreference(userID uint, tagID uint) error {
	if _, exists := m.users[userID]; !exists {
		return errors.New("user not found")
	}

	prefs := m.preferences[userID]
	for i, p := range prefs {
		if p.TagID == tagID {
			// Удаляем найденное предпочтение
			m.preferences[userID] = append(prefs[:i], prefs[i+1:]...)
			return nil
		}
	}

	return errors.New("preference not found")
}

func (m *MockUserRepository) GetPreferences(userID uint) ([]models.UserPreference, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	return m.preferences[userID], nil
}

func (m *MockUserRepository) UpdateLocation(location *models.UserLocation) error {
	if _, exists := m.users[location.UserID]; !exists {
		return errors.New("user not found")
	}

	m.locations[location.UserID] = location
	return nil
}

func (m *MockUserRepository) GetLocation(userID uint) (*models.UserLocation, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	location, exists := m.locations[userID]
	if !exists {
		return nil, errors.New("location not found")
	}

	return location, nil
}

func (m *MockUserRepository) FindNearbyUsers(lat, lon float64, radiusKm float64, limit int) ([]models.User, error) {
	// Упрощенная реализация для тестов
	var users []models.User
	for _, user := range m.users {
		users = append(users, *user)
		if len(users) >= limit {
			break
		}
	}
	return users, nil
}

func (m *MockUserRepository) GetStats(userID uint) (*models.UserStats, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	stats, exists := m.stats[userID]
	if !exists {
		return nil, errors.New("stats not found")
	}

	return stats, nil
}

func (m *MockUserRepository) UpdateStats(stats *models.UserStats) error {
	if _, exists := m.users[stats.UserID]; !exists {
		return errors.New("user not found")
	}

	m.stats[stats.UserID] = stats
	return nil
}

func (m *MockUserRepository) UpdateUserRating(userID uint, ratingChange float32) error {
	user, exists := m.users[userID]
	if !exists {
		return errors.New("user not found")
	}

	user.Rating += ratingChange
	return nil
}

func (m *MockUserRepository) UpdateLastActive(userID uint) error {
	if _, exists := m.users[userID]; !exists {
		return errors.New("user not found")
	}

	stats, exists := m.stats[userID]
	if !exists {
		return errors.New("stats not found")
	}

	now := time.Now()
	stats.LastActiveAt = &now
	stats.UpdatedAt = now
	return nil
}

func (m *MockUserRepository) GetNotificationSettings(userID uint) (*models.UserNotificationSetting, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	settings, exists := m.notifications[userID]
	if !exists {
		return nil, errors.New("notification settings not found")
	}

	return settings, nil
}

func (m *MockUserRepository) UpdateNotificationSettings(settings *models.UserNotificationSetting) error {
	if _, exists := m.users[settings.UserID]; !exists {
		return errors.New("user not found")
	}

	m.notifications[settings.UserID] = settings
	return nil
}

// Мок для кэш-репозитория
type MockCacheRepository struct {
	userCache         map[uint]*models.User
	locationCache     map[uint]*models.UserLocation
	preferencesCache  map[uint][]models.UserPreference
	statsCache        map[uint]*models.UserStats
	geoCache          map[string][]models.User
	notificationCache map[uint]*models.UserNotificationSetting
}

func NewMockCacheRepository() *MockCacheRepository {
	return &MockCacheRepository{
		userCache:         make(map[uint]*models.User),
		locationCache:     make(map[uint]*models.UserLocation),
		preferencesCache:  make(map[uint][]models.UserPreference),
		statsCache:        make(map[uint]*models.UserStats),
		geoCache:          make(map[string][]models.User),
		notificationCache: make(map[uint]*models.UserNotificationSetting),
	}
}

func (m *MockCacheRepository) SetUser(ctx context.Context, user *models.User) error {
	m.userCache[user.ID] = user
	return nil
}

func (m *MockCacheRepository) GetUser(ctx context.Context, id uint) (*models.User, error) {
	user, exists := m.userCache[id]
	if !exists {
		return nil, errors.New("user not found in cache")
	}
	return user, nil
}

func (m *MockCacheRepository) DeleteUser(ctx context.Context, id uint) error {
	delete(m.userCache, id)
	return nil
}

func (m *MockCacheRepository) SetUserLocation(ctx context.Context, location *models.UserLocation) error {
	m.locationCache[location.UserID] = location
	return nil
}

func (m *MockCacheRepository) GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error) {
	location, exists := m.locationCache[userID]
	if !exists {
		return nil, errors.New("location not found in cache")
	}
	return location, nil
}

func (m *MockCacheRepository) SetUserPreferences(ctx context.Context, userID uint, preferences []models.UserPreference) error {
	m.preferencesCache[userID] = preferences
	return nil
}

func (m *MockCacheRepository) GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error) {
	preferences, exists := m.preferencesCache[userID]
	if !exists {
		return nil, errors.New("preferences not found in cache")
	}
	return preferences, nil
}

func (m *MockCacheRepository) SetUserStats(ctx context.Context, stats *models.UserStats) error {
	m.statsCache[stats.UserID] = stats
	return nil
}

func (m *MockCacheRepository) GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error) {
	stats, exists := m.statsCache[userID]
	if !exists {
		return nil, errors.New("stats not found in cache")
	}
	return stats, nil
}

func (m *MockCacheRepository) SetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64, users []models.User) error {
	key := generateGeoKey(lat, lon, radiusKm)
	m.geoCache[key] = users
	return nil
}

func (m *MockCacheRepository) GetGeoSearchResults(ctx context.Context, lat, lon float64, radiusKm float64) ([]models.User, error) {
	key := generateGeoKey(lat, lon, radiusKm)
	users, exists := m.geoCache[key]
	if !exists {
		return nil, errors.New("geo search results not found in cache")
	}
	return users, nil
}

func (m *MockCacheRepository) SetNotificationSettings(ctx context.Context, settings *models.UserNotificationSetting) error {
	m.notificationCache[settings.UserID] = settings
	return nil
}

func (m *MockCacheRepository) GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error) {
	settings, exists := m.notificationCache[userID]
	if !exists {
		return nil, errors.New("notification settings not found in cache")
	}
	return settings, nil
}

func (m *MockCacheRepository) ClearUserCache(ctx context.Context, userID uint) error {
	delete(m.userCache, userID)
	delete(m.locationCache, userID)
	delete(m.preferencesCache, userID)
	delete(m.statsCache, userID)
	delete(m.notificationCache, userID)
	return nil
}

// Вспомогательная функция для генерации ключа кэша гео-поиска
func generateGeoKey(lat, lon, radius float64) string {
	return fmt.Sprintf("geo:%f:%f:%f", lat, lon, radius)
}

// Тесты

func TestCreateUser(t *testing.T) {
	// Инициализация моков и сервиса
	mockUserRepo := NewMockUserRepository()
	mockCacheRepo := NewMockCacheRepository()
	logger := zap.NewNop() // Используем nop-логгер для тестов
	service := NewUserService(mockUserRepo, mockCacheRepo, logger)

	ctx := context.Background()

	// Тест кейс: успешное создание пользователя
	t.Run("Success", func(t *testing.T) {
		req := &models.CreateUserRequest{
			TelegramID: 123456789,
			Username:   "testuser",
			Bio:        "Test bio",
		}

		user, err := service.CreateUser(ctx, req)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if user.TelegramID != req.TelegramID {
			t.Errorf("Expected TelegramID %d, got %d", req.TelegramID, user.TelegramID)
		}

		if user.Username != req.Username {
			t.Errorf("Expected Username %s, got %s", req.Username, user.Username)
		}

		if user.Bio != req.Bio {
			t.Errorf("Expected Bio %s, got %s", req.Bio, user.Bio)
		}

		// Проверяем, что пользователь сохранен в репозитории
		savedUser, err := mockUserRepo.GetByTelegramID(req.TelegramID)
		if err != nil {
			t.Fatalf("Expected user to be saved, got error: %v", err)
		}

		if savedUser.Username != req.Username {
			t.Errorf("Expected saved Username %s, got %s", req.Username, savedUser.Username)
		}

		// Проверяем, что пользователь сохранен в кэше
		cachedUser, err := mockCacheRepo.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Expected user to be cached, got error: %v", err)
		}

		if cachedUser.TelegramID != req.TelegramID {
			t.Errorf("Expected cached TelegramID %d, got %d", req.TelegramID, cachedUser.TelegramID)
		}
	})

	// Тест кейс: попытка создать пользователя с существующим TelegramID
	t.Run("DuplicateTelegramID", func(t *testing.T) {
		req := &models.CreateUserRequest{
			TelegramID: 123456789, // Тот же TelegramID, что и в предыдущем тесте
			Username:   "anotheruser",
			Bio:        "Another bio",
		}

		_, err := service.CreateUser(ctx, req)

		if err == nil {
			t.Fatalf("Expected error for duplicate TelegramID, got nil")
		}
	})
}

func TestGetUser(t *testing.T) {
	// Инициализация моков и сервиса
	mockUserRepo := NewMockUserRepository()
	mockCacheRepo := NewMockCacheRepository()
	logger := zap.NewNop()
	service := NewUserService(mockUserRepo, mockCacheRepo, logger)

	ctx := context.Background()

	// Создаем тестового пользователя
	user := &models.User{
		TelegramID: 987654321,
		Username:   "getuser",
		Bio:        "Get user test",
		Rating:     0,
	}

	err := mockUserRepo.Create(user)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Тест кейс: получение пользователя из БД (не из кэша)
	t.Run("GetFromDB", func(t *testing.T) {
		fetchedUser, err := service.GetUser(ctx, user.ID)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if fetchedUser.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, fetchedUser.ID)
		}

		if fetchedUser.Username != user.Username {
			t.Errorf("Expected Username %s, got %s", user.Username, fetchedUser.Username)
		}

		// Проверяем, что пользователь теперь в кэше
		_, err = mockCacheRepo.GetUser(ctx, user.ID)
		if err != nil {
			t.Errorf("Expected user to be cached after fetch, got error: %v", err)
		}
	})

	// Тест кейс: получение пользователя из кэша
	t.Run("GetFromCache", func(t *testing.T) {
		// Изменяем пользователя в БД, но не в кэше
		modifiedUser := *user
		modifiedUser.Username = "modified"
		err := mockUserRepo.Update(&modifiedUser)
		if err != nil {
			t.Fatalf("Failed to setup test: %v", err)
		}

		// Запрос должен вернуть кэшированную версию
		fetchedUser, err := service.GetUser(ctx, user.ID)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Здесь мы ожидаем получить оригинальное имя, т.к. оно в кэше
		if fetchedUser.Username != user.Username {
			t.Errorf("Expected cached Username %s, got %s", user.Username, fetchedUser.Username)
		}
	})

	// Тест кейс: получение несуществующего пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		_, err := service.GetUser(ctx, 9999)

		if err == nil {
			t.Fatalf("Expected error for non-existent user, got nil")
		}
	})
}

func TestUpdateUser(t *testing.T) {
	// Инициализация моков и сервиса
	mockUserRepo := NewMockUserRepository()
	mockCacheRepo := NewMockCacheRepository()
	logger := zap.NewNop()
	service := NewUserService(mockUserRepo, mockCacheRepo, logger)

	ctx := context.Background()

	// Создаем тестового пользователя
	user := &models.User{
		TelegramID: 123123123,
		Username:   "updateuser",
		Bio:        "Update user test",
		Rating:     0,
	}

	err := mockUserRepo.Create(user)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Кэшируем пользователя
	err = mockCacheRepo.SetUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Тест кейс: успешное обновление пользователя
	t.Run("Success", func(t *testing.T) {
		newUsername := "updated_username"
		newBio := "Updated bio"

		updatedUser, err := service.UpdateUser(ctx, user.ID, newUsername, newBio)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if updatedUser.Username != newUsername {
			t.Errorf("Expected Username %s, got %s", newUsername, updatedUser.Username)
		}

		if updatedUser.Bio != newBio {
			t.Errorf("Expected Bio %s, got %s", newBio, updatedUser.Bio)
		}

		// Проверяем, что пользователь обновлен в БД
		dbUser, err := mockUserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Expected user to be in DB, got error: %v", err)
		}

		if dbUser.Username != newUsername {
			t.Errorf("Expected updated Username in DB %s, got %s", newUsername, dbUser.Username)
		}

		// Проверяем, что пользователь обновлен в кэше
		cachedUser, err := mockCacheRepo.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Expected user to be in cache, got error: %v", err)
		}

		if cachedUser.Username != newUsername {
			t.Errorf("Expected updated Username in cache %s, got %s", newUsername, cachedUser.Username)
		}
	})

	// Тест кейс: обновление несуществующего пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		_, err := service.UpdateUser(ctx, 9999, "nonexistent", "bio")

		if err == nil {
			t.Fatalf("Expected error for non-existent user, got nil")
		}
	})
}
