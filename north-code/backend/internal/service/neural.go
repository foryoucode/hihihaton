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

type NeuralClient struct {
	url    string
	client *http.Client
}

func NewNeuralClient(url string) *NeuralClient {
	return &NeuralClient{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 120 * time.Second}}
}

func (n *NeuralClient) Health(ctx context.Context) bool {
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
		ModelReady bool `json:"model_ready"`
	}
	return res.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&result) == nil && result.ModelReady
}

func (n *NeuralClient) Recommend(ctx context.Context, prepared *planner.Prepared) ([]planner.Variant, error) {
	raw, err := json.Marshal(prepared)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url+"/recommend", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNeuralUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrNeuralUnavailable, res.StatusCode)
	}
	var result struct {
		Variants []planner.Variant `json:"variants"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 8*1024*1024)).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRecommendation, err)
	}
	if result.Variants == nil {
		return nil, ErrInvalidRecommendation
	}

	return result.Variants, nil
}
