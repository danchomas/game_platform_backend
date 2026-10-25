package middleware

import (
	"net/http"
	"strings"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/jwt"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

func Auth(secret string) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, err := parse(secret, token(r))
			if err != nil {
				response.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(request.ContextWithClaims(r.Context(), claims)))
		}
	}
}

func OptionalAuth(secret string) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if claims, err := parse(secret, token(r)); err == nil {
				r = r.WithContext(request.ContextWithClaims(r.Context(), claims))
			}
			next.ServeHTTP(w, r)
		}
	}
}

func token(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func parse(secret, tok string) (jwt.Claims, error) {
	if tok == "" {
		return jwt.Claims{}, apperr.ErrUnauthorized
	}
	claims, err := jwt.Parse(secret, tok)
	if err != nil {
		return jwt.Claims{}, apperr.ErrUnauthorized
	}
	return claims, nil
}
