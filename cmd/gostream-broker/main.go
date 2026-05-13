package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/elKokos/GoStream/internal/broker"
	"github.com/elKokos/GoStream/internal/config"
	"github.com/elKokos/GoStream/internal/logging"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel)
	b, err := broker.New(cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "broker init error: %v\n", err)
		os.Exit(1)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Run()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("signal_received", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			log.Error("broker_failed", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := b.Shutdown(ctx); err != nil {
		log.Error("broker_shutdown_failed", "error", err)
		os.Exit(1)
	}
}
