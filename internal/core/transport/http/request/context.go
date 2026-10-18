package request

import (
	"context"
	"net/http"

	"beeline-games/internal/core/jwt"
)

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
