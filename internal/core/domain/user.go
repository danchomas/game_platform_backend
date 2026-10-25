package domain

import "time"

const (
	RoleManager = "manager"
	RolePlayer  = "player"
)

type User struct {
	ID        string
	Name      string
	Role      string
	CreatedAt time.Time
}

func (u User) IsManager() bool { return u.Role == RoleManager }

type AuthResult struct {
	User      User
	Token     string
	AccessKey string
}
