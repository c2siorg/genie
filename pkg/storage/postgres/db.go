// Package postgres provides a pgxpool-backed implementation of Genie's
// storage interfaces (users, accounts, documents, eval records).
//
// Migrations are embedded SQL run on Open(); they are idempotent so multiple
// processes calling Open() concurrently are safe.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config is the pgxpool wiring Genie expects from env or flags.
type Config struct {
	DSN             string
	MaxConns        int32
	HealthCheckTime time.Duration
}

// DB is the package's facade over pgxpool.
type DB struct {
	Pool *pgxpool.Pool
}

// Open connects to Postgres and runs embedded migrations.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
	}
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.HealthCheckTime > 0 {
		pcfg.HealthCheckPeriod = cfg.HealthCheckTime
	}
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	db := &DB{Pool: pool}
	if err := db.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

// Close releases pool resources.
func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

// Migrate runs all embedded migrations in lexicographic order.
func (db *DB) Migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !hasSQLSuffix(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		sql, err := fs.ReadFile(migrationsFS, "migrations/"+n)
		if err != nil {
			return err
		}
		if _, err := db.Pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", n, err)
		}
	}
	return nil
}

func hasSQLSuffix(s string) bool {
	const suffix = ".sql"
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}
