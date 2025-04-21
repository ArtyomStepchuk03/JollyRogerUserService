package models

import (
	"time"
)

// User представляет основную модель пользователя
type User struct {
	ID         uint   `gorm:"primaryKey"`
	TelegramID int64  `gorm:"uniqueIndex"`
	Username   string
	Bio        string
	Rating     float32 `gorm:"default:0"`
	
	// Связи
	Stats               UserStats               `gorm:"foreignKey:UserID"`
	Preferences         []UserPreference        `gorm:"foreignKey:UserID"`
	Location            UserLocation            `gorm:"foreignKey:UserID"`
	NotificationSetting UserNotificationSetting `gorm:"foreignKey:UserID"`
}

// UserStats содержит статистику пользователя
type UserStats struct {
	ID                uint `gorm:"primaryKey"`
	UserID            uint
	EventsCreated     int       `gorm:"default:0"`
	EventsParticipated int       `gorm:"default:0"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
	LastActiveAt      *time.Time
	IsActive          bool      `gorm:"default:true"`
}

// UserPreference представляет интерес пользователя (тег)
type UserPreference struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint
	TagID     uint
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// UserLocation содержит информацию о местоположении пользователя
type UserLocation struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint
	Latitude  float64
	Longitude float64
	City      string
	Region    string
	Country   string
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// UserNotificationSetting содержит настройки уведомлений пользователя
type UserNotificationSetting struct {
	ID                  uint `gorm:"primaryKey"`
	UserID              uint
	NewEventNotification bool `gorm:"default:true"`
}

// CreateUserRequest представляет запрос на создание пользователя
type CreateUserRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	Bio        string `json:"bio,omitempty"`
}

// UserResponse представляет ответ с данными пользователя
type UserResponse struct {
	ID         uint    `json:"id"`
	TelegramID int64   `json:"telegram_id"`
	Username   string  `json:"username"`
	Bio        string  `json:"bio,omitempty"`
	Rating     float32 `json:"rating"`
}

// UserStatsResponse представляет ответ со статистикой пользователя
type UserStatsResponse struct {
	EventsCreated      int       `json:"events_created"`
	EventsParticipated int       `json:"events_participated"`
	CreatedAt          time.Time `json:"created_at"`
	LastActiveAt       *time.Time `json:"last_active_at,omitempty"`
	IsActive           bool      `json:"is_active"`
}

// UserPreferenceRequest представляет запрос на добавление предпочтения
type UserPreferenceRequest struct {
	UserID uint `json:"user_id"`
	TagID  uint `json:"tag_id"`
}

// UserLocationRequest представляет запрос на обновление местоположения
type UserLocationRequest struct {
	UserID    uint    `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city,omitempty"`
	Region    string  `json:"region,omitempty"`
	Country   string  `json:"country,omitempty"`
}

// UpdateNotificationSettingRequest представляет запрос на обновление настроек уведомлений
type UpdateNotificationSettingRequest struct {
	UserID              uint `json:"user_id"`
	NewEventNotification bool `json:"new_event_notification"`
}

// TableName устанавливает имя таблицы для модели User
func (User) TableName() string {
	return "users"
}

// TableName устанавливает имя таблицы для модели UserStats
func (UserStats) TableName() string {
	return "user_stats"
}

// TableName устанавливает имя таблицы для модели UserPreference
func (UserPreference) TableName() string {
	return "user_preferences"
}

// TableName устанавливает имя таблицы для модели UserLocation
func (UserLocation) TableName() string {
	return "user_locations"
}

// TableName устанавливает имя таблицы для модели UserNotificationSetting
func (UserNotificationSetting) TableName() string {
	return "user_notification_settings"
}
