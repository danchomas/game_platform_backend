package response

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"beeline-games/internal/core/apperr"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("ответ не сериализован", "err", err)
	}
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := classify(err)

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "ошибка обработки запроса",
			"err", err, "method", r.Method, "path", r.URL.Path)
	} else {
		slog.WarnContext(r.Context(), "запрос отклонён",
			"err", err, "code", code, "method", r.Method, "path", r.URL.Path)
	}

	body := errorBody{Error: errorPayload{Code: code, Message: message}}

	var vErr *apperr.ValidationError
	if errors.As(err, &vErr) {
		body.Error.Field = vErr.Field
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func classify(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound, "not_found", "Объект не найден"
	case errors.Is(err, apperr.ErrValidation):
		return http.StatusBadRequest, "validation", err.Error()
	case errors.Is(err, apperr.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized", "Требуется авторизация"
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden, "forbidden", "Нет доступа к этому объекту"
	case errors.Is(err, apperr.ErrNotGenerated):
		return http.StatusConflict, "not_generated", "В игре ещё нет сгенерированного кода"
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict, "conflict", "Для этой игры уже идёт генерация"
	case errors.Is(err, apperr.ErrCapacity):
		return http.StatusServiceUnavailable, "capacity", "Сервер занят генерациями, попробуйте через минуту"
	case errors.Is(err, apperr.ErrLLMUnavailable):
		return http.StatusServiceUnavailable, "llm_unavailable", "Языковая модель недоступна"
	case errors.Is(err, apperr.ErrLLMFailed):
		return http.StatusBadGateway, "llm_failed", "Модель не смогла сгенерировать игру, попробуйте переформулировать"
	case errors.Is(err, apperr.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, "payload_too_large", "Слишком большой запрос"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "timeout", "Превышено время обработки запроса"
	case errors.Is(err, context.Canceled):
		return 499, "canceled", "Запрос отменён клиентом"
	default:
		return http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера"
	}
}
