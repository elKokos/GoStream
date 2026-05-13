package broker

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/elKokos/GoStream/internal/config"
	"github.com/elKokos/GoStream/internal/httpapi"
	"github.com/elKokos/GoStream/internal/metrics"
)

type Broker struct {
	cfg    config.BrokerConfig
	log    *slog.Logger
	server *http.Server
	ready  atomic.Bool
}

func New(cfg config.BrokerConfig, log *slog.Logger) (*Broker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	registry := metrics.New()
	b := &Broker{
		cfg: cfg,
		log: log,
	}
	b.server = &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpapi.New(log, registry, b.ready.Load),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return b, nil
}

func (b *Broker) Run() error {
	b.ready.Store(true)
	b.log.Info("broker_started", "addr", b.cfg.HTTPAddr, "data_dir", b.cfg.DataDir)
	err := b.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (b *Broker) Shutdown(ctx context.Context) error {
	b.ready.Store(false)
	b.log.Info("broker_shutdown_started")
	err := b.server.Shutdown(ctx)
	if err != nil {
		return err
	}
	b.log.Info("broker_shutdown_completed")
	return nil
}
