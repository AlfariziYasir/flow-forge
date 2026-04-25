package repository

import (
	"context"
	"flowforge/internal/model"
	"fmt"

	"flowforge/pkg/errorx"
	"flowforge/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Get(ctx context.Context, req map[string]any, status bool, user *model.User) error
	List(ctx context.Context, limit, offset uint64, req map[string]any) ([]*model.User, int, error)
	Update(ctx context.Context, id string, currentVersion int, data map[string]any) error
	Delete(ctx context.Context, id string, currentVersion int) error
}

type userRepository struct {
	db postgres.PgxExecutor
	sq squirrel.StatementBuilderType
}

func NewUserRepository(db postgres.PgxExecutor) UserRepository {
	return &userRepository{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	query, args, err := r.sq.
		Insert(user.Tablename()).
		Columns(user.Columns()...).
		Values(user.Values()...).
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

func (r *userRepository) Get(ctx context.Context, filters map[string]any, status bool, user *model.User) error {
	query := r.sq.Select((&model.User{}).Columns()...).From((&model.User{}).Tablename())
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

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*user = *res

	return nil
}

func (r *userRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.User, int, error) {
	query := r.sq.Select((&model.User{}).Columns()...).From((&model.User{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id":
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

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}

	if len(results) == 0 {
		results = []*model.User{}
	}

	query2 := r.sq.Select("count(*)").From((&model.User{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			switch k {
			case "id", "tenant_id":
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

func (r *userRepository) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	data["version"] = currentVersion + 1
	data["updated_at"] = squirrel.Expr("NOW()")

	sqlQuery, args, err := r.sq.Update((&model.User{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"version": currentVersion}).
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
		return errorx.NewError(errorx.ErrTypeConflict,
			"user was modified by another request, please re-fetch and retry", nil)
	}

	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string, currentVersion int) error {
	sqlQuery, args, err := r.sq.Update((&model.User{}).Tablename()).
		Set("is_active", false).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"version": currentVersion}).
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
		return errorx.NewError(errorx.ErrTypeConflict,
			"user was modified by another request, please re-fetch and retry", nil)
	}

	return nil
}
