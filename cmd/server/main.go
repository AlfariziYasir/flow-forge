package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowforge/config"
	"flowforge/internal/broadcaster"
	"flowforge/internal/handler"
	"flowforge/internal/repository"
	"flowforge/internal/services"
	workerHandler "flowforge/internal/worker/handler"
	workerService "flowforge/internal/worker/service"
	"flowforge/pkg/jwt"
	appLogger "flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	l, err := appLogger.New("info", "flow-forge", "1.0")
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

	uow := postgres.NewTransaction(pool)
	tm := jwt.NewTokenManager("flowforge-super-secret-key-1234")

	// Redis (Cache)
	cache, err := redis.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		l.Warn("failed to connect to redis, caching will be disabled", zap.Error(err))
	}

	// Repositories
	userRepo := repository.NewRepository(pool)
	tenantRepo := repository.NewTenantRepository(pool)
	wfRepo := repository.NewWorkflowRepository(pool)
	execRepo := repository.NewExecutionRepository(pool)
	sExecRepo := repository.NewStepExecutionRepository(pool)

	// Services
	wfService := services.NewWorkflowService(wfRepo, execRepo, sExecRepo, uow, l)
	execService := services.NewExecutionService(execRepo, sExecRepo, wfRepo, l, uow)
	userService := services.NewUserService(userRepo, tenantRepo, l, cfg, cache)
	tenantService := services.NewTenantService(tenantRepo, l)
	aiService := services.NewAIService()
	
	// Execution Engine & Worker
	execEngine := workerService.NewExecutionEngine(execRepo, sExecRepo, uow, l, broadcaster.Get())
	jobPoller := workerHandler.NewJobPoller(execRepo, wfRepo, uow, execEngine, l, 5*time.Second)

	// Start Background Worker Poller
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go jobPoller.Start(workerCtx)

	// Handlers
	authHandler := handler.NewAuthHandler(userRepo, tm, l)
	workflowHandler := handler.NewWorkflowHandler(wfService, l)
	executionHandler := handler.NewExecutionHandler(execService, l)
	userHandler := handler.NewUserHandler(userService, l)
	tenantHandler := handler.NewTenantHandler(tenantService, l)
	aiHandler := handler.NewAIHandler(aiService, l)

	router := handler.NewRouter(
		authHandler,
		workflowHandler,
		executionHandler,
		userHandler,
		tenantHandler,
		aiHandler,
		tm,
	)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		l.Info("Starting server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Fatal("listen error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info("Shutting down server...")

	ctxDown, cancelDown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDown()

	if err := srv.Shutdown(ctxDown); err != nil {
		l.Fatal("Server forced to shutdown", zap.Error(err))
	}

	l.Info("Server exiting")
}
