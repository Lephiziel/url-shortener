package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url-shortener/internal/grpcserver"
	"url-shortener/internal/handler"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
	shortenerpb "url-shortener/proto"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	envErr := godotenv.Load()
	if envErr != nil {
		slog.Warn("something wrong with .env file")
	}

	baseURL := os.Getenv("BASE_URL")
	databaseURL := os.Getenv("DATABASE_URL")
	appPort := os.Getenv("APP_PORT")
	grpcPort := os.Getenv("GRPC_PORT")

	if databaseURL == "" {
		slog.Error("something wrong with database_url")
		os.Exit(1)
	}

	if baseURL == "" || appPort == "" || grpcPort == "" {
		slog.Error("Something wrong with base_url or app port")
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
	pgRepo := repository.NewRepository(pool)

	// Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		slog.Error("Something wrong with redis address")
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})

	repo := repository.NewCachedRepository(pgRepo, redisClient, 24*time.Hour)

	// Service
	svc := service.NewService(repo)

	// Handler
	h := handler.NewHandler(svc, baseURL)

	// gRPC
	grpcHandler := grpcserver.NewServer(svc, baseURL)

	listener, listenerErr := net.Listen("tcp", ":"+grpcPort)
	if listenerErr != nil {
		slog.Error("failed to start listener", "error", listenerErr)
		os.Exit(1)
	}
	defer listener.Close()

	grpcSrv := grpc.NewServer()

	shortenerpb.RegisterShortenerServiceServer(grpcSrv, grpcHandler)

	// Routing
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.CreateLink)
	mux.HandleFunc("GET /{code}", h.RedirectLink)

	// Grateful shutdown pattern
	srv := &http.Server{
		Addr:    ":" + appPort,
		Handler: mux,
	}

	// gRPC Listen
	go func() {
		if err := grpcSrv.Serve(listener); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// HTTP Listen
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
