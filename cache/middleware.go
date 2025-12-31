package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/abreed05/goexpress"
)

// CacheConfig holds cache middleware configuration
type CacheConfig struct {
	Cache      Cache
	TTL        time.Duration
	KeyFunc    func(*goexpress.Context) string
	SkipFunc   func(*goexpress.Context) bool
	OnlyStatus []int
}

// DefaultCacheConfig returns a default cache configuration
func DefaultCacheConfig(cache Cache) CacheConfig {
	return CacheConfig{
		Cache:      cache,
		TTL:        5 * time.Minute,
		OnlyStatus: []int{200},
		KeyFunc: func(c *goexpress.Context) string {
			return c.Method() + ":" + c.Path()
		},
	}
}

// Middleware returns a cache middleware for GoExpress
func Middleware(config CacheConfig) goexpress.Middleware {
	if config.Cache == nil {
		panic("cache is required")
	}

	if config.KeyFunc == nil {
		config.KeyFunc = func(c *goexpress.Context) string {
			return c.Method() + ":" + c.Path()
		}
	}

	if config.OnlyStatus == nil {
		config.OnlyStatus = []int{200}
	}

	return func(next goexpress.HandlerFunc) goexpress.HandlerFunc {
		return func(c *goexpress.Context) error {
			// Skip if skip function returns true
			if config.SkipFunc != nil && config.SkipFunc(c) {
				return next(c)
			}

			// Only cache GET and HEAD requests
			if c.Method() != "GET" && c.Method() != "HEAD" {
				return next(c)
			}

			// Generate cache key
			key := config.KeyFunc(c)

			// Try to get from cache
			var cached CachedResponse
			err := config.Cache.Get(key, &cached)
			if err == nil {
				// Cache hit - restore response
				for k, v := range cached.Headers {
					c.SetHeader(k, v)
				}
				c.Status(cached.Status)
				return c.Send(cached.Body)
			}

			// Cache miss - execute handler
			// Create a response recorder
			recorder := &responseRecorder{
				Context: c,
				headers: make(map[string]string),
			}

			err = next(recorder.Context)
			if err != nil {
				return err
			}

			// Check if status should be cached
			shouldCache := false
			for _, status := range config.OnlyStatus {
				if recorder.status == status {
					shouldCache = true
					break
				}
			}

			// Store in cache if appropriate
			if shouldCache && recorder.body != nil {
				cached := CachedResponse{
					Status:  recorder.status,
					Headers: recorder.headers,
					Body:    recorder.body,
				}
				config.Cache.Set(key, cached, config.TTL)
			}

			return nil
		}
	}
}

// CachedResponse holds a cached HTTP response
type CachedResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

// responseRecorder records the response for caching
type responseRecorder struct {
	Context *goexpress.Context
	status  int
	headers map[string]string
	body    []byte
}

// GenerateCacheKey generates a cache key from method, path, and query params
func GenerateCacheKey(c *goexpress.Context) string {
	data := fmt.Sprintf("%s:%s:%s", c.Method(), c.Path(), c.Request.URL.RawQuery)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Invalidate removes specific keys from cache
func Invalidate(cache Cache, keys ...string) error {
	for _, key := range keys {
		if err := cache.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

// InvalidatePattern removes keys matching a pattern (Redis only)
func InvalidatePattern(cache *RedisCache, pattern string) error {
	client := cache.GetClient()
	keys, err := client.Keys(cache.ctx, cache.prefix+pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return client.Del(cache.ctx, keys...).Err()
	}

	return nil
}

// CacheJSON caches a JSON response manually
func CacheJSON(cache Cache, key string, data interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cached := CachedResponse{
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: jsonData,
	}

	return cache.Set(key, cached, ttl)
}

// Helper function to create a cache key with parameters
func CacheKeyWithParams(c *goexpress.Context, params ...string) string {
	key := c.Method() + ":" + c.Path()
	for _, param := range params {
		key += ":" + c.Query(param)
	}
	return key
}
