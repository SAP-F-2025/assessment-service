package pkg

import (
	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	// Implementation for creating a new Redis client

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	defer client.Close()

	return client, nil
}
