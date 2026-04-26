package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowforge/config"
	"flowforge/internal/broadcaster"
	"flowforge/internal/repository"
	workerHandler "flowforge/internal/worker/handler"
	workerService "flowforge/internal/worker/service"
	appLogger "flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	l, err := appLogger.New("info", "flow-forge-worker", "1.0")
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		l.Warn("failed to load config from env, using defaults", zap.Error(err))
		cfg = &config.Config{
			RedisAddress: "localhost:6379",
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://forge:forge@localhost:5432/flowforge?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		l.Fatal("unable to connect to database", zap.Error(err))
	}
	defer pool.Close()

	trx := postgres.NewTransaction(pool)

	// Redis
	cache, err := redis.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		l.Fatal("failed to connect to redis", zap.Error(err))
	}

	// Repositories
	wfRepo := repository.NewWorkflowRepository(pool)
	execRepo := repository.NewExecutionRepository(pool)
	sExecRepo := repository.NewStepExecutionRepository(pool)

	// Broadcaster (init with redis)
	b := broadcaster.Init(cache, l.GetZapLogger())

	// Execution Engine & Worker
	execEngine := workerService.NewExecutionEngine(execRepo, sExecRepo, trx, l, b, 5*time.Minute)
	jobPoller := workerHandler.NewJobPoller(execRepo, wfRepo, trx, cache, execEngine, l, 5*time.Second)

	// Start Background Worker Poller
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	l.Info("Starting FlowForge Worker")
	go jobPoller.Start(workerCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info("Shutting down worker...")
	cancel()
	l.Info("Worker exiting")
}
