package postgres

import (
	"JollyRogerUserService/internal/models"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"regexp"
	"strings"
	"testing"
)

// TestResilientUserRepository_CreateUser тестирует создание пользователя с отказоустойчивостью
func TestResilientUserRepository_CreateUser(t *testing.T) {
	// Настраиваем логгер
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Настраиваем тестовую БД
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с отказоустойчивостью
	repo := NewResilientUserRepository(db, redisClient, logger)

	// Тестовый пользователь
	user := &models.User{
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     0,
	}

	// Тест 1: Успешное создание пользователя
	t.Run("SuccessfulCreate", func(t *testing.T) {
		// Настраиваем ожидаемое поведение SQL-мока
		mock.ExpectBegin() // Ожидаем начало транзакции

		// Проверка существования пользователя с таким telegram_id
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"})
		mock.ExpectQuery(`SELECT \* FROM "users" WHERE telegram_id = \$1 LIMIT \$2`).
			WithArgs(user.TelegramID, 1).
			WillReturnRows(rows)

		// Ожидаем вставку пользователя
		mock.ExpectQuery(`INSERT INTO "users" (.+) VALUES (.+) RETURNING "id"`).
			WithArgs(user.TelegramID, user.Username, user.Bio, user.Rating).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		// Ожидаем вставку статистики
		mock.ExpectQuery(`INSERT INTO "user_stats" (.+) VALUES (.+) RETURNING "id"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		// Ожидаем вставку настроек уведомлений
		mock.ExpectQuery(`INSERT INTO "user_notification_settings" (.+) VALUES (.+) RETURNING "id"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		mock.ExpectCommit() // Ожидаем коммит транзакции

		// Выполняем тестируемый метод
		err := repo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем, что ID был установлен
		if user.ID != 1 {
			t.Errorf("Expected user ID to be set to 1, got %d", user.ID)
		}
	})

	// Тест 2: Обработка ошибки при создании пользователя
	t.Run("ErrorHandling", func(t *testing.T) {
		// Создаем нового пользователя для теста
		userWithError := &models.User{
			TelegramID: 987654321,
			Username:   "erroruser",
			Bio:        "Error test",
			Rating:     0,
		}

		// Настраиваем ожидаемое поведение SQL-мока
		mock.ExpectBegin()

		// Проверка существования пользователя
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"})
		mock.ExpectQuery(`SELECT \* FROM "users" WHERE telegram_id = \$1 LIMIT \$2`).
			WithArgs(userWithError.TelegramID, 1).
			WillReturnRows(rows)

		// Симулируем ошибку при вставке пользователя
		mock.ExpectQuery(`INSERT INTO "users" (.+) VALUES (.+) RETURNING "id"`).
			WithArgs(userWithError.TelegramID, userWithError.Username, userWithError.Bio, userWithError.Rating).
			WillReturnError(errors.New("database error"))

		mock.ExpectRollback() // Ожидаем откат транзакции

		// Выполняем тестируемый метод
		err := repo.Create(userWithError)

		// Проверяем, что была возвращена ошибка
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

// TestResilientUserRepository_GetByID тестирует получение пользователя по ID с отказоустойчивостью
func TestResilientUserRepository_GetByID(t *testing.T) {
	// Настраиваем логгер
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Настраиваем тестовую БД
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с отказоустойчивостью
	repo := NewResilientUserRepository(db, redisClient, logger)

	// ID для тестирования
	userID := uint(1)

	// Тест 1: Успешное получение пользователя
	t.Run("SuccessfulGet", func(t *testing.T) {
		// Настраиваем ожидаемое поведение SQL-мока
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
			AddRow(userID, 123456789, "testuser", "Test bio", 0)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		user, err := repo.GetByID(userID)
		if err != nil {
			t.Fatalf("Failed to get user by ID: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем полученные данные
		if user.ID != userID {
			t.Errorf("Expected user ID %d, got %d", userID, user.ID)
		}
		if user.Username != "testuser" {
			t.Errorf("Expected Username 'testuser', got '%s'", user.Username)
		}
	})

	// Тест 2: Повторные попытки при ошибке
	t.Run("RetryOnError", func(t *testing.T) {
		// Настраиваем ожидаемое поведение SQL-мока
		// Первый запрос завершается ошибкой
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnError(errors.New("temporary network error"))

		// Второй запрос успешен (это будет повторная попытка)
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
			AddRow(userID, 123456789, "testuser", "Test bio", 0)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		user, err := repo.GetByID(userID)
		if err != nil {
			t.Fatalf("Failed to get user after retry: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем полученные данные
		if user.ID != userID {
			t.Errorf("Expected user ID %d, got %d", userID, user.ID)
		}
	})

	// Тест 3: Обработка отсутствия пользователя
	t.Run("UserNotFound", func(t *testing.T) {
		nonExistentID := uint(999)

		// Настраиваем ожидаемое поведение SQL-мока для ПЕРВОГО запроса
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(nonExistentID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		// Настраиваем ожидаемое поведение SQL-мока для ПОВТОРНОЙ попытки
		// ResilientUserRepository может попытаться выполнить повторный запрос при ошибке
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(nonExistentID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		// Настраиваем ожидаемое поведение SQL-мока для ПОВТОРНОЙ попытки
		// ResilientUserRepository может попытаться выполнить повторный запрос при ошибке
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(nonExistentID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		// Выполняем тестируемый метод
		_, err := repo.GetByID(nonExistentID)

		// Проверяем, что была возвращена ошибка о ненайденной записи
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("Expected gorm.ErrRecordNotFound, got %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

// TestResilientUserRepository_UpdateUserRating тестирует обновление рейтинга с отказоустойчивостью
func TestResilientUserRepository_UpdateUserRating(t *testing.T) {
	// Настраиваем логгер
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Настраиваем тестовую БД
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с отказоустойчивостью
	repo := NewResilientUserRepository(db, redisClient, logger)

	// Данные для тестирования
	userID := uint(1)
	initialRating := float32(3.0)
	ratingChange := float32(2.5)

	// Тест 1: Успешное обновление рейтинга
	t.Run("SuccessfulUpdate", func(t *testing.T) {
		// Получение текущего пользователя
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
			AddRow(userID, 123456789, "testuser", "Test bio", initialRating)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnRows(rows)

		// Обновление рейтинга - GORM обновляет все поля пользователя
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "users" SET "telegram_id"=$1,"username"=$2,"bio"=$3,"rating"=$4 WHERE "id" = $5`)).
			WithArgs(int64(123456789), "testuser", "Test bio", initialRating+ratingChange, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		// Выполняем тестируемый метод
		err := repo.UpdateUserRating(userID, ratingChange)
		if err != nil {
			t.Fatalf("Failed to update user rating: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

// TestResilientUserRepository_FindNearbyUsers тестирует поиск пользователей рядом
func TestResilientUserRepository_FindNearbyUsers(t *testing.T) {
	// Настраиваем логгер
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Настраиваем тестовую БД
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с отказоустойчивостью
	repo := NewResilientUserRepository(db, redisClient, logger)

	// Данные для тестирования
	lat := 55.7558
	lon := 37.6173
	radiusKm := 5.0
	limit := 10

	// Тест: Успешный поиск пользователей
	t.Run("SuccessfulFind", func(t *testing.T) {
		// Очищаем оставшиеся ожидания
		mock.ExpectationsWereMet()

		// Настраиваем ожидаемое поведение SQL-мока
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
			AddRow(1, 123456789, "user1", "Bio 1", 4.5).
			AddRow(2, 987654321, "user2", "Bio 2", 3.7)

		// Важно: используем ТОЧНОЕ соответствие SQL запроса, который фактически выполняется
		// Это запрос, который генерируется в файле user_repository.go в методе FindNearbyUsers
		expectedSQL := `
            SELECT u.* FROM users u
            JOIN user_locations l ON u.id = l.user_id
            WHERE (
                6371 * acos(
                    cos(radians($1)) * cos(radians(l.latitude)) * 
                    cos(radians(l.longitude) - radians($2)) + 
                    sin(radians($3)) * sin(radians(l.latitude))
                )
            ) <= $4
            LIMIT $5
        `
		// Убираем лишние пробелы и переводы строк
		expectedSQL = strings.ReplaceAll(expectedSQL, "\n", " ")
		expectedSQL = regexp.MustCompile(`\s+`).ReplaceAllString(expectedSQL, " ")
		expectedSQL = strings.TrimSpace(expectedSQL)

		mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
			WithArgs(lat, lon, lat, radiusKm, limit).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		users, err := repo.FindNearbyUsers(lat, lon, radiusKm, limit)
		if err != nil {
			t.Fatalf("Failed to find nearby users: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем количество найденных пользователей
		if len(users) != 2 {
			t.Errorf("Expected to find 2 users, got %d", len(users))
		}

		// Проверяем данные пользователей
		if users[0].Username != "user1" || users[1].Username != "user2" {
			t.Errorf("User data mismatch: %v", users)
		}
	})

	// Тест: Отсутствие пользователей рядом
	t.Run("NoNearbyUsers", func(t *testing.T) {
		// Очищаем оставшиеся ожидания
		mock.ExpectationsWereMet()

		// Настраиваем ожидаемое поведение SQL-мока - пустой результат
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"})

		// Используем тот же точный SQL запрос
		expectedSQL := `
            SELECT u.* FROM users u
            JOIN user_locations l ON u.id = l.user_id
            WHERE (
                6371 * acos(
                    cos(radians($1)) * cos(radians(l.latitude)) * 
                    cos(radians(l.longitude) - radians($2)) + 
                    sin(radians($3)) * sin(radians(l.latitude))
                )
            ) <= $4
            LIMIT $5
        `
		// Убираем лишние пробелы и переводы строк
		expectedSQL = strings.ReplaceAll(expectedSQL, "\n", " ")
		expectedSQL = regexp.MustCompile(`\s+`).ReplaceAllString(expectedSQL, " ")
		expectedSQL = strings.TrimSpace(expectedSQL)

		mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
			WithArgs(lat, lon, lat, radiusKm, limit).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		users, err := repo.FindNearbyUsers(lat, lon, radiusKm, limit)
		if err != nil {
			t.Fatalf("Failed to execute find nearby users: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем, что результат пуст
		if len(users) != 0 {
			t.Errorf("Expected empty result, got %d users", len(users))
		}
	})

	// Тест: Обработка ошибки при поиске
	t.Run("ErrorHandling", func(t *testing.T) {
		// Очищаем оставшиеся ожидания
		mock.ExpectationsWereMet()

		// Используем тот же точный SQL запрос
		expectedSQL := `
            SELECT u.* FROM users u
            JOIN user_locations l ON u.id = l.user_id
            WHERE (
                6371 * acos(
                    cos(radians($1)) * cos(radians(l.latitude)) * 
                    cos(radians(l.longitude) - radians($2)) + 
                    sin(radians($3)) * sin(radians(l.latitude))
                )
            ) <= $4
            LIMIT $5
        `
		// Убираем лишние пробелы и переводы строк
		expectedSQL = strings.ReplaceAll(expectedSQL, "\n", " ")
		expectedSQL = regexp.MustCompile(`\s+`).ReplaceAllString(expectedSQL, " ")
		expectedSQL = strings.TrimSpace(expectedSQL)

		// Симулируем ошибку при выполнении запроса
		mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
			WithArgs(lat, lon, lat, radiusKm, limit).
			WillReturnError(errors.New("database query error"))

		// Выполняем тестируемый метод
		_, err := repo.FindNearbyUsers(lat, lon, radiusKm, limit)

		// Проверяем, что была возвращена ошибка
		if err == nil {
			t.Fatal("Expected error during search, got nil")
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

// TestResilientUserRepository_GetNotificationSettings тестирует получение настроек уведомлений
func TestResilientUserRepository_GetNotificationSettings(t *testing.T) {
	// Настраиваем логгер
	logger := zap.NewNop()

	// Создаем тестовый Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mini redis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Настраиваем тестовую БД
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с отказоустойчивостью
	repo := NewResilientUserRepository(db, redisClient, logger)

	// Данные для тестирования
	userID := uint(1)

	// Тест: Успешное получение настроек
	t.Run("SuccessfulGet", func(t *testing.T) {
		// Настраиваем ожидаемое поведение SQL-мока
		rows := sqlmock.NewRows([]string{"id", "user_id", "new_event_notification"}).
			AddRow(1, userID, true)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_notification_settings" WHERE user_id = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		settings, err := repo.GetNotificationSettings(userID)
		if err != nil {
			t.Fatalf("Failed to get notification settings: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем настройки
		if !settings.NewEventNotification {
			t.Error("Expected NewEventNotification to be true")
		}
	})

	// Тест: Настройки не найдены
	t.Run("SettingsNotFound", func(t *testing.T) {
		nonExistentID := uint(999)

		// Настраиваем ожидаемое поведение SQL-мока
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_notification_settings" WHERE user_id = $1 LIMIT $2`)).
			WithArgs(nonExistentID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		// Выполняем тестируемый метод
		_, err := repo.GetNotificationSettings(nonExistentID)

		// Проверяем, что была возвращена ошибка
		if err == nil {
			t.Fatal("Expected error for non-existent settings, got nil")
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})

	// Тест: Обработка временной ошибки БД
	t.Run("DatabaseErrorHandling", func(t *testing.T) {
		// Первый запрос завершается ошибкой
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_notification_settings" WHERE user_id = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnError(errors.New("temporary database error"))

		// Второй запрос (повторная попытка) успешен
		rows := sqlmock.NewRows([]string{"id", "user_id", "new_event_notification"}).
			AddRow(1, userID, true)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_notification_settings" WHERE user_id = $1 LIMIT $2`)).
			WithArgs(userID, 1).
			WillReturnRows(rows)

		// Выполняем тестируемый метод
		settings, err := repo.GetNotificationSettings(userID)
		if err != nil {
			t.Fatalf("Failed to get notification settings after retry: %v", err)
		}

		// Проверяем, что все ожидания мока были удовлетворены
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}

		// Проверяем настройки
		if !settings.NewEventNotification {
			t.Error("Expected NewEventNotification to be true")
		}
	})
}
