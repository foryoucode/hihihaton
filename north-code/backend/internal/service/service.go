package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"north-code/internal/repository"
	"north-code/validate"
)

var ErrBadRequest = errors.New("некорректный запрос")

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Request(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	if method == http.MethodPost && (path == "/calculate" || path == "/plans") {
		var req validate.PlanRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, 0, ErrBadRequest
		}

		if req.Method != "" && req.Method != "exact" && req.Method != "neural" {
			return nil, 0, ErrBadRequest
		}
	}

	return s.repo.Request(ctx, method, path, body)
}
