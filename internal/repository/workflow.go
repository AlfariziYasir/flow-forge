package repository

import (
	"context"
	"errors"
	"fmt"

	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"flowforge/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type WorkflowRepository interface {
	Create(ctx context.Context, workflow *model.Workflow) error
	Get(ctx context.Context, filters map[string]any, workflow *model.Workflow) error
	GetForUpdate(ctx context.Context, tenantID, name string, version int, isCurrent bool) (string, error)
	List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Workflow, int, error)
	ListVersions(ctx context.Context, tenantID, name string) ([]*model.Workflow, error)
	Update(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, tenantID, name string) error
}

type workflowRepository struct {
	db postgres.PgxExecutor
	sq squirrel.StatementBuilderType
}

func NewWorkflowRepository(db postgres.PgxExecutor) WorkflowRepository {
	return &workflowRepository{
		db: db,
		sq: squirrel.StatementBuilderType{}.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *workflowRepository) getDB(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if !ok {
		return r.db
	}
	return tx
}

func (r *workflowRepository) Create(ctx context.Context, workflow *model.Workflow) error {
	workflow.IsCurrent = true
	workflow.IsActive = true

	query, args, err := r.sq.
		Insert(workflow.Tablename()).
		Columns(workflow.Columns()...).
		Values(workflow.Values()...).
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

func (r *workflowRepository) Get(ctx context.Context, filters map[string]any, workflow *model.Workflow) error {
	query := r.sq.Select((&model.Workflow{}).Columns()...).From((&model.Workflow{}).Tablename())

	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	if _, hasVersion := filters["version"]; !hasVersion {
		query = query.Where(squirrel.Eq{"is_current": true})
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

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Workflow])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*workflow = *res
	return nil
}

func (r *workflowRepository) GetForUpdate(ctx context.Context, tenantID, name string, version int, isCurrent bool) (string, error) {
	var lockedID string
	query, args, err := r.sq.Select("id").
		From((&model.Workflow{}).Tablename()).
		Where(squirrel.Eq{"tenant_id": tenantID}).
		Where(squirrel.Eq{"name": name}).
		Where(squirrel.Eq{"is_current": isCurrent}).
		Where(squirrel.Eq{"is_active": true}).
		Where(squirrel.Eq{"version": version}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed to build get query", err)
	}

	err = r.getDB(ctx).QueryRow(ctx, query, args...).Scan(&lockedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errorx.NewError(errorx.ErrTypeConflict,
				"workflow was modified by another request, please re-fetch and retry", nil)
		}
		return "", errorx.DbError(err, "failed to lock workflow row")
	}

	return lockedID, nil
}

func (r *workflowRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Workflow, int, error) {
	base := r.sq.Select((&model.Workflow{}).Columns()...).
		From((&model.Workflow{}).Tablename()).
		Where(squirrel.Eq{"is_current": true}).
		Where(squirrel.Eq{"is_active": true})

	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id":
				base = base.Where(squirrel.Eq{k: v})
			default:
				base = base.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
			}
		}
	}

	sqlQuery, args, err := base.Limit(limit).Offset(offset).OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build list query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Workflow])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	if len(results) == 0 {
		results = []*model.Workflow{}
	}

	count := 0
	countQ := r.sq.Select("count(*)").
		From((&model.Workflow{}).Tablename()).
		Where(squirrel.Eq{"is_current": true}).
		Where(squirrel.Eq{"is_active": true})

	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id":
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

	if err = r.getDB(ctx).QueryRow(ctx, sqlCount, args2...).Scan(&count); err != nil {
		return nil, 0, errorx.DbError(err, "failed to count workflows")
	}

	return results, count, nil
}

func (r *workflowRepository) ListVersions(ctx context.Context, tenantID, name string) ([]*model.Workflow, error) {
	sqlQuery, args, err := r.sq.Select((&model.Workflow{}).Columns()...).
		From((&model.Workflow{}).Tablename()).
		Where(squirrel.Eq{"tenant_id": tenantID}).
		Where(squirrel.Eq{"name": name}).
		Where(squirrel.Eq{"is_active": true}).
		OrderBy("version DESC").
		ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build list-versions query", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Workflow])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return results, nil
}

func (r *workflowRepository) Update(ctx context.Context, id string, data map[string]any) error {
	data["updated_at"] = squirrel.Expr("NOW()")
	query, args, err := r.sq.Update((&model.Workflow{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(
			errorx.ErrTypeValidation,
			"failed to update workflow: record not found or version conflict (data modified by another process)",
			pgx.ErrNoRows,
		)
	}

	return nil
}

func (r *workflowRepository) Delete(ctx context.Context, tenantID, name string) error {
	sqlQuery, args, err := r.sq.
		Update((&model.Workflow{}).Tablename()).
		Set("is_active", false).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"tenant_id": tenantID}).
		Where(squirrel.Eq{"name": name}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build delete query", err)
	}

	res, err := r.getDB(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "workflow not found", pgx.ErrNoRows)
	}

	return nil
}
