package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Name            string        `env:"APP_NAME"`
	Version         string        `env:"APP_VERSION"`
	HttpPort        int           `env:"HTTP_PORT"`
	AccessTokenKey  string        `env:"ACCESS_TOKEN_KEY"`
	RefreshTokenKey string        `env:"REFRESH_TOKEN_KEY"`
	AccessTokenExp  time.Duration `env:"ACCESS_TOKEN_MAX_AGE"`
	RefreshTokenExp time.Duration `env:"REFRESH_TOKEN_MAX_AGE"`
	DbDsn           string        `env:"DB_DSN"`
	LogLevel        string        `env:"LOG_LEVEL"`
	RedisAddress    string        `env:"REDIS_ADDRESS"`
	RedisPassword   string        `env:"REDIS_PASSWORD"`
	RedisDB         int           `env:"REDIS_DB"`
	SwaggerPath     string        `env:"SWAGGER_PATH"`
}

func NewConfig() (*Config, error) {
	config := Config{}

	if err := env.Parse(&config); err != nil {
		return nil, fmt.Errorf("read env error: %w", err)
	}

	return &config, nil
}
