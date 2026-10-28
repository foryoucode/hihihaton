package main

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"log/slog"
	"os"
	api "solution"
	DB "solution/db"
	"solution/internal/handler"
	"solution/internal/repository"
	"solution/internal/service"
	"time"

	"context"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var (
	redisHost = os.Getenv("REDIS_HOST")
	redisPort = os.Getenv("REDIS_PORT")
	ctx       = context.Background()
)

func main() {
	logger := slog.Default()

	pgURL := os.Getenv("POSTGRES_CONN")
	if pgURL == "" {
		logger.Error("missed POSTGRES_CONN env")
		os.Exit(1)
	}

	db, err := sqlx.Connect("pgx", pgURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
		DB:   0,
	})
	for range 3 {
		_, err = rdb.Ping(ctx).Result()
		if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatalf("Ошибка подключения к Redis:  %v", err)
	}

	defer rdb.Close()

	DB.MigrateDB(db)

	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		logger.Error("missed SERVER_ADDRESS env (export smth like ':8080')")
		os.Exit(1)
	}

	repo := repository.NewRepository(db, rdb)
	services := service.NewService(repo)
	handlers := handler.NewHandler(services)

	server := api.NewServer(serverAddress, logger, handlers.InitRoutes())

	if err := server.Start(); err != nil {
		logger.Error("server has been stopped", "error", err)
	}
}
