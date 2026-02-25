package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sentixxx/Zflow/backend/internal/config"
	"github.com/Sentixxx/Zflow/backend/internal/handler"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/scheduler"
	"github.com/Sentixxx/Zflow/backend/internal/service"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

func main() {
	cfg := config.Load()
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	feedStore, err := repository.NewSQLiteFeedRepository(cfg.DBPath)
	if err != nil {
		l := logger.NewModuleFromEnv("http")
		l.Error("create", "settings", "failed", "failed to init feed store", "error", err.Error())
		os.Exit(1)
	}
	defer feedStore.Close()

	feedService := service.NewFeedService(feedStore)
	srv := handler.NewServer(feedService, cfg.DataDir)
	refreshScheduler := scheduler.NewFeedRefreshScheduler(srv, cfg.RefreshInterval)
	go refreshScheduler.Start(rootCtx)

	l := logger.NewModuleFromEnv("http")
	l.Info("request", "http", "ok", "server started", "addr", cfg.Addr, "data_dir", cfg.DataDir, "refresh_interval", cfg.RefreshInterval.String())
	if err := http.ListenAndServe(cfg.Addr, srv.Handler()); err != nil {
		l.Error("request", "http", "failed", "server stopped", "error", err.Error())
		os.Exit(1)
	}
}
