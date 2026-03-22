package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alam0rt/headtotails/internal/api"
	"github.com/alam0rt/headtotails/internal/config"
	"github.com/alam0rt/headtotails/internal/headscale"
)

func main() {
	// Structured JSON logging.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create gRPC client.
	hs, err := headscale.New(cfg.HeadscaleAddr, cfg.HeadscaleAPIKey)
	if err != nil {
		slog.Error("failed to connect to headscale", "error", err)
		os.Exit(1)
	}

	// Build HTTP router.
	router := api.NewRouter(
		hs,
		cfg.TailnetName,
		cfg.HeadscaleAPIKey,
		cfg.OAuthClientID,
		cfg.OAuthClientSecret,
		cfg.OAuthHMACSecret,
	)
	r := router.Build()

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Optionally enable TLS.
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			slog.Error("failed to load TLS certificate", "error", err)
			os.Exit(1)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	// Start server in a goroutine.
	go func() {
		slog.Info("headtotails starting", "addr", cfg.ListenAddr)
		var err error
		if srv.TLSConfig != nil {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGTERM / SIGINT.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("shutting down headtotails")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("headtotails stopped")
}
