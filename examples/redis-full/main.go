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
	app := goexpress.New(&goexpress.Config{
		Port: "3000",
	})

	// Global middleware
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())

	// Initialize Redis session store
	sessionStore, err := session.NewRedisStore(session.RedisConfig{
		Addr:     "localhost:6379",
		Password: "", // Set if Redis has authentication
		DB:       0,
		Prefix:   "session:",
	})
	if err != nil {
		log.Fatal("Failed to connect to Redis for sessions:", err)
	}
	defer sessionStore.Close()

	// Session middleware
	sessionConfig := session.DefaultConfig(sessionStore)
	sessionConfig.MaxAge = 24 * time.Hour
	sessionConfig.Secure = false // Set to true in production with HTTPS
	app.Use(session.Middleware(sessionConfig))

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(cache.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1, // Use different DB for cache
		Prefix:   "cache:",
	})
	if err != nil {
		log.Fatal("Failed to connect to Redis for cache:", err)
	}
	defer redisCache.Close()

	// Routes
	app.GET("/", homeHandler)

	// Session examples
	app.POST("/login", loginHandler)
	app.GET("/profile", profileHandler)
	app.POST("/logout", func(c *goexpress.Context) error {
		return session.DestroySession(c, sessionConfig)
	})

	// Cache examples
	cacheConfig := cache.DefaultCacheConfig(redisCache)
	cacheConfig.TTL = 5 * time.Minute

	// Cached route - will cache for 5 minutes
	app.GET("/users", usersHandler, cache.Middleware(cacheConfig))

	// Manual cache usage
	app.GET("/products/:id", func(c *goexpress.Context) error {
		return getProductHandler(c, redisCache)
	})

	// Cache invalidation
	app.POST("/products/:id", func(c *goexpress.Context) error {
		return updateProductHandler(c, redisCache)
	})

	// Flash messages example
	app.POST("/submit", submitHandler)
	app.GET("/result", resultHandler)

	// Session counter example
	app.GET("/counter", counterHandler)

	log.Println("🚀 Server starting on http://localhost:3000")
	log.Println("📝 Make sure Redis is running on localhost:6379")
	log.Println("\nEndpoints:")
	log.Println("  POST /login - Login and create session")
	log.Println("  GET  /profile - View profile (requires session)")
	log.Println("  POST /logout - Logout and destroy session")
	log.Println("  GET  /users - List users (cached)")
	log.Println("  GET  /products/:id - Get product (cached)")
	log.Println("  POST /products/:id - Update product (invalidates cache)")
	log.Println("  GET  /counter - Increment session counter")

	if err := app.Listen(); err != nil {
		log.Fatal(err)
	}
}

func homeHandler(c *goexpress.Context) error {
	return c.JSON(map[string]interface{}{
		"message": "GoExpress with Redis Sessions and Cache",
		"endpoints": map[string]string{
			"login":    "POST /login",
			"profile":  "GET /profile",
			"logout":   "POST /logout",
			"users":    "GET /users (cached)",
			"products": "GET /products/:id (cached)",
			"counter":  "GET /counter (session)",
		},
	})
}

func loginHandler(c *goexpress.Context) error {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&credentials); err != nil {
		return goexpress.NewHTTPError(400, "Invalid request")
	}

	// Simple authentication (use proper validation in production)
	if credentials.Username == "" || credentials.Password == "" {
		return goexpress.NewHTTPError(400, "Username and password required")
	}

	// Get session
	sess, err := session.GetSession(c)
	if err != nil {
		return err
	}

	// Store user data in session
	sess.Set("user_id", "12345")
	sess.Set("username", credentials.Username)
	sess.Set("logged_in", true)
	sess.Set("login_time", time.Now().Format(time.RFC3339))

	return c.JSON(map[string]interface{}{
		"message": "Login successful",
		"user": map[string]interface{}{
			"username": credentials.Username,
		},
	})
}

func profileHandler(c *goexpress.Context) error {
	sess, err := session.GetSession(c)
	if err != nil {
		return goexpress.ErrUnauthorized
	}

	loggedIn, ok := sess.Get("logged_in")
	if !ok || !loggedIn.(bool) {
		return goexpress.ErrUnauthorized
	}

	username, _ := sess.Get("username")
	userID, _ := sess.Get("user_id")
	loginTime, _ := sess.Get("login_time")

	return c.JSON(map[string]interface{}{
		"user_id":    userID,
		"username":   username,
		"login_time": loginTime,
		"session_id": sess.ID,
	})
}

func usersHandler(c *goexpress.Context) error {
	// This response will be cached for 5 minutes
	users := []map[string]interface{}{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": "bob@example.com"},
		{"id": 3, "name": "Charlie", "email": "charlie@example.com"},
	}

	return c.JSON(map[string]interface{}{
		"users":      users,
		"cached_at":  time.Now().Format(time.RFC3339),
		"message":    "This response is cached for 5 minutes",
	})
}

func getProductHandler(c *goexpress.Context, cache *cache.RedisCache) error {
	productID := c.Param("id")
	cacheKey := "product:" + productID

	// Try to get from cache
	var product map[string]interface{}
	err := cache.Get(cacheKey, &product)
	
	if err == nil {
		// Cache hit
		product["from_cache"] = true
		return c.JSON(product)
	}

	// Cache miss - fetch from "database"
	product = map[string]interface{}{
		"id":          productID,
		"name":        "Product " + productID,
		"price":       99.99,
		"description": "This is a sample product",
		"from_cache":  false,
		"cached_at":   time.Now().Format(time.RFC3339),
	}

	// Store in cache for 10 minutes
	cache.Set(cacheKey, product, 10*time.Minute)

	return c.JSON(product)
}

func updateProductHandler(c *goexpress.Context, cache *cache.RedisCache) error {
	productID := c.Param("id")

	var input struct {
		Name        string  `json:"name"`
		Price       float64 `json:"price"`
		Description string  `json:"description"`
	}

	if err := c.BodyParser(&input); err != nil {
		return goexpress.NewHTTPError(400, "Invalid request")
	}

	// Update product (database operation)
	// ...

	// Invalidate cache
	cacheKey := "product:" + productID
	cache.Delete(cacheKey)

	return c.JSON(map[string]interface{}{
		"message":       "Product updated",
		"cache_cleared": true,
	})
}

func submitHandler(c *goexpress.Context) error {
	var input struct {
		Data string `json:"data"`
	}

	if err := c.BodyParser(&input); err != nil {
		return goexpress.NewHTTPError(400, "Invalid request")
	}

	// Store flash message
	session.Flash(c, "success", "Data submitted successfully!")
	session.Flash(c, "data", input.Data)

	return c.JSON(map[string]interface{}{
		"message": "Data submitted. Check /result for flash message",
	})
}

func resultHandler(c *goexpress.Context) error {
	// Get and remove flash messages
	success, hasSuccess := session.GetFlash(c, "success")
	data, hasData := session.GetFlash(c, "data")

	if !hasSuccess {
		return c.JSON(map[string]interface{}{
			"message": "No flash messages. Submit data first at POST /submit",
		})
	}

	return c.JSON(map[string]interface{}{
		"flash_message": success,
		"data":          data,
		"note":          "Flash messages are one-time only. Refresh to see they're gone.",
	})
}

func counterHandler(c *goexpress.Context) error {
	sess, err := session.GetSession(c)
	if err != nil {
		return err
	}

	// Get current counter value
	counter := 0
	if val, ok := sess.Get("counter"); ok {
		counter = int(val.(float64))
	}

	// Increment
	counter++
	sess.Set("counter", counter)

	return c.JSON(map[string]interface{}{
		"counter":    counter,
		"session_id": sess.ID,
		"message":    "Counter is stored in your session. Refresh to increment!",
	})
}
