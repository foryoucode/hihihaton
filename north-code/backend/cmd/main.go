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
	if len(os.Args) == 2 && os.Args[1] == "--healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		res, err := client.Get("http://127.0.0.1:8080/api/health")
		if err != nil {
			os.Exit(1)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}

	root := os.Getenv("NORTH_ROOT")
	if root == "" {
		root = ".."
	}
	conn, err := db.Open(os.Getenv("DATABASE_URL"))
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

	solverURL := os.Getenv("SOLVER_URL")
	if solverURL == "" {
		solverURL = "http://127.0.0.1:8091"
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
	services := service.NewService(repo, filepath.Join(root, "data"), solverURL)
	if err := services.EnsureData(context.Background()); err != nil {
		slog.Error("Ошибка начального импорта CSV", "error", err)
		os.Exit(1)
	}
	handlers := handler.NewHandler(services)
	server := &http.Server{
		Addr: address, Handler: handlers.InitRoutes(staticDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second, WriteTimeout: 130 * time.Second,
	}

	slog.Info("Северный код запущен", "address", address, "storage", "postgresql")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
