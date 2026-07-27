package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps Redis for caching frequent graph queries.
type Client struct {
	rdb *redis.Client
}

// New creates a new Redis cache client.
func New(ctx context.Context, addr, password string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Get retrieves a cached value.
func (c *Client) Get(ctx context.Context, key string, dest any) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("cache get: %w", err)
	}
	return json.Unmarshal(data, dest)
}

// Set stores a value in the cache with a TTL.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// Invalidate removes a key from the cache.
func (c *Client) Invalidate(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Cache key constants
const (
	KeyDashboardStats = "cache:dashboard:stats"
	KeyFindingDist    = "cache:findings:distribution"
	KeyGraphData      = "cache:graph:data"
)

// Cache TTL constants
const (
	TTLDashboardStats = 30 * time.Second
	TTLGraphData      = 60 * time.Second
)

var ErrCacheMiss = fmt.Errorf("cache miss")
