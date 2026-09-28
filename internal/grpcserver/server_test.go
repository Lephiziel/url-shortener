package grpcserver

import (
	"context"
	"errors"
	"testing"
	"time"
	"url-shortener/internal/domain"

	shortenerpb "url-shortener/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockService struct {
	CreateShortLinkFunc func(ctx context.Context, rawUrl string) (domain.Link, error)
	GetOriginalLinkFunc func(ctx context.Context, code string) (domain.Link, error)
}

func (m *mockService) CreateShortLink(ctx context.Context, rawUrl string) (domain.Link, error) {
	return m.CreateShortLinkFunc(ctx, rawUrl)
}

func (m *mockService) GetOriginalLink(ctx context.Context, code string) (domain.Link, error) {
	return m.GetOriginalLinkFunc(ctx, code)
}

func TestCreateShortLink(t *testing.T) {
	svc := mockService{
		CreateShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			link := domain.Link{
				Code:      "5",
				URL:       rawUrl,
				CreatedAt: time.Now(),
			}
			return link, nil
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	req := &shortenerpb.CreateLinkRequest{
		Url: "https://example.com",
	}

	ctx := context.Background()

	res, err := server.CreateLink(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}

	if res.GetShortUrl() != "http://localhost:8080/5" {
		t.Errorf("expectep url: 'http://localhost:8080/5', but we got %q", res.GetShortUrl())
	}
}

func TestCreateShortLink_InvalidArgument(t *testing.T) {
	svc := mockService{
		CreateShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			link := domain.Link{
				Code:      "1",
				URL:       rawUrl,
				CreatedAt: time.Now(),
			}
			return link, domain.ErrInvalidURL
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	req := &shortenerpb.CreateLinkRequest{
		Url: "github",
	}

	ctx := context.Background()

	_, err := server.CreateLink(ctx, req)
	if err == nil {
		t.Errorf("expected error, but we got nil")
	}

	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, but we got %d", status.Code(err))
	}
}

func TestCreateShortLink_Internal(t *testing.T) {
	svc := mockService{
		CreateShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			return domain.Link{}, errors.New("db failed")
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	ctx := context.Background()

	req := &shortenerpb.CreateLinkRequest{
		Url: "https://github.com",
	}

	_, err := server.CreateLink(ctx, req)
	if err == nil {
		t.Errorf("expected error, but we got nil")
	}

	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal error, but we got %d", status.Code(err))
	}
}

func TestGetOriginalLink_Success(t *testing.T) {
	svc := mockService{
		GetOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			link := domain.Link{
				Code:      code,
				URL:       "https://example.com",
				CreatedAt: time.Now(),
			}
			return link, nil
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	ctx := context.Background()

	req := &shortenerpb.GetLinkRequest{
		Code: "1",
	}

	res, err := server.GetLink(ctx, req)
	if err != nil {
		t.Fatalf("expected no errors, but we got %v", err)
	}

	if res.GetOriginalUrl() != "https://example.com" {
		t.Errorf("expected url:'https://example.com', but we got %q", res.GetOriginalUrl())
	}
}

func TestGetOriginalLink_InvalidArgument(t *testing.T) {
	svc := mockService{
		GetOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			link := domain.Link{
				Code:      code,
				URL:       "https://example.com",
				CreatedAt: time.Now(),
			}
			return link, nil
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	ctx := context.Background()

	req := &shortenerpb.GetLinkRequest{
		Code: "",
	}

	_, err := server.GetLink(ctx, req)
	if err == nil {
		t.Errorf("expected error, but zwe got nil")
	}

	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument error, but we got %d", status.Code(err))
	}
}

func TestGetOriginalLink_NotFound(t *testing.T) {
	svc := mockService{
		GetOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, domain.ErrNotFound
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	ctx := context.Background()

	req := &shortenerpb.GetLinkRequest{
		Code: "1",
	}

	_, err := server.GetLink(ctx, req)
	if err == nil {
		t.Errorf("expected error, but we got nil")
	}

	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound error, but we got %d", status.Code(err))
	}
}

func TestGetOriginalLink_Internal(t *testing.T) {
	svc := mockService{
		GetOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, errors.New("db failed")
		},
	}

	server := NewServer(&svc, "http://localhost:8080")

	ctx := context.Background()

	req := &shortenerpb.GetLinkRequest{
		Code: "1",
	}

	_, err := server.GetLink(ctx, req)
	if err == nil {
		t.Errorf("expected error, but we got nil")
	}

	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal error, but we got %d", status.Code(err))
	}
}
