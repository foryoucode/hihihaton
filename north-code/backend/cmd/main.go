package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"north-code/db"
	"north-code/internal/handler"
	"north-code/internal/repository"
	"north-code/internal/service"
)

func main() {
	root := os.Getenv("NORTH_ROOT")
	if root == "" {
		root = ".."
	}
	path := os.Getenv("NORTH_DB_PATH")
	if path == "" {
		path = filepath.Join(root, "runtime", "plans.db")
	}
	conn, err := db.Open(path)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = db.Migrate(ctx, conn)
	cancel()
	if err != nil {
		slog.Error("Ошибка миграции Goose", "error", err)
		os.Exit(1)
	}

	neuralURL := os.Getenv("NEURAL_URL")
	if neuralURL == "" {
		neuralURL = "http://127.0.0.1:8091"
	}
	address := os.Getenv("APP_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = filepath.Join(root, "client", "dist")
	}

	repo := repository.NewRepository(conn)
	services := service.NewService(repo, filepath.Join(root, "data"), filepath.Join(root, "engine", "models"), neuralURL)
	handlers := handler.NewHandler(services)
	server := &http.Server{
		Addr: address, Handler: handlers.InitRoutes(staticDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second, WriteTimeout: 130 * time.Second,
	}

	slog.Info("Северный код запущен", "address", address, "database", path)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
