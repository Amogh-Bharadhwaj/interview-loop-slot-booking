package config

import (
	"os"
	"testing"
	"time"
)

func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestEnvOrDefault(t *testing.T) {
	os.Unsetenv("TEST_CFG_STR")
	if got := envOrDefault("TEST_CFG_STR", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
	withEnv(t, "TEST_CFG_STR", "custom")
	if got := envOrDefault("TEST_CFG_STR", "fallback"); got != "custom" {
		t.Errorf("expected custom, got %q", got)
	}
}

func TestEnvIntOrDefault(t *testing.T) {
	os.Unsetenv("TEST_CFG_INT")
	if got := envIntOrDefault("TEST_CFG_INT", 7); got != 7 {
		t.Errorf("expected default 7, got %d", got)
	}
	withEnv(t, "TEST_CFG_INT", "42")
	if got := envIntOrDefault("TEST_CFG_INT", 7); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestEnvSecondsOrDefault(t *testing.T) {
	os.Unsetenv("TEST_CFG_SECONDS")
	if got := envSecondsOrDefault("TEST_CFG_SECONDS", 90); got != 90*time.Second {
		t.Errorf("expected 90s default, got %v", got)
	}
	withEnv(t, "TEST_CFG_SECONDS", "15")
	if got := envSecondsOrDefault("TEST_CFG_SECONDS", 90); got != 15*time.Second {
		t.Errorf("expected 15s, got %v", got)
	}
}

func TestLoadReadsDatabaseURL(t *testing.T) {
	withEnv(t, "DATABASE_URL", "postgres://example")
	withEnv(t, "PORT", "9090")
	withEnv(t, "HOLD_WINDOW_SECONDS", "5")
	withEnv(t, "REAPER_INTERVAL_SECONDS", "2")
	withEnv(t, "CACHE_WINDOW_DAYS", "3")

	cfg := Load()
	if cfg.DatabaseURL != "postgres://example" {
		t.Errorf("expected DatabaseURL to be read from env, got %q", cfg.DatabaseURL)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected Port 9090, got %q", cfg.Port)
	}
	if cfg.HoldWindow != 5*time.Second {
		t.Errorf("expected HoldWindow 5s, got %v", cfg.HoldWindow)
	}
	if cfg.ReaperInterval != 2*time.Second {
		t.Errorf("expected ReaperInterval 2s, got %v", cfg.ReaperInterval)
	}
	if cfg.CacheWindowDays != 3 {
		t.Errorf("expected CacheWindowDays 3, got %d", cfg.CacheWindowDays)
	}
}
