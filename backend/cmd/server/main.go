package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/config"
	"github.com/Sentixxx/Zflow/backend/internal/db"
	"github.com/Sentixxx/Zflow/backend/internal/handler"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/router"
	"github.com/Sentixxx/Zflow/backend/internal/scheduler"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

func main() {
	cfg := config.Load()
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	l := logger.NewModuleFromEnv("http")

	dbConn, err := db.OpenSQLite(cfg.DBPath)
	if err != nil {
		l.Error("create", "database", "failed", "failed to open sqlite", "path", cfg.DBPath, "error", err.Error())
		os.Exit(1)
	}
	defer dbConn.Close()

	if err := db.RunMigrations(rootCtx, dbConn); err != nil {
		l.Error("create", "database", "failed", "failed to run migrations", "error", err.Error())
		os.Exit(1)
	}

	feedStore := repository.NewSQLiteFeedRepository(dbConn)
	vectorStore := repository.NewSQLiteVectorRepository(dbConn)

	srv := handler.NewServer(feedStore, cfg.DataDir,
		handler.WithVectorRepository(vectorStore),
	)
	refreshScheduler := scheduler.NewFeedRefreshScheduler(srv.FeedRefreshService(), cfg.RefreshInterval)
	go refreshScheduler.Start(rootCtx)
	scoreRefreshScheduler := scheduler.NewArticleScoreRefreshScheduler(srv.ArticleService(), time.Minute, 50)
	go scoreRefreshScheduler.Start(rootCtx)
	if embSvc := srv.EmbeddingService(); embSvc != nil {
		embeddingScheduler := scheduler.NewEmbeddingRefreshScheduler(embSvc, 5*time.Minute, 20)
		go embeddingScheduler.Start(rootCtx)
	}

	httpServer := &http.Server{
		Addr:    cfg.Addr,
		Handler: router.NewHTTPHandler(srv),
	}

	go func() {
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			l := logger.NewModuleFromEnv("http")
			l.Error("request", "http", "failed", "graceful shutdown failed", "error", err.Error())
		}
	}()

	listener, err := listenHTTPListener(cfg.Addr)
	if err != nil {
		l.Error("request", "http", "failed", "failed to bind http listener", "addr", cfg.Addr, "error", err.Error())
		os.Exit(1)
	}

	l.Info("request", "http", "ok", "server started", "addr", cfg.Addr, "data_dir", cfg.DataDir, "db_path", cfg.DBPath, "refresh_interval", cfg.RefreshInterval.String())
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		l.Error("request", "http", "failed", "server stopped", "error", err.Error())
		os.Exit(1)
	}
}

func listenHTTPListener(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return nil, fmt.Errorf("listen tcp %s: address already in use; stop the existing process or set PORT/ZFLOW_ADDR to a free port: %w", addr, err)
	}
	return nil, err
}
