package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	Environment string
	Events      EventConfig
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/dbname"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "supersecretkey"),
		Environment: getEnv("ENVIRONMENT", "development"),
		Events: EventConfig{
			Enabled:           getEnv("EVENTS_ENABLED", "true") == "true",
			Publisher:         getEnv("EVENTS_PUBLISHER", "kafka"),
			KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
			NotificationTopic: getEnv("NOTIFICATION_TOPIC", "notifications"),
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
