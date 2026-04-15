package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Evge14n/go-sports-hub/internal/api"
	"github.com/Evge14n/go-sports-hub/internal/config"
	"github.com/Evge14n/go-sports-hub/internal/fetcher"
	"github.com/Evge14n/go-sports-hub/internal/processor"
	"github.com/Evge14n/go-sports-hub/internal/queue"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := storage.NewPostgres(ctx, cfg.DBURL)
	if err != nil {
		log.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("postgres connected")

	redisStore, err := storage.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer redisStore.Close()
	log.Info("redis connected")

	natsClient, err := queue.NewClient(cfg.NatsURL)
	if err != nil {
		log.Error("connect nats", "err", err)
		os.Exit(1)
	}
	defer natsClient.Close()
	log.Info("nats connected")

	proc := processor.New(db, redisStore, natsClient, log)
	go func() {
		if err := proc.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("processor stopped", "err", err)
		}
	}()

	fetch := fetcher.New(natsClient, cfg.APIKey, cfg.DemoMode, log)
	go fetch.Run(ctx)

	wsHub := api.NewWSHub(redisStore, log)
	go wsHub.Run(ctx)

	handlers := api.NewHandlers(db, redisStore, natsClient, log)
	router := api.NewRouter(handlers, wsHub, log, cfg.AllowedOrigins)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server listening", "port", cfg.Port, "demo", cfg.DemoMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info("shutdown signal received")
	case <-ctx.Done():
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown", "err", err)
	}

	log.Info("server stopped")
}
