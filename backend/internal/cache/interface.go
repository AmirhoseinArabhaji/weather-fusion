// Package cache defines the Cache interface used across the application.
package cache

import (
	"context"
	"time"
)

// Cache is a generic key-value store interface.
// Implementations include Redis (production) and in-memory (testing).
type Cache interface {
	// Set stores a value under key with an optional TTL (0 = no expiry).
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Get retrieves the value stored under key.
	// Returns ErrCacheMiss if the key does not exist.
	Get(ctx context.Context, key string) (string, error)

	// Delete removes one or more keys.
	Delete(ctx context.Context, keys ...string) error

	// Exists returns true if the key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// FlushAll removes all keys (use with care — mainly for testing).
	FlushAll(ctx context.Context) error

	// Close releases the underlying connection.
	Close() error
}

// ErrCacheMiss is returned when a requested key is not present.
var ErrCacheMiss = &cacheMissError{}

type cacheMissError struct{}

func (e *cacheMissError) Error() string { return "cache miss" }
