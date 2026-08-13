package internal

import (
	"context"
	"database/sql"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return pool, nil
}

// RunMigrations accepts an fs.FS so the caller (main.go) can embed the files.
// goose needs *sql.DB, not pgxpool, so we open a separate connection just for this.
func RunMigrations(migrations fs.FS, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(migrations)
	goose.SetDialect("postgres")

	return goose.Up(db, "migrations")
}
