package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"north-code/internal/service"
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
	{
		api.GET("/health", h.Request)
		api.GET("/data", h.Request)
		api.GET("/experiments", h.Request)
		api.POST("/search", h.Request)
		api.POST("/calculate", h.Request)
		api.GET("/plans", h.Request)
		api.POST("/plans", h.Request)
		api.GET("/plans/:id", h.Request)
		api.PATCH("/plans/:id", h.Request)
		api.DELETE("/plans/:id", h.Request)
	}
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"message": "Страница не найдена"})
			return
		}
		path := filepath.Join(staticDir, filepath.FromSlash(c.Request.URL.Path))
		root, err := filepath.Abs(staticDir)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		path, err = filepath.Abs(path)
		if err != nil || !strings.HasPrefix(path, root+string(os.PathSeparator)) {
			c.File(filepath.Join(root, "index.html"))
			return
		}
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			c.File(path)
			return
		}
		c.File(filepath.Join(root, "index.html"))
	})
	return router
}

func (h *Handler) Request(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Не удалось прочитать запрос"})
		return
	}
	path := strings.TrimPrefix(c.Request.URL.Path, "/api")
	data, status, err := h.service.Request(c.Request.Context(), c.Request.Method, path, body)
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Проверьте параметры и метод расчёта"})
			return
		}
		slog.Error(err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"message": "Расчётный сервис недоступен"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}
