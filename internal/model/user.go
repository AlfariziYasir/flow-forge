package model

import "time"

type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleEditor UserRole = "editor"
	RoleViewer UserRole = "viewer"
)

type User struct {
	ID           string     `db:"id"`
	TenantID     string     `db:"tenant_id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	Role         string     `db:"role"`
	Version      int        `db:"version"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    *time.Time `db:"updated_at"`
	IsActive     bool       `db:"is_active"`
}

func (t *User) Tablename() string {
	return "users"
}

func (t *User) Columns() []string {
	return []string{
		"id", "tenant_id", "email", "password_hash",
		"role", "version", "created_at", "updated_at", "is_active",
	}
}

func (t *User) Values() []any {
	return []any{
		t.ID, t.TenantID, t.Email, t.PasswordHash,
		t.Role, t.Version, t.CreatedAt, t.UpdatedAt, t.IsActive,
	}
}

type UserRequest struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UserUpdateRequest struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"-"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"-"`
}

type UserResponse struct {
	UserID     string     `json:"user_id"`
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Version    int        `json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	IsActive   bool       `json:"is_active"`
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UserRefreshResponse struct {
	AccessToken string `json:"access_token"`
}

type ListRequest struct {
	PageSize   uint64 `json:"page_size"`
	PageToken  string `json:"page_token"`
	Role       string `json:"-"`
	TenantID   string `json:"-"`
	TenantName string `json:"tenant_name"`
	Email      string `json:"email"`
}
