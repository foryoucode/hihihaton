package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"north-code/internal/planner"
	"north-code/internal/repository"
	"north-code/validate"
)

var (
	ErrBadRequest            = errors.New("некорректный запрос")
	ErrNeuralUnavailable     = errors.New("сервис нейросети недоступен")
	ErrInvalidRecommendation = errors.New("нейросеть вернула некорректный план")
)

type Service struct {
	repo     *repository.Repository
	neural   *NeuralClient
	dataDir  string
	modelDir string
}

func NewService(repo *repository.Repository, dataDir, modelDir, neuralURL string) *Service {
	return &Service{repo: repo, dataDir: dataDir, modelDir: modelDir, neural: NewNeuralClient(neuralURL)}
}

func (s *Service) Model() (json.RawMessage, error) {
	raw, err := os.ReadFile(filepath.Join(s.modelDir, "metrics.json"))
	if errors.Is(err, os.ErrNotExist) {
		return json.RawMessage("null"), nil
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("некорректный отчёт экспериментов")
	}

	return json.RawMessage(raw), nil
}

func (s *Service) Data() (map[string]any, error) {
	data, err := planner.Load(s.dataDir)
	if err != nil {
		return nil, err
	}
	model, err := s.Model()
	if err != nil {
		return nil, err
	}

	return map[string]any{"equipment": data.Equipment, "offers": data.Offers, "suppliers": data.Suppliers,
		"station": data.Station, "model": model}, nil
}

func (s *Service) Health(ctx context.Context) (map[string]any, error) {
	if err := s.repo.Ping(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok", "model_ready": s.neural.Health(ctx), "storage": "sqlite", "exact_backend": "go"}, nil
}

func (s *Service) Search(req validate.SearchRequest) ([]planner.Row, error) {
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

	rows, err := planner.ReadCSV(filepath.Join(s.dataDir, "equipment.csv"))
	if err != nil {
		return nil, err
	}
	if err := planner.ValidateEquipment(rows); err != nil {
		return nil, err
	}

	items := []planner.Row{}
	for _, row := range rows {
		if row["equipment_type"] == req.EquipmentType && planner.Number(row, "capacity") >= req.Capacity && planner.Number(row, "min_operating_temperature_c") <= temperature {
			items = append(items, row)
		}
	}
	return items, nil
}

func (s *Service) Calculate(ctx context.Context, req validate.PlanRequest) (map[string]any, error) {
	data, err := planner.Load(s.dataDir)
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
	if budget < 0 || budget > 100000000 || deadline < 1 || deadline > 365 || (req.Method != "exact" && req.Method != "neural") {
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
		variants, err = s.neural.Recommend(ctx, prepared)
	}
	if err != nil {
		return nil, err
	}
	result, err := planner.Result(data, prepared, variants, budget, req.Method)
	if err != nil {
		if req.Method == "neural" {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRecommendation, err)
		}
		return nil, err
	}
	if req.DisabledSuppliers == nil {
		req.DisabledSuppliers = []string{}
	}
	result["seconds"] = math.Round(time.Since(start).Seconds()*1000) / 1000
	result["disabled_suppliers"] = req.DisabledSuppliers

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
	return s.repo.Create(ctx, title, raw)
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
