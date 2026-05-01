package auth

import "time"

type Auth struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	FirstName string    `gorm:"size:100;not null;column:firstname" json:"firstname"`
	LastName  string    `gorm:"size:100;not null;column:lastname" json:"lastname"`
	Email     string    `gorm:"size:100;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	FirstName    string `json:"firstname"`
	LastName     string `json:"lastname"`
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

func (Auth) TableName() string {
	return "users"
}
