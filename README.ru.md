# URL Shortener

[English version](README.md)

Сервис сокращения ссылок на Go с HTTP- и gRPC-API, постоянным хранением в PostgreSQL, кэшем Redis и аналитикой переходов через Kafka. Отдельный Analytics Consumer сохраняет события переходов в PostgreSQL и защищает от повторного учёта одной и той же записи Kafka.

## Возможности

- Генерация коротких base62-кодов из ID, полученных из PostgreSQL sequence
- HTTP API для создания ссылок и редиректов `302 Found`
- gRPC API (`CreateLink` и `GetLink`) с общей бизнес-логикой
- Постоянное хранение в PostgreSQL и Redis cache-aside с fallback на БД
- Асинхронная best-effort публикация событий в Kafka без ожидания доставки при редиректе
- Отдельный Analytics Consumer с ручным подтверждением offset после записи в БД
- Защита от дубликатов по Kafka topic, partition и offset
- Docker Compose с автоматическим созданием Kafka topic и применением миграций
- Graceful shutdown HTTP/gRPC, попытка отправить накопленные Kafka-события перед остановкой и корректное закрытие Analytics Consumer
- Юнит-тесты бизнес-логики, HTTP- и gRPC-слоёв

## Архитектура

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

Проект использует слоистую архитектуру и внедрение зависимостей через конструкторы. HTTP и gRPC вызывают одну реализацию сервиса, которая зависит от интерфейсов репозитория и издателя событий. Точки входа — `cmd/shortener/main.go` и `cmd/analytics/main.go`.

### Создание и получение ссылок

Сервис получает ID из PostgreSQL sequence и кодирует его в base62. Повторное сокращение одного URL создаёт отдельный код.

При чтении Cached Repository сначала проверяет Redis. При cache miss или ошибке Redis используется PostgreSQL; полученное из БД значение сохраняется в Redis с TTL. Источник истины — PostgreSQL. HTTP использует `302`, а не постоянно кэшируемый `301`, чтобы последующие запросы продолжали поступать в сервис.

### Обработка событий переходов

После успешного получения оригинального URL сервис публикует `LinkVisitedEvent` в Kafka topic `link-visited`. Событие содержит короткий код, URL назначения и время:

```json
{
  "code": "4",
  "url": "https://example.com",
  "occurred_at": "2026-09-29T11:34:07Z"
}
```

Producer реализован на `franz-go`: он асинхронно ставит сообщения в очередь, используя короткий код как Kafka record key. Ошибки доставки логируются. Доступность редиректа важнее аналитики; при штатном завершении приложение пытается отправить накопленные сообщения с ограничением по времени.

Analytics работает отдельным процессом в Kafka consumer group (по умолчанию `url-shortener-analytics`). Для каждого сообщения Consumer сначала записывает переход в PostgreSQL и **только после успешной записи** подтверждает offset. В таблице `link_visits` действует уникальное ограничение `(kafka_topic, kafka_partition, kafka_offset)`, а вставка использует `ON CONFLICT DO NOTHING`. Поэтому повторное чтение **той же записи Kafka** не создаёт вторую запись о переходе.

Это не end-to-end exactly-once: публикация событий остаётся best-effort, а уникальное ограничение защищает от повторного чтения конкретного Kafka-сообщения, но не от двух отдельно опубликованных одинаковых событий.

Kafka запускается в KRaft-режиме без ZooKeeper. Одноразовый Compose-сервис `kafka-init` создаёт `link-visited`, если topic отсутствует; отдельный сервис применяет миграции PostgreSQL.

## Стек технологий

| Область | Технология |
| --- | --- |
| Язык / HTTP | Go / `net/http` |
| RPC | gRPC и Protocol Buffers |
| База данных | PostgreSQL, `pgx` / `pgxpool` |
| Кэш | Redis, `redis/go-redis/v9` |
| Обмен событиями | Apache Kafka (KRaft), `franz-go` |
| Миграции | `golang-migrate` |
| Логи / конфигурация | `log/slog`, переменные окружения, `godotenv` для локального запуска |
| Контейнеры / тесты | Docker Compose, Go `testing`, `httptest`, самописные моки |

## Быстрый запуск

**Требования:** Docker и Docker Compose. Для ручной проверки gRPC дополнительно пригодится `grpcurl`.

```bash
docker compose up -d --build
docker compose ps
```

Compose запускает PostgreSQL, Redis, Kafka, одноразовые сервисы создания topic и миграций, основное приложение и Analytics Consumer. Доступные с хоста адреса:

| Сервис | Адрес |
| --- | --- |
| HTTP API | `http://localhost:8080` |
| gRPC API | `localhost:9090` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| Kafka (внешний listener) | `localhost:9092` |

Для запуска через Compose **не нужно** копировать `.env.example`: Compose напрямую задаёт переменные окружения контейнеров. Пароль БД и открытые порты в этой конфигурации предназначены **только для локальной разработки**. Не разворачивайте её в production без изменений.

Остановить контейнеры, сохранив PostgreSQL volume:

```bash
docker compose down
```

### Локальный запуск Go-процессов

Сначала запустите инфраструктуру и миграции, затем настройте подключения к локальным портам:

```bash
docker compose up -d postgres redis kafka kafka-init migrate
cp .env.example .env
```

Измените `.env`: для PostgreSQL из прилагаемого Compose используйте такие значения:

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

В двух отдельных терминалах:

```bash
go run ./cmd/shortener
```

```bash
go run ./cmd/analytics
```

Не запускайте локально сервисы одновременно с их Compose-экземплярами, если они конфликтуют по портам или потребляют события одной consumer group.

## HTTP API

### Создать короткую ссылку

```bash
curl -i -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

Пример ответа (`201 Created`; код может отличаться):

```json
{"short_url":"http://localhost:8080/4"}
```

Неправильный JSON или невалидный URL приводят к `400`, неожиданная ошибка инфраструктуры — к `500`.

### Перейти по ссылке

```bash
curl -i http://localhost:8080/4
```

`GET /{code}` возвращает `302 Found` и заголовок `Location` с оригинальным URL. Для несуществующего кода возвращается `404`. Вместо `4` подставьте код, полученный из собственного запроса `POST /shorten`.

## gRPC API

Контракт описан в [`proto/shortener.proto`](proto/shortener.proto). gRPC-сервер слушает `localhost:9090` и предоставляет два unary RPC:

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

Замените `4` на реально созданный код. Ошибки валидации преобразуются в `InvalidArgument`, неизвестный код — в `NotFound`, другие ошибки — в `Internal`.

## Проверка аналитики

Сначала создайте ссылку и выполните переход. Посмотрите Kafka-события:

```bash
docker compose exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka:19092 \
  --topic link-visited --from-beginning
```

Остановите консольный Consumer через Ctrl+C. Проверьте сохранённые переходы:

```bash
docker compose exec postgres \
  psql -U postgres -d url-shortener \
  -c 'SELECT code, COUNT(*) FROM link_visits GROUP BY code ORDER BY code;'
```

Посмотрите прогресс Kafka Consumer Group:

```bash
docker compose exec kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server kafka:19092 \
  --group url-shortener-analytics --describe
```

Сейчас статистика **хранится только в PostgreSQL**; публичного endpoint `/stats/{code}` нет.

## Тестирование

```bash
go test ./...
```

Имеющиеся юнит-тесты проверяют бизнес-логику и публикацию событий (`internal/service`), HTTP handlers (`internal/handler`) и gRPC-методы / преобразование ошибок (`internal/grpcserver`). Kafka и запись аналитики в PostgreSQL дополнительно проверялись вручную через Compose; автоматических интеграционных тестов для этого сценария пока нет.

## Структура проекта

```text
.
├── cmd/
│   ├── shortener/main.go         # HTTP + gRPC-приложение
│   └── analytics/main.go         # Отдельный Kafka Consumer
├── internal/
│   ├── analytics/               # Сохранение переходов
│   ├── domain/                  # Доменные типы и ошибки
│   ├── event/                   # Модели событий и интерфейс Publisher
│   ├── grpcserver/              # gRPC transport
│   ├── handler/                 # HTTP transport
│   ├── kafka/                   # Kafka Producer и Consumer
│   ├── repository/              # PostgreSQL + Redis cache-aside
│   └── service/                 # Бизнес-логика / base62
├── migrations/                 # Sequence, links и link_visits
├── proto/                      # Protobuf и сгенерированный Go-код
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── README.md
└── README.ru.md
```

## Границы проекта

Это учебный backend-проект для портфолио, а не готовый публичный сервис. Текущая реализация отдаёт приоритет редиректам, а не гарантированной доставке событий. Здесь пока нет аккаунтов, авторизации, rate limiting, пользовательских alias, срока действия ссылок и публичного API статистики.
