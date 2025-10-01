package config

import (
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	Environment string
	LogLevel    slog.Level
	Events      EventConfig
	Casdoor     CasdoorConfig
}

type CasdoorConfig struct {
	Endpoint     string
	ClientID     string
	ClientSecret string
	Cert         string
	Organization string
	Application  string
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, proceeding with environment variables: ", err)
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/dbname"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "supersecretkey"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    parseLogLevel(getEnv("LOG_LEVEL", "info")),
		Events: EventConfig{
			Enabled:           getEnv("EVENTS_ENABLED", "true") == "true",
			Publisher:         getEnv("EVENTS_PUBLISHER", "kafka"),
			KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
			NotificationTopic: getEnv("NOTIFICATION_TOPIC", "notifications"),
		},
		Casdoor: CasdoorConfig{
			Endpoint:     getEnv("CASDOOR_ENDPOINT", "http://localhost:8000"),
			ClientID:     getEnv("CASDOOR_CLIENT_ID", ""),
			ClientSecret: getEnv("CASDOOR_CLIENT_SECRET", ""),
			Organization: getEnv("CASDOOR_ORGANIZATION", ""),
			Application:  getEnv("CASDOOR_APPLICATION", ""),
			Cert:         getEnv("CASDOOR_CERT", ""),
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

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
