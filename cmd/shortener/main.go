package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url-shortener/internal/handler"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	baseURL := os.Getenv("BASE_URL")
	databaseURL := os.Getenv("DATABASE_URL")
	appPort := os.Getenv("APP_PORT")

	if baseURL == "" || databaseURL == "" || appPort == "" {
		slog.Error("Something wrong with base_url or database_url or app port")
		os.Exit(1)
	}

	// Database connection
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Repository
	repo := repository.NewRepository(pool)

	// Service
	svc := service.NewService(repo)

	// Handler
	h := handler.NewHandler(svc, baseURL)

	// Routing
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.CreateLink)
	mux.HandleFunc("GET /{code}", h.RedirectLink)

	srv := &http.Server{
		Addr:    ":" + appPort,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Listen stop signals like Ctrl + C or something
	// from Docker or Kubernetes
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	// If we get a signal we're going to stop the server with timeout correctly
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
