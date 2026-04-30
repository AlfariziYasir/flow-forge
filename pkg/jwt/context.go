package jwt

import "context"

const (
	RefKey       string = "rt"
	BlacklistKey string = "blacklist"
)

type roleKey struct{}
type userKey struct{}
type tenantKey struct{}
type refUuidKey struct{}
type accUuidKey struct{}

var (
	RoleKey    = roleKey{}
	UserKey    = userKey{}
	TenantKey  = tenantKey{}
	RefUuidKey = refUuidKey{}
	AccUuidKey = accUuidKey{}
)

func getContextValue[T any](ctx context.Context, key any) T {
	val, ok := ctx.Value(key).(T)
	if !ok {
		var zero T
		return zero
	}
	return val
}
func GetRole(ctx context.Context) string {
	return getContextValue[string](ctx, RoleKey)
}

func GetUser(ctx context.Context) string {
	return getContextValue[string](ctx, UserKey)
}

func GetTenant(ctx context.Context) string {
	return getContextValue[string](ctx, TenantKey)
}

func GetRefUuid(ctx context.Context) string {
	return getContextValue[string](ctx, RefUuidKey)
}

func GetAccUuid(ctx context.Context) string {
	return getContextValue[string](ctx, AccUuidKey)
}

func SetContext[T any](ctx context.Context, key any, val T) context.Context {
	return context.WithValue(ctx, key, val)
}
