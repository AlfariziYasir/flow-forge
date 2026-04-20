package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ClaimOption func(jwt.MapClaims)

func WithOption(k string, v any) ClaimOption {
	return func(mc jwt.MapClaims) {
		mc[k] = v
	}
}

type BaseRequest struct {
	RefUuid     string
	AccUuid     string
	RefKey      string
	AccKey      string
	RefDuration time.Duration
	AccDuration time.Duration
}

func generateToken(uuid string, key string, duration time.Duration, opts ...ClaimOption) (string, error) {
	claims := jwt.MapClaims{
		"jti": uuid,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(duration).Unix(),
	}

	for _, opt := range opts {
		opt(claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(key))
}

func TokenPair(payload BaseRequest, opts ...ClaimOption) (string, string, error) {
	acc, err := generateToken(payload.AccUuid, payload.AccKey, payload.AccDuration, opts...)
	if err != nil {
		return "", "", err
	}

	ref, err := generateToken(payload.RefUuid, payload.RefKey, payload.RefDuration, opts...)
	if err != nil {
		return "", "", err
	}

	return acc, ref, nil
}

func AccessToken(payload BaseRequest, opts ...ClaimOption) (string, error) {
	return generateToken(payload.AccUuid, payload.AccKey, payload.AccDuration, opts...)
}
