package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"north-code/db"
	"north-code/internal/planner"
	"north-code/internal/repository"
	"north-code/internal/testdb"
)

func TestImportRoundTripAndHistoryVersion(t *testing.T) {
	conn := testdb.Open(t)
	repo := repository.NewRepository(conn)
	ctx := context.Background()
	source, err := planner.Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.Import(ctx, "Первый набор", source)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range source.Station {
		row["recommended_budget_rub"] = "3000000"
	}
	second, err := repo.Import(ctx, "Второй набор", source)
	if err != nil {
		t.Fatal(err)
	}
	data, current, err := repo.Data(ctx, nil)
	if err != nil || current.ID != second.ID || data.Station[0]["recommended_budget_rub"] != "3000000" {
		t.Fatal(current, err)
	}
	old, _, err := repo.Data(ctx, &first.ID)
	if err != nil || old.Station[0]["recommended_budget_rub"] != "2600000" || len(old.Offers) != 320 {
		t.Fatal(old, err)
	}
	saved, err := repo.Create(ctx, first.ID, "Старый план", json.RawMessage(`{"cost":2482000}`))
	if err != nil || saved.DatasetID != first.ID {
		t.Fatal(saved, err)
	}
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	read, err := repo.Get(ctx, saved.ID)
	if err != nil || read.DatasetID != first.ID || read.Name != "Старый план" {
		t.Fatal(read, err)
	}
}

func TestFailedImportRollsBackAllTables(t *testing.T) {
	conn := testdb.Open(t)
	repo := repository.NewRepository(conn)
	ctx := context.Background()
	data, err := planner.Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.Import(ctx, "Рабочий набор", data)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`CREATE FUNCTION fail_offers() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$;
        CREATE TRIGGER reject_offers BEFORE INSERT ON offers FOR EACH ROW EXECUTE FUNCTION fail_offers()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Import(ctx, "Не должен сохраниться", data); err == nil {
		t.Fatal("ошибка импорта проигнорирована")
	}
	_, active, err := repo.Data(ctx, nil)
	if err != nil || active.ID != first.ID {
		t.Fatal("активный набор повреждён", active, err)
	}
	for _, table := range []string{"datasets", "equipment", "suppliers", "objects", "requirements"} {
		var count int
		if err := conn.QueryRow("SELECT count(DISTINCT " + map[string]string{"datasets": "id", "equipment": "dataset_id", "suppliers": "dataset_id", "objects": "dataset_id", "requirements": "dataset_id"}[table] + ") FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatal("остались части откатившегося импорта", table, count, err)
		}
	}
}

func TestGooseDownAndUp(t *testing.T) {
	conn := testdb.Open(t)
	provider, err := db.Provider(conn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.NewRepository(conn).Data(ctx, nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
}
