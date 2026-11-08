package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/sentences.db"
	}

	store, err := Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if os.Getenv("SEED") == "1" {
		n, err := store.SeedSamples(context.Background())
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("seeded %d sample sentences", n)
		}
	}

	srv, err := NewServer(store)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("中英句子练习 listening on http://localhost:%s", port)
	log.Printf("database: %s", dbPath)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}
