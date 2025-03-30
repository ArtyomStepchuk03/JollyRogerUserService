package infrastructure

import (
	"JollyRogerUserService/internal/user"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) user.Repository {
	return &UserRepository{db}
}

func (r *UserRepository) GetByChatID(chatID int64) (*user.User, error) {
	var foundUser user.User
	if err := r.db.Where("chat_id = ?", chatID).First(&foundUser).Error; err != nil {
		return nil, err
	}
	return &foundUser, nil
}

func (r *UserRepository) Create(user *user.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) Update(user *user.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(chatID int64) error {
	return r.db.Where("chat_id = ?", chatID).Delete(&user.User{}).Error
}
