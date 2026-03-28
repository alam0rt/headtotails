package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alam0rt/headtotails/internal/api"
	"github.com/alam0rt/headtotails/internal/config"
	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/logging"
)

var version = "dev"

const targetTailscaleAPIVersion = "0.28.0"

func printVersion() {
	fmt.Printf("headtotails version: %s\n", version)
	fmt.Printf("target tailscale api: %s\n", targetTailscaleAPIVersion)
}

func shouldPrintVersion(args []string) bool {
	if len(args) < 2 {
		return false
	}

	switch args[1] {
	case "--version", "version":
		return true
	default:
		return false
	}
}

func main() {
	if shouldPrintVersion(os.Args) {
		printVersion()
		return
	}

	// Bootstrap logging before config loads so early failures are structured.
	if err := logging.Setup(logging.Options{
		Level:   "info",
		Service: "headtotails",
		Version: version,
		Env:     "production",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if err := logging.Setup(logging.Options{
		Level:     cfg.LogLevel,
		AddSource: cfg.LogAddSource,
		Service:   "headtotails",
		Version:   version,
		Env:       cfg.Environment,
	}); err != nil {
		slog.Error("failed to configure logger", "error", err)
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
		api.WIFConfig{
			Enabled:   cfg.WIFEnabled,
			IssuerURL: cfg.WIFIssuerURL,
			Audience:  cfg.WIFAudience,
			ClientID:  cfg.WIFClientID,
		},
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
