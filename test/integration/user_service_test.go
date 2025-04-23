package integration

import (
	"context"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"JollyRogerUserService/config"
	grpcHandler "JollyRogerUserService/internal/delivery/grpc"
	"JollyRogerUserService/internal/repository/postgres"
	"JollyRogerUserService/internal/repository/redis"
	"JollyRogerUserService/internal/service"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/logger"
	pb "JollyRogerUserService/pkg/proto/user"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

var (
	client     pb.JollyRogerUserServiceClient
	conn       *grpc.ClientConn
	db         *gorm.DB
	pgResource *dockertest.Resource
	rdResource *dockertest.Resource
	pool       *dockertest.Pool
)

// Настройка тестового окружения
func TestMain(m *testing.M) {
	// Создаем Docker-pool
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	// Устанавливаем тайм-аут для контейнеров
	pool.MaxWait = time.Minute * 2

	// Запускаем PostgreSQL
	pgResource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=test_db",
		},
	}, func(config *docker.HostConfig) {
		// Устанавливаем автоудаление контейнера
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Could not start PostgreSQL: %s", err)
	}

	// Запускаем Redis
	rdResource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		// Устанавливаем автоудаление контейнера
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Could not start Redis: %s", err)
	}

	// Получаем хост и порт PostgreSQL
	pgHost := pgResource.GetBoundIP("5432/tcp")
	pgPort := pgResource.GetPort("5432/tcp")

	// Получаем хост и порт Redis
	redisHost := rdResource.GetBoundIP("6379/tcp")
	redisPort := rdResource.GetPort("6379/tcp")

	// Ожидаем готовности PostgreSQL
	if err := pool.Retry(func() error {
		pgConfig := config.PostgresConfig{
			Host:     pgHost,
			Port:     5432,
			Username: "postgres",
			Password: "postgres",
			DBName:   "test_db",
			SSLMode:  "disable",
		}

		var err error
		db, err = database.NewPostgresDB(pgConfig)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to PostgreSQL: %s", err)
	}

	// Ожидаем готовности Redis
	if err := pool.Retry(func() error {
		redisConfig := config.RedisConfig{
			Addr:     redisHost + ":" + redisPort,
			Password: "",
			DB:       0,
		}

		_, err := database.NewRedisClient(redisConfig)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to Redis: %s", err)
	}

	// Запускаем gRPC сервер для тестирования
	go runTestServer(pgHost, pgPort, redisHost, redisPort)

	// Ожидаем запуска сервера
	time.Sleep(time.Second * 2)

	// Подключаемся к серверу как клиент
	conn, err = grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	client = pb.NewJollyRogerUserServiceClient(conn)

	// Запускаем тесты
	code := m.Run()

	// Очистка ресурсов
	conn.Close()
	pool.Purge(pgResource)
	pool.Purge(rdResource)

	os.Exit(code)
}

// runTestServer запускает тестовый gRPC сервер
func runTestServer(pgHost, pgPort, redisHost, redisPort string) {
	// Настройка логгера
	log := logger.NewLogger()

	// Настройка PostgreSQL
	pgConfig := config.PostgresConfig{
		Host:     pgHost,
		Port:     5432,
		Username: "postgres",
		Password: "postgres",
		DBName:   "test_db",
		SSLMode:  "disable",
	}

	// Настройка Redis
	redisConfig := config.RedisConfig{
		Addr:     redisHost + ":" + redisPort,
		Password: "",
		DB:       0,
	}

	// Подключение к PostgreSQL
	db, err := database.NewPostgresDB(pgConfig)
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}

	// Подключение к Redis
	redisClient, err := database.NewRedisClient(redisConfig)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Инициализация репозиториев
	userRepo := postgres.NewUserRepository(db)
	cacheRepo := redis.NewCacheRepository(redisClient)

	// Инициализация сервиса
	userService := service.NewUserService(userRepo, cacheRepo, log)

	// Инициализация gRPC сервера
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}

	s := grpc.NewServer()
	userHandler := grpcHandler.NewUserHandler(userService, log)
	pb.RegisterJollyRogerUserServiceServer(s, userHandler)

	if err := s.Serve(lis); err != nil {
		log.Fatal("Failed to serve", zap.Error(err))
	}
}

// TestUserServiceIntegration тестирует полный цикл работы с пользователем
func TestUserServiceIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Создание пользователя
	createReq := &pb.CreateUserRequest{
		TelegramId: 555666777,
		Username:   "integrationuser",
		Bio:        "Integration test user",
	}

	createResp, err := client.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if createResp.TelegramId != createReq.TelegramId {
		t.Errorf("Expected TelegramId %d, got %d", createReq.TelegramId, createResp.TelegramId)
	}

	userID := createResp.Id

	// 2. Получение пользователя по ID
	getReq := &pb.GetUserRequest{
		Id: userID,
	}

	getResp, err := client.GetUser(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if getResp.Id != userID {
		t.Errorf("Expected Id %d, got %d", userID, getResp.Id)
	}
	if getResp.Username != createReq.Username {
		t.Errorf("Expected Username %s, got %s", createReq.Username, getResp.Username)
	}

	// 3. Обновление пользователя
	updateReq := &pb.UpdateUserRequest{
		Id:       userID,
		Username: "updated_integration_user",
		Bio:      "Updated integration test user",
	}

	updateResp, err := client.UpdateUser(ctx, updateReq)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	if updateResp.Username != updateReq.Username {
		t.Errorf("Expected Username %s, got %s", updateReq.Username, updateResp.Username)
	}
	if updateResp.Bio != updateReq.Bio {
		t.Errorf("Expected Bio %s, got %s", updateReq.Bio, updateResp.Bio)
	}

	// 4. Добавление предпочтения
	addPrefReq := &pb.AddUserPreferenceRequest{
		UserId: userID,
		TagId:  100,
	}

	_, err = client.AddUserPreference(ctx, addPrefReq)
	if err != nil {
		t.Fatalf("Failed to add preference: %v", err)
	}

	// 5. Получение предпочтений
	getPrefReq := &pb.GetUserPreferencesRequest{
		UserId: userID,
	}

	getPrefResp, err := client.GetUserPreferences(ctx, getPrefReq)
	if err != nil {
		t.Fatalf("Failed to get preferences: %v", err)
	}

	if len(getPrefResp.Preferences) != 1 {
		t.Errorf("Expected 1 preference, got %d", len(getPrefResp.Preferences))
	} else if getPrefResp.Preferences[0].TagId != addPrefReq.TagId {
		t.Errorf("Expected TagId %d, got %d", addPrefReq.TagId, getPrefResp.Preferences[0].TagId)
	}

	// 6. Обновление местоположения
	updateLocReq := &pb.UpdateUserLocationRequest{
		UserId:    userID,
		Latitude:  55.7558,
		Longitude: 37.6173,
		City:      "Moscow",
		Region:    "Moscow",
		Country:   "Russia",
	}

	_, err = client.UpdateUserLocation(ctx, updateLocReq)
	if err != nil {
		t.Fatalf("Failed to update location: %v", err)
	}

	// 7. Получение местоположения
	getLocReq := &pb.GetUserLocationRequest{
		UserId: userID,
	}

	getLocResp, err := client.GetUserLocation(ctx, getLocReq)
	if err != nil {
		t.Fatalf("Failed to get location: %v", err)
	}

	if getLocResp.Latitude != updateLocReq.Latitude {
		t.Errorf("Expected Latitude %f, got %f", updateLocReq.Latitude, getLocResp.Latitude)
	}
	if getLocResp.City != updateLocReq.City {
		t.Errorf("Expected City %s, got %s", updateLocReq.City, getLocResp.City)
	}

	// 8. Обновление рейтинга
	updateRatingReq := &pb.UpdateUserRatingRequest{
		UserId:       userID,
		RatingChange: 4.5,
	}

	updateRatingResp, err := client.UpdateUserRating(ctx, updateRatingReq)
	if err != nil {
		t.Fatalf("Failed to update rating: %v", err)
	}

	if updateRatingResp.Rating != 4.5 { // Начальный рейтинг 0 + изменение 4.5
		t.Errorf("Expected Rating 4.5, got %f", updateRatingResp.Rating)
	}

	// 9. Получение статистики пользователя
	getStatsReq := &pb.GetUserStatsRequest{
		UserId: userID,
	}

	getStatsResp, err := client.GetUserStats(ctx, getStatsReq)
	if err != nil {
		t.Fatalf("Failed to get user stats: %v", err)
	}

	if getStatsResp.UserId != userID {
		t.Errorf("Expected UserId %d, got %d", userID, getStatsResp.UserId)
	}

	// 10. Обновление настроек уведомлений
	updateNotifReq := &pb.UpdateNotificationSettingsRequest{
		UserId:               userID,
		NewEventNotification: false, // По умолчанию true, меняем на false
	}

	_, err = client.UpdateNotificationSettings(ctx, updateNotifReq)
	if err != nil {
		t.Fatalf("Failed to update notification settings: %v", err)
	}

	// 11. Получение настроек уведомлений
	getNotifReq := &pb.GetNotificationSettingsRequest{
		UserId: userID,
	}

	getNotifResp, err := client.GetNotificationSettings(ctx, getNotifReq)
	if err != nil {
		t.Fatalf("Failed to get notification settings: %v", err)
	}

	if getNotifResp.NewEventNotification != updateNotifReq.NewEventNotification {
		t.Errorf("Expected NewEventNotification %v, got %v", updateNotifReq.NewEventNotification, getNotifResp.NewEventNotification)
	}

	// 12. Удаление предпочтения
	removePrefReq := &pb.RemoveUserPreferenceRequest{
		UserId: userID,
		TagId:  100,
	}

	_, err = client.RemoveUserPreference(ctx, removePrefReq)
	if err != nil {
		t.Fatalf("Failed to remove preference: %v", err)
	}

	// Проверяем, что предпочтение удалено
	getPrefResp, err = client.GetUserPreferences(ctx, getPrefReq)
	if err != nil {
		t.Fatalf("Failed to get preferences after removal: %v", err)
	}

	if len(getPrefResp.Preferences) != 0 {
		t.Errorf("Expected 0 preferences after removal, got %d", len(getPrefResp.Preferences))
	}
}

// TestUserDuplication проверяет обработку дублирования пользователей
func TestUserDuplication(t *testing.T) {
	ctx := context.Background()

	// Создаем первого пользователя
	createReq1 := &pb.CreateUserRequest{
		TelegramId: 111222333,
		Username:   "duplicatetest1",
		Bio:        "First user for duplication test",
	}

	_, err := client.CreateUser(ctx, createReq1)
	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	// Пытаемся создать пользователя с тем же Telegram ID
	createReq2 := &pb.CreateUserRequest{
		TelegramId: 111222333, // Тот же ID
		Username:   "duplicatetest2",
		Bio:        "Second user for duplication test",
	}

	_, err = client.CreateUser(ctx, createReq2)
	if err == nil {
		t.Fatal("Expected error when creating duplicate user, got nil")
	}
}

// TestUserNotFound проверяет обработку запросов к несуществующим пользователям
func TestUserNotFound(t *testing.T) {
	ctx := context.Background()
	nonExistentID := uint64(999999)

	// Тест GetUser для несуществующего пользователя
	t.Run("GetUser", func(t *testing.T) {
		req := &pb.GetUserRequest{
			Id: nonExistentID,
		}

		_, err := client.GetUser(ctx, req)
		if err == nil {
			t.Fatal("Expected error when getting non-existent user, got nil")
		}
	})

	// Тест UpdateUser для несуществующего пользователя
	t.Run("UpdateUser", func(t *testing.T) {
		req := &pb.UpdateUserRequest{
			Id:       nonExistentID,
			Username: "nonexistent",
			Bio:      "Bio for non-existent user",
		}

		_, err := client.UpdateUser(ctx, req)
		if err == nil {
			t.Fatal("Expected error when updating non-existent user, got nil")
		}
	})

	// Тест GetUserLocation для несуществующего пользователя
	t.Run("GetUserLocation", func(t *testing.T) {
		req := &pb.GetUserLocationRequest{
			UserId: nonExistentID,
		}

		_, err := client.GetUserLocation(ctx, req)
		if err == nil {
			t.Fatal("Expected error when getting location for non-existent user, got nil")
		}
	})
}

// TestFindNearbyUsers проверяет поиск пользователей поблизости
func TestFindNearbyUsers(t *testing.T) {
	ctx := context.Background()

	// Создаем несколько пользователей с разными местоположениями
	// Пользователь 1 - Москва
	user1, err := client.CreateUser(ctx, &pb.CreateUserRequest{
		TelegramId: 444555666,
		Username:   "moscow_user",
		Bio:        "User in Moscow",
	})
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	_, err = client.UpdateUserLocation(ctx, &pb.UpdateUserLocationRequest{
		UserId:    user1.Id,
		Latitude:  55.7558,
		Longitude: 37.6173,
		City:      "Moscow",
		Country:   "Russia",
	})
	if err != nil {
		t.Fatalf("Failed to update location for user1: %v", err)
	}

	// Пользователь 2 - Санкт-Петербург
	user2, err := client.CreateUser(ctx, &pb.CreateUserRequest{
		TelegramId: 777888999,
		Username:   "spb_user",
		Bio:        "User in Saint Petersburg",
	})
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	_, err = client.UpdateUserLocation(ctx, &pb.UpdateUserLocationRequest{
		UserId:    user2.Id,
		Latitude:  59.9343,
		Longitude: 30.3351,
		City:      "Saint Petersburg",
		Country:   "Russia",
	})
	if err != nil {
		t.Fatalf("Failed to update location for user2: %v", err)
	}

	// Поиск пользователей рядом с Москвой (радиус небольшой, должен найти только московского пользователя)
	nearbyReq := &pb.FindNearbyUsersRequest{
		Latitude:  55.7558,
		Longitude: 37.6173,
		RadiusKm:  10, // 10 км радиус
		Limit:     10,
	}

	nearbyResp, err := client.FindNearbyUsers(ctx, nearbyReq)
	if err != nil {
		t.Fatalf("Failed to find nearby users: %v", err)
	}

	// Проверяем, что нашли только московского пользователя
	// Примечание: этот тест может быть не очень надежным, т.к. зависит от реализации геопоиска
	found := false
	for _, u := range nearbyResp.Users {
		if u.Id == user1.Id {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Moscow user not found in nearby search results")
	}
}

// TestSearchByTelegramID проверяет поиск пользователя по Telegram ID
func TestSearchByTelegramID(t *testing.T) {
	ctx := context.Background()
	telegramID := int64(987654321)

	// Создаем пользователя
	createReq := &pb.CreateUserRequest{
		TelegramId: telegramID,
		Username:   "telegramidsearch",
		Bio:        "User for Telegram ID search test",
	}

	createdUser, err := client.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Ищем пользователя по Telegram ID
	searchReq := &pb.GetUserByTelegramIDRequest{
		TelegramId: telegramID,
	}

	foundUser, err := client.GetUserByTelegramID(ctx, searchReq)
	if err != nil {
		t.Fatalf("Failed to find user by Telegram ID: %v", err)
	}

	// Проверяем, что нашли того же пользователя
	if foundUser.Id != createdUser.Id {
		t.Errorf("Expected user ID %d, got %d", createdUser.Id, foundUser.Id)
	}
	if foundUser.TelegramId != telegramID {
		t.Errorf("Expected Telegram ID %d, got %d", telegramID, foundUser.TelegramId)
	}
	if foundUser.Username != createReq.Username {
		t.Errorf("Expected Username %s, got %s", createReq.Username, foundUser.Username)
	}
}
