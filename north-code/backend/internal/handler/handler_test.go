package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"north-code/internal/repository"
	"north-code/internal/service"
)

func TestCalculateForwardsMethod(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/calculate" || !strings.Contains(string(body), `"method":"neural"`) {
			t.Error("неверный запрос расчётному сервису")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"ok","method":"neural"}`)
	}))
	defer engine.Close()
	router := NewHandler(service.NewService(repository.NewRepository(engine.URL))).InitRoutes(t.TempDir())
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/calculate", strings.NewReader(`{"method":"neural"}`))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"method":"neural"`) {
		t.Fatal("нейросетевой режим не передан в сервис", res.Code, res.Body.String())
	}
}

func TestUnknownMethodIsRejected(t *testing.T) {
	router := NewHandler(service.NewService(repository.NewRepository("http://127.0.0.1:1"))).InitRoutes(t.TempDir())
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/calculate", strings.NewReader(`{"method":"other"}`))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatal("неизвестный метод должен быть отклонён", res.Code)
	}
}
