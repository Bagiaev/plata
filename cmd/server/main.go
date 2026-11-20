package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"plata/internal/config"
	"plata/internal/db"
	httpapi "plata/internal/http"
	"plata/internal/provider"
	"plata/internal/service"
	"plata/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("server stopped", "error", err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	repo := db.NewRepository(pool)
	quotes := service.NewService(repo)
	exchange := provider.NewClient(cfg.ExchangeAPIBaseURL, &http.Client{Timeout: cfg.ExchangeAPITimeout})
	jobs := worker.NewWorker(repo, exchange, logger)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewServer(httpapi.NewHandler(quotes, pool), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	return serve(ctx, srv, jobs, logger)
}

func serve(ctx context.Context, srv *http.Server, jobs *worker.Worker, logger *slog.Logger) error {
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		jobs.Run(workerCtx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting", "address", srv.Addr)
		serverErr <- srv.ListenAndServe()
	}()

	var listenErr error
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			listenErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}

	stopWorker()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		listenErr = errors.Join(listenErr, fmt.Errorf("shut down HTTP server: %w", err), srv.Close())
	}

	<-workerDone

	return listenErr
}
