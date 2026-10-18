package domain

import "time"

type User struct {
	ID		string		`json:"id"`
	Name		string		`json:"name"`
	Role		string		`json:"role"`
	CreatedAt	time.Time	`json:"created_at"`
}

type AuthResult struct {
	User		User	`json:"user"`
	Token		string	`json:"token"`
	AccessKey	string	`json:"access_key,omitempty"`
}
