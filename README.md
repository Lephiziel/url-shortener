# URL Shortener

[Русская версия](README.ru.md)

A URL shortener written in Go. It exposes HTTP and gRPC APIs, stores links in PostgreSQL, uses Redis as a cache, and sends link-visit events through Kafka to a separate analytics consumer. The consumer persists visits in PostgreSQL and avoids counting the same Kafka record twice.

## Features

- Short URLs generated from PostgreSQL sequence IDs encoded in base62
- HTTP API for creating links and redirecting with `302 Found`
- gRPC API (`CreateLink` and `GetLink`) sharing the same business logic
- PostgreSQL persistence with a Redis cache-aside layer and database fallback
- Asynchronous, best-effort Kafka publishing that does not block redirects on broker delivery
- Separate Kafka analytics consumer with manual offset commits after database writes
- Visit deduplication using Kafka topic, partition, and offset
- Docker Compose stack with migrations and automatic Kafka topic creation
- Graceful shutdown for the HTTP/gRPC application, Kafka producer flush, and analytics consumer
- Unit tests for the service, HTTP handler, and gRPC server

## Architecture

```text
                ┌────────────────────┐
HTTP / gRPC ────►│  Shortener service │
                └─────────┬──────────┘
                          │
             ┌────────────┴─────────────┐
             │                          │
             ▼                          ▼
    Cached repository           Async Kafka producer
       │       │                        │
       ▼       ▼                        ▼
     Redis  PostgreSQL            link-visited topic
                                          │
                                          ▼
                                   Analytics consumer
                                          │
                                          ▼
                                 PostgreSQL link_visits
```

The application uses a layered design with constructor-based dependency injection. HTTP and gRPC call the same service, which depends on repository and publisher interfaces. Entrypoints are in `cmd/shortener/main.go` and `cmd/analytics/main.go`.

### Link creation and lookup

The service obtains IDs from a PostgreSQL sequence and encodes them using base62. Shortening the same destination multiple times creates separate codes.

For lookups, the cached repository checks Redis first. A cache miss or Redis failure falls back to PostgreSQL; successful database lookups are cached with a TTL. PostgreSQL remains the source of truth. HTTP uses `302`, rather than a permanently cacheable `301`, so requests can continue reaching the service.

### Visit-event pipeline

Successful URL lookups publish a `LinkVisitedEvent` to the `link-visited` Kafka topic. Events contain the short code, destination URL, and event timestamp:

```json
{
  "code": "4",
  "url": "https://example.com",
  "occurred_at": "2026-09-29T11:34:07Z"
}
```

The producer uses `franz-go` and queues records asynchronously, using the short code as the Kafka record key. Delivery errors are logged; redirect availability takes priority over analytics. During normal shutdown, the application attempts to flush outstanding events with a deadline.

The separate analytics process uses a Kafka consumer group (`url-shortener-analytics` by default). For each record, it writes the visit to PostgreSQL and **only then** commits the Kafka offset. The `link_visits` table has a unique constraint on `(kafka_topic, kafka_partition, kafka_offset)`, and inserts use `ON CONFLICT DO NOTHING`. Replaying the **same Kafka record** therefore does not create a second visit row.

This is not end-to-end exactly-once processing: event publication is best-effort, and the database constraint deduplicates repeated consumption of the same Kafka record, not independently produced duplicate events.

Kafka runs in KRaft mode (without ZooKeeper). A one-shot `kafka-init` Compose service creates `link-visited` if necessary; a separate migration job prepares PostgreSQL.

## Technology

| Component | Implementation |
| --- | --- |
| Language / HTTP | Go / `net/http` |
| RPC | gRPC and Protocol Buffers |
| Database | PostgreSQL, `pgx` / `pgxpool` |
| Cache | Redis, `redis/go-redis/v9` |
| Messaging | Apache Kafka (KRaft), `franz-go` |
| Migrations | `golang-migrate` |
| Logging / config | `log/slog`, environment variables, `godotenv` for local runs |
| Containers / tests | Docker Compose, Go `testing`, `httptest`, handwritten mocks |

## Quick start

**Requirements:** Docker with Docker Compose. Install `grpcurl` only if you want to test the gRPC API manually.

```bash
docker compose up -d --build
docker compose ps
```

Compose starts PostgreSQL, Redis, Kafka, a one-shot topic initializer, a migration job, the shortener application, and the analytics consumer. The application and infrastructure ports available on the host are:

| Service | Address |
| --- | --- |
| HTTP API | `http://localhost:8080` |
| gRPC API | `localhost:9090` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| Kafka (external listener) | `localhost:9092` |

Compose supplies container-specific environment variables directly; copying `.env.example` is **not required** for this command. Its database password and exposed ports are for **local development only**; do not deploy this Compose configuration unchanged to production.

To stop the stack without deleting its PostgreSQL volume:

```bash
docker compose down
```

### Run the Go processes locally

Start the infrastructure and migrations first, then configure local connection addresses:

```bash
docker compose up -d postgres redis kafka kafka-init migrate
cp .env.example .env
```

Edit `.env` before running: for the included Compose PostgreSQL instance, use:

```dotenv
DATABASE_URL=postgres://postgres:postgres@localhost:5432/url-shortener?sslmode=disable
BASE_URL=http://localhost:8080/
APP_PORT=8080
GRPC_PORT=9090
REDIS_ADDR=localhost:6379
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=link-visited
KAFKA_GROUP_ID=url-shortener-analytics
```

In separate terminals:

```bash
go run ./cmd/shortener
```

```bash
go run ./cmd/analytics
```

Do not run the locally started services at the same time as the equivalent Compose services if they would conflict on ports or consumer-group membership.

## HTTP API

### Create a link

```bash
curl -i -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

Example response (`201 Created`; the code will vary):

```json
{"short_url":"http://localhost:8080/4"}
```

Malformed requests or invalid URLs return `400`; unexpected infrastructure errors return `500`.

### Redirect

```bash
curl -i http://localhost:8080/4
```

`GET /{code}` returns `302 Found` with the destination in the `Location` header. Unknown codes return `404`. Use the code returned by your own `POST /shorten` request rather than assuming it will be `4`.

## gRPC API

The contract is in [`proto/shortener.proto`](proto/shortener.proto). The gRPC server listens on `localhost:9090` and implements these unary methods:

```text
shortener.ShortenerService/CreateLink
shortener.ShortenerService/GetLink
```

```bash
grpcurl -plaintext -proto proto/shortener.proto \
  -d '{"url":"https://example.com"}' \
  localhost:9090 shortener.ShortenerService/CreateLink
```

```bash
grpcurl -plaintext -proto proto/shortener.proto \
  -d '{"code":"4"}' \
  localhost:9090 shortener.ShortenerService/GetLink
```

Replace `4` with an actual generated code. Validation failures map to `InvalidArgument`, unknown codes to `NotFound`, and unexpected failures to `Internal`.

## Verify analytics

First create a short link, then request its redirect. Inspect the resulting Kafka events:

```bash
docker compose exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka:19092 \
  --topic link-visited --from-beginning
```

Stop the console consumer with Ctrl+C. Check persisted visits:

```bash
docker compose exec postgres \
  psql -U postgres -d url-shortener \
  -c 'SELECT code, COUNT(*) FROM link_visits GROUP BY code ORDER BY code;'
```

Inspect the consumer group's progress:

```bash
docker compose exec kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server kafka:19092 \
  --group url-shortener-analytics --describe
```

Analytics is currently **stored in PostgreSQL only**; there is no public `/stats/{code}` endpoint.

## Tests

```bash
go test ./...
```

Existing unit tests exercise business logic and event-publishing behavior (`internal/service`), HTTP handlers (`internal/handler`), and gRPC methods/status mapping (`internal/grpcserver`). Kafka and PostgreSQL analytics were also checked manually through the Compose integration flow; this is not a claim of automated integration-test coverage.

## Project layout

```text
.
├── cmd/
│   ├── shortener/main.go         # HTTP + gRPC application
│   └── analytics/main.go         # Kafka consumer process
├── internal/
│   ├── analytics/               # Visit persistence
│   ├── domain/                  # Domain types/errors
│   ├── event/                   # Event models and publisher interface
│   ├── grpcserver/              # gRPC transport
│   ├── handler/                 # HTTP transport
│   ├── kafka/                   # Kafka producer and consumer
│   ├── repository/              # PostgreSQL + Redis cache-aside
│   └── service/                 # Business logic / base62
├── migrations/                 # Sequence, links, and visits
├── proto/                      # Protocol Buffers and generated Go code
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── README.md
└── README.ru.md
```

## Scope and trade-offs

This is a learning/portfolio backend project, not a production-hosted URL shortener. The current implementation deliberately prioritizes redirects over guaranteed event delivery. It does not include user accounts, access control, rate limiting, custom aliases, link expiration, or a public statistics API.
