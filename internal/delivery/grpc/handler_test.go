package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"JollyRogerUserService/internal/models"
	pb "JollyRogerUserService/pkg/proto/user"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockUserService представляет мок для сервиса пользователей
type MockUserService struct {
	users       map[uint]*models.User
	usersByTg   map[int64]*models.User
	locations   map[uint]*models.UserLocation
	preferences map[uint][]models.UserPreference
	stats       map[uint]*models.UserStats
	settings    map[uint]*models.UserNotificationSetting
}

func NewMockUserService() *MockUserService {
	return &MockUserService{
		users:       make(map[uint]*models.User),
		usersByTg:   make(map[int64]*models.User),
		locations:   make(map[uint]*models.UserLocation),
		preferences: make(map[uint][]models.UserPreference),
		stats:       make(map[uint]*models.UserStats),
		settings:    make(map[uint]*models.UserNotificationSetting),
	}
}

// CreateUser - мок для создания пользователя
func (m *MockUserService) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	// Проверяем, существует ли пользователь с таким telegram_id
	if _, exists := m.usersByTg[req.TelegramID]; exists {
		return nil, errors.New("user with this telegram_id already exists")
	}

	user := &models.User{
		ID:         uint(len(m.users) + 1),
		TelegramID: req.TelegramID,
		Username:   req.Username,
		Bio:        req.Bio,
		Rating:     0,
	}

	m.users[user.ID] = user
	m.usersByTg[user.TelegramID] = user

	return user, nil
}

// GetUser - мок для получения пользователя по ID
func (m *MockUserService) GetUser(ctx context.Context, id uint) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// GetUserByTelegramID - мок для получения пользователя по Telegram ID
func (m *MockUserService) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	user, exists := m.usersByTg[telegramID]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// UpdateUser - мок для обновления пользователя
func (m *MockUserService) UpdateUser(ctx context.Context, id uint, username, bio string) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}

	user.Username = username
	user.Bio = bio

	return user, nil
}

// AddUserPreference - мок для добавления предпочтения
func (m *MockUserService) AddUserPreference(ctx context.Context, userID, tagID uint) error {
	if _, exists := m.users[userID]; !exists {
		return errors.New("user not found")
	}

	for _, pref := range m.preferences[userID] {
		if pref.TagID == tagID {
			return errors.New("preference already exists")
		}
	}

	m.preferences[userID] = append(m.preferences[userID], models.UserPreference{
		UserID:    userID,
		TagID:     tagID,
		CreatedAt: time.Now(),
	})

	return nil
}

// RemoveUserPreference - мок для удаления предпочтения
func (m *MockUserService) RemoveUserPreference(ctx context.Context, userID, tagID uint) error {
	if _, exists := m.users[userID]; !exists {
		return errors.New("user not found")
	}

	prefs := m.preferences[userID]
	for i, pref := range prefs {
		if pref.TagID == tagID {
			m.preferences[userID] = append(prefs[:i], prefs[i+1:]...)
			return nil
		}
	}

	return errors.New("preference not found")
}

// GetUserPreferences - мок для получения предпочтений
func (m *MockUserService) GetUserPreferences(ctx context.Context, userID uint) ([]models.UserPreference, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	return m.preferences[userID], nil
}

// UpdateUserLocation - мок для обновления местоположения
func (m *MockUserService) UpdateUserLocation(ctx context.Context, req *models.UserLocationRequest) error {
	if _, exists := m.users[req.UserID]; !exists {
		return errors.New("user not found")
	}

	m.locations[req.UserID] = &models.UserLocation{
		UserID:    req.UserID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		City:      req.City,
		Region:    req.Region,
		Country:   req.Country,
		UpdatedAt: time.Now(),
	}

	return nil
}

// GetUserLocation - мок для получения местоположения
func (m *MockUserService) GetUserLocation(ctx context.Context, userID uint) (*models.UserLocation, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	location, exists := m.locations[userID]
	if !exists {
		return nil, errors.New("location not found")
	}

	return location, nil
}

// FindNearbyUsers - мок для поиска пользователей рядом
func (m *MockUserService) FindNearbyUsers(ctx context.Context, lat, lon float64, radiusKm float64, limit int) ([]models.User, error) {
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

// GetUserStats - мок для получения статистики
func (m *MockUserService) GetUserStats(ctx context.Context, userID uint) (*models.UserStats, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	stats, exists := m.stats[userID]
	if !exists {
		// Создаем статистику по умолчанию для тестов
		stats = &models.UserStats{
			UserID:    userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.stats[userID] = stats
	}

	return stats, nil
}

// UpdateUserRating - мок для обновления рейтинга
func (m *MockUserService) UpdateUserRating(ctx context.Context, userID uint, ratingChange float32) (*models.User, error) {
	user, exists := m.users[userID]
	if !exists {
		return nil, errors.New("user not found")
	}

	user.Rating += ratingChange
	return user, nil
}

// GetNotificationSettings - мок для получения настроек уведомлений
func (m *MockUserService) GetNotificationSettings(ctx context.Context, userID uint) (*models.UserNotificationSetting, error) {
	if _, exists := m.users[userID]; !exists {
		return nil, errors.New("user not found")
	}

	settings, exists := m.settings[userID]
	if !exists {
		// Создаем настройки по умолчанию для тестов
		settings = &models.UserNotificationSetting{
			UserID:               userID,
			NewEventNotification: true,
		}
		m.settings[userID] = settings
	}

	return settings, nil
}

// UpdateNotificationSettings - мок для обновления настроек уведомлений
func (m *MockUserService) UpdateNotificationSettings(ctx context.Context, req *models.UpdateNotificationSettingRequest) error {
	if _, exists := m.users[req.UserID]; !exists {
		return errors.New("user not found")
	}

	m.settings[req.UserID] = &models.UserNotificationSetting{
		UserID:               req.UserID,
		NewEventNotification: req.NewEventNotification,
	}

	return nil
}

// TestCreateUser тестирует метод CreateUser обработчика
func TestCreateUser(t *testing.T) {
	// Создаем мок сервиса
	mockService := NewMockUserService()
	logger := zap.NewNop()
	handler := NewUserHandler(mockService, logger)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Тест кейс: успешное создание пользователя
	t.Run("Success", func(t *testing.T) {
		req := &pb.CreateUserRequest{
			TelegramId: 123456789,
			Username:   "testuser",
			Bio:        "Test bio",
		}

		resp, err := handler.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		if resp.TelegramId != req.TelegramId {
			t.Errorf("Expected TelegramId %d, got %d", req.TelegramId, resp.TelegramId)
		}
		if resp.Username != req.Username {
			t.Errorf("Expected Username %s, got %s", req.Username, resp.Username)
		}
		if resp.Bio != req.Bio {
			t.Errorf("Expected Bio %s, got %s", req.Bio, resp.Bio)
		}
	})

	// Тест кейс: попытка создать пользователя с существующим TelegramID
	t.Run("DuplicateTelegramID", func(t *testing.T) {
		req := &pb.CreateUserRequest{
			TelegramId: 123456789, // Тот же TelegramID, что и в предыдущем тесте
			Username:   "anotheruser",
			Bio:        "Another bio",
		}

		_, err := handler.CreateUser(ctx, req)
		if err == nil {
			t.Fatalf("Expected error for duplicate TelegramID, got nil")
		}

		// Проверяем код ошибки gRPC
		status, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if status.Code() != codes.Internal {
			t.Errorf("Expected status code %v, got %v", codes.Internal, status.Code())
		}
	})
}

// TestGetUser тестирует метод GetUser обработчика
func TestGetUser(t *testing.T) {
	// Setup
	mockService := NewMockUserService()
	logger := zap.NewNop()
	handler := NewUserHandler(mockService, logger)
	ctx := context.Background()

	// Создаем тестового пользователя
	user, _ := mockService.CreateUser(ctx, &models.CreateUserRequest{
		TelegramID: 987654321,
		Username:   "getuser",
		Bio:        "Get user test",
	})

	tests := []struct {
		name          string
		req           *pb.GetUserRequest
		expectedID    uint
		expectedError codes.Code
	}{
		{
			name: "Valid user",
			req: &pb.GetUserRequest{
				Id: uint64(user.ID),
			},
			expectedID:    user.ID,
			expectedError: codes.OK,
		},
		{
			name: "Non-existent user",
			req: &pb.GetUserRequest{
				Id: 9999,
			},
			expectedID:    0,
			expectedError: codes.NotFound,
		},
		{
			name: "Zero ID",
			req: &pb.GetUserRequest{
				Id: 0,
			},
			expectedID:    0,
			expectedError: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handler.GetUser(ctx, tt.req)

			if tt.expectedError == codes.OK {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				if uint(resp.Id) != tt.expectedID {
					t.Errorf("Expected ID %d, got %d", tt.expectedID, resp.Id)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Expected gRPC status error, got: %v", err)
				}
				if st.Code() != tt.expectedError {
					t.Errorf("Expected status code %v, got %v", tt.expectedError, st.Code())
				}
			}
		})
	}
}

// TestGetUserByTelegramID тестирует метод GetUserByTelegramID обработчика
func TestGetUserByTelegramID(t *testing.T) {
	// Создаем мок сервиса
	mockService := NewMockUserService()
	logger := zap.NewNop()
	handler := NewUserHandler(mockService, logger)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Создаем тестового пользователя
	telegramID := int64(123123123)
	user, err := mockService.CreateUser(ctx, &models.CreateUserRequest{
		TelegramID: telegramID,
		Username:   "tguser",
		Bio:        "Telegram user test",
	})
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Тест кейс: успешное получение пользователя
	t.Run("Success", func(t *testing.T) {
		req := &pb.GetUserByTelegramIDRequest{
			TelegramId: telegramID,
		}

		resp, err := handler.GetUserByTelegramID(ctx, req)
		if err != nil {
			t.Fatalf("GetUserByTelegramID failed: %v", err)
		}

		if resp.Id != uint64(user.ID) {
			t.Errorf("Expected ID %d, got %d", user.ID, resp.Id)
		}
		if resp.TelegramId != user.TelegramID {
			t.Errorf("Expected TelegramId %d, got %d", user.TelegramID, resp.TelegramId)
		}
		if resp.Username != user.Username {
			t.Errorf("Expected Username %s, got %s", user.Username, resp.Username)
		}
	})

	// Тест кейс: получение несуществующего пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		req := &pb.GetUserByTelegramIDRequest{
			TelegramId: 9999999,
		}

		_, err := handler.GetUserByTelegramID(ctx, req)
		if err == nil {
			t.Fatalf("Expected error for non-existent user, got nil")
		}

		// Проверяем код ошибки gRPC
		status, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if status.Code() != codes.NotFound {
			t.Errorf("Expected status code %v, got %v", codes.NotFound, status.Code())
		}
	})
}

// TestUpdateUser тестирует метод UpdateUser обработчика
func TestUpdateUser(t *testing.T) {
	// Создаем мок сервиса
	mockService := NewMockUserService()
	logger := zap.NewNop()
	handler := NewUserHandler(mockService, logger)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Создаем тестового пользователя
	user, err := mockService.CreateUser(ctx, &models.CreateUserRequest{
		TelegramID: 111222333,
		Username:   "updateuser",
		Bio:        "Update user test",
	})
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Тест кейс: успешное обновление пользователя
	t.Run("Success", func(t *testing.T) {
		req := &pb.UpdateUserRequest{
			Id:       uint64(user.ID),
			Username: "updated_username",
			Bio:      "Updated bio",
		}

		resp, err := handler.UpdateUser(ctx, req)
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		if resp.Id != uint64(user.ID) {
			t.Errorf("Expected ID %d, got %d", user.ID, resp.Id)
		}
		if resp.Username != req.Username {
			t.Errorf("Expected Username %s, got %s", req.Username, resp.Username)
		}
		if resp.Bio != req.Bio {
			t.Errorf("Expected Bio %s, got %s", req.Bio, resp.Bio)
		}
	})

	// Тест кейс: обновление несуществующего пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		req := &pb.UpdateUserRequest{
			Id:       9999,
			Username: "nonexistent",
			Bio:      "Bio for non-existent user",
		}

		_, err := handler.UpdateUser(ctx, req)
		if err == nil {
			t.Fatalf("Expected error for non-existent user, got nil")
		}

		// Проверяем код ошибки gRPC
		status, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if status.Code() != codes.Internal {
			t.Errorf("Expected status code %v, got %v", codes.Internal, status.Code())
		}
	})
}

// TestUpdateUserRating тестирует метод UpdateUserRating обработчика
func TestUpdateUserRating(t *testing.T) {
	// Создаем мок сервиса
	mockService := NewMockUserService()
	logger := zap.NewNop()
	handler := NewUserHandler(mockService, logger)

	// Создаем тестовый контекст
	ctx := context.Background()

	// Создаем тестового пользователя
	user, err := mockService.CreateUser(ctx, &models.CreateUserRequest{
		TelegramID: 444555666,
		Username:   "ratinguser",
		Bio:        "Rating test",
	})
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Тест кейс: успешное обновление рейтинга
	t.Run("Success", func(t *testing.T) {
		req := &pb.UpdateUserRatingRequest{
			UserId:       uint64(user.ID),
			RatingChange: 2.5,
		}

		resp, err := handler.UpdateUserRating(ctx, req)
		if err != nil {
			t.Fatalf("UpdateUserRating failed: %v", err)
		}

		if resp.Id != uint64(user.ID) {
			t.Errorf("Expected ID %d, got %d", user.ID, resp.Id)
		}
		if resp.Rating != 2.5 { // Начальный рейтинг 0 + изменение 2.5
			t.Errorf("Expected Rating 2.5, got %f", resp.Rating)
		}
	})

	// Тест кейс: обновление рейтинга несуществующего пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		req := &pb.UpdateUserRatingRequest{
			UserId:       9999,
			RatingChange: 1.0,
		}

		_, err := handler.UpdateUserRating(ctx, req)
		if err == nil {
			t.Fatalf("Expected error for non-existent user, got nil")
		}

		// Проверяем код ошибки gRPC
		status, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if status.Code() != codes.Internal {
			t.Errorf("Expected status code %v, got %v", codes.Internal, status.Code())
		}
	})
}
