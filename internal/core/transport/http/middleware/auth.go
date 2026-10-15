package middleware

import (
	"net/http"
	"strings"

	"beeline-games/internal/core/jwt"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func Auth(secret string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

			// Поддержка передачи токена в query для превью игр (iframe)
			if tok == "" {
				tok = r.URL.Query().Get("token")
			}

			if tok == "" {
				response.Error(w, http.StatusUnauthorized, "нужна авторизация")
				return
			}

			claims, err := jwt.Parse(secret, tok)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "недействительный или просроченный токен")
				return
			}

			// Кладем claims в контекст и идем дальше
			ctx := request.ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}
