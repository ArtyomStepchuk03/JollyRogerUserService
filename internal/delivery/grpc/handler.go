package grpc

import (
	"JollyRogerUserService/internal/models"
	"JollyRogerUserService/internal/service"
	pb "JollyRogerUserService/pkg/proto/user"
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

// UserHandler представляет обработчик gRPC запросов
type UserHandler struct {
	pb.UnsafeJollyRogerUserServiceServer
	service *service.UserService
	logger  *zap.Logger
}

// NewUserHandler создает новый экземпляр UserHandler
func NewUserHandler(service *service.UserService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		service: service,
		logger:  logger,
	}
}

// GetUser возвращает пользователя по ID
func (h *UserHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	// Преобразуем uint64 в uint
	id := uint(req.Id)

	// Получаем пользователя из сервиса
	user, err := h.service.GetUser(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get user", zap.Error(err), zap.Uint64("user_id", req.Id))
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserResponse{
		Id:         uint64(user.ID),
		TelegramId: user.TelegramID,
		Username:   user.Username,
		Bio:        user.Bio,
		Rating:     user.Rating,
	}

	return response, nil
}

// GetUserByTelegramID возвращает пользователя по Telegram ID
func (h *UserHandler) GetUserByTelegramID(ctx context.Context, req *pb.GetUserByTelegramIDRequest) (*pb.UserResponse, error) {
	// Получаем пользователя из сервиса
	user, err := h.service.GetUserByTelegramID(ctx, req.TelegramId)
	if err != nil {
		h.logger.Error("Failed to get user by telegram_id", zap.Error(err), zap.Int64("telegram_id", req.TelegramId))
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserResponse{
		Id:         uint64(user.ID),
		TelegramId: user.TelegramID,
		Username:   user.Username,
		Bio:        user.Bio,
		Rating:     user.Rating,
	}

	return response, nil
}

// CreateUser создает нового пользователя
func (h *UserHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.UserResponse, error) {
	// Преобразуем запрос протобафа в модель
	createReq := &models.CreateUserRequest{
		TelegramID: req.TelegramId,
		Username:   req.Username,
		Bio:        req.Bio,
	}

	// Создаем пользователя через сервис
	user, err := h.service.CreateUser(ctx, createReq)
	if err != nil {
		h.logger.Error("Failed to create user", zap.Error(err), zap.Int64("telegram_id", req.TelegramId))
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserResponse{
		Id:         uint64(user.ID),
		TelegramId: user.TelegramID,
		Username:   user.Username,
		Bio:        user.Bio,
		Rating:     user.Rating,
	}

	return response, nil
}

// UpdateUser обновляет пользователя
func (h *UserHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserResponse, error) {
	// Преобразуем uint64 в uint
	id := uint(req.Id)

	// Обновляем пользователя через сервис
	user, err := h.service.UpdateUser(ctx, id, req.Username, req.Bio)
	if err != nil {
		h.logger.Error("Failed to update user", zap.Error(err), zap.Uint64("user_id", req.Id))
		return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserResponse{
		Id:         uint64(user.ID),
		TelegramId: user.TelegramID,
		Username:   user.Username,
		Bio:        user.Bio,
		Rating:     user.Rating,
	}

	return response, nil
}

// AddUserPreference добавляет предпочтение пользователю
func (h *UserHandler) AddUserPreference(ctx context.Context, req *pb.AddUserPreferenceRequest) (*pb.SimpleResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)
	tagID := uint(req.TagId)

	// Добавляем предпочтение через сервис
	err := h.service.AddUserPreference(ctx, userID, tagID)
	if err != nil {
		h.logger.Error("Failed to add preference",
			zap.Error(err),
			zap.Uint64("user_id", req.UserId),
			zap.Uint64("tag_id", req.TagId))
		return nil, status.Errorf(codes.Internal, "failed to add preference: %v", err)
	}

	return &pb.SimpleResponse{
		Success: true,
		Message: "Preference added successfully",
	}, nil
}

// RemoveUserPreference удаляет предпочтение пользователя
func (h *UserHandler) RemoveUserPreference(ctx context.Context, req *pb.RemoveUserPreferenceRequest) (*pb.SimpleResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)
	tagID := uint(req.TagId)

	// Удаляем предпочтение через сервис
	err := h.service.RemoveUserPreference(ctx, userID, tagID)
	if err != nil {
		h.logger.Error("Failed to remove preference",
			zap.Error(err),
			zap.Uint64("user_id", req.UserId),
			zap.Uint64("tag_id", req.TagId))
		return nil, status.Errorf(codes.Internal, "failed to remove preference: %v", err)
	}

	return &pb.SimpleResponse{
		Success: true,
		Message: "Preference removed successfully",
	}, nil
}

// GetUserPreferences получает все предпочтения пользователя
func (h *UserHandler) GetUserPreferences(ctx context.Context, req *pb.GetUserPreferencesRequest) (*pb.UserPreferencesResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)

	// Получаем предпочтения через сервис
	preferences, err := h.service.GetUserPreferences(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get preferences", zap.Error(err), zap.Uint64("user_id", req.UserId))
		return nil, status.Errorf(codes.Internal, "failed to get preferences: %v", err)
	}

	// Преобразуем модели в ответ протобафа
	response := &pb.UserPreferencesResponse{
		Preferences: make([]*pb.UserPreference, len(preferences)),
	}

	for i, pref := range preferences {
		response.Preferences[i] = &pb.UserPreference{
			TagId: uint64(pref.TagID),
		}
	}

	return response, nil
}

// UpdateUserLocation обновляет местоположение пользователя
func (h *UserHandler) UpdateUserLocation(ctx context.Context, req *pb.UpdateUserLocationRequest) (*pb.SimpleResponse, error) {
	// Преобразуем запрос протобафа в модель
	location := &models.UserLocationRequest{
		UserID:    uint(req.UserId),
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		City:      req.City,
		Region:    req.Region,
		Country:   req.Country,
	}

	// Обновляем местоположение через сервис
	err := h.service.UpdateUserLocation(ctx, location)
	if err != nil {
		h.logger.Error("Failed to update location",
			zap.Error(err),
			zap.Uint64("user_id", req.UserId),
			zap.Float64("lat", req.Latitude),
			zap.Float64("lon", req.Longitude))
		return nil, status.Errorf(codes.Internal, "failed to update location: %v", err)
	}

	return &pb.SimpleResponse{
		Success: true,
		Message: "Location updated successfully",
	}, nil
}

// GetUserLocation получает местоположение пользователя
func (h *UserHandler) GetUserLocation(ctx context.Context, req *pb.GetUserLocationRequest) (*pb.UserLocationResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)

	// Получаем местоположение через сервис
	location, err := h.service.GetUserLocation(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get location", zap.Error(err), zap.Uint64("user_id", req.UserId))
		return nil, status.Errorf(codes.Internal, "failed to get location: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserLocationResponse{
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
		City:      location.City,
		Region:    location.Region,
		Country:   location.Country,
	}

	return response, nil
}

// FindNearbyUsers находит пользователей рядом с заданными координатами
func (h *UserHandler) FindNearbyUsers(ctx context.Context, req *pb.FindNearbyUsersRequest) (*pb.UsersResponse, error) {
	// Преобразуем uint64 в int
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10 // Значение по умолчанию
	}

	// Находим пользователей рядом через сервис
	users, err := h.service.FindNearbyUsers(ctx, req.Latitude, req.Longitude, req.RadiusKm, limit)
	if err != nil {
		h.logger.Error("Failed to find nearby users",
			zap.Error(err),
			zap.Float64("lat", req.Latitude),
			zap.Float64("lon", req.Longitude),
			zap.Float64("radius", req.RadiusKm))
		return nil, status.Errorf(codes.Internal, "failed to find nearby users: %v", err)
	}

	// Преобразуем модели в ответ протобафа
	response := &pb.UsersResponse{
		Users: make([]*pb.UserResponse, len(users)),
	}

	for i, user := range users {
		response.Users[i] = &pb.UserResponse{
			Id:         uint64(user.ID),
			TelegramId: user.TelegramID,
			Username:   user.Username,
			Bio:        user.Bio,
			Rating:     user.Rating,
		}
	}

	return response, nil
}

// GetUserStats получает статистику пользователя
func (h *UserHandler) GetUserStats(ctx context.Context, req *pb.GetUserStatsRequest) (*pb.UserStatsResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)

	// Получаем статистику через сервис
	stats, err := h.service.GetUserStats(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user stats", zap.Error(err), zap.Uint64("user_id", req.UserId))
		return nil, status.Errorf(codes.Internal, "failed to get user stats: %v", err)
	}

	// Преобразуем время в формат строки
	createdAt := stats.CreatedAt.Format(time.RFC3339)
	var lastActiveAt string
	if stats.LastActiveAt != nil {
		lastActiveAt = stats.LastActiveAt.Format(time.RFC3339)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserStatsResponse{
		UserId:             uint64(stats.UserID),
		EventsCreated:      int32(stats.EventsCreated),
		EventsParticipated: int32(stats.EventsParticipated),
		CreatedAt:          createdAt,
		LastActiveAt:       lastActiveAt,
		IsActive:           stats.IsActive,
	}

	return response, nil
}

// UpdateUserRating обновляет рейтинг пользователя
func (h *UserHandler) UpdateUserRating(ctx context.Context, req *pb.UpdateUserRatingRequest) (*pb.UserResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)

	// Обновляем рейтинг через сервис
	user, err := h.service.UpdateUserRating(ctx, userID, req.RatingChange)
	if err != nil {
		h.logger.Error("Failed to update user rating",
			zap.Error(err),
			zap.Uint64("user_id", req.UserId),
			zap.Float32("rating_change", req.RatingChange))
		return nil, status.Errorf(codes.Internal, "failed to update user rating: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.UserResponse{
		Id:         uint64(user.ID),
		TelegramId: user.TelegramID,
		Username:   user.Username,
		Bio:        user.Bio,
		Rating:     user.Rating,
	}

	return response, nil
}

// UpdateNotificationSettings обновляет настройки уведомлений пользователя
func (h *UserHandler) UpdateNotificationSettings(ctx context.Context, req *pb.UpdateNotificationSettingsRequest) (*pb.SimpleResponse, error) {
	// Преобразуем запрос протобафа в модель
	settings := &models.UpdateNotificationSettingRequest{
		UserID:               uint(req.UserId),
		NewEventNotification: req.NewEventNotification,
	}

	// Обновляем настройки через сервис
	err := h.service.UpdateNotificationSettings(ctx, settings)
	if err != nil {
		h.logger.Error("Failed to update notification settings",
			zap.Error(err),
			zap.Uint64("user_id", req.UserId),
			zap.Bool("new_event_notification", req.NewEventNotification))
		return nil, status.Errorf(codes.Internal, "failed to update notification settings: %v", err)
	}

	return &pb.SimpleResponse{
		Success: true,
		Message: "Notification settings updated successfully",
	}, nil
}

// GetNotificationSettings получает настройки уведомлений пользователя
func (h *UserHandler) GetNotificationSettings(ctx context.Context, req *pb.GetNotificationSettingsRequest) (*pb.NotificationSettingsResponse, error) {
	// Преобразуем uint64 в uint
	userID := uint(req.UserId)

	// Получаем настройки через сервис
	settings, err := h.service.GetNotificationSettings(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get notification settings", zap.Error(err), zap.Uint64("user_id", req.UserId))
		return nil, status.Errorf(codes.Internal, "failed to get notification settings: %v", err)
	}

	// Преобразуем модель в ответ протобафа
	response := &pb.NotificationSettingsResponse{
		NewEventNotification: settings.NewEventNotification,
	}

	return response, nil
}
