package session

import (
	"net/http"
	"time"

	"github.com/abreed05/goexpress"
)

// Config holds session middleware configuration
type Config struct {
	Store        Store
	CookieName   string
	CookiePath   string
	CookieDomain string
	MaxAge       time.Duration
	Secure       bool
	HttpOnly     bool
	SameSite     http.SameSite
	ContextKey   string
}

// DefaultConfig returns a default session configuration
func DefaultConfig(store Store) Config {
	return Config{
		Store:      store,
		CookieName: "session_id",
		CookiePath: "/",
		MaxAge:     24 * time.Hour,
		HttpOnly:   true,
		Secure:     false,
		SameSite:   http.SameSiteLaxMode,
		ContextKey: "session",
	}
}

// Middleware returns a session middleware for GoExpress
func Middleware(config Config) goexpress.Middleware {
	if config.Store == nil {
		panic("session store is required")
	}

	if config.CookieName == "" {
		config.CookieName = "session_id"
	}

	if config.ContextKey == "" {
		config.ContextKey = "session"
	}

	if config.MaxAge == 0 {
		config.MaxAge = 24 * time.Hour
	}

	return func(next goexpress.HandlerFunc) goexpress.HandlerFunc {
		return func(c *goexpress.Context) error {
			var session *Session

			// Try to get existing session from cookie
			cookie, err := c.GetCookie(config.CookieName)
			if err == nil && cookie.Value != "" {
				session, err = config.Store.Get(cookie.Value)
				if err != nil && err != ErrSessionNotFound && err != ErrSessionExpired {
					// Log error but continue with new session
					session = nil
				}
			}

			// Create new session if none exists
			if session == nil {
				session = NewSession(config.MaxAge)
				if err := config.Store.Set(session); err != nil {
					return err
				}
			} else {
				// Touch existing session to update last access time
				config.Store.Touch(session.ID)
			}

			// Store session in context
			c.Set(config.ContextKey, session)
			c.Set("session_id", session.ID)

			// Execute handler
			err = next(c)

			// Save session after handler execution
			if sessionData, ok := c.Get(config.ContextKey); ok {
				if sess, ok := sessionData.(*Session); ok {
					// Update expiration time
					sess.ExpiresAt = time.Now().Add(config.MaxAge)
					
					if err := config.Store.Set(sess); err != nil {
						return err
					}

					// Set cookie
					c.Cookie(&http.Cookie{
						Name:     config.CookieName,
						Value:    sess.ID,
						Path:     config.CookiePath,
						Domain:   config.CookieDomain,
						MaxAge:   int(config.MaxAge.Seconds()),
						Secure:   config.Secure,
						HttpOnly: config.HttpOnly,
						SameSite: config.SameSite,
					})
				}
			}

			return err
		}
	}
}

// GetSession retrieves the session from the context
func GetSession(c *goexpress.Context) (*Session, error) {
	if session, ok := c.Get("session"); ok {
		if sess, ok := session.(*Session); ok {
			return sess, nil
		}
	}
	return nil, ErrSessionNotFound
}

// DestroySession removes the session
func DestroySession(c *goexpress.Context, config Config) error {
	session, err := GetSession(c)
	if err != nil {
		return err
	}

	// Delete from store
	if err := config.Store.Delete(session.ID); err != nil {
		return err
	}

	// Clear cookie
	c.Cookie(&http.Cookie{
		Name:     config.CookieName,
		Value:    "",
		Path:     config.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
	})

	return nil
}

// RegenerateSession creates a new session ID and migrates data
func RegenerateSession(c *goexpress.Context, config Config) error {
	oldSession, err := GetSession(c)
	if err != nil {
		return err
	}

	// Create new session with old data
	newSession := NewSession(config.MaxAge)
	newSession.Data = oldSession.Data

	// Save new session
	if err := config.Store.Set(newSession); err != nil {
		return err
	}

	// Delete old session
	config.Store.Delete(oldSession.ID)

	// Update context
	c.Set(config.ContextKey, newSession)
	c.Set("session_id", newSession.ID)

	// Set new cookie
	c.Cookie(&http.Cookie{
		Name:     config.CookieName,
		Value:    newSession.ID,
		Path:     config.CookiePath,
		Domain:   config.CookieDomain,
		MaxAge:   int(config.MaxAge.Seconds()),
		Secure:   config.Secure,
		HttpOnly: config.HttpOnly,
		SameSite: config.SameSite,
	})

	return nil
}

// Flash adds a one-time message to the session
func Flash(c *goexpress.Context, key string, value interface{}) error {
	session, err := GetSession(c)
	if err != nil {
		return err
	}

	flashKey := "_flash_" + key
	session.Set(flashKey, value)
	return nil
}

// GetFlash retrieves and removes a flash message
func GetFlash(c *goexpress.Context, key string) (interface{}, bool) {
	session, err := GetSession(c)
	if err != nil {
		return nil, false
	}

	flashKey := "_flash_" + key
	value, ok := session.Get(flashKey)
	if ok {
		session.Delete(flashKey)
	}
	return value, ok
}
