package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"north-code/internal/planner"
	"north-code/internal/repository"
	"north-code/internal/service"
	"north-code/internal/testdb"
)

func setup(t *testing.T, solverURL string) *gin.Engine {
	t.Helper()
	conn := testdb.Open(t)
	root := filepath.Join("..", "..", "..")
	s := service.NewService(repository.NewRepository(conn), filepath.Join(root, "data"), solverURL)
	if err := s.EnsureData(context.Background()); err != nil {
		t.Fatal(err)
	}
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

func TestExactAndCRUDWorkWithoutORTools(t *testing.T) {
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

func TestInputAndMissingORToolsService(t *testing.T) {
	router := setup(t, "http://127.0.0.1:1")
	for _, body := range []map[string]any{{"method": "wrong"}, {"budget": -1}, {"deadline": 0}, {"budget": 2.5}, {"disabled_suppliers": []string{"unknown"}}} {
		if status, _ := request(t, router, "POST", "/api/calculate", body); status != 400 {
			t.Fatal(body, status)
		}
	}
	if status, _ := request(t, router, "POST", "/api/calculate", map[string]any{"method": "ortools"}); status != 502 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "GET", "/api/plans/abc", nil); status != 400 {
		t.Fatal(status)
	}
	if status, _ := request(t, router, "POST", "/api/plans", map[string]any{"name": "  "}); status != 400 {
		t.Fatal(status)
	}
}

func TestORToolsResponseIsValidatedByGo(t *testing.T) {
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
	solver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/solve" {
			t.Error("неверный маршрут решателя")
		}
		var payload planner.Prepared
		if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.Candidates) == 0 {
			t.Error("не переданы признаки")
		}
		reply := append([]planner.Variant{}, variants...)
		reply[3].Plan = append([]planner.Assignment{}, reply[3].Plan...)
		if broken {
			reply[3].Plan[0].Price = 1
		}
		json.NewEncoder(w).Encode(map[string]any{"variants": reply, "complete": true, "infeasible_counts": []int{}})
	}))
	defer solver.Close()
	router := setup(t, solver.URL)
	status, result := request(t, router, "POST", "/api/calculate", map[string]any{"method": "ortools"})
	if status != 200 || result["optimal"] != true || result["validated"] != true {
		t.Fatal(status, result)
	}
	broken = true
	if status, _ := request(t, router, "POST", "/api/plans", map[string]any{"method": "ortools", "name": "Неверный план"}); status != 502 {
		t.Fatal(status)
	}
	_, result = request(t, router, "GET", "/api/plans", nil)
	if len(result["plans"].([]any)) != 0 {
		t.Fatal("неверный план записан в базу")
	}
}

func TestCSVImportPreservesHistoryAndRejectsBrokenFiles(t *testing.T) {
	router := setup(t, "http://127.0.0.1:1")
	_, before := request(t, router, "GET", "/api/data", nil)
	originalID := before["dataset"].(map[string]any)["id"]
	status, saved := request(t, router, "POST", "/api/plans", map[string]any{"name": "План до импорта"})
	if status != 201 {
		t.Fatal(status, saved)
	}

	upload := func(broken bool) (int, map[string]any) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		writer.WriteField("name", "Новый CSV-набор")
		for field, file := range map[string]string{"equipment": "equipment.csv", "offers": "offers.csv", "suppliers": "suppliers.csv", "station": "station_1.csv"} {
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "data", file))
			if err != nil {
				t.Fatal(err)
			}
			if broken && field == "offers" {
				raw = bytes.Replace(raw, []byte(";GEN001;"), []byte(";UNKNOWN;"), 1)
			}
			part, err := writer.CreateFormFile(field, file)
			if err != nil {
				t.Fatal(err)
			}
			part.Write(raw)
		}
		writer.Close()
		req := httptest.NewRequest("POST", "/api/import", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		var result map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
			t.Fatal(err, res.Body.String())
		}
		return res.Code, result
	}
	if status, result := upload(true); status != 400 {
		t.Fatal(status, result)
	}
	_, after := request(t, router, "GET", "/api/data", nil)
	if after["dataset"].(map[string]any)["id"] != originalID {
		t.Fatal("ошибка импорта изменила активный набор")
	}
	status, imported := upload(false)
	if status != 201 {
		t.Fatal(status, imported)
	}
	newID := imported["dataset"].(map[string]any)["id"]
	if newID == originalID {
		t.Fatal("не создана новая версия данных")
	}
	_, result := request(t, router, "POST", "/api/calculate", map[string]any{"dataset_id": originalID, "method": "exact"})
	if result["cost"] != float64(2482000) || result["dataset_id"] != originalID {
		t.Fatal("старый снимок повреждён", result)
	}
	_, read := request(t, router, "GET", fmtID(int64(saved["id"].(float64))), nil)
	if read["dataset_id"] != originalID {
		t.Fatal("история перепривязана к новым данным")
	}
}
