package main

import (
	"log"
	"time"

	"github.com/abreed05/goexpress"
	"github.com/abreed05/goexpress/middleware"
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

	// Initialize in-memory session store (no Redis needed!)
	sessionStore := session.NewMemoryStore(5 * time.Minute) // Cleanup every 5 minutes
	defer sessionStore.Close()

	// Session middleware
	sessionConfig := session.DefaultConfig(sessionStore)
	sessionConfig.MaxAge = 30 * time.Minute
	app.Use(session.Middleware(sessionConfig))

	// Routes
	app.GET("/", func(c *goexpress.Context) error {
		return c.JSON(map[string]string{
			"message": "In-Memory Session Example",
			"note":    "No Redis required!",
		})
	})

	app.POST("/login", func(c *goexpress.Context) error {
		var creds struct {
			Username string `json:"username"`
		}

		if err := c.BodyParser(&creds); err != nil {
			return goexpress.NewHTTPError(400, "Invalid request")
		}

		sess, _ := session.GetSession(c)
		sess.Set("username", creds.Username)
		sess.Set("logged_in", true)

		return c.JSON(map[string]interface{}{
			"message":    "Login successful",
			"username":   creds.Username,
			"session_id": sess.ID,
		})
	})

	app.GET("/profile", func(c *goexpress.Context) error {
		sess, err := session.GetSession(c)
		if err != nil {
			return goexpress.ErrUnauthorized
		}

		username, ok := sess.Get("username")
		if !ok {
			return goexpress.NewHTTPError(401, "Not logged in")
		}

		return c.JSON(map[string]interface{}{
			"username":   username,
			"session_id": sess.ID,
			"created_at": sess.CreatedAt,
			"expires_at": sess.ExpiresAt,
		})
	})

	app.POST("/logout", func(c *goexpress.Context) error {
		return session.DestroySession(c, sessionConfig)
	})

	log.Println("🚀 Server starting on http://localhost:3000")
	log.Println("💾 Using in-memory sessions (no Redis needed)")
	if err := app.Listen(); err != nil {
		log.Fatal(err)
	}
}
