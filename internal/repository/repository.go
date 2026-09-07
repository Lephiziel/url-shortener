package repository

import (
	"context"
	"errors"
	"url-shortener/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	NextID(ctx context.Context) (int64, error)
	SaveLink(ctx context.Context, link domain.Link) (domain.Link, error)
	FindLink(ctx context.Context, code string) (domain.Link, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) NextID(ctx context.Context) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, "SELECT nextval('link_id_seq')").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *PostgresRepository) SaveLink(ctx context.Context, link domain.Link) (domain.Link, error) {
	_, err := r.db.Exec(ctx,
		"INSERT INTO links (code, url, created_at) VALUES ($1, $2, $3)",
		link.Code, link.URL, link.CreatedAt)

	if err != nil {
		return domain.Link{}, err
	}
	return link, nil
}

func (r *PostgresRepository) FindLink(ctx context.Context, code string) (domain.Link, error) {
	var link domain.Link
	err := r.db.QueryRow(ctx,
		"SELECT code, url, created_at FROM links WHERE code = $1", code,
	).Scan(&link.Code, &link.URL, &link.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.Link{}, err
	}
	return link, nil
}
