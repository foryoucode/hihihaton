package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"strings"
	"time"
	"unicode/utf8"

	"north-code/internal/planner"
	"north-code/internal/repository"
	"north-code/validate"
)

var (
	ErrSolverTimeout     = errors.New("OR-Tools не подтвердил оптимальность за отведённое время")
	ErrBadRequest        = errors.New("некорректный запрос")
	ErrSolverUnavailable = errors.New("сервис OR-Tools недоступен")
	ErrInvalidResult     = errors.New("решатель вернул некорректный план")
)

type Service struct {
	repo    *repository.Repository
	solver  *SolverClient
	dataDir string
}

func NewService(repo *repository.Repository, dataDir, solverURL string) *Service {
	return &Service{repo: repo, dataDir: dataDir, solver: NewSolverClient(solverURL)}
}

func (s *Service) EnsureData(ctx context.Context) error {
	_, _, err := s.repo.Data(ctx, nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	data, err := planner.Load(s.dataDir)
	if err != nil {
		return err
	}
	_, err = s.Import(ctx, "Учебная станция", data)
	return err
}

func (s *Service) Data(ctx context.Context, id *int64) (map[string]any, error) {
	data, dataset, err := s.repo.Data(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"equipment": data.Equipment, "offers": data.Offers, "suppliers": data.Suppliers, "station": data.Station, "dataset": dataset}, nil
}

func (s *Service) Import(ctx context.Context, title string, data *planner.Data) (map[string]any, error) {
	if title == "" {
		title = "Импорт CSV"
	}
	title, err := name(title)
	if err != nil {
		return nil, err
	}
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if _, err := planner.Prepare(data, planner.Integer(data.Station[0], "deadline_days")); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	dataset, err := s.repo.Import(ctx, title, data)
	if err != nil {
		return nil, err
	}
	return map[string]any{"dataset": dataset, "counts": map[string]int{"equipment": len(data.Equipment), "suppliers": len(data.Suppliers), "offers": len(data.Offers), "requirements": len(data.Station)}}, nil
}

func (s *Service) Health(ctx context.Context) (map[string]any, error) {
	if err := s.repo.Ping(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok", "ortools_ready": s.solver.Health(ctx), "storage": "postgresql", "exact_backend": "go"}, nil
}

func (s *Service) Search(ctx context.Context, req validate.SearchRequest) ([]planner.Row, error) {
	temperature := -30.0
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	if req.EquipmentType == "" {
		req.EquipmentType = "Генератор"
	}
	if math.IsNaN(req.Capacity) || math.IsInf(req.Capacity, 0) || math.IsNaN(temperature) || math.IsInf(temperature, 0) || req.Capacity < 0 {
		return nil, ErrBadRequest
	}

	data, _, err := s.repo.Data(ctx, req.DatasetID)
	if err != nil {
		return nil, err
	}
	rows := data.Equipment

	items := []planner.Row{}
	for _, row := range rows {
		if row["equipment_type"] == req.EquipmentType && planner.Number(row, "capacity") >= req.Capacity && planner.Number(row, "min_operating_temperature_c") <= temperature {
			items = append(items, row)
		}
	}
	return items, nil
}

func (s *Service) Calculate(ctx context.Context, req validate.PlanRequest) (map[string]any, error) {
	data, dataset, err := s.repo.Data(ctx, req.DatasetID)
	if err != nil {
		return nil, err
	}
	budget, deadline := planner.Integer(data.Station[0], "recommended_budget_rub"), planner.Integer(data.Station[0], "deadline_days")
	if req.Budget != nil {
		budget = *req.Budget
	}
	if req.Deadline != nil {
		deadline = *req.Deadline
	}
	if req.Method == "" {
		req.Method = "exact"
	}
	if budget < 0 || budget > 100000000 || deadline < 1 || deadline > 365 || (req.Method != "exact" && req.Method != "ortools") {
		return nil, ErrBadRequest
	}

	known := map[string]bool{}
	for _, row := range data.Suppliers {
		known[row["supplier_id"]] = true
	}
	disabled := map[string]bool{}
	for _, id := range req.DisabledSuppliers {
		if !known[id] {
			return nil, fmt.Errorf("%w: неизвестный поставщик", ErrBadRequest)
		}
		disabled[id] = true
	}
	offers := []planner.Row{}
	for _, row := range data.Offers {
		if !disabled[row["supplier_id"]] {
			offers = append(offers, row)
		}
	}
	data.Offers = offers

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	prepared, err := planner.Prepare(data, deadline)
	if err != nil {
		return nil, err
	}
	var variants []planner.Variant
	if req.Method == "exact" {
		variants, err = planner.Exact(ctx, data, prepared)
	} else {
		variants, err = s.solver.Solve(ctx, prepared)
	}
	if err != nil {
		return nil, err
	}
	result, err := planner.Result(data, prepared, variants, budget, req.Method)
	if err != nil {
		if req.Method == "ortools" {
			return nil, fmt.Errorf("%w: %v", ErrInvalidResult, err)
		}
		return nil, err
	}
	if req.DisabledSuppliers == nil {
		req.DisabledSuppliers = []string{}
	}
	result["seconds"] = math.Round(time.Since(start).Seconds()*1000) / 1000
	result["disabled_suppliers"] = req.DisabledSuppliers
	result["dataset_id"] = dataset.ID
	result["dataset_name"] = dataset.Name

	return result, nil
}

func name(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 100 {
		return "", fmt.Errorf("%w: название должно содержать от 1 до 100 символов", ErrBadRequest)
	}
	return value, nil
}

func (s *Service) Create(ctx context.Context, req validate.PlanRequest) (repository.Plan, error) {
	title, err := name(req.Name)
	if err != nil {
		return repository.Plan{}, err
	}
	result, err := s.Calculate(ctx, req)
	if err != nil {
		return repository.Plan{}, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return repository.Plan{}, err
	}
	return s.repo.Create(ctx, result["dataset_id"].(int64), title, raw)
}

func (s *Service) Get(ctx context.Context, id int64) (repository.Plan, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]repository.Plan, error) {
	return s.repo.List(ctx)
}

func (s *Service) Rename(ctx context.Context, id int64, value string) (repository.Plan, error) {
	title, err := name(value)
	if err != nil {
		return repository.Plan{}, err
	}
	return s.repo.Rename(ctx, id, title)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
