# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /shortener ./cmd/shortener
RUN CGO_ENABLED=0 GOOS=linux go build -o /analytics ./cmd/analytics

# Kafka analytics
FROM alpine:3.20 AS analytics

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /analytics .

CMD ["./analytics"]

# Final stage - minimal iso
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /shortener .
COPY migrations ./migrations

EXPOSE 8080 9090

CMD ["./shortener"]
