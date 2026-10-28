package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"north-code/db"
)

func main() {
	conn, err := db.Open(os.Getenv("DATABASE_URL"))
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
