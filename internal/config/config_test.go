package config

import (
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestConfigValidationRejectsMissingDataDir(t *testing.T) {
	cfg := Default()
	cfg.DataDir = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConfigValidationRejectsInvalidTimeout(t *testing.T) {
	cfg := Default()
	cfg.ReadTimeout = -time.Second
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
