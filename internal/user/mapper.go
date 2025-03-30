package user

import "JollyRogerUserService/pb"

func ToProto(u *User) *pb.User {
	if u == nil {
		return nil
	}

	return &pb.User{
		Id:       u.ID,
		ChatId:   u.ChatID,
		Name:     u.Name,
		Age:      uint32(u.Age),
		About:    u.About,
		Karma:    u.Karma,
		Country:  u.Settings.Country,
		City:     u.Settings.City,
		Language: u.Settings.Language,
	}
}

func FromProto(p *pb.User) *User {
	if p == nil {
		return nil
	}

	return &User{
		ID:     p.Id,
		ChatID: p.ChatId,
		Name:   p.Name,
		Age:    uint8(p.Age),
		About:  p.About,
		Karma:  p.Karma,
		Settings: Settings{
			Country:  p.Country,
			City:     p.City,
			Language: p.Language,
		},
	}
}
