package service

import (
	"context"
	"net/url"
	"time"
	"url-shortener/internal/domain"
	"url-shortener/internal/repository"
)

type Service interface {
	CreateShortLink(ctx context.Context, rawUrl string) (domain.Link, error)
	GetOriginalLink(ctx context.Context, code string) (domain.Link, error)
}

type LinkService struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) *LinkService {
	return &LinkService{
		repo: repo,
	}
}

// Base 62 algorithm
const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func encodeBase62(id int64) string {
	if id == 0 {
		return string(base62Alphabet[0])
	}
	var result []byte
	for id > 0 {
		remainder := id % 62
		result = append(result, base62Alphabet[remainder])
		id /= 62
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

func (ls *LinkService) CreateShortLink(ctx context.Context, rawUrl string) (domain.Link, error) {
	u, parseErr := url.ParseRequestURI(rawUrl)
	if parseErr != nil {
		return domain.Link{}, domain.ErrInvalidURL
	}

	if u.Scheme == "" || u.Host == "" {
		return domain.Link{}, domain.ErrInvalidURL
	}

	nextID, idErr := ls.repo.NextID(ctx)
	if idErr != nil {
		return domain.Link{}, idErr
	}

	code := encodeBase62(nextID)

	link := domain.Link{
		Code:      code,
		URL:       rawUrl,
		CreatedAt: time.Now().UTC(),
	}

	res, err := ls.repo.SaveLink(ctx, link)
	if err != nil {
		return domain.Link{}, err
	}

	return res, nil
}

func (ls *LinkService) GetOriginalLink(ctx context.Context, code string) (domain.Link, error) {
	res, err := ls.repo.FindLink(ctx, code)
	if err != nil {
		return domain.Link{}, err
	}

	return res, nil
}
