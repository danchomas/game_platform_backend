package apperr

import "errors"

var (
	ErrNotFound = errors.New("объект не найден")

	ErrValidation = errors.New("некорректные данные")

	ErrUnauthorized = errors.New("требуется авторизация")

	ErrForbidden = errors.New("доступ запрещён")

	ErrConflict = errors.New("операция конфликтует с текущим состоянием")

	ErrCapacity = errors.New("сервер занят генерациями")

	ErrLLMUnavailable = errors.New("языковая модель недоступна")

	ErrLLMFailed = errors.New("модель не смогла сгенерировать игру")

	ErrNotGenerated = errors.New("в игре ещё нет сгенерированного кода")

	ErrPayloadTooLarge = errors.New("слишком большой запрос")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Reason
	}
	return e.Field + ": " + e.Reason
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

func NewValidationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
