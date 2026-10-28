package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"north-code/internal/planner"
)

type SolverClient struct {
	url    string
	client *http.Client
}

func NewSolverClient(url string) *SolverClient {
	return &SolverClient{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 120 * time.Second}}
}

func (n *SolverClient) Health(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.url+"/health", nil)
	if err != nil {
		return false
	}
	res, err := n.client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	return res.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&result) == nil && result.Status == "ok"
}

func (n *SolverClient) Solve(ctx context.Context, prepared *planner.Prepared) ([]planner.Variant, error) {
	raw, err := json.Marshal(prepared)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url+"/solve", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSolverUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusGatewayTimeout {
		return nil, ErrSolverTimeout
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrSolverUnavailable, res.StatusCode)
	}
	var result struct {
		Variants   []planner.Variant `json:"variants"`
		Complete   bool              `json:"complete"`
		Infeasible []int             `json:"infeasible_counts"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 8*1024*1024)).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResult, err)
	}
	if result.Variants == nil || !result.Complete {
		return nil, ErrInvalidResult
	}

	counts := map[int]bool{}
	for _, v := range result.Variants {
		count := len(v.Selected)
		if counts[count] || count > len(prepared.Optional) {
			return nil, ErrInvalidResult
		}
		counts[count] = true
	}
	for _, count := range result.Infeasible {
		if count < 0 || count > len(prepared.Optional) || counts[count] {
			return nil, ErrInvalidResult
		}
		counts[count] = true
	}
	if len(counts) != len(prepared.Optional)+1 {
		return nil, ErrInvalidResult
	}
	return result.Variants, nil
}
