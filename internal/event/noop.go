package event

import "context"

type NoopPublisher struct {
}

func (np *NoopPublisher) PublishLinkVisited(ctx context.Context, event LinkVisitedEvent) error {
	return nil
}
