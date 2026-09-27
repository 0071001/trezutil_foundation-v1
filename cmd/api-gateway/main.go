package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/gateway"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cfg := gateway.LoadConfig()
	gw := gateway.New(cfg, logger)

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: gw.Handler(),
	}

	go func() {
		logger.Info().Str("addr", cfg.Addr).Msg("api-gateway starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down api-gateway...")
	if err := server.Close(); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
}
