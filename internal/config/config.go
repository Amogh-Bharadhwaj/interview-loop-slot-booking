package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	Port              string
	HoldWindow        time.Duration
	ReaperInterval    time.Duration
	CacheWindowDays   int
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on process environment")
	}

	return Config{
		DatabaseURL:     mustEnv("DATABASE_URL"),
		Port:            envOrDefault("PORT", "8080"),
		HoldWindow:      envSecondsOrDefault("HOLD_WINDOW_SECONDS", 90),
		ReaperInterval:  envSecondsOrDefault("REAPER_INTERVAL_SECONDS", 15),
		CacheWindowDays: envIntOrDefault("CACHE_WINDOW_DAYS", 3),
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("invalid int for %s: %v", key, err)
	}
	return n
}

func envSecondsOrDefault(key string, defSeconds int) time.Duration {
	return time.Duration(envIntOrDefault(key, defSeconds)) * time.Second
}
