package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"beeline-games/internal/auth"
	"beeline-games/internal/store"
)

const tokenTTL = 30 * 24 * time.Hour

type registerReq struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	ManagerCode string `json:"manager_code"`
}

// POST /api/auth/register — достаточно имени. Возвращает JWT и access_key (код для входа с другого устройства).
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if !readJSON(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < 2 || n > 40 {
		apiErr(w, 400, "имя: от 2 до 40 символов")
		return
	}
	role := "player"
	if in.Role == "manager" {
		if s.cfg.ManagerCode != "" && in.ManagerCode != s.cfg.ManagerCode {
			apiErr(w, 403, "неверный код менеджера")
			return
		}
		role = "manager"
	}
	u, key, err := s.st.CreateUser(r.Context(), name, role)
	if err != nil {
		apiErr(w, 500, "не удалось создать пользователя")
		return
	}
	writeJSON(w, 201, map[string]any{
		"user":       u,
		"token":      auth.Issue(s.cfg.JWTSecret, u.ID, u.Role, tokenTTL),
		"access_key": key,
	})
}

// POST /api/auth/login {access_key}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AccessKey string `json:"access_key"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	u, err := s.st.UserByAccessKey(r.Context(), strings.TrimSpace(in.AccessKey))
	if err != nil {
		apiErr(w, 401, "неверный код доступа")
		return
	}
	writeJSON(w, 200, map[string]any{"user": u, "token": auth.Issue(s.cfg.JWTSecret, u.ID, u.Role, tokenTTL)})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	u, err := s.st.UserByID(r.Context(), c.Sub)
	if err != nil {
		apiErr(w, 401, "пользователь не найден")
		return
	}
	writeJSON(w, 200, u)
}

var _ = store.ErrNotFound
