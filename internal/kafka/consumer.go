package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"url-shortener/internal/event"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

func NewConsumer(brokers []string, topic string, groupID string) (*Consumer, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumerGroup(groupID))
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client: client,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		var visitedEvent event.LinkVisitedEvent

		fetches := c.client.PollFetches(ctx)

		if ctx.Err() != nil {
			return nil
		}

		for _, fetchErr := range fetches.Errors() {
			slog.Error(
				"failed to fetch Kafka records",
				"topic", fetchErr.Topic,
				"partition", fetchErr.Partition,
				"error", fetchErr.Err,
			)
		}

		for _, record := range fetches.Records() {
			if err := json.Unmarshal(record.Value, &visitedEvent); err != nil {
				slog.Error("failed to unmarshal event", "error", err)
				continue
			} else {
				slog.Info(
					"link visited",
					"code", visitedEvent.Code,
					"url", visitedEvent.URL,
					"occurred_at", visitedEvent.OccurredAt)
			}
		}
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
