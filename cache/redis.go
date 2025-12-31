package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss is returned when a key is not found
	ErrCacheMiss = errors.New("cache miss")
)

// Cache is the interface for cache operations
type Cache interface {
	// Get retrieves a value from cache
	Get(key string, dest interface{}) error
	
	// Set stores a value in cache
	Set(key string, value interface{}, ttl time.Duration) error
	
	// Delete removes a value from cache
	Delete(key string) error
	
	// Exists checks if a key exists
	Exists(key string) (bool, error)
	
	// Clear removes all cached items
	Clear() error
	
	// Close closes the cache connection
	Close() error
}

// RedisCache implements a Redis-based cache
type RedisCache struct {
	client *redis.Client
	prefix string
	ctx    context.Context
}

// RedisConfig holds Redis cache configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Prefix   string
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(config RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	prefix := config.Prefix
	if prefix == "" {
		prefix = "cache:"
	}

	return &RedisCache{
		client: client,
		prefix: prefix,
		ctx:    ctx,
	}, nil
}

// Get retrieves a value from cache
func (r *RedisCache) Get(key string, dest interface{}) error {
	fullKey := r.prefix + key

	data, err := r.client.Get(r.ctx, fullKey).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// GetString retrieves a string value from cache
func (r *RedisCache) GetString(key string) (string, error) {
	fullKey := r.prefix + key
	result, err := r.client.Get(r.ctx, fullKey).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return result, err
}

// GetBytes retrieves raw bytes from cache
func (r *RedisCache) GetBytes(key string) ([]byte, error) {
	fullKey := r.prefix + key
	result, err := r.client.Get(r.ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	return result, err
}

// Set stores a value in cache
func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	fullKey := r.prefix + key

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(r.ctx, fullKey, data, ttl).Err()
}

// SetString stores a string value in cache
func (r *RedisCache) SetString(key string, value string, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Set(r.ctx, fullKey, value, ttl).Err()
}

// SetBytes stores raw bytes in cache
func (r *RedisCache) SetBytes(key string, value []byte, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Set(r.ctx, fullKey, value, ttl).Err()
}

// Delete removes a value from cache
func (r *RedisCache) Delete(key string) error {
	fullKey := r.prefix + key
	return r.client.Del(r.ctx, fullKey).Err()
}

// DeleteMany removes multiple keys from cache
func (r *RedisCache) DeleteMany(keys ...string) error {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = r.prefix + key
	}
	return r.client.Del(r.ctx, fullKeys...).Err()
}

// Exists checks if a key exists
func (r *RedisCache) Exists(key string) (bool, error) {
	fullKey := r.prefix + key
	result, err := r.client.Exists(r.ctx, fullKey).Result()
	return result > 0, err
}

// Clear removes all cached items with the prefix
func (r *RedisCache) Clear() error {
	keys, err := r.client.Keys(r.ctx, r.prefix+"*").Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.client.Del(r.ctx, keys...).Err()
	}

	return nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

// Increment increments a numeric value
func (r *RedisCache) Increment(key string) (int64, error) {
	fullKey := r.prefix + key
	return r.client.Incr(r.ctx, fullKey).Result()
}

// Decrement decrements a numeric value
func (r *RedisCache) Decrement(key string) (int64, error) {
	fullKey := r.prefix + key
	return r.client.Decr(r.ctx, fullKey).Result()
}

// IncrementBy increments by a specific amount
func (r *RedisCache) IncrementBy(key string, value int64) (int64, error) {
	fullKey := r.prefix + key
	return r.client.IncrBy(r.ctx, fullKey, value).Result()
}

// TTL returns the remaining time to live for a key
func (r *RedisCache) TTL(key string) (time.Duration, error) {
	fullKey := r.prefix + key
	return r.client.TTL(r.ctx, fullKey).Result()
}

// Expire sets a timeout on a key
func (r *RedisCache) Expire(key string, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Expire(r.ctx, fullKey, ttl).Err()
}

// Remember retrieves from cache or executes a function and stores the result
func (r *RedisCache) Remember(key string, ttl time.Duration, fn func() (interface{}, error), dest interface{}) error {
	// Try to get from cache
	err := r.Get(key, dest)
	if err == nil {
		return nil
	}

	if err != ErrCacheMiss {
		return err
	}

	// Execute function
	value, err := fn()
	if err != nil {
		return err
	}

	// Store in cache
	if err := r.Set(key, value, ttl); err != nil {
		return err
	}

	// Marshal and unmarshal to populate dest
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// Tags support for cache invalidation
type TaggedCache struct {
	cache  *RedisCache
	tags   []string
	prefix string
}

// Tags creates a tagged cache instance
func (r *RedisCache) Tags(tags ...string) *TaggedCache {
	return &TaggedCache{
		cache:  r,
		tags:   tags,
		prefix: "tag:",
	}
}

// Set stores a value with tags
func (t *TaggedCache) Set(key string, value interface{}, ttl time.Duration) error {
	// Store the actual value
	if err := t.cache.Set(key, value, ttl); err != nil {
		return err
	}

	// Store tag references
	for _, tag := range t.tags {
		tagKey := t.prefix + tag
		// Add key to tag's set
		t.cache.client.SAdd(t.cache.ctx, tagKey, key)
		// Set expiration on tag key if ttl is specified
		if ttl > 0 {
			t.cache.client.Expire(t.cache.ctx, tagKey, ttl)
		}
	}

	return nil
}

// Flush removes all cached items with the specified tags
func (t *TaggedCache) Flush() error {
	for _, tag := range t.tags {
		tagKey := t.prefix + tag
		
		// Get all keys associated with this tag
		keys, err := t.cache.client.SMembers(t.cache.ctx, tagKey).Result()
		if err != nil {
			return err
		}

		// Delete all keys
		if len(keys) > 0 {
			fullKeys := make([]string, len(keys))
			for i, key := range keys {
				fullKeys[i] = t.cache.prefix + key
			}
			t.cache.client.Del(t.cache.ctx, fullKeys...)
		}

		// Delete the tag key itself
		t.cache.client.Del(t.cache.ctx, tagKey)
	}

	return nil
}
