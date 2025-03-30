package infrastructure

import (
	"JollyRogerUserService/internal/user"
	"JollyRogerUserService/pb"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	pb.UnimplementedUserServiceServer
	service user.Service
}

func NewGRPCHandler(service user.Service) *GRPCHandler {
	return &GRPCHandler{service: service}
}

func (h *GRPCHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	if req == nil || req.ChatId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "chat_id is required")
	}

	u, err := h.service.GetByChatID(req.ChatId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user with chat_id %d not found: %v", req.ChatId, err)
	}

	return &pb.GetUserResponse{
		User: user.ToProto(u),
	}, nil
}

func (h *GRPCHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	if req == nil || req.ChatId == 0 || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "chat_id and name are required")
	}

	newUser := &user.User{
		ChatID: req.ChatId,
		Name:   req.Name,
		Age:    uint8(req.Age),
		About:  req.About,
		Settings: user.Settings{
			Country:  req.Country,
			City:     req.City,
			Language: req.Language,
		},
	}

	if err := h.service.Create(newUser); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	return &pb.CreateUserResponse{User: user.ToProto(newUser)}, nil
}

func (h *GRPCHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	if req == nil || req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	existingUser, err := h.service.GetByChatID(req.ChatId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user with chat_id %d not found", req.ChatId)
	}

	// Обновляем поля
	existingUser.Name = req.Name
	existingUser.Age = uint8(req.Age)
	existingUser.About = req.About
	existingUser.Karma = req.Karma
	existingUser.Settings.Country = req.Country
	existingUser.Settings.City = req.City
	existingUser.Settings.Language = req.Language

	if err := h.service.Update(existingUser); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
	}

	return &pb.UpdateUserResponse{User: user.ToProto(existingUser)}, nil
}

func (h *GRPCHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if req == nil || req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	err := h.service.Delete(req.ChatId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete user: %v", err)
	}

	return &pb.DeleteUserResponse{Success: true}, nil
}
