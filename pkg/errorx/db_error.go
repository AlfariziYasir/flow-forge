package errorx

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func DbError(err error, message string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return NewError(ErrTypeNotFound, "query error", err)
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case "23505":
			return NewError(ErrTypeConflict, pgErr.Message, pgErr)
		case "23503":
			return NewError(ErrTypeNotFound, pgErr.Message, pgErr)
		}
	}

	return NewError(ErrTypeInternal, "internal server error", err)
}
