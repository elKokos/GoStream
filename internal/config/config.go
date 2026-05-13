package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type BrokerConfig struct {
	DataDir         string
	HTTPAddr        string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	LogLevel        string
}

func Default() BrokerConfig {
	return BrokerConfig{
		DataDir:         "data",
		HTTPAddr:        ":8080",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		LogLevel:        "info",
	}
}

func FromEnv() (BrokerConfig, error) {
	cfg := Default()
	cfg.DataDir = getenv("GOSTREAM_DATA_DIR", cfg.DataDir)
	cfg.HTTPAddr = getenv("GOSTREAM_HTTP_ADDR", cfg.HTTPAddr)
	cfg.LogLevel = getenv("GOSTREAM_LOG_LEVEL", cfg.LogLevel)

	var err error
	if cfg.ReadTimeout, err = getenvDuration("GOSTREAM_READ_TIMEOUT", cfg.ReadTimeout); err != nil {
		return cfg, err
	}
	if cfg.WriteTimeout, err = getenvDuration("GOSTREAM_WRITE_TIMEOUT", cfg.WriteTimeout); err != nil {
		return cfg, err
	}
	if cfg.ShutdownTimeout, err = getenvDuration("GOSTREAM_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

func (c BrokerConfig) Validate() error {
	if c.DataDir == "" {
		return errors.New("data dir is required")
	}
	if c.HTTPAddr == "" {
		return errors.New("http addr is required")
	}
	if c.ReadTimeout <= 0 {
		return errors.New("read timeout must be positive")
	}
	if c.WriteTimeout <= 0 {
		return errors.New("write timeout must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be positive")
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}
