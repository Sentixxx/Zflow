package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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

	feedStore, err := store.NewFeedStore(filepath.Join(dataDir, "feeds.json"))
	if err != nil {
		log.Fatalf("failed to init feed store: %v", err)
	}

	srv := api.NewServer(feedStore)
	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
