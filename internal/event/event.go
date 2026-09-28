package event

import (
	"context"
	"time"
)

type LinkVisitedEvent struct {
	Code       string
	URL        string
	OccurredAt time.Time
}

type Publisher interface {
	PublishLinkVisited(ctx context.Context, event LinkVisitedEvent) error
}
