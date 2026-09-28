package service

import (
	"context"
	"errors"
	"testing"
	"time"
	"url-shortener/internal/domain"
	"url-shortener/internal/event"
)

type mockPublisher struct {
	publishedEvent         event.LinkVisitedEvent
	called                 bool
	publishLinkVisitedFunc func(ctx context.Context, event event.LinkVisitedEvent) error
}

func (m *mockPublisher) PublishLinkVisited(ctx context.Context, e event.LinkVisitedEvent) error {
	m.called = true
	m.publishedEvent = e

	if m.publishLinkVisitedFunc != nil {
		return m.publishLinkVisitedFunc(ctx, e)
	}

	return nil
}

type mockRepository struct {
	nextIDFunc   func(ctx context.Context) (int64, error)
	saveLinkFunc func(ctx context.Context, link domain.Link) (domain.Link, error)
	findLinkFunc func(ctx context.Context, code string) (domain.Link, error)
}

func (m *mockRepository) NextID(ctx context.Context) (int64, error) {
	return m.nextIDFunc(ctx)
}

func (m *mockRepository) SaveLink(ctx context.Context, link domain.Link) (domain.Link, error) {
	return m.saveLinkFunc(ctx, link)
}

func (m *mockRepository) FindLink(ctx context.Context, code string) (domain.Link, error) {
	return m.findLinkFunc(ctx, code)
}

func TestCreateShortLink_Success(t *testing.T) {
	repo := mockRepository{
		nextIDFunc: func(ctx context.Context) (int64, error) {
			return 1, nil
		},
		saveLinkFunc: func(ctx context.Context, link domain.Link) (domain.Link, error) {
			return link, nil
		},
	}
	svc := NewService(&repo, &event.NoopPublisher{})

	result, err := svc.CreateShortLink(context.Background(), "https://example.com")

	if err != nil {
		t.Fatalf("expected no errors, but we got %v", err)
	}
	if result.Code != "1" {
		t.Errorf("expected code 1, but we got %q", result.Code)
	}
	if result.URL != "https://example.com" {
		t.Errorf("expected URL: 'https://example.com', but we got %q", result.URL)
	}
}

func TestCreateShortLink_InvalidURL(t *testing.T) {
	repo := mockRepository{}
	svc := NewService(&repo, &event.NoopPublisher{})

	_, err := svc.CreateShortLink(context.Background(), "some-string")

	if !errors.Is(err, domain.ErrInvalidURL) {
		t.Errorf("expected ErrInvalidURL, but we got %v", err)
	}
}

func TestCreateShortLink_FailedDBConnection(t *testing.T) {
	dbErr := errors.New("some error")
	repo := mockRepository{
		nextIDFunc: func(ctx context.Context) (int64, error) {
			return 0, dbErr
		},
	}
	svc := NewService(&repo, &event.NoopPublisher{})

	_, err := svc.CreateShortLink(context.Background(), "https://example.com")

	if !errors.Is(err, dbErr) {
		t.Errorf("expected an db error %v, but we got %v", dbErr, err)
	}
}

func TestGetOriginalLink_Success(t *testing.T) {
	link := domain.Link{
		Code:      "1",
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	}

	repo := mockRepository{
		findLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return link, nil
		},
	}
	publisher := mockPublisher{}

	svc := NewService(&repo, &publisher)

	result, err := svc.GetOriginalLink(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected no errors, but we got %v", err)
	}
	if result.Code != "1" {
		t.Errorf("expected code 1, but we got %q", result.Code)
	}
	if result.URL != "https://example.com" {
		t.Errorf("expected url: 'https://example.com', but we got %q", result.URL)
	}

	if !publisher.called {
		t.Fatal("expected event to be published")
	}
	if publisher.publishedEvent.Code != link.Code {
		t.Errorf("expected code %s, but got %s", link.Code, publisher.publishedEvent.Code)
	}
	if publisher.publishedEvent.URL != link.URL {
		t.Errorf("expected url: %q, but we got: %q", link.URL, publisher.publishedEvent.URL)
	}
	if publisher.publishedEvent.OccurredAt.IsZero() {
		t.Errorf("expected OccurredAt to be set, but we got %v", publisher.publishedEvent.OccurredAt)
	}
}

func TestGetOriginalLink_NotFound(t *testing.T) {
	repo := mockRepository{
		findLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, domain.ErrNotFound
		},
	}

	svc := NewService(&repo, &event.NoopPublisher{})

	_, err := svc.GetOriginalLink(context.Background(), "1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound error, but we got %v", err)
	}
}

func TestGetOriginalLink_FailedKafka(t *testing.T) {
	link := domain.Link{
		Code:      "1",
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	}

	repo := mockRepository{
		findLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return link, nil
		},
	}
	publisher := mockPublisher{
		publishLinkVisitedFunc: func(ctx context.Context, event event.LinkVisitedEvent) error {
			return errors.New("kafka unavailaible")
		},
	}

	svc := NewService(&repo, &publisher)

	result, err := svc.GetOriginalLink(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected no errors, but we got %v", err)
	}
	if result.Code != "1" {
		t.Errorf("expected code 1, but we got %q", result.Code)
	}
	if result.URL != "https://example.com" {
		t.Errorf("expected url: 'https://example.com', but we got %q", result.URL)
	}
}
