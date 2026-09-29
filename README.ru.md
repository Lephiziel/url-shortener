# URL Shortener

[English version](README.md)

Сервис сокращения ссылок на Go с HTTP- и gRPC-API, PostgreSQL для постоянного хранения, Redis для кэширования и Kafka для событий переходов по коротким ссылкам.

## Возможности

- Сокращение длинных URL в компактные base62-коды
- Получение оригинальной ссылки через HTTP и gRPC
- HTTP-редиректы через `302 Found`
- Redis cache-aside перед PostgreSQL для частых запросов на чтение
- gRPC API, сгенерированный из Protocol Buffers
- Kafka producer для аналитических событий `LinkVisitedEvent`
- Best-effort публикация аналитики: недоступность Kafka не ломает получение ссылки и редиректы
- Graceful shutdown HTTP и gRPC при SIGINT/SIGTERM
- Полностью контейнеризированный стек: PostgreSQL, Redis, Kafka, миграции и приложение
- Юнит-тесты service-, HTTP- и gRPC-слоёв с написанными вручную моками

## Архитектура

Проект построен по слоям с dependency injection через конструкторы. Зависимости собираются в `cmd/shortener/main.go`.

```text
domain/       — базовые типы и sentinel-ошибки
event/        — модель аналитических событий и абстракция Publisher
repository/   — PostgreSQL и cache-aside обёртка с Redis
service/      — бизнес-логика, валидация URL, base62, публикация событий
handler/      — HTTP transport
grpcserver/   — gRPC transport
kafka/        — Kafka-реализация event.Publisher
proto/        — protobuf-контракт и сгенерированный Go/gRPC-код
```

Основной поток зависимостей:

```text
HTTP handler ─┐
              ├→ service.Service → repository.Repository
 gRPC server ─┘          │
                         └→ event.Publisher → KafkaPublisher → Kafka

CachedRepository → PostgresRepository + Redis
```

`service` зависит от интерфейсов, а не от конкретной инфраструктуры. Благодаря этому HTTP/gRPC transport, хранилище, кэш и доставка событий можно тестировать и заменять независимо друг от друга.

### Генерация коротких кодов

Короткие коды генерируются из ID, получаемых из отдельной PostgreSQL `SEQUENCE` через `NextID`. ID кодируется собственным base62-энкодером. Sequence ID запрашивается до вставки записи, поэтому логика генерации кода остаётся в `service`, а не переносится в `repository`.

### Кэширование

Поиск ссылки использует cache-aside:

1. Ищем короткий код в Redis
2. При cache miss или ошибке Redis обращаемся к PostgreSQL
3. Кладём найденный результат обратно в Redis с TTL

Redis используется как оптимизация, а не как source of truth. Если Redis недоступен, поиск может продолжить работу через PostgreSQL.

### gRPC

Контракт находится в `proto/shortener.proto` и содержит два unary RPC:

```text
CreateLink(CreateLinkRequest) → CreateLinkReply
GetLink(GetLinkRequest)       → GetLinkReply
```

HTTP и gRPC используют одну и ту же реализацию `service.Service`, поэтому бизнес-правила и валидация не дублируются между transport-слоями.

Domain-ошибки преобразуются в gRPC status codes на уровне transport:

- невалидный URL → `InvalidArgument`
- неизвестный короткий код → `NotFound`
- неожиданная инфраструктурная ошибка → `Internal`

### Kafka-события

После успешного получения оригинальной ссылки service создаёт `LinkVisitedEvent`:

```json
{
  "code": "4",
  "url": "https://example.com",
  "occurred_at": "2026-09-29T11:34:07.13781655Z"
}
```

Событие публикуется в Kafka topic `link-visited` через интерфейс `event.Publisher`. Текущая реализация producer использует `franz-go`.

Публикация сделана best-effort: если Kafka недоступна, ошибка логируется, но оригинальный URL всё равно возвращается. Аналитика не должна становиться зависимостью, способной сломать редиректы.

Kafka запускается в Docker Compose в KRaft-режиме, без ZooKeeper.

### Осознанные компромиссы

- **Дубликаты URL разрешены.** Повторное сокращение одного URL создаёт новый короткий код. Это позволяет независимо отслеживать один destination для разных кампаний или каналов.
- **Редиректы используют `302 Found`, а не `301 Moved Permanently`.** Постоянный редирект может агрессивно кэшироваться клиентом, что мешает аналитике и будущему изменению ссылки.
- **Доставка Kafka-события сейчас best-effort.** Основной redirect path остаётся доступным даже при проблемах с аналитической инфраструктурой. Более сильные гарантии можно добавить позже, например через outbox pattern.

## Стек технологий

| Область | Технология |
|---|---|
| HTTP | Go `net/http` |
| gRPC | `google.golang.org/grpc` |
| Сериализация | Protocol Buffers для gRPC, JSON для Kafka events |
| База данных | PostgreSQL через `pgx` / `pgxpool` |
| Кэш | Redis через `redis/go-redis/v9` |
| Messaging | Apache Kafka в KRaft-режиме |
| Kafka client | `franz-go` |
| Миграции | `golang-migrate` |
| Логирование | `log/slog` |
| Конфигурация | Переменные окружения, `godotenv` для локального `.env` |
| Контейнеризация | Docker, multi-stage build, Docker Compose |
| Тестирование | Go `testing`, `net/http/httptest`, handwritten mocks |

## Быстрый старт

### Требования

- Docker
- Docker Compose
- `grpcurl` для ручного тестирования gRPC

### Запуск через Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

Поднимаются PostgreSQL, Redis, Kafka (KRaft), одноразовый сервис миграций и URL Shortener.

```text
HTTP: http://localhost:8080
gRPC: localhost:9090
Kafka: localhost:9092
```

### Локальный запуск

Для полного запуска вне Docker нужны PostgreSQL, Redis и Kafka с корректными значениями в `.env`.

```bash
cp .env.example .env
migrate -path migrations -database "$DATABASE_URL" up
go run ./cmd/shortener
```

### Переменные окружения

| Переменная | Описание | Пример |
|---|---|---|
| `DATABASE_URL` | Строка подключения к PostgreSQL | `postgres://postgres:postgres@localhost:5432/url-shortener?sslmode=disable` |
| `BASE_URL` | Базовый URL для формирования короткой ссылки | `http://localhost:8080/` |
| `APP_PORT` | Порт HTTP-сервера | `8080` |
| `GRPC_PORT` | Порт gRPC-сервера | `9090` |
| `REDIS_ADDR` | Адрес Redis | `localhost:6379` |
| `KAFKA_BROKERS` | Kafka bootstrap brokers через запятую | `localhost:9092` |
| `KAFKA_TOPIC` | Topic для событий переходов | `link-visited` |

В Docker Compose приложение подключается к внутреннему listener Kafka (`kafka:19092`), а инструменты на хосте — через `localhost:9092`.

## HTTP API

### Создать короткую ссылку

```http
POST /shorten
Content-Type: application/json
```

```json
{
  "url": "https://example.com/some/very/long/path"
}
```

**Ответ — `201 Created`**

```json
{
  "short_url": "http://localhost:8080/1"
}
```

### Редирект на оригинальный URL

```http
GET /{code}
```

Возвращает `302 Found` с заголовком `Location`, указывающим на оригинальный URL.

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

### GetLink

```bash
grpcurl \
  -plaintext \
  -proto proto/shortener.proto \
  -d '{"code":"4"}' \
  localhost:9090 \
  shortener.ShortenerService/GetLink
```

## Просмотр Kafka-событий

```bash
docker exec -it url-shortener-kafka-1 \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic link-visited \
  --from-beginning
```

## Тестирование

```bash
go test ./... -v
```

Сейчас тестами покрыты:

- `service` — создание/поиск ссылок, валидация URL, ошибки repository, публикация `LinkVisitedEvent` и поведение при ошибке publisher
- `handler` — HTTP transport через `httptest` и handwritten `mockService`
- `grpcserver` — успешные RPC и mapping ошибок в `InvalidArgument`, `NotFound`, `Internal`

## Структура проекта

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

## Следующие шаги

- Kafka consumer / отдельный analytics service
- Постоянное хранение статистики переходов и analytics API
- Более сильные гарантии доставки событий, например через outbox pattern
- Rate limiting
- Кастомные aliases
- Expiration ссылок
