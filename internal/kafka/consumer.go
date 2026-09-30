package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"url-shortener/internal/event"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	recorder VisitRecorder
	client   *kgo.Client
}

type VisitRecorder interface {
	RecordVisit(
		ctx context.Context,
		visit *event.Visit,
	) error
}

func NewConsumer(brokers []string, topic string, groupID string, recorder VisitRecorder) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(groupID),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll())
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:   client,
		recorder: recorder,
	}, nil
}

func (c *Consumer) processFetches(ctx context.Context, fetches kgo.Fetches) error {
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
		var visitedEvent event.LinkVisitedEvent

		if err := json.Unmarshal(record.Value, &visitedEvent); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}

		eventVisit := event.Visit{
			Code:           visitedEvent.Code,
			OccurredAt:     visitedEvent.OccurredAt,
			KafkaTopic:     record.Topic,
			KafkaPartition: record.Partition,
			KafkaOffset:    record.Offset,
		}

		err := c.recorder.RecordVisit(ctx, &eventVisit)
		if err != nil {
			return fmt.Errorf("failed to save event: %w", err)
		}

		if err := c.client.CommitRecords(ctx, record); err != nil {
			return fmt.Errorf("failed to commit Kafka offset: %w", err)
		}

		c.client.AllowRebalance()
	}

	return nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollRecords(ctx, 1)

		if err := c.processFetches(ctx, fetches); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *Consumer) Close() {
	c.client.AllowRebalance()
	c.client.Close()
}
