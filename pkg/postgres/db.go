package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
)

const (
	_defaultMaxPoolSize  = 10
	_defaultConnAttempts = 10
	_defaultConnTimeout  = time.Second
)

type ZapAdapter struct {
	logger *zap.Logger
}

func (l *ZapAdapter) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	fields := make([]zap.Field, 0, len(data))
	for k, v := range data {
		fields = append(fields, zap.Any(k, v))
	}

	switch level {
	case tracelog.LogLevelTrace, tracelog.LogLevelDebug:
		l.logger.Debug(msg, fields...)
	case tracelog.LogLevelInfo:
		l.logger.Info(msg, fields...)
	case tracelog.LogLevelWarn:
		l.logger.Warn(msg, fields...)
	case tracelog.LogLevelError:
		l.logger.Error(msg, fields...)
	default:
		l.logger.Info(msg, fields...)
	}
}

type Postgres struct {
	*pgxpool.Pool

	maxPoolSize  int32
	connAttempts int
	connTimeout  time.Duration
}

type Option func(*Postgres)

func WithMaxPoolSize(size int32) Option {
	return func(p *Postgres) {
		p.maxPoolSize = size
	}
}

func WithConnAttempts(attempts int) Option {
	return func(p *Postgres) {
		p.connAttempts = attempts
	}
}

func WithConnTimeout(timeout time.Duration) Option {
	return func(p *Postgres) {
		p.connTimeout = timeout
	}
}

func New(ctx context.Context, url string, log *zap.Logger, opts ...Option) (*Postgres, error) {
	pg := &Postgres{
		maxPoolSize:  _defaultMaxPoolSize,
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	for _, opt := range opts {
		opt(pg)
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres - New - pgxpool.ParseConfig: %w", err)
	}

	poolConfig.MaxConns = pg.maxPoolSize
	if log != nil {
		poolConfig.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   &ZapAdapter{logger: log},
			LogLevel: tracelog.LogLevelDebug,
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("postgres - New - pgxpool.NewWithConfig: %w", err)
	}
	pg.Pool = pool

	err = pg.retryConnection(ctx, log)
	if err != nil {
		return nil, err
	}

	return pg, nil
}

func (p *Postgres) retryConnection(ctx context.Context, log *zap.Logger) error {
	var err error
	for p.connAttempts > 0 {
		err = p.Pool.Ping(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("postgres connection cancelled: %w", ctx.Err())
		default:
		}

		if log != nil {
			log.Warn("Postgres is trying to connect...",
				zap.Int("attempts_left", p.connAttempts),
				zap.Error(err),
			)
		}

		time.Sleep(p.connTimeout)
		p.connAttempts--
	}

	return fmt.Errorf("postgres - retryConnection - attempts exhausted: %w", err)
}

func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}
