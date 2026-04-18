package handler_test

import (
	"bytes"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/errorx"
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
