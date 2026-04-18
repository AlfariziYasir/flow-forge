package errorx

import (
	"encoding/json"
	"errors"
	"net/http"
)

type HTTPError struct {
	Code    int               `json:"-"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *HTTPError) Error() string {
	return e.Message
}

func (h *HTTPError) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.Code)
	json.NewEncoder(w).Encode(h)
}

func HttpNewError(w http.ResponseWriter, code int, message string) {
	h := &HTTPError{Code: code, Message: message}
	h.Write(w)
}

func MapError(err error) *HTTPError {
	if err == nil {
		return nil
	}

	if appErr, ok := errors.AsType[*AppError](err); ok {
		switch appErr.Type {
		case ErrTypeValidation:
			return &HTTPError{
				Code:    http.StatusUnprocessableEntity,
				Message: appErr.Message,
				Fields:  appErr.Fields,
			}

		case ErrTypeConflict:
			return &HTTPError{
				Code:    http.StatusConflict,
				Message: appErr.Message,
			}

		case ErrTypeNotFound:
			return &HTTPError{
				Code:    http.StatusNotFound,
				Message: appErr.Message,
			}

		case ErrTypeUnauthorized:
			return &HTTPError{
				Code:    http.StatusUnauthorized,
				Message: appErr.Message,
			}

		case ErrTypeInternal:
			return &HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "internal server error",
			}
		}
	}

	return &HTTPError{
		Code:    http.StatusInternalServerError,
		Message: "an unexpected error occurred",
	}
}
