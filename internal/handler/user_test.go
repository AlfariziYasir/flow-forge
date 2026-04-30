package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"

	"flowforge/pkg/logger"
	"flowforge/pkg/response"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockUserService struct {
	mock.Mock
}

func (m *mockUserService) Login(ctx context.Context, req model.UserLoginRequest) (string, string, error) {
	args := m.Called(ctx, req)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockUserService) Logout(ctx context.Context, acc, ref string) error {
	return m.Called(ctx, acc, ref).Error(0)
}
func (m *mockUserService) Refresh(ctx context.Context, ref, uid, role, tid string) (string, error) {
	args := m.Called(ctx, ref, uid, role, tid)
	return args.String(0), args.Error(1)
}
func (m *mockUserService) Create(ctx context.Context, req model.UserRequest) (*model.UserResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*model.UserResponse), args.Error(1)
}
func (m *mockUserService) Get(ctx context.Context, id string) (*model.UserResponse, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.UserResponse), args.Error(1)
}
func (m *mockUserService) List(ctx context.Context, req model.ListRequest) ([]*model.UserResponse, int, string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*model.UserResponse), args.Int(1), args.String(2), args.Error(3)
}
func (m *mockUserService) Update(ctx context.Context, req model.UserUpdateRequest) (*model.UserResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*model.UserResponse), args.Error(1)
}
func (m *mockUserService) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestUserHandler_Create(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewUserHandler(svc, l)

	userReq := model.UserRequest{Email: "test@example.com"}
	body, _ := json.Marshal(userReq)

	svc.On("Create", mock.Anything, userReq).Return(&model.UserResponse{UserID: "u-1", Email: "test@example.com"}, nil)

	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp response.Response[model.UserResponse]
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "u-1", resp.Data.UserID)

	svc.AssertExpectations(t)
}
