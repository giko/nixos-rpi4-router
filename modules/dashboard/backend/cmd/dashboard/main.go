package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/server"
)

func main() {
	cfg, err := config.FromFlags(os.Args[1:])
	if err != nil {
		// No logger yet — cfg hasn't told us what level to use. Write to stderr.
		fmt.Fprintf(os.Stderr, "dashboard: config error: %v\n", err)
		os.Exit(1)
	}

	// Honor the configured log level. slog.Level.UnmarshalText accepts
	// DEBUG / INFO / WARN / ERROR (uppercase); we ToUpper cfg.LogLevel so
	// the user-facing lowercase form ("debug", "info", ...) works. Invalid
	// values emit a warning and fall back to INFO rather than crash — a
	// typo in a config file should not take down the dashboard.
	var level slog.Level
	if uerr := level.UnmarshalText([]byte(strings.ToUpper(cfg.LogLevel))); uerr != nil {
		fmt.Fprintf(os.Stderr, "dashboard: invalid --log-level %q, defaulting to info\n", cfg.LogLevel)
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	httpServer := &http.Server{
		Addr:              cfg.Bind,
		Handler:           server.New(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("dashboard starting", "bind", cfg.Bind)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("dashboard stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "err", err)
			os.Exit(1)
		}
	}
}
