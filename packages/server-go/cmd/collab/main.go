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

	"borgee-server/internal/admin"
	"borgee-server/internal/config"
	"borgee-server/internal/migrations"
	"borgee-server/internal/server"
	"borgee-server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	var handler slog.Handler
	level := cfg.LogLevel()
	if cfg.IsDevelopment() {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)

	s, err := store.OpenWithOptions(cfg.DatabasePath, store.OpenOptions{
		MaxOpenConns: cfg.SQLiteMaxOpenConns,
		TxLock:       cfg.SQLiteTxLock,
	})
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// INFRA-1a: forward-only versioned migrations. Coexists with the legacy
	// Store.Migrate() above during v0; new schema (Phase 1+) lands here as
	// numbered migrations. See internal/migrations and docs/current/server/migrations.md.
	if err := migrations.Default(s.DB()).Run(0); err != nil {
		logger.Error("schema_migrations failed", "error", err)
		os.Exit(1)
	}

	// ADM-0.1: bootstrap the env-configured admin into `admins`. Fail loud
	// if BORGEE_ADMIN_LOGIN / BORGEE_ADMIN_PASSWORD_HASH are missing —
	// admin auth must not silently degrade. Idempotent on re-run.
	// Blueprint: admin-model §1.2 (B env bootstrap, 无 promote).
	if err := admin.Bootstrap(s.DB()); err != nil {
		logger.Error("admin bootstrap failed", "error", err)
		os.Exit(1)
	}

	// TEST-FIX-2: server.New now takes ctx; production wires it to a
	// background context that lives for the process lifetime (cancelled
	// during shutdown via httpServer.Shutdown / explicit srvCancel below).
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()

	srv := server.New(srvCtx, cfg, logger, s)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: srv.Handler(),
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
