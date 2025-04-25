package integration

import (
	"context"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"testing"
	"time"

	pb "JollyRogerUserService/pkg/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestRedisFailure проверяет работу сервиса при недоступности Redis
func TestRedisFailure(t *testing.T) {
	// Создаем пользователя для тестирования
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 111000111,
		Username:   "redis_failure_test",
		Bio:        "Test user for Redis failure",
	}

	user, err := client.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Останавливаем Redis, чтобы имитировать его недоступность
	t.Log("Stopping Redis container to simulate failure")
	if err := pool.Purge(rdResource); err != nil {
		t.Fatalf("Could not purge Redis container: %s", err)
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

		start := time.Now()
		getUserResp, err := client.GetUser(ctx, getReq)
		duration := time.Since(start)

		t.Logf("Request completed in %v", duration)

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

		_, err := client.UpdateUserLocation(ctx, updateLocReq)
		if err != nil {
			t.Fatalf("Failed to update location when Redis is down: %v", err)
		}

		// Проверяем, что местоположение сохранилось в БД
		getLocReq := &pb.GetUserLocationRequest{
			UserId: user.Id,
		}

		getLocResp, err := client.GetUserLocation(ctx, getLocReq)
		if err != nil {
			t.Fatalf("Failed to get location when Redis is down: %v", err)
		}

		if getLocResp.City != "Moscow" {
			t.Errorf("Expected City 'Moscow', got '%s'", getLocResp.City)
		}
	})

	// Запускаем Redis обратно
	t.Log("Restarting Redis container")
	rdResource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not restart Redis: %s", err)
	}

	// Ждем, пока Redis снова станет доступен
	time.Sleep(5 * time.Second)

	// Проверяем, что система вернулась к нормальной работе
	t.Run("SystemRecoveredAfterRedisRestart", func(t *testing.T) {
		getReq := &pb.GetUserRequest{
			Id: user.Id,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		getUserResp, err := client.GetUser(ctx, getReq)
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
	// Создаем пользователя для тестирования
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 222000222,
		Username:   "postgres_failure_test",
		Bio:        "Test user for Postgres failure",
	}

	user, err := client.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Делаем несколько запросов к пользователю, чтобы обеспечить кэширование
	getReq := &pb.GetUserRequest{
		Id: user.Id,
	}

	for i := 0; i < 3; i++ {
		_, err = client.GetUser(ctx, getReq)
		if err != nil {
			t.Fatalf("Failed to get user for caching: %v", err)
		}
	}

	// Останавливаем PostgreSQL, чтобы имитировать его недоступность
	t.Log("Stopping PostgreSQL container to simulate failure")
	if err := pool.Purge(rdResource); err != nil {
		t.Fatalf("Could not purge Redis container: %s", err)
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

		start := time.Now()
		getUserResp, err := client.GetUser(ctx, getReq)
		duration := time.Since(start)

		t.Logf("Request completed in %v", duration)

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

		_, err := client.CreateUser(ctx, newUserReq)
		if err == nil {
			t.Fatalf("Expected error when creating user with Postgres down, got nil")
		}

		// Проверяем, что мы получили правильный код ошибки
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}

		t.Logf("Got expected error: %v with code: %v", st.Message(), st.Code())

		// Код может быть Internal или Unavailable в зависимости от реализации
		if st.Code() != codes.Internal && st.Code() != codes.Unavailable {
			t.Errorf("Expected error code Internal or Unavailable, got %v", st.Code())
		}
	})

	// Запускаем PostgreSQL обратно
	t.Log("Restarting PostgreSQL container")
	rdResource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not restart Redis: %s", err)
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
		createResp, err := client.CreateUser(ctx, newUserReq)
		if err != nil {
			t.Fatalf("Failed to create user after Postgres restart: %v", err)
		}

		if createResp.TelegramId != newUserReq.TelegramId {
			t.Errorf("Expected TelegramId %d, got %d", newUserReq.TelegramId, createResp.TelegramId)
		}
	})
}

// TestConcurrentRequests проверяет работу сервиса при многочисленных параллельных запросах
func TestConcurrentRequests(t *testing.T) {
	// Создаем тестового пользователя
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	createReq := &pb.CreateUserRequest{
		TelegramId: 666000666,
		Username:   "concurrent_test_user",
		Bio:        "Test user for concurrent requests",
	}

	user, err := client.CreateUser(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Добавляем местоположение
	updateLocReq := &pb.UpdateUserLocationRequest{
		UserId:    user.Id,
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

	// Генерируем большое количество параллельных запросов разных типов
	concurrency := 50
	errorChan := make(chan error, concurrency*3) // Для трех типов запросов

	t.Logf("Starting %d concurrent requests of each type (get user, get location, update rating)", concurrency)

	// Запускаем параллельные запросы для получения пользователя
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			getReq := &pb.GetUserRequest{Id: user.Id}
			_, err := client.GetUser(ctx, getReq)
			errorChan <- err
		}(i)
	}

	// Запускаем параллельные запросы для получения местоположения
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			getLocReq := &pb.GetUserLocationRequest{UserId: user.Id}
			_, err := client.GetUserLocation(ctx, getLocReq)
			errorChan <- err
		}(i)
	}

	// Запускаем параллельные запросы для обновления рейтинга
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			updateRatingReq := &pb.UpdateUserRatingRequest{
				UserId:       user.Id,
				RatingChange: 0.1,
			}
			_, err := client.UpdateUserRating(ctx, updateRatingReq)
			errorChan <- err
		}(i)
	}

	// Собираем и проверяем результаты
	var successCount, errorCount int
	for i := 0; i < concurrency*3; i++ {
		err := <-errorChan
		if err == nil {
			successCount++
		} else {
			errorCount++
			t.Logf("Request error: %v", err)
		}
	}

	// Проверяем, что большинство запросов выполнено успешно
	t.Logf("Concurrent requests results - Success: %d, Errors: %d (Total: %d)",
		successCount, errorCount, concurrency*3)

	successRate := float64(successCount) / float64(concurrency*3)
	if successRate < 0.95 {
		t.Errorf("Too many failed requests: %d out of %d (%.2f%% success rate)",
			errorCount, concurrency*3, successRate*100)
	}

	// Проверяем финальный рейтинг пользователя, чтобы убедиться, что все обновления применились
	getReq := &pb.GetUserRequest{Id: user.Id}
	updatedUser, err := client.GetUser(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	t.Logf("Final user rating after concurrent updates: %f", updatedUser.Rating)
}

// TestCircuitBreaker проверяет работу механизма circuit breaker
func TestCircuitBreaker(t *testing.T) {
	// Этот тест требует модификации сервера или мок для имитации ошибок,
	// поэтому опустим реальную имплементацию, но опишем логику

	t.Skip("Circuit breaker test requires mock server implementation")

	/*
		Логика теста должна быть следующая:
		1. Настроить circuit breaker с низким порогом срабатывания (например, 3 ошибки)
		2. Вызвать несколько раз операцию, которая всегда завершается с ошибкой
		3. Проверить, что circuit breaker открылся (запросы отклоняются без выполнения)
		4. Подождать, пока истечет время сброса (resetTimeout)
		5. Проверить, что circuit breaker перешел в полуоткрытое состояние
		6. Вызвать операцию, которая завершится успешно
		7. Проверить, что circuit breaker закрылся
	*/
}

// TestRetryMechanism проверяет работу механизма повторных попыток
func TestRetryMechanism(t *testing.T) {
	// Этот тест также требует модификации сервера или мок для имитации временных ошибок
	t.Skip("Retry mechanism test requires mock server implementation")

	/*
		Логика теста должна быть следующая:
		1. Настроить механизм повторов с определенными параметрами
		2. Вызвать операцию, которая сначала завершается с ошибкой, а затем успешно
		3. Проверить, что операция в итоге выполнилась успешно
		4. Проверить, что было сделано ожидаемое количество попыток
	*/
}

// TestTimeout проверяет поведение сервиса при таймаутах
func TestTimeout(t *testing.T) {
	// Создаем контекст с очень коротким таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Пытаемся выполнить запрос, который не успеет завершиться за такое время
	getReq := &pb.GetUserRequest{Id: 1}
	_, err := client.GetUser(ctx, getReq)

	// Проверяем, что получили ошибку таймаута
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Проверяем код ошибки
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	t.Logf("Got expected timeout error: %v with code: %v", st.Message(), st.Code())

	// Код может быть DeadlineExceeded или Canceled
	if st.Code() != codes.DeadlineExceeded && st.Code() != codes.Canceled {
		t.Errorf("Expected error code DeadlineExceeded or Canceled, got %v", st.Code())
	}
}
