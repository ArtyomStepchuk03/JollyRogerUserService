package postgres

import (
	"JollyRogerUserService/internal/models"
	"log"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB создает мок базы данных для тестов
func setupTestDB() (*gorm.DB, sqlmock.Sqlmock, error) {
	// Создаем мок SQL-соединения
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	// Создаем логгер для GORM
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Silent, // Тихий режим для тестов
			Colorful:      false,
		},
	)

	// Подключаем GORM к моку базы данных
	dialector := postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 mockDB,
		PreferSimpleProtocol: true,
	})

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, nil, err
	}

	return db, mock, nil
}

// TestCreateUser тестирует метод Create репозитория
func TestCreateUser(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// Тестовый пользователь
	user := &models.User{
		TelegramID: 123456789,
		Username:   "testuser",
		Bio:        "Test bio",
		Rating:     0,
	}

	// Настраиваем ожидаемое поведение SQL-мока
	mock.ExpectBegin() // Ожидаем начало транзакции

	// Проверка существования пользователя с таким telegram_id
	rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"})
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE telegram_id = $1 LIMIT $2`)).
		WithArgs(user.TelegramID, 1).
		WillReturnRows(rows) // Пустой результат - пользователь не найден

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
	err = repo.Create(user)
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
}

// TestGetByID тестирует метод GetByID репозитория
func TestGetByID(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// ID для тестирования
	userID := uint(1)

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
	if user.TelegramID != 123456789 {
		t.Errorf("Expected TelegramID 123456789, got %d", user.TelegramID)
	}
	if user.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", user.Username)
	}
}

// TestGetByTelegramID тестирует метод GetByTelegramID репозитория
func TestGetByTelegramID(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// TelegramID для тестирования
	telegramID := int64(123456789)

	// Настраиваем ожидаемое поведение SQL-мока
	rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
		AddRow(1, telegramID, "testuser", "Test bio", 0)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE telegram_id = $1 LIMIT $2`)).
		WithArgs(telegramID, 1).
		WillReturnRows(rows)

	// Выполняем тестируемый метод
	user, err := repo.GetByTelegramID(telegramID)
	if err != nil {
		t.Fatalf("Failed to get user by TelegramID: %v", err)
	}

	// Проверяем, что все ожидания мока были удовлетворены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}

	// Проверяем полученные данные
	if user.TelegramID != telegramID {
		t.Errorf("Expected TelegramID %d, got %d", telegramID, user.TelegramID)
	}
	if user.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", user.Username)
	}
}

// TestGetByTelegramIDNotFound тестирует случай, когда пользователь не найден
func TestGetByTelegramIDNotFound(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// TelegramID для тестирования
	telegramID := int64(999999)

	// Настраиваем ожидаемое поведение SQL-мока
	rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"})
	// Пустой результат - пользователь не найден

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE telegram_id = $1 LIMIT $2`)).
		WithArgs(telegramID, 1).
		WillReturnRows(rows)

	// Выполняем тестируемый метод
	_, err = repo.GetByTelegramID(telegramID)

	// Ожидаем ошибку
	if err == nil {
		t.Fatalf("Expected error when user not found, got nil")
	}

	// Проверяем, что все ожидания мока были удовлетворены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestUpdateUser тестирует метод Update репозитория
func TestUpdateUser(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// Тестовый пользователь
	user := &models.User{
		ID:         1,
		TelegramID: 123456789,
		Username:   "updateduser",
		Bio:        "Updated bio",
		Rating:     5.0,
	}

	// Настраиваем ожидаемое поведение SQL-мока
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET (.+) WHERE "id" = (.+)`).
		WithArgs(user.TelegramID, user.Username, user.Bio, user.Rating, user.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Выполняем тестируемый метод
	err = repo.Update(user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Проверяем, что все ожидания мока были удовлетворены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestAddPreference тестирует метод AddPreference репозитория
func TestAddPreference(t *testing.T) {
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	repo := NewUserRepository(db)

	preference := &models.UserPreference{
		UserID:    1,
		TagID:     10,
		CreatedAt: time.Now(),
	}

	// Проверка существования предпочтения (ожидаем пустой результат)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_preferences" WHERE user_id = $1 AND tag_id = $2 LIMIT $3`)).
		WithArgs(preference.UserID, preference.TagID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "tag_id", "created_at"})) // Пусто

	// Вставка нового предпочтения
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_preferences" ("user_id","tag_id","created_at") VALUES ($1,$2,$3) RETURNING "id"`)).
		WithArgs(preference.UserID, preference.TagID, sqlmock.AnyArg()). // CreatedAt может быть любым временем
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	err = repo.AddPreference(preference)
	if err != nil {
		t.Fatalf("Failed to add preference: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestRemovePreference тестирует метод RemovePreference репозитория
func TestRemovePreference(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// Данные для тестирования
	userID := uint(1)
	tagID := uint(10)

	// Настраиваем ожидаемое поведение SQL-мока
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "user_preferences" WHERE user_id = (.+) AND tag_id = (.+)`).
		WithArgs(userID, tagID).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 строка удалена
	mock.ExpectCommit()

	// Выполняем тестируемый метод
	err = repo.RemovePreference(userID, tagID)
	if err != nil {
		t.Fatalf("Failed to remove preference: %v", err)
	}

	// Проверяем, что все ожидания мока были удовлетворены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestUpdateUserRating тестирует метод UpdateUserRating репозитория
func TestUpdateUserRating(t *testing.T) {
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	repo := NewUserRepository(db)

	userID := uint(1)
	ratingChange := float32(2.5)
	initialRating := float32(3.0)
	finalRating := initialRating + ratingChange

	// Получаем текущего пользователя
	rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
		AddRow(userID, 123456789, "testuser", "Test bio", initialRating)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE "users"."id" = $1 LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(rows)

	// Обновление всех полей (стандартное поведение GORM)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "users" SET "telegram_id"=$1,"username"=$2,"bio"=$3,"rating"=$4 WHERE "id" = $5`)).
		WithArgs(123456789, "testuser", "Test bio", finalRating, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Выполняем метод
	err = repo.UpdateUserRating(userID, ratingChange)
	if err != nil {
		t.Fatalf("Failed to update user rating: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestUpdateLocation тестирует метод UpdateLocation репозитория
func TestUpdateLocation(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// Тестовое местоположение
	location := &models.UserLocation{
		UserID:    1,
		Latitude:  55.7558,
		Longitude: 37.6173,
		City:      "Moscow",
		Region:    "Moscow",
		Country:   "Russia",
		UpdatedAt: time.Now(),
	}

	// Настраиваем ожидаемое поведение SQL-мока
	// Проверка существования местоположения
	rows := sqlmock.NewRows([]string{"id", "user_id", "latitude", "longitude", "city", "region", "country", "updated_at"})
	mock.ExpectQuery(`SELECT \* FROM "user_locations" WHERE user_id = \$1 LIMIT \$2`).
		WithArgs(location.UserID, 1).
		WillReturnRows(rows)

	// Ожидаем вставку нового местоположения
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "user_locations" (.+) VALUES (.+) RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// Выполняем тестируемый метод
	err = repo.UpdateLocation(location)
	if err != nil {
		t.Fatalf("Failed to update location: %v", err)
	}

	// Проверяем, что все ожидания мока были удовлетворены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestFindNearbyUsers тестирует метод FindNearbyUsers репозитория
func TestFindNearbyUsers(t *testing.T) {
	// Настраиваем тестовую базу данных
	db, mock, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	// Создаем репозиторий с мок-базой
	repo := NewUserRepository(db)

	// Данные для тестирования
	lat := 55.7558
	lon := 37.6173
	radiusKm := 5.0
	limit := 10

	// Настраиваем ожидаемое поведение SQL-мока
	rows := sqlmock.NewRows([]string{"id", "telegram_id", "username", "bio", "rating"}).
		AddRow(1, 123456789, "user1", "Bio 1", 4.5).
		AddRow(2, 987654321, "user2", "Bio 2", 3.7)

	// Запрос с использованием SQL-функции для расчета расстояния
	mock.ExpectQuery(`SELECT u\.\* FROM users u JOIN user_locations l ON u\.id = l\.user_id WHERE \( 6371 \* acos\( cos\(radians\(\$1\)\) \* cos\(radians\(l\.latitude\)\) \* cos\(radians\(l\.longitude\) - radians\(\$2\)\) \+ sin\(radians\(\$3\)\) \* sin\(radians\(l\.latitude\)\) \) \) <= \$4 LIMIT \$5`).
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
}
