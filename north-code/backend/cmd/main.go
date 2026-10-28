package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"north-code/internal/handler"
	"north-code/internal/repository"
	"north-code/internal/service"
)

func main() {
	engineURL := os.Getenv("ENGINE_URL")
	if engineURL == "" {
		engineURL = "http://127.0.0.1:8091"
	}

	address := os.Getenv("APP_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../client/dist"
	}
	repo := repository.NewRepository(engineURL)
	services := service.NewService(repo)
	handlers := handler.NewHandler(services)

	server := &http.Server{
		Addr: address, Handler: handlers.InitRoutes(staticDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second, WriteTimeout: 130 * time.Second,
	}
	slog.Info("Северный код запущен", "address", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
