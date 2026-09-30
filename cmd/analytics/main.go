package analytics

import (
	"log/slog"
	"os"
	"strings"

	"context"
	"os/signal"
	"syscall"
	"url-shortener/internal/kafka"

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

	if kafkaBrokers == "" || kafkaTopic == "" || kafkaGroupID == "" {
		slog.Error("something wrong with kafka variables")
		os.Exit(1)
	}

	brokers := strings.Split(kafkaBrokers, ",")

	consumer, err := kafka.NewConsumer(brokers, kafkaTopic, kafkaGroupID)
	if err != nil {
		slog.Error("something wrong with kafka consumer")
		os.Exit(1)
	}

	defer consumer.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM)

	defer stop()

	runErr := consumer.Run(ctx)
	if runErr != nil {
		slog.Error("failed running consumer", "error", runErr)
	}

	slog.Info(
		"analytics consumer started",
		"brokers", brokers,
		"topic", kafkaTopic,
		"group_id", kafkaGroupID,
	)
}
