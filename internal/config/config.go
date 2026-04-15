package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DBURL          string
	RedisURL       string
	NatsURL        string
	APIKey         string
	DemoMode       bool
	AllowedOrigins string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		DBURL:          getEnv("DB_URL", "postgres://sports:sports@localhost:5432/sportsdb?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
		NatsURL:        getEnv("NATS_URL", "nats://localhost:4222"),
		APIKey:         getEnv("API_KEY", ""),
		DemoMode:       getBoolEnv("DEMO_MODE", false),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
