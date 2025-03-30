package infrastructure

import (
	"JollyRogerUserService/internal/user"
	"time"
)

type UserService struct {
	userRepository user.Repository
	userCache      user.Cache
}

func NewService(userRepository user.Repository, cache user.Cache) user.Service {
	return &UserService{userRepository: userRepository, userCache: cache}
}

func (s *UserService) GetByChatID(chatID int64) (*user.User, error) {
	foundUser, err := s.userCache.Get(chatID)
	if err == nil && foundUser != nil {
		return foundUser, nil
	}

	foundUser, err = s.userRepository.GetByChatID(chatID)
	if err != nil {
		return nil, err
	}

	_ = s.userCache.Set(foundUser, time.Minute*10)

	return foundUser, nil
}
