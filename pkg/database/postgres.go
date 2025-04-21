package database

import (
	"fmt"
	"github.com/yourusername/tg-team-finder/user-service/config"
	"github.com/yourusername/tg-team-finder/user-service/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

// NewPostgresDB создает новое подключение к PostgreSQL и выполняет миграции
func NewPostgresDB(cfg config.PostgresConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.DBName, cfg.SSLMode)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,  // Порог для медленных запросов
			LogLevel:                  logger.Error, // Уровень логирования (в production лучше использовать Error)
			IgnoreRecordNotFoundError: true,        // Игнорировать ошибки "запись не найдена"
			Colorful:                  true,        // Цветной вывод в консоль
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	// Настройка пула соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	
	// Установка максимального количества открытых соединений
	sqlDB.SetMaxOpenConns(25)
	
	// Установка максимального количества соединений в пуле
	sqlDB.SetMaxIdleConns(10)
	
	// Установка максимального времени жизни соединения
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Автоматическая миграция моделей
	err = db.AutoMigrate(
		&models.User{},
		&models.UserStats{},
		&models.UserPreference{},
		&models.UserLocation{},
		&models.UserNotificationSetting{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
