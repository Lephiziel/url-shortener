package kafka

import (
	"context"
	"encoding/json"
	"url-shortener/internal/event"

	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaPublisher struct {
	client *kgo.Client
	topic  string
}

func NewPublisher(brokers []string, topic string) (*KafkaPublisher, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
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

	res := p.client.ProduceSync(ctx, record)
	if err := res.FirstErr(); err != nil {
		return err
	}

	return nil
}

func (p *KafkaPublisher) Close() {
	var _ event.Publisher = (*KafkaPublisher)(nil)
	p.client.Close()
}
