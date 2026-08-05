// Package redis provides a Redis-backed implementation of cache.Cache.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/amirhosein/weather-fusion/internal/cache"
	"github.com/amirhosein/weather-fusion/internal/config"
)

// client wraps go-redis to satisfy the cache.Cache interface.
type client struct {
	rdb *goredis.Client
}

// NewClient creates and validates a Redis connection.
func NewClient(cfg *config.Config) (cache.Cache, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &client{rdb: rdb}, nil
}

func (c *client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

func (c *client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", cache.ErrCacheMiss
	}
	return val, err
}

func (c *client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *client) FlushAll(ctx context.Context) error {
	return c.rdb.FlushAll(ctx).Err()
}

func (c *client) Close() error {
	return c.rdb.Close()
}
