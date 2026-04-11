package errorx

import "fmt"

type ErrorType string

const (
	ErrTypeNotFound     ErrorType = "NOT_FOUND"
	ErrTypeConflict     ErrorType = "CONFLICT"
	ErrTypeInternal     ErrorType = "INTERNAL_ERROR"
	ErrTypeValidation   ErrorType = "VALIDATION_ERROR"
	ErrTypeUnauthorized ErrorType = "UNAUTHORIZED"
)

type AppError struct {
	Type    ErrorType
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func NewValidationError(fields map[string]string) *AppError {
	return &AppError{
		Type:    ErrTypeValidation,
		Message: "validation failed",
		Fields:  fields,
	}
}

func NewError(errType ErrorType, msg string, err error) *AppError {
	appErr := AppError{
		Message: msg,
		Err:     err,
	}

	switch errType {
	case ErrTypeNotFound:
		appErr.Type = ErrTypeNotFound
	case ErrTypeConflict:
		appErr.Type = ErrTypeConflict
	case ErrTypeInternal:
		appErr.Type = ErrTypeInternal
	case ErrTypeUnauthorized:
		appErr.Type = ErrTypeUnauthorized
	}

	return &appErr
}
