package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv         string
	Port           string
	DatabaseURL    string
	JWTSecret      string
	TokenTTL       time.Duration
	AllowedOrigins string
}

func Load() Config {
	return Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://banking:banking@localhost:5432/banking?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		TokenTTL:       getDuration("TOKEN_TTL_MINUTES", 60),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallbackMinutes int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallbackMinutes) * time.Minute
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(fallbackMinutes) * time.Minute
	}
	return time.Duration(parsed) * time.Minute
}
