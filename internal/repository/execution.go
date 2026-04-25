package repository

import (
	"context"
	"fmt"

	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"flowforge/pkg/postgres"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type ExecutionRepository interface {
	Create(ctx context.Context, execution *model.Execution) error
	Get(ctx context.Context, filters map[string]any, execution *model.Execution) error
	List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Execution, int, error)
	Update(ctx context.Context, id string, currentVersion int, data map[string]any) error
	AcquireForWorker(ctx context.Context, limit int) ([]*model.Execution, error)
	AcquireByIDForWorker(ctx context.Context, id string) (*model.Execution, error)
	RecoverStuckJobs(ctx context.Context, timeout time.Duration) (int64, error)
}

type executionRepository struct {
	db postgres.PgxExecutor
	sq squirrel.StatementBuilderType
}

func NewExecutionRepository(db postgres.PgxExecutor) ExecutionRepository {
	return &executionRepository{
		db: db,
		sq: squirrel.StatementBuilderType{}.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *executionRepository) getDB(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if !ok {
		return r.db
	}
	return tx
}

func (r *executionRepository) Create(ctx context.Context, execution *model.Execution) error {
	query, args, err := r.sq.
		Insert(execution.Tablename()).
		Columns(execution.Columns()...).
		Values(execution.Values()...).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build create query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(
			errorx.ErrTypeInternal,
			"failed to insert execution: no rows affected",
			nil,
		)
	}

	return nil
}

func (r *executionRepository) Get(ctx context.Context, filters map[string]any, execution *model.Execution) error {
	query := r.sq.Select((&model.Execution{}).Columns()...).
		From((&model.Execution{}).Tablename())

	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build get query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Execution])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*execution = *res
	return nil
}

func (r *executionRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Execution, int, error) {
	query := r.sq.Select((&model.Execution{}).Columns()...).From((&model.Execution{}).Tablename())

	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id", "workflow_id", "status":
				query = query.Where(squirrel.Eq{k: v})
			default:
				query = query.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
			}
		}
	}

	sqlQuery, args, err := query.Limit(limit).Offset(offset).OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build list query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Execution])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	if len(results) == 0 {
		results = []*model.Execution{}
	}

	countQ := r.sq.Select("count(*)").From((&model.Execution{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id", "workflow_id", "status":
				countQ = countQ.Where(squirrel.Eq{k: v})
			default:
				countQ = countQ.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
			}
		}
	}

	sqlCount, args2, err := countQ.ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build count query", err)
	}

	var count int
	if err = r.getDB(ctx).QueryRow(ctx, sqlCount, args2...).Scan(&count); err != nil {
		return nil, 0, errorx.DbError(err, "failed to count executions")
	}

	return results, count, nil
}

func (r *executionRepository) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	data["version"] = currentVersion + 1
	data["updated_at"] = squirrel.Expr("NOW()")

	sqlQuery, args, err := r.sq.Update((&model.Execution{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"version": currentVersion}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict,
			"execution was modified by another request, please re-fetch and retry", nil)
	}

	return nil
}

func (r *executionRepository) AcquireForWorker(ctx context.Context, limit int) ([]*model.Execution, error) {
	query, args, err := r.sq.Select((&model.Execution{}).Columns()...).
		From((&model.Execution{}).Tablename()).
		Where(squirrel.Eq{"status": "PENDING"}).
		OrderBy("created_at ASC").
		Limit(uint64(limit)).
		Suffix("FOR UPDATE SKIP LOCKED").ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build acquire query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errorx.DbError(err, "failed to acquire executions for worker")
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Execution])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return results, nil
}

func (r *executionRepository) AcquireByIDForWorker(ctx context.Context, id string) (*model.Execution, error) {
	query, args, err := r.sq.Select((&model.Execution{}).Columns()...).
		From((&model.Execution{}).Tablename()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"status": "PENDING"}).
		Suffix("FOR UPDATE SKIP LOCKED").ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build acquire query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errorx.DbError(err, "failed to acquire execution by ID for worker")
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Execution])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return res, nil
}

func (r *executionRepository) RecoverStuckJobs(ctx context.Context, timeout time.Duration) (int64, error) {
	query, args, err := r.sq.Update((&model.Execution{}).Tablename()).
		Set("status", "PENDING").
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"status": "RUNNING"}).
		Where(squirrel.Expr("updated_at < NOW() - ? * INTERVAL '1 second'", int64(timeout.Seconds()))).
		ToSql()
	if err != nil {
		return 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build recover query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return 0, errorx.DbError(err, "failed to recover stuck jobs")
	}

	return res.RowsAffected(), nil
}
