package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	db "north-code/internal/repository/sqlc"
)

type Plan struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Result    json.RawMessage `json:"result"`
	CreatedAt string          `json:"created_at"`
}

type Repository struct {
	db      *sql.DB
	queries *db.Queries
}

func NewRepository(conn *sql.DB) *Repository {
	return &Repository{db: conn, queries: db.New(conn)}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func plan(row db.Plan) Plan {
	return Plan{ID: row.ID, Name: row.Name, Result: json.RawMessage(row.Result), CreatedAt: row.CreatedAt}
}

func (r *Repository) Create(ctx context.Context, name string, result json.RawMessage) (Plan, error) {
	row, err := r.queries.CreatePlan(ctx, db.CreatePlanParams{Name: name, Result: string(result)})
	return plan(row), err
}

func (r *Repository) Get(ctx context.Context, id int64) (Plan, error) {
	row, err := r.queries.GetPlan(ctx, id)
	return plan(row), err
}

func (r *Repository) List(ctx context.Context) ([]Plan, error) {
	rows, err := r.queries.ListPlans(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]Plan, 0, len(rows))
	for _, row := range rows {
		items = append(items, plan(row))
	}

	return items, nil
}

func (r *Repository) Rename(ctx context.Context, id int64, name string) (Plan, error) {
	row, err := r.queries.RenamePlan(ctx, db.RenamePlanParams{ID: id, Name: name})
	return plan(row), err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	count, err := r.queries.DeletePlan(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}
