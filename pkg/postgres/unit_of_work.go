package postgres

import (
	"context"
	"errors"

	"flowforge/pkg/errorx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TrxKey struct{}

type Trx interface {
	Begin(ctx context.Context) (context.Context, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type unitOfWork struct {
	pool *pgxpool.Pool
}

func NewTransaction(pool *pgxpool.Pool) Trx {
	return &unitOfWork{
		pool: pool,
	}
}

func (u *unitOfWork) Begin(ctx context.Context) (context.Context, error) {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return nil, errorx.DbError(err, "failed to begin transaction")
	}

	return context.WithValue(ctx, TrxKey{}, tx), nil
}

func (u *unitOfWork) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(TrxKey{}).(pgx.Tx)
	if !ok {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to fetch data", errors.New("no transaction found in context"))
	}

	err := tx.Commit(ctx)
	if err != nil {
		return errorx.DbError(err, "failed to commit transaction")
	}

	return nil
}

func (u *unitOfWork) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(TrxKey{}).(pgx.Tx)
	if !ok {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to fetch data", errors.New("no transaction found in context"))
	}

	if err := tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return errorx.DbError(err, "failed to rollback transaction")
	}
	return nil
}
