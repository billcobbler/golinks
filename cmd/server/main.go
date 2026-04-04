package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/billcobbler/golinks/internal/api"
	"github.com/billcobbler/golinks/internal/config"
	"github.com/billcobbler/golinks/internal/store"
)

func main() {
	// Allow overriding the port via a -port flag for convenience.
	portFlag := flag.String("port", "", "listen port (overrides GOLINKS_PORT)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	if *portFlag != "" {
		cfg.Port = *portFlag
	}

	log := newLogger(cfg.LogLevel)

	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Error("failed to open database", "path", cfg.DBPath, "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	log.Info("database opened", "path", cfg.DBPath)

	// Start the analytics pruning background job if retention is configured.
	if cfg.AnalyticsRetention > 0 {
		go runPruner(db, cfg.AnalyticsRetention, log)
	}

	router := api.NewRouter(db, cfg, log)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}

	log.Info("shutdown complete")
}

// newLogger builds a structured slog.Logger for the given level string.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// runPruner runs the analytics pruning job daily.
func runPruner(s store.Store, retentionDays int, log *slog.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	prune := func() {
		cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
		n, err := s.PruneAnalytics(cutoff)
		if err != nil {
			log.Error("analytics pruning failed", "err", err)
		} else if n > 0 {
			log.Info("analytics pruned", "rows_deleted", n, "cutoff", cutoff.Format(time.DateOnly))
		}
	}

	// Run once immediately on startup, then on the daily tick.
	prune()
	for range ticker.C {
		prune()
	}
}
