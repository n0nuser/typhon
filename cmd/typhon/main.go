// Command typhon serves a Battlesnake whose move comes from an
// iterative-deepening search under the turn budget the engine reports.
//
// This file is wiring only: environment, logger, routes, shutdown. The search,
// the board primitives and the rules live under internal/.
//
// Configuration is read from the environment:
//
//	PORT            listen port (default 8080)
//	LOG_LEVEL       debug, info, warn or error (default info)
//	TYPHON_AUTHOR   author shown on the info response
//	TYPHON_COLOR, TYPHON_HEAD, TYPHON_TAIL
//	                override the look; several of these servers play in the
//	                same local match, where distinct colours are worth having
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// shutdownGrace is how long in-flight turns get to finish on SIGTERM. A
// Battlesnake turn is bounded by the engine's own timeout, so this only has to
// outlast one of them.
const shutdownGrace = 5 * time.Second

// info is the body returned by GET /.
//
// Typhon is the serpent-headed monster that fought Zeus and was buried under
// Etna, so the look is a fanged head and a deep volcanic red.
type info struct {
	APIVersion string `json:"apiversion"`
	Author     string `json:"author,omitempty"`
	Color      string `json:"color,omitempty"`
	Head       string `json:"head,omitempty"`
	Tail       string `json:"tail,omitempty"`
	Version    string `json:"version,omitempty"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := newLogger()
	slog.SetDefault(log)

	self := info{
		APIVersion: "1",
		Author:     env("TYPHON_AUTHOR", "n0nuser"),
		Color:      env("TYPHON_COLOR", "#8A0303"),
		Head:       env("TYPHON_HEAD", "fang"),
		Tail:       env("TYPHON_TAIL", "sharp"),
		Version:    env("TYPHON_VERSION", "0.1.0"),
	}

	srv := &http.Server{
		Addr:    ":" + env("PORT", "8080"),
		Handler: routes(self, log),
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
		log.Info("listening", "addr", srv.Addr, "color", self.Color, "head", self.Head, "tail", self.Tail)
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

// routes serves the Battlesnake webhooks. Only the info webhook is wired here;
// the move loop arrives with internal/server.
func routes(self info, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(self); err != nil {
			// The status line is already written, so this can only be logged.
			log.Error("write info response", "err", err)
		}
	})
	return mux
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
