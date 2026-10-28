package db

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(url string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func Provider(conn *sql.DB) (*goose.Provider, error) {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, conn, files)
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	provider, err := Provider(conn)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
