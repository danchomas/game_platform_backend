package http

import (
	"time"

	"beeline-games/internal/core/domain"
)

type registerRequest struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	ManagerCode string `json:"manager_code"`
}

type loginRequest struct {
	AccessKey string `json:"access_key"`
}

type userDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type authDTO struct {
	User      userDTO `json:"user"`
	Token     string  `json:"token"`
	AccessKey string  `json:"access_key,omitempty"`
}

func toUserDTO(u domain.User) userDTO {
	return userDTO{
		ID:        u.ID,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}

func toAuthDTO(res domain.AuthResult) authDTO {
	return authDTO{
		User:      toUserDTO(res.User),
		Token:     res.Token,
		AccessKey: res.AccessKey,
	}
}
