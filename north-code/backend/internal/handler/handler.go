package handler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"north-code/internal/service"
	"north-code/validate"
)

type Handler struct {
	service *service.Service
}

func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) InitRoutes(staticDir string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
		c.Next()
	})
	{
		api.GET("/health", h.Health)
		api.GET("/data", h.Data)
		api.GET("/experiments", h.Experiments)
		api.POST("/search", h.Search)
		api.POST("/calculate", h.Calculate)
		api.GET("/plans", h.ListPlans)
		api.POST("/plans", h.CreatePlan)
		api.GET("/plans/:id", h.GetPlan)
		api.PATCH("/plans/:id", h.RenamePlan)
		api.DELETE("/plans/:id", h.DeletePlan)
	}

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"message": "Страница не найдена"})
			return
		}

		root, err := filepath.Abs(staticDir)
		if err != nil {
			h.sendError(c, err)
			return
		}
		path, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(c.Request.URL.Path)))
		if err == nil && strings.HasPrefix(path, root+string(os.PathSeparator)) {
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				c.File(path)
				return
			}
		}

		c.File(filepath.Join(root, "index.html"))
	})
	return router
}

func (h *Handler) sendError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"message": "План не найден"})
	case errors.Is(err, service.ErrNeuralUnavailable):
		c.JSON(http.StatusBadGateway, gin.H{"message": "Сервис нейросети недоступен. Точный расчёт продолжает работать."})
	case errors.Is(err, service.ErrInvalidRecommendation):
		c.JSON(http.StatusBadGateway, gin.H{"message": "Ответ нейросети не прошёл проверку ограничений. План не сохранён."})
	case errors.Is(err, context.DeadlineExceeded):
		c.JSON(http.StatusGatewayTimeout, gin.H{"message": "Расчёт не завершён в установленное время. Оптимальность не подтверждена."})
	default:
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка сервера. Проверьте журнал приложения."})
	}
}

func (h *Handler) Health(c *gin.Context) {
	result, err := h.service.Health(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Data(c *gin.Context) {
	result, err := h.service.Data()
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Experiments(c *gin.Context) {
	result, err := h.service.Model()
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": result})
}

func (h *Handler) Search(c *gin.Context) {
	var req validate.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, service.ErrBadRequest)
		return
	}

	result, err := h.service.Search(req)
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipment": result})
}

func (h *Handler) Calculate(c *gin.Context) {
	var req validate.PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, service.ErrBadRequest)
		return
	}

	result, err := h.service.Calculate(c.Request.Context(), req)
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ListPlans(c *gin.Context) {
	result, err := h.service.List(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": result})
}

func (h *Handler) CreatePlan(c *gin.Context) {
	var req validate.PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, service.ErrBadRequest)
		return
	}

	result, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func getID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, service.ErrBadRequest
	}
	return id, nil
}

func (h *Handler) GetPlan(c *gin.Context) {
	id, err := getID(c)
	if err != nil {
		h.sendError(c, err)
		return
	}

	result, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) RenamePlan(c *gin.Context) {
	id, err := getID(c)
	if err != nil {
		h.sendError(c, err)
		return
	}
	var req validate.RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, service.ErrBadRequest)
		return
	}

	result, err := h.service.Rename(c.Request.Context(), id, req.Name)
	if err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) DeletePlan(c *gin.Context) {
	id, err := getID(c)
	if err != nil {
		h.sendError(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
