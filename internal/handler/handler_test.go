package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"url-shortener/internal/domain"
)

type mockService struct {
	createShortLinkFunc func(ctx context.Context, rawUrl string) (domain.Link, error)
	getOriginalLinkFunc func(ctx context.Context, code string) (domain.Link, error)
}

func (m *mockService) CreateShortLink(ctx context.Context, rawUrl string) (domain.Link, error) {
	return m.createShortLinkFunc(ctx, rawUrl)
}

func (m *mockService) GetOriginalLink(ctx context.Context, code string) (domain.Link, error) {
	return m.getOriginalLinkFunc(ctx, code)
}

func TestCreateShortLink_Success(t *testing.T) {
	svc := mockService{
		createShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			link := domain.Link{
				Code:      "1",
				URL:       rawUrl,
				CreatedAt: time.Now(),
			}
			return link, nil
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	jsonBody := `{"url": "https://example.com"}`

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.CreateLink(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, but we got %d", rec.Code)
	}

	var response CreateLinkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ShortURL != "http://localhost:8080"+"/"+"1" {
		t.Errorf("expected 'http:localhost:8080/1' but we got %q", response.ShortURL)
	}
}

func TestCreateShortLink_InvalidJSON(t *testing.T) {
	svc := mockService{
		createShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			return domain.Link{}, nil
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	jsonBody := `{"url":"not-url"`

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.CreateLink(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid json, but we got %d", rec.Code)
	}
}

func TestCreateShortLink_InvalidURL(t *testing.T) {
	svc := mockService{
		createShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			return domain.Link{}, domain.ErrInvalidURL
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	jsonBody := `{"url":""}`

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.CreateLink(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid url, but we got %d", rec.Code)
	}
}

func TestCreateShortLink_InternalServerError(t *testing.T) {
	svc := mockService{
		createShortLinkFunc: func(ctx context.Context, rawUrl string) (domain.Link, error) {
			return domain.Link{}, errors.New("some error")
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	jsonBody := `{"url":""}`

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.CreateLink(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, but we got %d", rec.Code)
	}
}

func TestRedirectLink_Success(t *testing.T) {
	svc := mockService{
		getOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			link := domain.Link{
				Code:      "1",
				URL:       "https://example.com",
				CreatedAt: time.Now(),
			}
			return link, nil
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/1", nil)
	req.SetPathValue("code", "1")

	rec := httptest.NewRecorder()

	handler.RedirectLink(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status found, but we got %d", rec.Code)
	}

	url := rec.Header().Get("Location")

	if url != "https://example.com" {
		t.Errorf("expected url: 'https://example.com', but we got %q", url)
	}
}

func TestRedirectLink_NotFound(t *testing.T) {
	svc := mockService{
		getOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, domain.ErrNotFound
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/2", nil)
	req.SetPathValue("code", "2")

	rec := httptest.NewRecorder()

	handler.RedirectLink(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, but we got %d", rec.Code)
	}
}

func TestRedirectLink_EmptyCode(t *testing.T) {
	svc := mockService{
		getOriginalLinkFunc: func(ctx context.Context, code string) (domain.Link, error) {
			return domain.Link{}, nil
		},
	}

	handler := NewHandler(&svc, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("code", "")

	rec := httptest.NewRecorder()

	handler.RedirectLink(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, but we got %d", rec.Code)
	}
}
