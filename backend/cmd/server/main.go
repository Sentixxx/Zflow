package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/api"
	"github.com/Sentixxx/Zflow/backend/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	feedStore, err := store.NewFeedStore(filepath.Join(dataDir, "zflow.db"))
	if err != nil {
		log.Fatalf("failed to init feed store: %v", err)
	}
	defer feedStore.Close()

	srv := api.NewServer(feedStore)
	go srv.StartRefreshLoop(context.Background(), 15*time.Minute)

	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
