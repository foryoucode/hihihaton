package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"north-code/db"
	"north-code/internal/planner"
	"north-code/internal/repository"
	"north-code/internal/service"
)

func setup(t *testing.T, neuralURL string) *gin.Engine {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "plans.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..", "..")
	s := service.NewService(repository.NewRepository(conn), filepath.Join(root, "data"), filepath.Join(root, "engine", "models"), neuralURL)
	return NewHandler(s).InitRoutes(t.TempDir())
}

func request(t *testing.T, router *gin.Engine, method, path string, body any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	var result map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(res.Code, res.Body.String(), err)
	}
	return res.Code, result
}

func TestExactAndCRUDWorkWithoutPython(t *testing.T) {
	router := setup(t, "http://127.0.0.1:1")
	status, result := request(t, router, "POST", "/api/calculate", map[string]any{"method": "exact"})
	if status != 200 || result["cost"] != float64(2482000) {
		t.Fatal(status, result)
	}
	status, result = request(t, router, "POST", "/api/plans", map[string]any{"name": "План Go", "method": "exact"})
	if status != 201 {
		t.Fatal(status, result)
	}
	id := int64(result["id"].(float64))
	path := fmtID(id)
	status, result = request(t, router, "GET", path, nil)
	if status != 200 || result["name"] != "План Go" {
		t.Fatal(status, result)
	}
	status, result = request(t, router, "PATCH", path, map[string]any{"name": "Новый план"})
	if status != 200 || result["name"] != "Новый план" {
		t.Fatal(status, result)
	}
	status, result = request(t, router, "GET", "/api/plans", nil)
	if status != 200 || len(result["plans"].([]any)) != 1 {
		t.Fatal(status, result)
	}
	if status, _ := request(t, router, "DELETE", path, nil); status != 200 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "GET", path, nil); status != 404 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "DELETE", path, nil); status != 404 {
		t.Fatal(status)
	}
}

func fmtID(id int64) string { return "/api/plans/" + strconv.FormatInt(id, 10) }

func TestInputAndMissingNeuralService(t *testing.T) {
	router := setup(t, "http://127.0.0.1:1")
	for _, body := range []map[string]any{{"method": "wrong"}, {"budget": -1}, {"deadline": 0}, {"budget": 2.5}, {"disabled_suppliers": []string{"unknown"}}} {
		if status, _ := request(t, router, "POST", "/api/calculate", body); status != 400 {
			t.Fatal(body, status)
		}
	}
	if status, _ := request(t, router, "POST", "/api/calculate", map[string]any{"method": "neural"}); status != 502 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "GET", "/api/plans/abc", nil); status != 400 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "POST", "/api/plans", map[string]any{"name": "  "}); status != 400 {
		t.Fatal(status)
	}
}

func TestNeuralResponseIsValidatedByGo(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := planner.Load(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := planner.Prepare(data, 14)
	variants, err := planner.Exact(context.Background(), data, prepared)
	if err != nil {
		t.Fatal(err)
	}
	broken := false
	neural := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/recommend" {
			t.Error("неверный маршрут нейросети")
		}
		var payload planner.Prepared
		if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.Candidates) == 0 {
			t.Error("не переданы признаки")
		}
		reply := variants[3]
		reply.Plan = append([]planner.Assignment{}, reply.Plan...)
		if broken {
			reply.Plan[0].Price = 1
		}
		json.NewEncoder(w).Encode(map[string]any{"variants": []planner.Variant{reply}})
	}))
	defer neural.Close()
	router := setup(t, neural.URL)
	status, result := request(t, router, "POST", "/api/calculate", map[string]any{"method": "neural"})
	if status != 200 || result["optimal"] != false || result["validated"] != true {
		t.Fatal(status, result)
	}
	broken = true
	if status, _ := request(t, router, "POST", "/api/plans", map[string]any{"method": "neural", "name": "Неверный план"}); status != 502 {
		t.Fatal(status)
	}
	_, result = request(t, router, "GET", "/api/plans", nil)
	if len(result["plans"].([]any)) != 0 {
		t.Fatal("неверный план записан в базу")
	}
}
