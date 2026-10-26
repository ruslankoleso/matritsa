// Command api is the credit pipeline backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"matritsa/internal/ai"
	"matritsa/internal/auth"
	"matritsa/internal/config"
	"matritsa/internal/demo"
	"matritsa/internal/events"
	"matritsa/internal/httpapi"
	"matritsa/internal/integrations"
	"matritsa/internal/pipeline"
	"matritsa/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := openWithRetry(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}
	if cfg.SeedOnStart {
		if err := store.Seed(ctx, st); err != nil {
			return err
		}
		slog.Info("demo data ready")
	}

	hub := events.NewHub()
	// Events go through PostgreSQL LISTEN/NOTIFY, so several instances of the API can serve the same users.
	events.Relay(ctx, st.Pool, hub)
	client := integrations.NewClient(cfg.BKIURL, cfg.EGRULURL)
	pipe := pipeline.New(st, client, hub, pipeline.DefaultConfig())
	pipeDone := make(chan struct{})
	go func() {
		defer close(pipeDone)
		pipe.Run(ctx)
	}()
	// Let the worker finish its current step before the database pool is closed.
	defer func() {
		stop()
		select {
		case <-pipeDone:
		case <-time.After(10 * time.Second):
			slog.Warn("pipeline worker did not stop in time")
		}
	}()

	if cfg.SeedOnStart {
		// Filed through the real pipeline in the background: the sources may still be starting.
		go func() {
			if err := demo.Run(ctx, st, pipe); err != nil && ctx.Err() == nil {
				slog.Error("demo applications failed", "err", err)
			}
		}()
	}

	var llm ai.LLM
	if cfg.AIEnabled {
		llm = ai.NewOllama(cfg.OllamaURL, cfg.OllamaModel)
	}
	handler := httpapi.New(httpapi.Deps{
		Store: st, Pipeline: pipe, Hub: hub, Issuer: auth.NewIssuer(cfg.JWTSecret), Lookup: client,
		Assistant: ai.NewAssistant(llm), Chat: ai.NewChat(llm), Explainer: ai.NewExplainer(llm), DataDir: cfg.DataDir,
		LoginLimiter: auth.NewLimiter(5, time.Minute), TrustedProxies: cfg.TrustedProxies,
	}).Handler()

	// No WriteTimeout: Server-Sent Events streams are long-lived.
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr, "product", cfg.ProductName, "ai", cfg.AIEnabled)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		hub.Close() // open event streams would otherwise keep Shutdown waiting until its timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

// openWithRetry waits for the database: in Docker it may start after the API.
func openWithRetry(ctx context.Context, url string) (*store.Store, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		st, err := store.Open(ctx, url)
		if err == nil {
			return st, nil
		}
		lastErr = err
		slog.Warn("database not ready, retrying", "err", err)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}
