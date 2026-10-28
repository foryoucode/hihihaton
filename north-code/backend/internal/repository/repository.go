package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Repository struct {
	url    string
	client *http.Client
}

func NewRepository(url string) *Repository {
	return &Repository{
		url:    strings.TrimRight(url, "/"),
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (r *Repository) Request(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.url+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 8*1024*1024+1))
	if err != nil {
		return nil, 0, err
	}

	if len(data) > 8*1024*1024 {
		return nil, 0, fmt.Errorf("слишком большой ответ расчётного сервиса")
	}

	return data, res.StatusCode, nil
}
