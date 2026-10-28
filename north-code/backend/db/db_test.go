package db

import (
	"context"
	"path/filepath"
	"testing"

	"north-code/internal/repository"
)

func TestGooseAdoptsLegacyDatabaseWithoutDataLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plans.db")
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`CREATE TABLE plans(id INTEGER PRIMARY KEY, name TEXT NOT NULL, result TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO plans(id,name,result) VALUES(42,'Старый план','{"cost":2482000}')`); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, conn); err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.NewRepository(conn)
	old, err := repo.Get(ctx, 42)
	if err != nil || old.Name != "Старый план" || string(old.Result) != `{"cost":2482000}` {
		t.Fatal("данные потеряны", old, err)
	}

	created, err := repo.Create(ctx, "Новый план", []byte(`{"method":"exact"}`))
	if err != nil || created.ID <= 42 {
		t.Fatal(created, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rows, err := repository.NewRepository(reopened).List(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatal("история не сохранилась после открытия базы", rows, err)
	}
}

func TestGooseDownAndUp(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "plans.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	provider, err := Provider(conn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, err := repository.NewRepository(conn).List(ctx); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
