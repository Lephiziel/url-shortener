package analytics

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) RecordVisit(ctx context.Context, visit Visit) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO link_visits (
			code,
			occurred_at,
			kafka_topic,
			kafka_partition,
			kafka_offset
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (
			kafka_topic,
			kafka_partition,
			kafka_offset
		)
		DO NOTHING;`,
		visit.Code,
		visit.OccurredAt,
		visit.KafkaTopic,
		visit.KafkaPartition,
		visit.KafkaOffset,
	)

	return err
}
