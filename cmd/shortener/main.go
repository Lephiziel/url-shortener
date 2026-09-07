package main

import (
	"context"
	"fmt"
	"os"
	"url-shortener/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Database connection
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Repository
	repo := repository.NewRepository(pool)

	// Service
	// Service gets repo and then handler gets service
}
