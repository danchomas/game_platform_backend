package errors

import "errors"

var (
	ErrNotFound	= errors.New("not found")
	ErrInvalidInput	= errors.New("invalid input")
	ErrForbidden	= errors.New("forbidden")
	ErrUnauthorized	= errors.New("unauthorized")
	ErrInternal	= errors.New("internal server error")
)
