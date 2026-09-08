package repository

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
	"url-shortener/internal/domain"

	"github.com/redis/go-redis/v9"
)

type CachedRepository struct {
	repo  Repository
	cache *redis.Client
	ttl   time.Duration
}

func NewCachedRepository(repo Repository, cache *redis.Client, ttl time.Duration) *CachedRepository {
	return &CachedRepository{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

func (c *CachedRepository) NextID(ctx context.Context) (int64, error) {
	return c.repo.NextID(ctx)
}

func (c *CachedRepository) SaveLink(ctx context.Context, link domain.Link) (domain.Link, error) {
	return c.repo.SaveLink(ctx, link)
}

func (c *CachedRepository) FindLink(ctx context.Context, code string) (domain.Link, error) {
	var link domain.Link

	queryRes, redisErr := c.cache.Get(ctx, code).Result()
	if redisErr == nil {
		if err := json.Unmarshal([]byte(queryRes), &link); err != nil {
			return domain.Link{}, err
		}
		return link, nil
	}

	if !errors.Is(redisErr, redis.Nil) {
		slog.Warn("something bad with redis", "error", redisErr)
	}

	res, err := c.repo.FindLink(ctx, code)
	if err != nil {
		return domain.Link{}, err
	}

	jsonBytes, jsonErr := json.Marshal(res)
	if jsonErr != nil {
		slog.Warn("failed to marshal link for redis", "error", jsonErr)
		return res, nil
	}

	if err := c.cache.Set(ctx, code, jsonBytes, c.ttl).Err(); err != nil {
		slog.Warn("failed to write cache >~<", "error", err)
	}

	return res, nil
}
