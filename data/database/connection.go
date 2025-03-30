package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

var DB *gorm.DB

func ConnectDB(host string, user string, password string, dbname string) {
	dsn := "host=localhost user=postgres password=postgres dbname=user_jolly_roger_postgres port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	DB = db
}

func AutoMigrate(models ...interface{}) {
	log.Println("Устанавливаем миграции...")
	err := DB.AutoMigrate(models...)
	if err != nil {
		log.Fatal("Ошибка миграции:", err)
	} else {
		log.Println("Миграции успешно установлены!")
	}
}
