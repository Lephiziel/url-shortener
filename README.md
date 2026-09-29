# URL Shortener

[Русская версия](README.ru.md)

A URL shortening service written in Go with HTTP and gRPC APIs, PostgreSQL for persistent storage, Redis for caching, and Kafka for link-visit events.

## Features

- Shorten long URLs into compact base62 codes
- Resolve short codes through both HTTP and gRPC
- HTTP redirects with `302 Found`
- Redis cache-aside layer in front of PostgreSQL for read-heavy lookup traffic
- gRPC API generated from Protocol Buffers
- Kafka producer for `LinkVisitedEvent` analytics events
- Analytics publishing is best-effort: Kafka failures do not break URL resolution or redirects
- Graceful shutdown for HTTP and gRPC on SIGINT/SIGTERM
- Fully containerized setup with PostgreSQL, Redis, Kafka, migrations, and the application
- Unit tests for the service, HTTP transport, and gRPC transport using handwritten mocks

## Architecture

The project follows a layered architecture with constructor-based dependency injection. Dependencies are wired in `cmd/shortener/main.go`.

```text
domain/       — core types and sentinel errors
event/        — analytics event model and Publisher abstraction
repository/   — PostgreSQL persistence and Redis cache-aside wrapper
service/      — business logic, URL validation, base62 encoding, event publishing
a handler/     — HTTP transport
grpcserver/   — gRPC transport
kafka/        — Kafka implementation of event.Publisher
proto/        — Protocol Buffers contract and generated Go/gRPC code
```

The main dependency flow is:

```text
HTTP handler ─┐
              ├→ service.Service → repository.Repository
 gRPC server ─┘          │
                         └→ event.Publisher → KafkaPublisher → Kafka

CachedRepository → PostgresRepository + Redis
```

The service layer depends on interfaces rather than concrete infrastructure. This keeps HTTP/gRPC transport, persistence, caching, and event delivery replaceable and independently testable.

### Short Code Generation

Short codes are generated from IDs obtained from a dedicated PostgreSQL `SEQUENCE` through `NextID`.

The ID is encoded with a small custom base62 encoder. The sequence ID is requested before inserting the record so code-generation logic remains in the service layer instead of the repository layer.

### Caching

URL lookup uses the cache-aside pattern:

1. Look up the short code in Redis
2. On a cache miss or Redis error, query PostgreSQL
3. Store the result back in Redis with a TTL

Redis is an optimization rather than the source of truth. If Redis is unavailable, lookups can still fall back to PostgreSQL.

### gRPC

The gRPC contract is defined in `proto/shortener.proto` and exposes two unary RPCs:

```text
CreateLink(CreateLinkRequest) → CreateLinkReply
GetLink(GetLinkRequest)       → GetLinkReply
```

Both HTTP and gRPC transports use the same `service.Service` implementation, so validation and business rules are shared rather than duplicated.

Domain errors are translated into gRPC status codes in the transport layer, for example:

- invalid URL → `InvalidArgument`
- unknown short code → `NotFound`
- unexpected infrastructure error → `Internal`

### Kafka Events

After a short code is resolved successfully, the service creates a `LinkVisitedEvent` containing:

```json
{
  "code": "4",
  "url": "https://example.com",
  "occurred_at": "2026-09-29T11:34:07.13781655Z"
}
```

The event is published to the `link-visited` Kafka topic through the `event.Publisher` abstraction. The current implementation uses `franz-go`.

Publishing is intentionally best-effort: if Kafka is unavailable, the error is logged but the original URL is still returned. Analytics must not become a dependency that can break redirects.

The Kafka broker runs in KRaft mode in Docker Compose, without ZooKeeper.

### Design Trade-offs

- **Duplicate URLs are allowed.** Shortening the same URL multiple times creates different short codes. This makes it possible to track the same destination independently across campaigns or channels.
- **Redirects use `302 Found` instead of `301 Moved Permanently`.** Permanent redirects may be cached aggressively by clients, which would interfere with analytics and future link changes.
- **Kafka event delivery is currently best-effort.** This keeps the redirect path available when analytics infrastructure is down. Stronger delivery guarantees can be added later with patterns such as an outbox.

## Tech Stack

| Area | Technology |
|---|---|
| HTTP | Go `net/http` |
| gRPC | `google.golang.org/grpc` |
| Serialization | Protocol Buffers for gRPC, JSON for Kafka events |
| Database | PostgreSQL via `pgx` / `pgxpool` |
| Cache | Redis via `redis/go-redis/v9` |
| Messaging | Apache Kafka in KRaft mode |
| Kafka client | `franz-go` |
| Migrations | `golang-migrate` |
| Logging | `log/slog` |
| Configuration | Environment variables, `godotenv` for local `.env` loading |
| Containers | Docker, multi-stage build, Docker Compose |
| Testing | Go `testing`, `net/http/httptest`, handwritten mocks |

## Quick Start

### Requirements

- Docker
- Docker Compose

`grpcurl` is useful for manually testing the gRPC API.

### Run with Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

The stack includes:

```text
PostgreSQL
Redis
Kafka (KRaft)
migration job
URL Shortener application
```

The application exposes:

```text
HTTP: http://localhost:8080
gRPC: localhost:9090
Kafka: localhost:9092
```

### Run Locally

For a full local setup outside Docker, provide PostgreSQL, Redis, and Kafka and configure `.env` accordingly.

```bash
cp .env.example .env
migrate -path migrations -database "$DATABASE_URL" up
go run ./cmd/shortener
```

### Environment Variables

| Variable | Description | Example |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@localhost:5432/url-shortener?sslmode=disable` |
| `BASE_URL` | Base URL used to construct shortened URLs | `http://localhost:8080/` |
| `APP_PORT` | HTTP server port | `8080` |
| `GRPC_PORT` | gRPC server port | `9090` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `KAFKA_BROKERS` | Comma-separated Kafka bootstrap brokers | `localhost:9092` |
| `KAFKA_TOPIC` | Topic for link-visit events | `link-visited` |

Inside Docker Compose the application uses Kafka's internal listener, for example `kafka:19092`, while host tools connect through `localhost:9092`.

## HTTP API

### Create a Short URL

```http
POST /shorten
Content-Type: application/json
```

```json
{
  "url": "https://example.com/some/very/long/path"
}
```

**Response — `201 Created`**

```json
{
  "short_url": "http://localhost:8080/1"
}
```

**Errors**

- `400 Bad Request` — malformed JSON or missing/invalid URL
- `500 Internal Server Error` — unexpected server error

### Redirect to the Original URL

```http
GET /{code}
```

Returns `302 Found` with a `Location` header pointing to the original URL.

**Errors**

- `400 Bad Request` — missing code
- `404 Not Found` — no URL exists for the supplied code
- `500 Internal Server Error` — unexpected server error

## gRPC API

### CreateLink

```bash
grpcurl \
  -plaintext \
  -proto proto/shortener.proto \
  -d '{"url":"https://example.com"}' \
  localhost:9090 \
  shortener.ShortenerService/CreateLink
```

Example response:

```json
{
  "shortUrl": "http://localhost:8080/4"
}
```

### GetLink

```bash
grpcurl \
  -plaintext \
  -proto proto/shortener.proto \
  -d '{"code":"4"}' \
  localhost:9090 \
  shortener.ShortenerService/GetLink
```

Example response:

```json
{
  "originalUrl": "https://example.com"
}
```

## Inspect Kafka Events

After resolving a short link, events can be inspected from the Kafka container:

```bash
docker exec -it url-shortener-kafka-1 \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic link-visited \
  --from-beginning
```

## Testing

```bash
go test ./... -v
```

Current tests include:

- `service` — URL creation/lookup, validation, repository errors, link-visit event publishing, and publisher-failure behavior
- `handler` — HTTP transport branches with `httptest` and a handwritten `mockService`
- `grpcserver` — successful RPCs and gRPC status mapping for invalid input, missing links, and internal errors

## Project Structure

```text
.
├── cmd/
│   └── shortener/
│       └── main.go
├── internal/
│   ├── domain/
│   ├── event/
│   ├── grpcserver/
│   ├── handler/
│   ├── kafka/
│   ├── repository/
│   └── service/
├── proto/
│   ├── shortener.proto
│   ├── shortener.pb.go
│   └── shortener_grpc.pb.go
├── migrations/
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── go.mod
```

## Next Steps

Planned improvements include:

- Kafka consumer / analytics service
- Persistent click statistics and analytics API
- Stronger event-delivery guarantees, for example an outbox pattern
- Rate limiting
- Custom aliases
- Link expiration
