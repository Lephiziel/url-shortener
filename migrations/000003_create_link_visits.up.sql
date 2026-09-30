CREATE TABLE IF NOT EXISTS link_visits (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    kafka_topic TEXT NOT NULL,
    kafka_partition INTEGER NOT NULL,
    kafka_offset BIGINT NOT NULL,

    CONSTRAINT unique_kafka_event
        UNIQUE (kafka_topic, kafka_partition, kafka_offset)
);

CREATE INDEX idx_link_visits_code_occurred_at
ON link_visits (code, occurred_at);