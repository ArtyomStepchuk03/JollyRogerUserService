package seed

import (
	"context"
	"fmt"
	"os"
	"time"

	"JollyRogerUserService/internal/models"
	"JollyRogerUserService/internal/repository/postgres"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DevEnvironmentSeeder обрабатывает заполнение тестовыми данными среды разработки
type DevEnvironmentSeeder struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewDevEnvironmentSeeder создает новый объект для заполнения тестовыми данными
func NewDevEnvironmentSeeder(db *gorm.DB, logger *zap.Logger) *DevEnvironmentSeeder {
	return &DevEnvironmentSeeder{
		db:     db,
		logger: logger,
	}
}

// SeedTestUser создает тестового пользователя, если мы находимся в режиме разработки
func (s *DevEnvironmentSeeder) SeedTestUser() error {
	// Проверяем, находимся ли мы в режиме разработки
	if os.Getenv("APP_ENV") != "development" {
		s.logger.Debug("Не в режиме разработки, пропускаем создание тестового пользователя")
		return nil
	}

	s.logger.Info("Заполнение тестовым пользователем для среды разработки")

	// Создаем репозиторий
	repo := postgres.NewUserRepository(s.db)

	// Проверяем, существует ли уже тестовый пользователь
	existingUser, err := repo.GetByTelegramID(1)
	if err == nil && existingUser != nil {
		s.logger.Info("Тестовый пользователь уже существует", zap.Uint("user_id", existingUser.ID))
		return nil
	}

	// Создаем тестового пользователя
	testUser := &models.User{
		TelegramID: 1,
		Username:   "Uruz",
		Bio:        "Это тестовый пользователь, автоматически созданный в режиме разработки",
		Rating:     5.0,
	}

	// Сохраняем в базу данных внутри транзакции
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Создаем пользователя
		if err := tx.Create(testUser).Error; err != nil {
			return fmt.Errorf("не удалось создать тестового пользователя: %w", err)
		}

		// Создаем статистику пользователя
		now := time.Now()
		stats := models.UserStats{
			UserID:             testUser.ID,
			EventsCreated:      5,    // Пользователь создал 5 событий
			EventsParticipated: 10,   // Участвовал в 10 событиях
			CreatedAt:          now,  // Время создания статистики
			UpdatedAt:          now,  // Время обновления статистики
			LastActiveAt:       &now, // Время последней активности
			IsActive:           true, // Пользователь активен
		}
		if err := tx.Create(&stats).Error; err != nil {
			return fmt.Errorf("не удалось создать статистику тестового пользователя: %w", err)
		}

		// Создаем настройки уведомлений пользователя
		settings := models.UserNotificationSetting{
			UserID:               testUser.ID,
			NewEventNotification: true, // Уведомления о новых событиях включены
		}
		if err := tx.Create(&settings).Error; err != nil {
			return fmt.Errorf("не удалось создать настройки уведомлений тестового пользователя: %w", err)
		}

		// Создаем местоположение пользователя
		location := models.UserLocation{
			UserID:    testUser.ID,
			Latitude:  55.7558, // Широта (Москва)
			Longitude: 37.6173, // Долгота (Москва)
			City:      "Москва",
			Region:    "Московская область",
			Country:   "Россия",
			UpdatedAt: now, // Время обновления местоположения
		}
		if err := tx.Create(&location).Error; err != nil {
			return fmt.Errorf("не удалось создать местоположение тестового пользователя: %w", err)
		}

		// Создаем предпочтения пользователя (теги)
		preferences := []models.UserPreference{
			{UserID: testUser.ID, TagID: 1, CreatedAt: now}, // Предполагаем, что ID тега 1 - "Страйкбол"
			{UserID: testUser.ID, TagID: 2, CreatedAt: now}, // Предполагаем, что ID тега 2 - "Командные игры"
			{UserID: testUser.ID, TagID: 3, CreatedAt: now}, // Предполагаем, что ID тега 3 - "Активный отдых"
		}

		for _, pref := range preferences {
			if err := tx.Create(&pref).Error; err != nil {
				return fmt.Errorf("не удалось создать предпочтение тестового пользователя: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Не удалось заполнить тестовым пользователем", zap.Error(err))
		return err
	}

	s.logger.Info("Успешно создан тестовый пользователь", zap.Uint("user_id", testUser.ID))
	return nil
}

// SeedAllDevData заполняет все данные для разработки
func (s *DevEnvironmentSeeder) SeedAllDevData(ctx context.Context) error {
	// В настоящее время у нас есть только заполнение тестовым пользователем
	// В будущем вы можете добавить здесь больше функций заполнения
	return s.SeedTestUser()
}
