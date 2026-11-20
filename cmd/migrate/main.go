package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"plata/migrations"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := migrate(ctx)
	cancel()

	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func migrate(ctx context.Context) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}

	conn := stdlib.OpenDB(*cfg)
	defer conn.Close()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("configure migration lock: %w", err)
	}

	migrator, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.Files, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("configure migrations: %w", err)
	}

	results, err := migrator.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	log.Printf("applied %d migrations", len(results))

	return nil
}
