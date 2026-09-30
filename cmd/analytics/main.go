package main

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"context"
	"os/signal"
	"syscall"
	"url-shortener/internal/analytics"
	"url-shortener/internal/kafka"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	envErr := godotenv.Load()
	if envErr != nil {
		slog.Warn("something wrong with .env file")
	}

	// Load variables from .env file
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	kafkaGroupID := os.Getenv("KAFKA_GROUP_ID")
	databaseURL := os.Getenv("DATABASE_URL")

	// Pool
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}

	defer func() {
		slog.Info("closing PostgreSQL pool")
		pool.Close()
		slog.Info("PostgreSQL pool closed")
	}()

	pingCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	err = pool.Ping(pingCtx)
	cancel()

	if err != nil {
		slog.Error("database is not answering", "error", err)
		pool.Close()
		os.Exit(1)
	}
	// Repo
	repo := analytics.NewRepository(pool)

	// Check that variables are not empty
	if kafkaBrokers == "" ||
		kafkaTopic == "" ||
		kafkaGroupID == "" ||
		databaseURL == "" {
		slog.Error("missing required environment variables")
		os.Exit(1)
	}

	brokers := strings.Split(kafkaBrokers, ",")

	// Consumer
	consumer, err := kafka.NewConsumer(brokers, kafkaTopic, kafkaGroupID, repo)
	if err != nil {
		slog.Error("failed to create Kafka consumer", "error", err)
		os.Exit(1)
	}

	defer func() {
		slog.Info("closing Kafka consumer")
		defer consumer.Close()
		slog.Info("Kafka consumer closed")
	}()

	// Graceful shutdown for Kafka
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received")
	}()

	defer stop()

	slog.Info(
		"analytics consumer started",
		"brokers", brokers,
		"topic", kafkaTopic,
		"group_id", kafkaGroupID,
	)

	runErr := consumer.Run(ctx)
	slog.Info("consumer.Run finished", "error", runErr)

	if runErr != nil {
		slog.Error("failed running consumer", "error", runErr)
		consumer.Close()
		pool.Close()
		os.Exit(1)
	}
}
