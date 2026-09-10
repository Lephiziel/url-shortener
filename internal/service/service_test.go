package service

import (
	"context"
	"errors"
	"testing"
	"time"
	"url-shortener/internal/domain"
)

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
	svc := NewService(&repo)

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
	svc := NewService(&repo)

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
	svc := NewService(&repo)

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

	svc := NewService(&repo)

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

func TestGetOriginalLink_NotFound(t *testing.T) {
	repo := mockRepository{
		findLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, domain.ErrNotFound
		},
	}

	svc := NewService(&repo)

	_, err := svc.GetOriginalLink(context.Background(), "1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound error, but we got %v", err)
	}
}
