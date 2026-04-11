package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowforge/internal/handler"
	"flowforge/internal/repository"
	"flowforge/internal/services"
	workerHandler "flowforge/internal/worker/handler"
	workerService "flowforge/internal/worker/service"
	"flowforge/pkg/jwt"
	appLogger "flowforge/pkg/logger"
	"flowforge/pkg/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	l, err := appLogger.New("info", "flow-forge", "1.0")
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
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

	userRepo := repository.NewRepository(pool)
	wfRepo := repository.NewWorkflowRepository(pool)
	execRepo := repository.NewExecutionRepository(pool)
	sExecRepo := repository.NewStepExecutionRepository(pool)

	wfService := services.NewWorkflowService(wfRepo, execRepo, sExecRepo, uow, l)
	
	execEngine := workerService.NewExecutionEngine(execRepo, sExecRepo, l)
	jobPoller := workerHandler.NewJobPoller(execRepo, wfRepo, uow, execEngine, l, 5*time.Second)

	// Start Background Worker Poller
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go jobPoller.Start(workerCtx)

	authHandler := handler.NewAuthHandler(userRepo, tm, l)
	workflowHandler := handler.NewWorkflowHandler(wfService, l)

	router := handler.NewRouter(authHandler, workflowHandler, tm)

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
