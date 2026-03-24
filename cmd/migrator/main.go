package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/Rdegnen/logpipe/internal/config"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		cfg.DBUser, cfg.DBPassword, cfg.Host, cfg.DBPort, cfg.DB)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		log.Fatalf("create schema_migrations: %v", err)
	}

	entries, err := os.ReadDir("infra/migrations")
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		name := entry.Name()

		var exists bool
		err = conn.QueryRow(ctx, `SELECT true FROM schema_migrations WHERE name = $1`, name).Scan(&exists)
		if err != nil && err != pgx.ErrNoRows {
			log.Fatalf("check migration %s: %v", name, err)
		}
		if exists {
			log.Printf("skipping %s", name)
			continue
		}

		sql, err := os.ReadFile(filepath.Join("infra/migrations", name))
		if err != nil {
			log.Fatalf("read migration %s: %v", name, err)
		}

		_, err = conn.Exec(ctx, string(sql))
		if err != nil {
			log.Fatalf("apply migration %s: %v", name, err)
		}

		_, err = conn.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name)
		if err != nil {
			log.Fatalf("record migration %s: %v", name, err)
		}

		log.Printf("applied %s", name)
	}
}
