// Command typhon serves a Battlesnake whose move comes from an
// iterative-deepening search under the turn budget the engine reports.
//
// This file is wiring only: environment, logger, routes, shutdown. The search,
// the board primitives and the rules live under internal/.
//
// Configuration is read from the environment:
//
//	PORT              listen port (default 8080)
//	LOG_LEVEL         debug, info, warn or error (default info)
//	TYPHON_OPPONENTS  how many rivals the search models properly (default 2)
//	TYPHON_TABLE_BITS transposition table size, as a power of two (default 20)
//	TYPHON_NO_TABLE   set to "true" to search without the table
//	TYPHON_AUTHOR     author shown on the info response
//	TYPHON_COLOR, TYPHON_HEAD, TYPHON_TAIL
//	                  override the look; several of these servers play in the
//	                  same local match, where distinct colours are worth having
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/n0nuser/typhon/internal/api"
	"github.com/n0nuser/typhon/internal/search"
	"github.com/n0nuser/typhon/internal/server"
)

// shutdownGrace is how long in-flight turns get to finish on SIGTERM. A turn is
// bounded by the engine's own timeout, so this only has to outlast one.
const shutdownGrace = 5 * time.Second

// gameTTL drops games that never sent /end, which is what the engine does with
// a match it abandons.
const gameTTL = 30 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := newLogger()
	slog.SetDefault(log)

	// Typhon is the serpent-headed monster that fought Zeus and was buried
	// under Etna, so: a fanged head and a deep volcanic red.
	info := api.InfoResponse{
		APIVersion: "1",
		Author:     env("TYPHON_AUTHOR", "n0nuser"),
		Color:      env("TYPHON_COLOR", "#8A0303"),
		Head:       env("TYPHON_HEAD", "fang"),
		Tail:       env("TYPHON_TAIL", "sharp"),
		Version:    env("TYPHON_VERSION", "0.1.0"),
	}

	cfg := search.DefaultConfig()
	cfg.Opponents = envInt("TYPHON_OPPONENTS", cfg.Opponents)
	cfg.TableBits = uint(envInt("TYPHON_TABLE_BITS", int(cfg.TableBits)))
	cfg.UseTable = !envBool("TYPHON_NO_TABLE", false)

	handler := server.New(info, cfg, gameTTL, log)

	srv := &http.Server{
		Addr:    ":" + env("PORT", "8080"),
		Handler: handler.Routes(),
		// Explicit timeouts: an unbounded server will eventually hang.
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		log.Info("listening",
			"addr", srv.Addr, "opponents", cfg.Opponents,
			"table", cfg.UseTable, "table_bits", cfg.TableBits,
			"color", info.Color, "head", info.Head, "tail", info.Tail)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}
