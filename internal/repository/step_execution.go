package repository

import (
	"context"

	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"flowforge/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type StepExecutionRepository interface {
	Create(ctx context.Context, step *model.StepExecution) error
	Get(ctx context.Context, filters map[string]any, step *model.StepExecution) error
	ListByExecution(ctx context.Context, executionID string) ([]*model.StepExecution, error)
	List(ctx context.Context, executionID string, limit, offset uint64) ([]*model.StepExecution, int, error)
	Update(ctx context.Context, id string, currentVersion int, data map[string]any) error
	GetByExecutionAndStep(ctx context.Context, execID, stepID string) (*model.StepExecution, error)
}

type stepExecutionRepository struct {
	db postgres.PgxExecutor
	sq squirrel.StatementBuilderType
}

func NewStepExecutionRepository(db postgres.PgxExecutor) StepExecutionRepository {
	return &stepExecutionRepository{
		db: db,
		sq: squirrel.StatementBuilderType{}.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *stepExecutionRepository) getDB(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if !ok {
		return r.db
	}
	return tx
}

func (r *stepExecutionRepository) Create(ctx context.Context, step *model.StepExecution) error {
	query, args, err := r.sq.
		Insert(step.Tablename()).
		Columns(step.Columns()...).
		Values(step.Values()...).
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
			"failed to insert workflow: no rows affected",
			nil,
		)
	}

	return nil
}

func (r *stepExecutionRepository) Get(ctx context.Context, filters map[string]any, step *model.StepExecution) error {
	query := r.sq.Select((&model.StepExecution{}).Columns()...).
		From((&model.StepExecution{}).Tablename())

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

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.StepExecution])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*step = *res
	return nil
}

func (r *stepExecutionRepository) ListByExecution(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	sqlQuery, args, err := r.sq.Select((&model.StepExecution{}).Columns()...).
		From((&model.StepExecution{}).Tablename()).
		Where(squirrel.Eq{"execution_id": executionID}).
		OrderBy("started_at ASC NULLS LAST").
		ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build list query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.StepExecution])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return results, nil
}

func (r *stepExecutionRepository) List(ctx context.Context, executionID string, limit, offset uint64) ([]*model.StepExecution, int, error) {
	baseQuery := r.sq.Select((&model.StepExecution{}).Columns()...).
		From((&model.StepExecution{}).Tablename()).
		Where(squirrel.Eq{"execution_id": executionID})

	// Get total count
	countQuery := r.sq.Select("count(*)").From((&model.StepExecution{}).Tablename()).Where(squirrel.Eq{"execution_id": executionID})

	sqlCount, argsCount, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build count query", err)
	}

	var total int
	if err := r.getDB(ctx).QueryRow(ctx, sqlCount, argsCount...).Scan(&total); err != nil {
		return nil, 0, errorx.DbError(err, "failed to count step executions")
	}

	// Get paginated results
	sqlQuery, args, err := baseQuery.Limit(limit).Offset(offset).OrderBy("started_at ASC NULLS LAST").ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build list query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.StepExecution])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}

	return results, total, nil
}

func (r *stepExecutionRepository) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	data["version"] = currentVersion + 1
	data["updated_at"] = squirrel.Expr("NOW()")

	query, args, err := r.sq.Update((&model.StepExecution{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"version": currentVersion}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update status query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict,
			"step execution was modified by another request, please re-fetch and retry", nil)
	}

	return nil
}

func (r *stepExecutionRepository) GetByExecutionAndStep(ctx context.Context, execID, stepID string) (*model.StepExecution, error) {
	query, args, err := r.sq.Select((&model.StepExecution{}).Columns()...).
		From((&model.StepExecution{}).Tablename()).
		Where(squirrel.Eq{"execution_id": execID}).
		Where(squirrel.Eq{"step_id": stepID}).
		ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build get query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.StepExecution])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return res, nil
}
