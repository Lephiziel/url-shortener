package event

import (
	"context"
	"time"
)

type LinkVisitedEvent struct {
	Code       string    `json:"code"`
	URL        string    `json:"url"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Publisher interface {
	PublishLinkVisited(ctx context.Context, event LinkVisitedEvent) error
}
