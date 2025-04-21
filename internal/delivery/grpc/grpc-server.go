package grpc

import (
	pb "JollyRogerUserService/pkg/proto/user"
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

// Server представляет собой gRPC сервер
type Server struct {
	grpcServer *grpc.Server
	logger     *zap.Logger
	port       int
	handler    *UserHandler
}

// NewServer создает новый экземпляр gRPC сервера
func NewServer(handler *UserHandler, logger *zap.Logger, port int) *Server {
	return &Server{
		logger:  logger,
		port:    port,
		handler: handler,
	}
}

// Run запускает gRPC сервер
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		s.logger.Error("Failed to listen", zap.Error(err), zap.Int("port", s.port))
		return err
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			s.loggingInterceptor(),
			s.recoveryInterceptor(),
		),
	}

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterJollyRogerUserServiceServer(s.grpcServer, s.handler)

	// Включаем reflection для удобства отладки через grpcurl
	reflection.Register(s.grpcServer)

	s.logger.Info("Starting gRPC server", zap.Int("port", s.port))
	return s.grpcServer.Serve(lis)
}

// Stop останавливает gRPC сервер
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server")
	s.grpcServer.GracefulStop()
}

// loggingInterceptor создает перехватчик для логирования запросов
func (s *Server) loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		s.logger.Info("gRPC request",
			zap.String("method", info.FullMethod),
			zap.Any("request", req))

		resp, err := handler(ctx, req)

		if err != nil {
			s.logger.Error("gRPC error",
				zap.String("method", info.FullMethod),
				zap.Error(err))
		} else {
			s.logger.Info("gRPC response",
				zap.String("method", info.FullMethod),
				zap.Any("response", resp))
		}

		return resp, err
	}
}

// recoveryInterceptor создает перехватчик для восстановления после паники
func (s *Server) recoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("Recovered from panic",
					zap.Any("panic", r),
					zap.String("method", info.FullMethod))
			}
		}()

		return handler(ctx, req)
	}
}
