package user

type Repository interface {
	GetByChatID(chatID int64) (*User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(chatID int64) error
}
