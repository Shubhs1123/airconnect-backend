package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	JWTRefreshSecret string
	MQTTBrokerURL   string
	MQTTAdminUser   string
	MQTTAdminPass   string
	OpenAIKey       string
	OpenAIModel     string
	PlatformIOPath  string
	BuildTimeout    int
	BuildConcurrency int
	StoragePath     string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:             getEnv("PORT", "3000"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/airconnect?sslmode=disable"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "change-me-refresh-secret"),
		MQTTBrokerURL:    getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		MQTTAdminUser:    getEnv("MQTT_ADMIN_USER", "airconnect"),
		MQTTAdminPass:    getEnv("MQTT_ADMIN_PASS", "airconnect"),
		OpenAIKey:        getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:      getEnv("OPENAI_MODEL", "gpt-4o"),
		PlatformIOPath:   getEnv("PLATFORMIO_PATH", "platformio"),
		BuildTimeout:     getEnvInt("BUILD_TIMEOUT_MS", 300000),
		BuildConcurrency: getEnvInt("BUILD_CONCURRENCY", 2),
		StoragePath:      getEnv("STORAGE_PATH", "./storage"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
