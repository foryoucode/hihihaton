package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"north-code/internal/planner"
	db "north-code/internal/repository/sqlc"
)

type Plan struct {
	ID        int64           `json:"id"`
	DatasetID int64           `json:"dataset_id"`
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
	return Plan{ID: row.ID, DatasetID: row.DatasetID, Name: row.Name, Result: row.Result, CreatedAt: row.CreatedAt.UTC().Format("2006-01-02 15:04:05")}
}

func (r *Repository) Create(ctx context.Context, datasetID int64, name string, result json.RawMessage) (Plan, error) {
	row, err := r.queries.CreatePlan(ctx, db.CreatePlanParams{DatasetID: datasetID, Name: name, Result: result})
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

func (r *Repository) Data(ctx context.Context, id *int64) (*planner.Data, db.Dataset, error) {
	var dataset db.Dataset
	var err error
	if id == nil {
		dataset, err = r.queries.ActiveDataset(ctx)
	} else {
		dataset, err = r.queries.GetDataset(ctx, *id)
	}
	if err != nil {
		return nil, dataset, err
	}
	raw, err := r.queries.ReadDataset(ctx, dataset.ID)
	if err != nil {
		return nil, dataset, err
	}
	var data planner.Data
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, dataset, err
	}
	if err = data.Validate(); err != nil {
		return nil, dataset, err
	}
	return &data, dataset, nil
}

func (r *Repository) Import(ctx context.Context, title string, data *planner.Data) (db.Dataset, error) {
	var empty db.Dataset
	if err := data.Validate(); err != nil {
		return empty, err
	}
	equipment, err := json.Marshal(data.Equipment)
	if err != nil {
		return empty, err
	}
	suppliers, err := json.Marshal(data.Suppliers)
	if err != nil {
		return empty, err
	}
	offers, err := json.Marshal(data.Offers)
	if err != nil {
		return empty, err
	}
	station, err := json.Marshal(data.Station)
	if err != nil {
		return empty, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	q := r.queries.WithTx(tx)
	row := data.Station[0]
	dataset, err := q.CreateDataset(ctx, db.CreateDatasetParams{Name: title, StationID: row["station_id"], StationName: row["station_name"], Budget: int64(planner.Integer(row, "recommended_budget_rub")), Deadline: int32(planner.Integer(row, "deadline_days"))})
	if err != nil {
		return empty, err
	}
	steps := []func() error{
		func() error {
			return q.ImportEquipment(ctx, db.ImportEquipmentParams{DatasetID: dataset.ID, Rows: equipment})
		},
		func() error {
			return q.ImportSuppliers(ctx, db.ImportSuppliersParams{DatasetID: dataset.ID, Rows: suppliers})
		},
		func() error { return q.ImportOffers(ctx, db.ImportOffersParams{DatasetID: dataset.ID, Rows: offers}) },
		func() error {
			return q.ImportObjects(ctx, db.ImportObjectsParams{DatasetID: dataset.ID, Rows: station})
		},
		func() error {
			return q.ImportRequirements(ctx, db.ImportRequirementsParams{DatasetID: dataset.ID, Rows: station})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return empty, err
		}
	}
	if err := tx.Commit(); err != nil {
		return empty, err
	}
	return dataset, nil
}
