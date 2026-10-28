package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"north-code/db"
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
		log.Fatal(err)
	}
	defer conn.Close()

	provider, err := db.Provider(conn)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := "status"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "up":
		_, err = provider.Up(ctx)
	case "down":
		_, err = provider.Down(ctx)
	case "status":
		rows, statusErr := provider.Status(ctx)
		if statusErr != nil {
			log.Fatal(statusErr)
		}
		raw, jsonErr := json.MarshalIndent(rows, "", "  ")
		if jsonErr != nil {
			log.Fatal(jsonErr)
		}
		fmt.Println(string(raw))
	default:
		log.Fatal("Команды: up, down, status")
	}
	if err != nil {
		log.Fatal(err)
	}
}
