package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"url-shortener/internal/event"

	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaPublisher struct {
	client *kgo.Client
	topic  string
}

var _ event.Publisher = (*KafkaPublisher)(nil)

func NewPublisher(brokers []string, topic string) (*KafkaPublisher, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.MaxBufferedRecords(1000),
		kgo.RecordDeliveryTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	return &KafkaPublisher{
		client: client,
		topic:  topic,
	}, nil
}

func (p *KafkaPublisher) PublishLinkVisited(ctx context.Context, e event.LinkVisitedEvent) error {
	data, jsonErr := json.Marshal(e)
	if jsonErr != nil {
		return jsonErr
	}

	record := &kgo.Record{
		Topic: p.topic,
		Value: data,
		Key:   []byte(e.Code),
	}

	p.client.TryProduce(context.Background(), record, func(r *kgo.Record, err error) {
		if err != nil {
			slog.Error(
				"failed to deliver Kafka event",
				"error", err,
				"topic", r.Topic,
				"key", string(r.Key),
			)
		}
	},
	)

	return nil
}

func (p *KafkaPublisher) Close() {
	var _ event.Publisher = (*KafkaPublisher)(nil)
	p.client.Close()
}

func (p *KafkaPublisher) Flush(ctx context.Context) error {
	return p.client.Flush(ctx)
}
