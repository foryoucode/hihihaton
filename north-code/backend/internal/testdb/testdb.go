package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"north-code/db"
)

func Open(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" && os.Getenv("PGHOST") == "" {
		t.Skip("Для интеграционных тестов нужен PostgreSQL: TEST_DATABASE_URL или PGHOST")
	}
	admin, err := db.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	dsn := base + " search_path=" + schema
	if strings.HasPrefix(base, "postgres://") || strings.HasPrefix(base, "postgresql://") {
		u, err := url.Parse(base)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	conn, err := db.Open(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return conn
}
