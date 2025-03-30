package user

type User struct {
	ID       string `gorm:"type:char(26);primaryKey"` // ULID
	ChatID   int64  `gorm:"uniqueIndex;not null"`
	Name     string `gorm:"not null"`
	Age      uint8  `gorm:"not null"`
	About    string
	Details  Details  `gorm:"constraint:OnDelete:CASCADE;foreignKey:UserID;references:ID"`
	Settings Settings `gorm:"constraint:OnDelete:CASCADE;foreignKey:UserID;references:ID"`
	Karma    uint32   `gorm:"not null"`
}

type Details struct {
	UserID                string `gorm:"primaryKey;type:char(26)"` // ULID
	RegistrationTimestamp uint   `gorm:"not null"`
}

type Settings struct {
	UserID   string `gorm:"primaryKey;type:char(26)"` // ULID
	Country  string `gorm:"not null"`
	City     string `gorm:"not null"`
	Language string `gorm:"not null"`
}
