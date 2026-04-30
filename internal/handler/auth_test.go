package handler_test

import (
	"bytes"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"flowforge/pkg/response"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuthHandler_Login_Success(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	accessToken := "access-token"
	refreshToken := "refresh-token"
	loginReq := model.UserLoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)
	svc.On("Login", mock.Anything, loginReq).Return(accessToken, refreshToken, nil)

	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var res response.Response[model.UserLoginResponse]
	json.NewDecoder(rr.Body).Decode(&res)

	fmt.Println("access token: ", res.Data.AccessToken)
	assert.NotEmpty(t, res.Data.AccessToken)
	assert.NotEmpty(t, res.Data.RefreshToken)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Login_Failed(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	loginReq := model.UserLoginRequest{
		Email: "test@example.com",
	}
	body, _ := json.Marshal(loginReq)
	svc.On("Login", mock.Anything, loginReq).Return(mock.Anything, mock.Anything, errorx.NewError(errorx.ErrTypeNotFound, "user not found", nil))

	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	var res errorx.HTTPError
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "user not found", res.Message)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	refUuID := "ref-uuid"
	userID := "user-ID"
	role := string(model.RoleAdmin)
	tenantID := "tenant-id"

	svc.On("Refresh", mock.Anything, refUuID, userID, role, tenantID).Return("new-access-token", nil)

	req := httptest.NewRequest("POST", "/refresh", nil)
	ctx := req.Context()
	ctx = jwt.SetContext(ctx, jwt.RefUuidKey, refUuID)
	ctx = jwt.SetContext(ctx, jwt.UserKey, userID)
	ctx = jwt.SetContext(ctx, jwt.RoleKey, role)
	ctx = jwt.SetContext(ctx, jwt.TenantKey, tenantID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var res response.Response[model.UserRefreshResponse]
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "new-access-token", res.Data.AccessToken)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Refresh_Failed(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	refUuID := "ref-uuid"

	req := httptest.NewRequest("POST", "/refresh", nil)
	ctx := req.Context()
	ctx = jwt.SetContext(ctx, jwt.RefUuidKey, refUuID)

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var res errorx.HTTPError
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "invalid request", res.Message)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Refresh_Revoke(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	refUuID := "ref-uuid"
	userID := "user-ID"
	role := string(model.RoleAdmin)
	tenantID := "tenant-id"

	svc.On("Refresh", mock.Anything, refUuID, userID, role, tenantID).Return("", errorx.NewError(errorx.ErrTypeUnauthorized, "token has been revoked", nil))

	req := httptest.NewRequest("POST", "/refresh", nil)
	ctx := req.Context()
	ctx = jwt.SetContext(ctx, jwt.RefUuidKey, refUuID)
	ctx = jwt.SetContext(ctx, jwt.UserKey, userID)
	ctx = jwt.SetContext(ctx, jwt.RoleKey, role)
	ctx = jwt.SetContext(ctx, jwt.TenantKey, tenantID)

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var res errorx.HTTPError
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "token has been revoked", res.Message)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	refUuID := "ref-uuid"
	accUuID := "acc-uuid"

	svc.On("Logout", mock.Anything, accUuID, refUuID).Return(nil)

	req := httptest.NewRequest("POST", "/logout", nil)
	ctx := req.Context()
	ctx = jwt.SetContext(ctx, jwt.AccUuidKey, accUuID)
	ctx = jwt.SetContext(ctx, jwt.RefUuidKey, refUuID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var res response.Response[any]
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "logout success", res.Message)
	svc.AssertExpectations(t)
}

func TestAuthHandler_Logout_Failed(t *testing.T) {
	svc := new(mockUserService)
	l := logger.NewNop()
	h := handler.NewAuthHandler(svc, l)

	req := httptest.NewRequest("POST", "/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var res errorx.HTTPError
	json.NewDecoder(rr.Body).Decode(&res)
	assert.Equal(t, "invalid request", res.Message)
	svc.AssertExpectations(t)
}
