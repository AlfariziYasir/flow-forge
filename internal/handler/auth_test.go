package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockUserRepo) Get(ctx context.Context, filters map[string]any, lock bool, user *model.User) error {
	args := m.Called(ctx, filters, lock, user)
	if args.Get(0) != nil {
		*user = *args.Get(0).(*model.User)
	}
	return args.Error(1)
}
func (m *mockUserRepo) Update(ctx context.Context, id string, version int, data map[string]any) error {
	return m.Called(ctx, id, version, data).Error(0)
}
func (m *mockUserRepo) Delete(ctx context.Context, id string, version int) error {
	return m.Called(ctx, id, version).Error(0)
}
func (m *mockUserRepo) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.User, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.User), args.Int(1), args.Error(2)
}

func TestAuthHandler_Login_Bcrypt(t *testing.T) {
	repo := new(mockUserRepo)
	tm := jwt.NewTokenManager("secret")
	l := logger.NewNop()
	h := handler.NewAuthHandler(repo, tm, l)

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	
	user := &model.User{
		ID:           "u-1",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
		TenantID:     "t-1",
	}

	repo.On("Get", mock.Anything, map[string]any{"email": "test@example.com"}, true, mock.Anything).
		Return(user, nil)

	loginReq := handler.LoginRequest{
		Email:    "test@example.com",
		Password: password,
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var resp handler.LoginResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.NotEmpty(t, resp.Token)
	
	repo.AssertExpectations(t)
}
