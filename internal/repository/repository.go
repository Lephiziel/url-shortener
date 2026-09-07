package repository

import (
	"context"
	"url-shortener/internal/domain"
)

type Repository interface {
	NextID(ctx context.Context) (int64, error)
	SaveLink(ctx context.Context, link domain.Link) (domain.Link, error)
	FindLink(ctx context.Context, code string) (domain.Link, error)
}

type PostgresRepository struct {
	// database
}

func NewRepository() *PostgresRepository { // Тут будем принимать бдшку
	return &PostgresRepository{} // Сюда будем закидывать бдшку
}

func (r *PostgresRepository) NextID(ctx context.Context) (int64, error) {
	return 0, nil
}

func (r *PostgresRepository) SaveLink(ctx context.Context, link domain.Link) (domain.Link, error) {
	return domain.Link{}, nil
}

func (r *PostgresRepository) FindLink(code string) (domain.Link, error) {
	return domain.Link{}, nil
}
