package repository

import (
	"context"
	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"flowforge/pkg/postgres"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type TenantRepository interface {
	Create(ctx context.Context, tenant *model.Tenant) error
	Get(ctx context.Context, req map[string]any, status bool, tenant *model.Tenant) error
	List(ctx context.Context, limit, offset uint64, req map[string]any) ([]*model.Tenant, int, error)
	Update(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, id string) error
}

type tenantRepository struct {
	db postgres.PgxExecutor
	sq squirrel.StatementBuilderType
}

func NewTenantRepository(db postgres.PgxExecutor) TenantRepository {
	return &tenantRepository{
		db: db,
		sq: squirrel.StatementBuilderType{}.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *tenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	query, args, err := r.sq.
		Insert(tenant.Tablename()).
		Columns(tenant.Columns()...).
		Values(tenant.Values()...).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build create query", err)
	}

	res, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}

func (r *tenantRepository) Get(ctx context.Context, filters map[string]any, status bool, tenant *model.Tenant) error {
	query := r.sq.Select((&model.Tenant{}).Columns()...).From((&model.Tenant{}).Tablename())
	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	if status {
		query = query.Where(squirrel.Expr("is_active is true"))
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build get query", err)
	}
	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Tenant])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*tenant = *res

	return nil
}

func (r *tenantRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Tenant, int, error) {
	query := r.sq.Select((&model.Tenant{}).Columns()...).From((&model.Tenant{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id":
				query = query.Where(squirrel.Eq{k: v})
			default:
				query = query.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
			}
		}
	}

	sqlQuery, args, err := query.Limit(limit).Offset(offset).ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build data query", err)
	}

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Tenant])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}

	if len(results) == 0 {
		results = []*model.Tenant{}
	}

	query2 := r.sq.Select("count(*)").From((&model.Tenant{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id":
				query2 = query2.Where(squirrel.Eq{k: v})
			default:
				query2 = query2.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
			}
		}
	}

	sqlQuery2, args, err := query2.ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build count query", err)
	}

	var count int
	err = r.db.QueryRow(ctx, sqlQuery2, args...).Scan(&count)
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to count users")
	}

	return results, count, nil
}

func (r *tenantRepository) Update(ctx context.Context, id string, data map[string]any) error {
	data["updated_at"] = squirrel.Expr("NOW()")

	sqlQuery, args, err := r.sq.Update((&model.Tenant{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"is_active": true}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.db.Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}

func (r *tenantRepository) Delete(ctx context.Context, id string) error {
	sqlQuery, args, err := r.sq.Update((&model.Tenant{}).Tablename()).
		Set("is_active", false).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"is_active": true}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build delete query", err)
	}

	res, err := r.db.Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}
