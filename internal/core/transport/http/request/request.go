package request

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/jwt"
)

const DefaultMaxBody = 1 << 20

type contextKey string

const claimsKey contextKey = "jwt_claims"

func ContextWithClaims(ctx context.Context, claims jwt.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func UserID(r *http.Request) (string, bool) {
	claims, ok := r.Context().Value(claimsKey).(jwt.Claims)
	if !ok {
		return "", false
	}
	return claims.Sub, true
}

func Role(r *http.Request) (string, bool) {
	claims, ok := r.Context().Value(claimsKey).(jwt.Claims)
	if !ok {
		return "", false
	}
	return claims.Role, true
}

func RequireUserID(r *http.Request) (string, error) {
	id, ok := UserID(r)
	if !ok || id == "" {
		return "", apperr.ErrUnauthorized
	}
	return id, nil
}

func Decode(r *http.Request, dst any) error {
	return DecodeLimit(r, dst, DefaultMaxBody)
}

func DecodeLimit(r *http.Request, dst any, maxBytes int64) error {
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBytes)).Decode(dst); err != nil {
		if err == io.EOF {
			return apperr.NewValidationError("body", "тело запроса пустое")
		}
		return apperr.NewValidationError("body", "некорректный JSON")
	}
	return nil
}

func Int(r *http.Request, name string, def int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func Str(r *http.Request, name string) string {
	return strings.TrimSpace(r.URL.Query().Get(name))
}

func PathInt(r *http.Request, name string) (int, error) {
	n, err := strconv.Atoi(r.PathValue(name))
	if err != nil {
		return 0, apperr.NewValidationError(name, "ожидается целое число")
	}
	return n, nil
}

func PathID(r *http.Request, name string) (string, error) {
	id := strings.TrimSpace(r.PathValue(name))
	if id == "" {
		return "", apperr.NewValidationError(name, "идентификатор не указан")
	}
	return id, nil
}
