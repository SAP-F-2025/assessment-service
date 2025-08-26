package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type CacheService interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	DeletePattern(ctx context.Context, pattern string) error
}

type redisCache struct {
	client *redis.Client
	logger *zap.Logger
}

func (r redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	//TODO implement me
	panic("implement me")
}

func (r redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (r redisCache) Delete(ctx context.Context, key string) error {
	//TODO implement me
	panic("implement me")
}

func (r redisCache) DeletePattern(ctx context.Context, pattern string) error {
	//TODO implement me
	panic("implement me")
}

func NewRedisCache(client *redis.Client, logger *zap.Logger) CacheService {
	return &redisCache{
		client: client,
		logger: logger,
	}
}
