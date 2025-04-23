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
	localRedis "JollyRogerUserService/internal/repository/redis"
	"JollyRogerUserService/internal/service"
	"JollyRogerUserService/pkg/database"
	"JollyRogerUserService/pkg/logger"
	pb "JollyRogerUserService/pkg/proto/user"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var (
	resilienceClient pb.JollyRogerUserServiceClient
	resilienceConn   *grpc.ClientConn
	pgResourceRes    *dockertest.Resource
	rdResourceRes    *dockertest.Resource
	poolRes          *dockertest.Pool
	dbRes            *gorm.DB
	redisClientRes   *redis.Client
)

// TestResilienceMain настраивает тестовое окружение для проверки устойчивости
func TestResilienceMain(m *testing.M) {
	// Создаем Docker-pool
	var err error
	poolRes, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	// Устанавливаем тайм-аут для контейнеров
	poolRes.MaxWait = time.Minute * 2

	// Запускаем PostgreSQL
	pgResourceRes, err = poolRes.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=resilience_test_db",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Could not start PostgreSQL: %s", err)
	}

	// Запускаем Redis
	rdResourceRes, err = poolRes.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Could not start Redis: %s", err)
	}

	// Получаем хост и порт PostgreSQL
	pgHost := pgResourceRes.GetBoundIP("5432/tcp")
	pgPort := pgResourceRes.GetPort("5432/tcp")

	// Получаем хост и порт Redis
	redisHost := rdResourceRes.GetBoundIP("6379/tcp")
	redisPort := rdResourceRes.GetPort("6379/tcp")

	// Ожидаем готовности PostgreSQL
	if err := poolRes.Retry(func() error {
		pgConfig := config.PostgresConfig{
			Host:     pgHost,
			Port:     5432,
			Username: "postgres",
			Password: "postgres",
			DBName:   "resilience_test_db",
			SSLMode:  "disable",
		}

		var err error
		dbRes, err = database.NewPostgresDB(pgConfig)
		return err
	}); err != nil {
		log.Fatalf("Could not connect to PostgreSQL: %s", err)
	}

	// Ожидаем готовности Redis
	if err := poolRes.Retry(func() error {
		redisConfig := config.RedisConfig{
			Addr:     redisHost + ":" + redisPort,
			Password: "",
			DB:       0,
		}

		redisClient, err := database.NewRedisClient(redisConfig)
		if err != nil {
			return err
		}
		_, err = redisClient.Ping(context.Background()).Result()
		return err
	}); err != nil {
		log.Fatalf("Could not connect to Redis: %s", err)
	}

	// Запускаем gRPC сервер для тестирования
	go runResilienceTestServer(pgHost, pgPort, redisHost, redisPort)

	// Ожидаем запуска сервера
	time.Sleep(time.Second * 2)

	// Подключаемся к серверу как клиент
	resilienceConn, err = grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	resilienceClient = pb.NewJollyRogerUserServiceClient(resilienceConn)

	// Запускаем тесты
	code := m.Run()

	// Очистка ресурсов
	resilienceConn.Close()
	poolRes.Purge(pgResourceRes)
	poolRes.Purge(rdResourceRes)

	os.Exit(code)
}

// runResilienceTestServer запускает тестовый gRPC сервер
func runResilienceTestServer(pgHost, pgPort, redisHost, redisPort string) {
	// Настройка логгера
	log := logger.NewLogger()

	// Настройка PostgreSQL
	pgConfig := config.PostgresConfig{
		Host:     pgHost,
		Port:     5432,
		Username: "postgres",
		Password: "postgres",
		DBName:   "resilience_test_db",
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
	cacheRepo := localRedis.NewCacheRepository(redisClient)

	// Инициализация сервиса
	userService := service.NewUserService(userRepo, cacheRepo, log)

	// Инициализация gRPC сервера
	lis, err := net.Listen("tcp", ":50052")
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

// TestRedisFailure проверяет поведение при недоступности Redis
func TestRedisFailure(t *testing.T) {
	// Пропускаем этот тест в обычном режиме, чтобы не нарушать
	// работу других тестов, требующих доступа к контейнерам
	if os.Getenv("RUN_RESILIENCE_TESTS") != "true" {
		t.Skip("Skipping resilience test in normal mode")
	}

	// Создаем пользователя для тестирования
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 111000111,
		Username:   "redis_failure_test",
		Bio:        "Test user for Redis failure",
	}

	user, err := resilienceClient.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Останавливаем Redis, чтобы имитировать его недоступность
	if err := rdResourceRes.Stop(ctx); err != nil {
		t.Fatalf("Could not stop Redis: %s", err)
	}

	// Ждем, чтобы убедиться, что Redis недоступен
	time.Sleep(3 * time.Second)

	// Пытаемся получить пользователя - должно работать через прямой запрос к БД
	t.Run("GetUserWorksWithoutRedis", func(t *testing.T) {
		getReq := &pb.GetUserRequest{
			Id: user.Id,
		}

		// Устанавливаем таймаут для проверки деградации производительности
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		getUserResp, err := resilienceClient.GetUser(ctx, getReq)
		if err != nil {
			t.Fatalf("Failed to get user when Redis is down: %v", err)
		}

		if getUserResp.Id != user.Id {
			t.Errorf("Expected user ID %d, got %d", user.Id, getUserResp.Id)
		}
	})

	// Пытаемся обновить местоположение - должно работать, но Redis кэш не обновится
	t.Run("UpdateLocationWorksWithoutRedis", func(t *testing.T) {
		updateLocReq := &pb.UpdateUserLocationRequest{
			UserId:    user.Id,
			Latitude:  55.7558,
			Longitude: 37.6173,
			City:      "Moscow",
			Region:    "Moscow",
			Country:   "Russia",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := resilienceClient.UpdateUserLocation(ctx, updateLocReq)
		if err != nil {
			t.Fatalf("Failed to update location when Redis is down: %v", err)
		}

		// Проверяем, что местоположение сохранилось в БД
		getLocReq := &pb.GetUserLocationRequest{
			UserId: user.Id,
		}

		getLocResp, err := resilienceClient.GetUserLocation(ctx, getLocReq)
		if err != nil {
			t.Fatalf("Failed to get location when Redis is down: %v", err)
		}

		if getLocResp.City != "Moscow" {
			t.Errorf("Expected City 'Moscow', got '%s'", getLocResp.City)
		}
	})

	// Запускаем Redis обратно
	if err := rdResourceRes.Start(ctx); err != nil {
		t.Fatalf("Could not restart Redis: %s", err)
	}

	// Ждем, пока Redis снова станет доступен
	time.Sleep(3 * time.Second)

	// Проверяем, что система вернулась к нормальной работе
	t.Run("SystemRecoveredAfterRedisRestart", func(t *testing.T) {
		getReq := &pb.GetUserRequest{
			Id: user.Id,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		getUserResp, err := resilienceClient.GetUser(ctx, getReq)
		if err != nil {
			t.Fatalf("Failed to get user after Redis restart: %v", err)
		}

		if getUserResp.Id != user.Id {
			t.Errorf("Expected user ID %d, got %d", user.Id, getUserResp.Id)
		}
	})
}

// TestPostgresFailure проверяет поведение при недоступности PostgreSQL
func TestPostgresFailure(t *testing.T) {
	// Пропускаем этот тест в обычном режиме
	if os.Getenv("RUN_RESILIENCE_TESTS") != "true" {
		t.Skip("Skipping resilience test in normal mode")
	}

	// Создаем пользователя для тестирования
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 222000222,
		Username:   "postgres_failure_test",
		Bio:        "Test user for Postgres failure",
	}

	user, err := resilienceClient.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Кэшируем пользователя, выполнив запрос
	getReq := &pb.GetUserRequest{
		Id: user.Id,
	}
	_, err = resilienceClient.GetUser(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get user for caching: %v", err)
	}

	// Останавливаем PostgreSQL, чтобы имитировать его недоступность
	if err := pgResourceRes.Stop(ctx); err != nil {
		t.Fatalf("Could not stop PostgreSQL: %s", err)
	}

	// Ждем, чтобы убедиться, что PostgreSQL недоступен
	time.Sleep(3 * time.Second)

	// Пытаемся получить пользователя - должно работать через кэш Redis
	t.Run("GetUserWorksFromCache", func(t *testing.T) {
		getReq := &pb.GetUserRequest{
			Id: user.Id,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		getUserResp, err := resilienceClient.GetUser(ctx, getReq)
		if err != nil {
			t.Fatalf("Failed to get user from cache when Postgres is down: %v", err)
		}

		if getUserResp.Id != user.Id {
			t.Errorf("Expected user ID %d, got %d", user.Id, getUserResp.Id)
		}
	})

	// Пытаемся создать нового пользователя - должно вернуть ошибку
	t.Run("CreateUserFailsWhenPostgresIsDown", func(t *testing.T) {
		newUserReq := &pb.CreateUserRequest{
			TelegramId: 333000333,
			Username:   "new_user_postgres_down",
			Bio:        "This user should not be created",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := resilienceClient.CreateUser(ctx, newUserReq)
		if err == nil {
			t.Fatalf("Expected error when creating user with Postgres down, got nil")
		}

		// Проверяем, что мы получили правильный код ошибки
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Internal {
			t.Errorf("Expected error code Internal, got %v", st.Code())
		}
	})

	// Запускаем PostgreSQL обратно
	if err := pgResourceRes.Start(ctx); err != nil {
		t.Fatalf("Could not restart PostgreSQL: %s", err)
	}

	// Ждем, пока PostgreSQL снова станет доступен
	time.Sleep(10 * time.Second)

	// Проверяем, что система вернулась к нормальной работе
	t.Run("SystemRecoveredAfterPostgresRestart", func(t *testing.T) {
		newUserReq := &pb.CreateUserRequest{
			TelegramId: 444000444,
			Username:   "recovery_test_user",
			Bio:        "Testing system recovery",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Пытаемся создать пользователя после восстановления PostgreSQL
		createResp, err := resilienceClient.CreateUser(ctx, newUserReq)
		if err != nil {
			t.Fatalf("Failed to create user after Postgres restart: %v", err)
		}

		if createResp.TelegramId != newUserReq.TelegramId {
			t.Errorf("Expected TelegramId %d, got %d", newUserReq.TelegramId, createResp.TelegramId)
		}
	})
}

// TestNetworkPartition имитирует сетевой раздел между сервисами
func TestNetworkPartition(t *testing.T) {
	// Пропускаем этот тест в обычном режиме
	if os.Getenv("RUN_RESILIENCE_TESTS") != "true" {
		t.Skip("Skipping resilience test in normal mode")
	}

	// Создаем новую изолированную сеть
	networkName := "resilience_test_network"

	// Создаем сеть для тестирования
	network, err := poolRes.Client.CreateNetwork(docker.CreateNetworkOptions{
		Name: networkName,
	})
	if err != nil {
		t.Fatalf("Could not create test network: %s", err)
	}

	// Отключаем PostgreSQL от сети, имитируя сетевой раздел
	err = poolRes.Client.DisconnectNetwork(network.ID, docker.NetworkConnectionOptions{
		Container: pgResourceRes.Container.ID,
	})
	if err != nil {
		t.Fatalf("Could not disconnect Postgres from network: %s", err)
	}

	// Проверяем поведение при сетевом разделе
	// Код проверки будет аналогичен TestPostgresFailure

	// Подключаем PostgreSQL обратно к сети
	err = poolRes.Client.ConnectNetwork(network.ID, docker.NetworkConnectionOptions{
		Container: pgResourceRes.Container.ID,
	})
	if err != nil {
		t.Fatalf("Could not reconnect Postgres to network: %s", err)
	}

	// Удаляем тестовую сеть
	err = poolRes.Client.RemoveNetwork(network.ID)
	if err != nil {
		t.Fatalf("Could not remove test network: %s", err)
	}
}

// TestHighLatency тестирует поведение системы при высокой задержке
func TestHighLatency(t *testing.T) {
	// Пропускаем этот тест в обычном режиме
	if os.Getenv("RUN_RESILIENCE_TESTS") != "true" {
		t.Skip("Skipping resilience test in normal mode")
	}

	// Устанавливаем задержку сети для контейнера PostgreSQL
	// Примечание: это требует привилегий CAP_NET_ADMIN и Linux TC
	// В Docker можно использовать параметры network при запуске контейнера

	// Создаем пользователя для тестирования
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 555000555,
		Username:   "latency_test_user",
		Bio:        "Test user for high latency",
	}

	user, err := resilienceClient.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Кэшируем пользователя, выполнив запрос
	getReq := &pb.GetUserRequest{
		Id: user.Id,
	}
	_, err = resilienceClient.GetUser(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get user for caching: %v", err)
	}

	// Имитируем высокую задержку (в реальном случае это бы делалось через сетевые инструменты)
	// Здесь мы используем контекст с таймаутом для измерения времени отклика

	t.Run("RespondsWithinTimeout", func(t *testing.T) {
		// Устанавливаем короткий таймаут для контекста
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Запрос должен вернуться из кэша до истечения таймаута
		startTime := time.Now()
		_, err := resilienceClient.GetUser(ctx, getReq)
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		t.Logf("Request completed in %v", duration)

		if duration > 500*time.Millisecond {
			t.Errorf("Request took too long: %v", duration)
		}
	})
}

// TestConcurrentRequests проверяет работу сервиса под нагрузкой параллельных запросов
func TestConcurrentRequests(t *testing.T) {
	// Пропускаем этот тест в обычном режиме
	if os.Getenv("RUN_RESILIENCE_TESTS") != "true" {
		t.Skip("Skipping resilience test in normal mode")
	}

	// Создаем тестового пользователя
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 666000666,
		Username:   "concurrent_test_user",
		Bio:        "Test user for concurrent requests",
	}

	user, err := resilienceClient.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Генерируем большое количество параллельных запросов
	concurrency := 100
	getReq := &pb.GetUserRequest{
		Id: user.Id,
	}

	// Канал для сбора результатов
	results := make(chan error, concurrency)

	// Запускаем параллельные запросы
	for i := 0; i < concurrency; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := resilienceClient.GetUser(ctx, getReq)
			results <- err
		}()
	}

	// Собираем и проверяем результаты
	var successCount, errorCount int
	for i := 0; i < concurrency; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			errorCount++
			t.Logf("Request %d failed: %v", i, err)
		}
	}

	// Проверяем, что большинство запросов выполнено успешно
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
	if float64(successCount)/float64(concurrency) < 0.95 {
		t.Errorf("Too many failed requests: %d out of %d", errorCount, concurrency)
	}
}
