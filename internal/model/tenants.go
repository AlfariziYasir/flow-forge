package model

import "time"

type Tenant struct {
	ID        string     `db:"id"`
	Name      string     `db:"name"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	IsActive  bool       `db:"is_active"`
}

func (t *Tenant) Tablename() string {
	return "tenants"
}

func (t *Tenant) Columns() []string {
	return []string{"id", "name", "created_at", "updated_at", "is_active"}
}

func (t *Tenant) Values() []any {
	return []any{t.ID, t.Name, t.CreatedAt, t.UpdatedAt, t.IsActive}
}

type TenantRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type TenantResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type ListTenantRequest struct {
	PageSize   uint64 `json:"page_size"`
	PageToken  string `json:"page_token"`
	TenantName string `json:"tenant_name"`
}
