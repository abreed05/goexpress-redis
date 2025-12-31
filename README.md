# GoExpress Redis Integration

Redis support for GoExpress with session management and caching.

## Features

- 🔐 **Session Management** - Redis, in-memory, and cookie-based sessions
- 💾 **Caching** - Full-featured Redis cache with TTL support
- ⚡ **High Performance** - Fast Redis operations
- 🔄 **Multiple Backends** - Switch between Redis and in-memory stores
- 🎯 **Type-Safe** - Full TypeScript-like type safety in Go
- 🏷️ **Tagged Cache** - Cache invalidation by tags
- ⏱️ **TTL Support** - Automatic expiration
- 🔒 **Secure** - HttpOnly, Secure, SameSite cookie options

## Installation

```bash
go get github.com/abreed05/goexpress-redis
go get github.com/redis/go-redis/v9
```

## Quick Start

### Redis Sessions

```go
import (
    "github.com/abreed05/goexpress"
    "github.com/abreed05/goexpress-redis/session"
)

// Initialize Redis session store
store, _ := session.NewRedisStore(session.RedisConfig{
    Addr:   "localhost:6379",
    Prefix: "session:",
})

// Add session middleware
sessionConfig := session.DefaultConfig(store)
app.Use(session.Middleware(sessionConfig))

// Use sessions in handlers
app.POST("/login", func(c *goexpress.Context) error {
    sess, _ := session.GetSession(c)
    sess.Set("user_id", "123")
    return c.JSON(map[string]string{"message": "Logged in"})
})
```

### Redis Cache

```go
import "github.com/abreed05/goexpress-redis/cache"

// Initialize Redis cache
redisCache, _ := cache.NewRedisCache(cache.RedisConfig{
    Addr:   "localhost:6379",
    Prefix: "cache:",
})

// Cache middleware
cacheConfig := cache.DefaultCacheConfig(redisCache)
app.GET("/users", usersHandler, cache.Middleware(cacheConfig))

// Manual cache usage
app.GET("/products/:id", func(c *goexpress.Context) error {
    var product Product
    key := "product:" + c.Param("id")
    
    // Try cache first
    err := redisCache.Get(key, &product)
    if err == nil {
        return c.JSON(product)
    }
    
    // Fetch from database
    product = fetchFromDB()
    redisCache.Set(key, product, 10*time.Minute)
    
    return c.JSON(product)
})
```

## Session Management

### Session Stores

#### 1. Redis Store

```go
store, err := session.NewRedisStore(session.RedisConfig{
    Addr:     "localhost:6379",
    Password: "",           // Set if Redis has auth
    DB:       0,            // Redis database number
    Prefix:   "session:",   // Key prefix
})
```

#### 2. Memory Store (No Redis Required)

```go
store := session.NewMemoryStore(5 * time.Minute) // Cleanup interval
```

#### 3. Cookie Store

```go
store := session.NewCookieStore(24 * time.Hour)
```

### Session Configuration

```go
config := session.Config{
    Store:        store,
    CookieName:   "session_id",
    CookiePath:   "/",
    CookieDomain: "",
    MaxAge:       24 * time.Hour,
    Secure:       true,      // HTTPS only
    HttpOnly:     true,      // No JavaScript access
    SameSite:     http.SameSiteLaxMode,
    ContextKey:   "session",
}

app.Use(session.Middleware(config))
```

### Working with Sessions

```go
// Get session
sess, err := session.GetSession(c)

// Set values
sess.Set("user_id", 123)
sess.Set("username", "john")
sess.Set("role", "admin")

// Get values
userID, ok := sess.Get("user_id")
username, _ := sess.Get("username")

// Delete values
sess.Delete("temp_data")

// Clear all data
sess.Clear()

// Check expiration
if sess.IsExpired() {
    // Session expired
}
```

### Flash Messages

One-time messages that survive a single redirect:

```go
// Set flash message
session.Flash(c, "success", "Profile updated!")
session.Flash(c, "error", "Invalid input")

// Get flash message (automatically deleted)
message, ok := session.GetFlash(c, "success")
if ok {
    // Display message
}
```

### Session Operations

```go
// Destroy session
session.DestroySession(c, config)

// Regenerate session ID (prevents fixation attacks)
session.RegenerateSession(c, config)
```

## Caching

### Cache Middleware

Automatically cache GET requests:

```go
cacheConfig := cache.DefaultCacheConfig(redisCache)
cacheConfig.TTL = 5 * time.Minute
cacheConfig.OnlyStatus = []int{200} // Only cache successful responses

app.GET("/users", usersHandler, cache.Middleware(cacheConfig))
```

Custom cache key:

```go
cacheConfig.KeyFunc = func(c *goexpress.Context) string {
    return c.Path() + ":" + c.Query("page")
}
```

Skip caching conditionally:

```go
cacheConfig.SkipFunc = func(c *goexpress.Context) bool {
    return c.Query("nocache") == "true"
}
```

### Manual Cache Operations

```go
// Set
redisCache.Set("key", data, 10*time.Minute)

// Get
var data MyStruct
err := redisCache.Get("key", &data)

// String operations
redisCache.SetString("key", "value", time.Hour)
value, _ := redisCache.GetString("key")

// Delete
redisCache.Delete("key")

// Check existence
exists, _ := redisCache.Exists("key")

// Clear all
redisCache.Clear()
```

### Advanced Cache Features

#### Increment/Decrement

```go
count, _ := redisCache.Increment("page_views")
redisCache.IncrementBy("counter", 5)
redisCache.Decrement("stock")
```

#### TTL Management

```go
// Get remaining TTL
ttl, _ := redisCache.TTL("key")

// Set expiration
redisCache.Expire("key", 1*time.Hour)
```

#### Remember Pattern

Execute function only on cache miss:

```go
var users []User
err := redisCache.Remember("users", 5*time.Minute, func() (interface{}, error) {
    return fetchUsersFromDB()
}, &users)
```

#### Tagged Cache

Group related cache entries for easy invalidation:

```go
// Cache with tags
tagged := redisCache.Tags("users", "api", "v1")
tagged.Set("user:123", user, 10*time.Minute)

// Flush all cache entries with these tags
tagged.Flush()
```

### Cache Invalidation

```go
// Invalidate specific keys
cache.Invalidate(redisCache, "user:123", "user:456")

// Invalidate by pattern (Redis only)
cache.InvalidatePattern(redisCache, "user:*")

// Invalidate on data changes
app.POST("/users/:id", func(c *goexpress.Context) error {
    // Update user...
    
    // Clear cache
    redisCache.Delete("user:" + c.Param("id"))
    cache.InvalidatePattern(redisCache, "users:*")
    
    return c.JSON(user)
})
```

## Complete Examples

### E-commerce API with Redis

```go
package main

import (
    "github.com/abreed05/goexpress"
    "github.com/abreed05/goexpress-redis/cache"
    "github.com/abreed05/goexpress-redis/session"
)

func main() {
    app := goexpress.New()
    
    // Redis session
    sessionStore, _ := session.NewRedisStore(session.RedisConfig{
        Addr: "localhost:6379",
    })
    app.Use(session.Middleware(session.DefaultConfig(sessionStore)))
    
    // Redis cache
    redisCache, _ := cache.NewRedisCache(cache.RedisConfig{
        Addr: "localhost:6379",
        DB:   1,
    })
    
    // Public routes
    app.POST("/login", loginHandler)
    app.POST("/register", registerHandler)
    
    // Cached product catalog
    cacheConfig := cache.DefaultCacheConfig(redisCache)
    app.GET("/products", productsHandler, cache.Middleware(cacheConfig))
    app.GET("/products/:id", productHandler, cache.Middleware(cacheConfig))
    
    // Protected routes
    protected := app.Group("/api", requireAuth)
    {
        // Shopping cart in session
        protected.GET("/cart", getCartHandler)
        protected.POST("/cart/add", addToCartHandler)
        protected.DELETE("/cart/:id", removeFromCartHandler)
        
        // Checkout
        protected.POST("/checkout", checkoutHandler)
        
        // Profile (cached per user)
        protected.GET("/profile", profileHandler)
    }
    
    app.Listen("3000")
}

func loginHandler(c *goexpress.Context) error {
    // Authenticate user...
    
    sess, _ := session.GetSession(c)
    sess.Set("user_id", user.ID)
    sess.Set("email", user.Email)
    
    return c.JSON(map[string]string{"message": "Logged in"})
}

func addToCartHandler(c *goexpress.Context) error {
    sess, _ := session.GetSession(c)
    
    // Get cart from session
    cart, _ := sess.Get("cart")
    if cart == nil {
        cart = []CartItem{}
    }
    
    // Add item...
    
    sess.Set("cart", cart)
    return c.JSON(cart)
}

func requireAuth(next goexpress.HandlerFunc) goexpress.HandlerFunc {
    return func(c *goexpress.Context) error {
        sess, _ := session.GetSession(c)
        if _, ok := sess.Get("user_id"); !ok {
            return goexpress.ErrUnauthorized
        }
        return next(c)
    }
}
```

### API Rate Limiting with Redis

```go
func RateLimitMiddleware(cache *cache.RedisCache, maxRequests int, window time.Duration) goexpress.Middleware {
    return func(next goexpress.HandlerFunc) goexpress.HandlerFunc {
        return func(c *goexpress.Context) error {
            ip := c.IP()
            key := "ratelimit:" + ip
            
            // Increment counter
            count, _ := cache.Increment(key)
            
            if count == 1 {
                // First request, set expiration
                cache.Expire(key, window)
            }
            
            if count > int64(maxRequests) {
                return c.Status(429).JSON(map[string]string{
                    "error": "Rate limit exceeded",
                })
            }
            
            return next(c)
        }
    }
}

// Usage
app.Use(RateLimitMiddleware(redisCache, 100, time.Minute))
```

## Configuration Options

### Redis Connection

```go
config := session.RedisConfig{
    Addr:     "localhost:6379",
    Password: "secret",
    DB:       0,
    Prefix:   "myapp:",
}
```

### Session Options

```go
sessionConfig := session.Config{
    Store:        store,
    CookieName:   "sid",
    MaxAge:       7 * 24 * time.Hour, // 7 days
    Secure:       true,
    HttpOnly:     true,
    SameSite:     http.SameSiteStrictMode,
}
```

### Cache Options

```go
cacheConfig := cache.CacheConfig{
    Cache:      redisCache,
    TTL:        10 * time.Minute,
    OnlyStatus: []int{200, 201},
    KeyFunc: func(c *goexpress.Context) string {
        return cache.GenerateCacheKey(c)
    },
}
```

## Best Practices

### 1. Use Redis for Production

```go
// Development: In-memory
store := session.NewMemoryStore(5 * time.Minute)

// Production: Redis
store, _ := session.NewRedisStore(session.RedisConfig{
    Addr: os.Getenv("REDIS_URL"),
})
```

### 2. Separate Redis Databases

```go
// DB 0 for sessions
sessionStore, _ := session.NewRedisStore(session.RedisConfig{
    Addr: "localhost:6379",
    DB:   0,
})

// DB 1 for cache
cacheStore, _ := cache.NewRedisCache(cache.RedisConfig{
    Addr: "localhost:6379",
    DB:   1,
})
```

### 3. Set Appropriate TTLs

```go
// Short TTL for frequently changing data
redisCache.Set("stock:123", stock, 1*time.Minute)

// Long TTL for static data
redisCache.Set("config", config, 24*time.Hour)

// Sessions
sessionConfig.MaxAge = 7 * 24 * time.Hour // 7 days
```

### 4. Invalidate Cache on Updates

```go
app.PUT("/products/:id", func(c *goexpress.Context) error {
    id := c.Param("id")
    
    // Update product...
    
    // Invalidate cache
    redisCache.Delete("product:" + id)
    cache.InvalidatePattern(redisCache, "products:*")
    
    return c.JSON(product)
})
```

### 5. Use Flash Messages for Form Feedback

```go
app.POST("/contact", func(c *goexpress.Context) error {
    // Process form...
    
    if err != nil {
        session.Flash(c, "error", "Failed to send message")
        return c.Redirect("/contact")
    }
    
    session.Flash(c, "success", "Message sent!")
    return c.Redirect("/thank-you")
})
```

## Testing

```go
import "testing"

func TestSessionStore(t *testing.T) {
    store := session.NewMemoryStore(0)
    
    sess := session.NewSession(time.Hour)
    sess.Set("key", "value")
    
    store.Set(sess)
    
    retrieved, err := store.Get(sess.ID)
    if err != nil {
        t.Fatal(err)
    }
    
    if val, _ := retrieved.Get("key"); val != "value" {
        t.Error("Value mismatch")
    }
}
```

## Troubleshooting

### Redis Connection Failed

```bash
# Check if Redis is running
redis-cli ping

# Start Redis
redis-server

# Or with Docker
docker run -d -p 6379:6379 redis:alpine
```

### Sessions Not Persisting

- Check cookie settings (Secure flag with HTTP)
- Verify Redis connection
- Check session TTL configuration

### Cache Not Working

- Verify cache middleware is applied
- Check TTL is not expired
- Ensure cache key is consistent

## License

MIT License
