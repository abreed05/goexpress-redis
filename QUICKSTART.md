# GoExpress Redis Integration - Quick Start

## Overview

This package adds Redis support to GoExpress for:
- **Session Management** - Store user sessions in Redis, memory, or cookies
- **Caching** - High-performance Redis caching with TTL support
- **Flash Messages** - One-time messages that survive redirects

## Installation

```bash
# Install the Redis package
go get github.com/abreed05/goexpress-redis

# Install Redis Go client
go get github.com/redis/go-redis/v9
```

## Option 1: Redis Sessions & Cache (Recommended for Production)

**Requirements:** Redis server running on `localhost:6379`

```go
package main

import (
    "log"
    "time"
    "github.com/abreed05/goexpress"
    "github.com/abreed05/goexpress/middleware"
    "github.com/abreed05/goexpress-redis/cache"
    "github.com/abreed05/goexpress-redis/session"
)

func main() {
    app := goexpress.New()
    app.Use(middleware.Logger())
    app.Use(middleware.CORS())

    // Redis Sessions
    sessionStore, err := session.NewRedisStore(session.RedisConfig{
        Addr:   "localhost:6379",
        Prefix: "session:",
    })
    if err != nil {
        log.Fatal("Redis connection failed:", err)
    }
    defer sessionStore.Close()

    sessionConfig := session.DefaultConfig(sessionStore)
    app.Use(session.Middleware(sessionConfig))

    // Redis Cache
    redisCache, err := cache.NewRedisCache(cache.RedisConfig{
        Addr:   "localhost:6379",
        DB:     1, // Use different DB for cache
        Prefix: "cache:",
    })
    if err != nil {
        log.Fatal("Redis connection failed:", err)
    }
    defer redisCache.Close()

    // Routes
    app.POST("/login", func(c *goexpress.Context) error {
        sess, _ := session.GetSession(c)
        sess.Set("user_id", "123")
        sess.Set("username", "john")
        return c.JSON(map[string]string{"message": "Logged in"})
    })

    app.GET("/profile", func(c *goexpress.Context) error {
        sess, _ := session.GetSession(c)
        username, ok := sess.Get("username")
        if !ok {
            return goexpress.ErrUnauthorized
        }
        return c.JSON(map[string]interface{}{"username": username})
    })

    // Cached route (5 minutes)
    cacheConfig := cache.DefaultCacheConfig(redisCache)
    cacheConfig.TTL = 5 * time.Minute
    app.GET("/users", usersHandler, cache.Middleware(cacheConfig))

    app.Listen("3000")
}

func usersHandler(c *goexpress.Context) error {
    // This will be cached for 5 minutes
    return c.JSON([]string{"Alice", "Bob", "Charlie"})
}
```

## Option 2: In-Memory Sessions (No Redis Required)

Perfect for development or small applications:

```go
package main

import (
    "time"
    "github.com/abreed05/goexpress"
    "github.com/abreed05/goexpress-redis/session"
)

func main() {
    app := goexpress.New()

    // In-memory session store (no Redis needed!)
    store := session.NewMemoryStore(5 * time.Minute)
    defer store.Close()

    sessionConfig := session.DefaultConfig(store)
    app.Use(session.Middleware(sessionConfig))

    app.POST("/login", func(c *goexpress.Context) error {
        sess, _ := session.GetSession(c)
        sess.Set("user_id", "123")
        return c.JSON(map[string]string{"message": "Logged in"})
    })

    app.GET("/profile", func(c *goexpress.Context) error {
        sess, _ := session.GetSession(c)
        userID, ok := sess.Get("user_id")
        if !ok {
            return goexpress.ErrUnauthorized
        }
        return c.JSON(map[string]interface{}{"user_id": userID})
    })

    app.Listen("3000")
}
```

## Common Use Cases

### 1. User Authentication

```go
// Login
app.POST("/login", func(c *goexpress.Context) error {
    // Verify credentials...
    
    sess, _ := session.GetSession(c)
    sess.Set("user_id", user.ID)
    sess.Set("email", user.Email)
    sess.Set("role", user.Role)
    
    return c.JSON(map[string]string{"message": "Success"})
})

// Protected route
app.GET("/dashboard", func(c *goexpress.Context) error {
    sess, _ := session.GetSession(c)
    
    userID, ok := sess.Get("user_id")
    if !ok {
        return goexpress.ErrUnauthorized
    }
    
    return c.JSON(map[string]interface{}{"user_id": userID})
})

// Logout
app.POST("/logout", func(c *goexpress.Context) error {
    return session.DestroySession(c, sessionConfig)
})
```

### 2. Shopping Cart in Session

```go
app.POST("/cart/add", func(c *goexpress.Context) error {
    var item struct {
        ProductID string `json:"product_id"`
        Quantity  int    `json:"quantity"`
    }
    c.BodyParser(&item)
    
    sess, _ := session.GetSession(c)
    
    // Get existing cart
    cart := []map[string]interface{}{}
    if data, ok := sess.Get("cart"); ok {
        cart = data.([]map[string]interface{})
    }
    
    // Add item
    cart = append(cart, map[string]interface{}{
        "product_id": item.ProductID,
        "quantity":   item.Quantity,
    })
    
    sess.Set("cart", cart)
    return c.JSON(cart)
})
```

### 3. Caching Database Queries

```go
app.GET("/products/:id", func(c *goexpress.Context) error {
    id := c.Param("id")
    cacheKey := "product:" + id
    
    // Try cache first
    var product Product
    err := redisCache.Get(cacheKey, &product)
    if err == nil {
        return c.JSON(product) // Cache hit!
    }
    
    // Cache miss - fetch from database
    product = db.FindProduct(id)
    
    // Store in cache for 10 minutes
    redisCache.Set(cacheKey, product, 10*time.Minute)
    
    return c.JSON(product)
})
```

### 4. Cache Invalidation

```go
app.PUT("/products/:id", func(c *goexpress.Context) error {
    id := c.Param("id")
    
    // Update product in database...
    
    // Invalidate cache
    redisCache.Delete("product:" + id)
    
    return c.JSON(map[string]string{"message": "Updated"})
})
```

### 5. Flash Messages

```go
app.POST("/contact", func(c *goexpress.Context) error {
    // Process form...
    
    if err != nil {
        session.Flash(c, "error", "Failed to send")
        return c.Redirect("/contact")
    }
    
    session.Flash(c, "success", "Message sent!")
    return c.Redirect("/thank-you")
})

app.GET("/thank-you", func(c *goexpress.Context) error {
    // Get and remove flash message
    message, ok := session.GetFlash(c, "success")
    
    return c.JSON(map[string]interface{}{
        "message": message,
        "shown":   ok,
    })
})
```

### 6. Rate Limiting with Redis

```go
func rateLimitMiddleware(cache *cache.RedisCache) goexpress.Middleware {
    return func(next goexpress.HandlerFunc) goexpress.HandlerFunc {
        return func(c *goexpress.Context) error {
            ip := c.IP()
            key := "ratelimit:" + ip
            
            count, _ := cache.Increment(key)
            
            if count == 1 {
                cache.Expire(key, time.Minute)
            }
            
            if count > 100 {
                return c.Status(429).JSON(map[string]string{
                    "error": "Rate limit exceeded",
                })
            }
            
            return next(c)
        }
    }
}

app.Use(rateLimitMiddleware(redisCache))
```

## Configuration

### Session Configuration

```go
sessionConfig := session.Config{
    Store:        store,
    CookieName:   "session_id",
    CookiePath:   "/",
    MaxAge:       24 * time.Hour,
    Secure:       true,  // HTTPS only
    HttpOnly:     true,  // No JavaScript access
    SameSite:     http.SameSiteLaxMode,
}
```

### Cache Configuration

```go
cacheConfig := cache.CacheConfig{
    Cache:      redisCache,
    TTL:        5 * time.Minute,
    OnlyStatus: []int{200}, // Only cache 200 responses
    KeyFunc: func(c *goexpress.Context) string {
        return c.Method() + ":" + c.Path()
    },
}
```

## Running Redis

### Using Docker

```bash
docker run -d -p 6379:6379 redis:alpine
```

### Using Homebrew (Mac)

```bash
brew install redis
brew services start redis
```

### Ubuntu/Debian

```bash
sudo apt install redis-server
sudo systemctl start redis
```

## Testing

Test your Redis connection:

```bash
redis-cli ping
# Should respond: PONG
```

## Examples

Run the included examples:

```bash
# Full Redis example (requires Redis)
go run examples/redis-full/main.go

# In-memory example (no Redis needed)
go run examples/memory-session/main.go
```

## Package Structure

```
goexpress-redis/
├── session/
│   ├── store.go       # Session interface and base types
│   ├── redis.go       # Redis session store
│   └── middleware.go  # Session middleware
├── cache/
│   ├── redis.go       # Redis cache implementation
│   └── middleware.go  # Cache middleware
└── examples/
    ├── redis-full/    # Complete example with Redis
    └── memory-session/ # In-memory example
```

## Migration from Other Frameworks

### From Express.js

```javascript
// Express.js
req.session.userId = 123;
const userId = req.session.userId;
```

```go
// GoExpress
sess, _ := session.GetSession(c)
sess.Set("user_id", 123)
userID, _ := sess.Get("user_id")
```

### From Gorilla Sessions

```go
// Gorilla
session, _ := store.Get(r, "session-name")
session.Values["user_id"] = 123
session.Save(r, w)
```

```go
// GoExpress (automatic saving)
sess, _ := session.GetSession(c)
sess.Set("user_id", 123)
// Automatically saved after handler
```

## Troubleshooting

**Sessions not persisting?**
- Check Redis connection
- Verify cookie settings (Secure flag with HTTP)
- Check session TTL

**Cache not working?**
- Ensure cache middleware is applied correctly
- Verify cache key is consistent
- Check TTL hasn't expired

**Redis connection failed?**
- Make sure Redis is running: `redis-cli ping`
- Check connection settings (address, port, password)

## Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Check out the [examples](examples/) for complete applications
- Integrate with your existing GoExpress application

## Support

- GitHub Issues: Report bugs or request features
- Documentation: Check README.md for full API reference
- Examples: See working code in examples/

Happy coding! 🚀
