package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements a Redis-based session store
type RedisStore struct {
	client *redis.Client
	prefix string
	ctx    context.Context
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string // Redis server address (e.g., "localhost:6379")
	Password string // Password for authentication
	DB       int    // Database number
	Prefix   string // Key prefix for sessions (e.g., "session:")
}

// NewRedisStore creates a new Redis session store
func NewRedisStore(config RedisConfig) (*RedisStore, error) {
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
		prefix = "session:"
	}

	return &RedisStore{
		client: client,
		prefix: prefix,
		ctx:    ctx,
	}, nil
}

// Get retrieves a session from Redis
func (r *RedisStore) Get(id string) (*Session, error) {
	key := r.prefix + id

	data, err := r.client.Get(r.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	if session.IsExpired() {
		r.Delete(id)
		return nil, ErrSessionExpired
	}

	return &session, nil
}

// Set stores a session in Redis
func (r *RedisStore) Set(session *Session) error {
	key := r.prefix + session.ID

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	// Calculate TTL
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return ErrSessionExpired
	}

	return r.client.Set(r.ctx, key, data, ttl).Err()
}

// Delete removes a session from Redis
func (r *RedisStore) Delete(id string) error {
	key := r.prefix + id
	return r.client.Del(r.ctx, key).Err()
}

// Touch updates the session's expiration time
func (r *RedisStore) Touch(id string) error {
	session, err := r.Get(id)
	if err != nil {
		return err
	}

	session.UpdatedAt = time.Now()
	return r.Set(session)
}

// Cleanup is a no-op for Redis (it handles expiration automatically)
func (r *RedisStore) Cleanup() error {
	return nil
}

// Close closes the Redis connection
func (r *RedisStore) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client for advanced operations
func (r *RedisStore) GetClient() *redis.Client {
	return r.client
}

// SetWithTTL stores a session with a custom TTL
func (r *RedisStore) SetWithTTL(session *Session, ttl time.Duration) error {
	key := r.prefix + session.ID

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	return r.client.Set(r.ctx, key, data, ttl).Err()
}

// Exists checks if a session exists
func (r *RedisStore) Exists(id string) (bool, error) {
	key := r.prefix + id
	result, err := r.client.Exists(r.ctx, key).Result()
	return result > 0, err
}

// Count returns the number of active sessions
func (r *RedisStore) Count() (int64, error) {
	keys, err := r.client.Keys(r.ctx, r.prefix+"*").Result()
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// Clear removes all sessions
func (r *RedisStore) Clear() error {
	keys, err := r.client.Keys(r.ctx, r.prefix+"*").Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.client.Del(r.ctx, keys...).Err()
	}

	return nil
}
