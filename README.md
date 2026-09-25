# URL Shortener

[Русская версия](README.ru.md)

A URL shortening service written in Go, using the standard `net/http` package for the web layer, PostgreSQL for persistent storage, and Redis for caching.

## Features

- Shorten long URLs into compact codes
- Redirect short codes to their original URLs
- Redis cache-aside layer in front of PostgreSQL for read-heavy redirect traffic
- Graceful shutdown with SIGINT/SIGTERM handling
- Fully containerized setup: one command starts the application, PostgreSQL, Redis, and runs database migrations

## Architecture

The project follows a layered architecture with constructor-based dependency injection. All dependencies are wired together in `main.go`:

```text
domain/       — core types and sentinel errors (Link, ErrNotFound, ErrInvalidURL)
repository/   — data access: Repository interface, PostgresRepository, CachedRepository
service/      — business logic: URL validation, base62 encoding, orchestration
handler/      — HTTP layer: request/response handling and status codes
```

Each layer depends on an interface provided by the layer below it rather than on a concrete implementation:

```text
handler → service.Service
service → repository.Repository
repository.CachedRepository → repository.Repository (PostgresRepository) + Redis client
```

As a result, the `service` layer does not need to know whether it is backed directly by PostgreSQL or by a Redis-cached repository. That decision is made once during dependency wiring in `main.go`.

### Short Code Generation

Short codes are generated from auto-incrementing IDs obtained from a dedicated PostgreSQL `SEQUENCE` through `NextID`.

The ID is encoded using a small custom base62 encoder without external dependencies or unnecessary allocations on the hot path.

The sequence ID is requested **before** inserting the record. This keeps encoding logic inside the `service` layer rather than the `repository` layer and preserves the separation between persistence and business logic.

### Caching

`GET /{code}` is expected to be the most frequently used endpoint in a URL shortener, so URL lookup uses the cache-aside pattern:

1. Look up the code in Redis
2. On a cache miss or Redis error, query PostgreSQL
3. Store the result back in Redis with a TTL

Redis is treated as an optimization rather than a required dependency.

If Redis is unavailable or returns an error, the service continues to operate correctly using PostgreSQL. The cache therefore never becomes a single point of failure.

### Design Trade-offs

- **Duplicate URLs are allowed.** Shortening the same URL multiple times creates different short codes. This makes it possible to track the same destination independently across different campaigns or channels. HTTP idempotency is a separate concern and could be implemented later using a mechanism such as `Idempotency-Key`.

- **Redirects use `302 Found` instead of `301 Moved Permanently`.** A permanent redirect may be cached indefinitely by browsers, which would interfere with future analytics and make changing or deleting links harder.

## Tech Stack

| Area | Technology |
|---|---|
| HTTP | `net/http` (Go 1.27 `ServeMux`, method + path patterns) |
| Database | PostgreSQL via `pgx` / `pgxpool` |
| Cache | Redis via `redis/go-redis/v9` |
| Migrations | `golang-migrate` |
| Logging | `log/slog` |
| Configuration | Environment variables (`godotenv` for local `.env` loading) |
| Containers | Docker, multi-stage build, Docker Compose |
| Testing | `testing`, `net/http/httptest`, handwritten mocks |

## Quick Start

### Requirements

- Docker
- Docker Compose

### Run with Docker Compose

```bash
cp .env.example .env
docker-compose up --build
```

The services are started in the following order:

```text
PostgreSQL → migrations → Redis → application
```

The API will be available at:

```text
http://localhost:8080
```

### Run Locally

This requires PostgreSQL and Redis to be installed locally.

```bash
cp .env.example .env

# Adjust DATABASE_URL and REDIS_ADDR in .env for your local environment

migrate -path migrations -database "$DATABASE_URL" up
go run ./cmd/shortener
```

### Environment Variables

| Variable | Description | Example |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@localhost:5432/url-shortener?sslmode=disable` |
| `BASE_URL` | Base URL used to construct shortened URLs | `http://localhost:8080/` |
| `APP_PORT` | HTTP server port | `8080` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |

## API

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

## Testing

```bash
go test ./... -v
```

Test coverage includes:

- `service` — unit tests using a handwritten `mockRepository`, covering successful URL creation and lookup, URL validation, and repository error propagation
- `handler` — unit tests using a handwritten `mockService` and `net/http/httptest`, covering HTTP status branches for both endpoints

## Project Structure

```text
.
├── cmd/
│   └── shortener/
│       └── main.go          # dependency wiring, server startup, graceful shutdown
├── internal/
│   ├── domain/              # Link type and sentinel errors
│   ├── repository/          # Repository interface, PostgreSQL and Redis cache implementations
│   ├── service/             # business logic, validation, base62 encoding
│   └── handler/             # HTTP handlers
├── migrations/              # SQL migrations for golang-migrate
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── go.mod
```

## Possible Improvements

Potential future additions include:

- Custom aliases
- Link expiration
- Rate limiting
- Click analytics
- gRPC API alongside the REST API