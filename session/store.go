package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	// ErrSessionNotFound is returned when a session is not found
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a session has expired
	ErrSessionExpired = errors.New("session expired")
)

// Store is the interface for session storage backends
type Store interface {
	// Get retrieves a session by ID
	Get(id string) (*Session, error)
	
	// Set stores a session
	Set(session *Session) error
	
	// Delete removes a session
	Delete(id string) error
	
	// Cleanup removes expired sessions
	Cleanup() error
	
	// Touch updates the last access time
	Touch(id string) error
}

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// NewSession creates a new session
func NewSession(maxAge time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:        generateSessionID(),
		Data:      make(map[string]interface{}),
		CreatedAt: now,
		ExpiresAt: now.Add(maxAge),
		UpdatedAt: now,
	}
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Set sets a value in the session
func (s *Session) Set(key string, value interface{}) {
	s.Data[key] = value
	s.UpdatedAt = time.Now()
}

// Get gets a value from the session
func (s *Session) Get(key string) (interface{}, bool) {
	val, ok := s.Data[key]
	return val, ok
}

// Delete removes a key from the session
func (s *Session) Delete(key string) {
	delete(s.Data, key)
	s.UpdatedAt = time.Now()
}

// Clear removes all data from the session
func (s *Session) Clear() {
	s.Data = make(map[string]interface{})
	s.UpdatedAt = time.Now()
}

// MemoryStore implements an in-memory session store
type MemoryStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	store := &MemoryStore{
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
	}
	
	// Start cleanup goroutine
	if cleanupInterval > 0 {
		go store.startCleanup(cleanupInterval)
	}
	
	return store
}

// Get retrieves a session
func (m *MemoryStore) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[id]
	if !exists {
		return nil, ErrSessionNotFound
	}
	
	if session.IsExpired() {
		return nil, ErrSessionExpired
	}
	
	return session, nil
}

// Set stores a session
func (m *MemoryStore) Set(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.sessions[session.ID] = session
	return nil
}

// Delete removes a session
func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.sessions, id)
	return nil
}

// Touch updates the last access time
func (m *MemoryStore) Touch(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[id]
	if !exists {
		return ErrSessionNotFound
	}
	
	session.UpdatedAt = time.Now()
	return nil
}

// Cleanup removes expired sessions
func (m *MemoryStore) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for id, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, id)
		}
	}
	
	return nil
}

// startCleanup runs periodic cleanup
func (m *MemoryStore) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// Close stops the cleanup goroutine
func (m *MemoryStore) Close() error {
	close(m.stopCh)
	return nil
}

// CookieStore implements cookie-based session storage
type CookieStore struct {
	// Cookie sessions are stored entirely in the cookie
	// This store just validates and manages cookie data
	maxAge time.Duration
}

// NewCookieStore creates a new cookie store
func NewCookieStore(maxAge time.Duration) *CookieStore {
	return &CookieStore{
		maxAge: maxAge,
	}
}

// Get decodes a session from cookie data
func (c *CookieStore) Get(cookieValue string) (*Session, error) {
	if cookieValue == "" {
		return nil, ErrSessionNotFound
	}
	
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(cookieValue)
	if err != nil {
		return nil, err
	}
	
	// Unmarshal JSON
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	
	if session.IsExpired() {
		return nil, ErrSessionExpired
	}
	
	return &session, nil
}

// Set encodes a session to cookie format
func (c *CookieStore) Set(session *Session) error {
	return nil // Cookie store doesn't need to store anything server-side
}

// Delete is a no-op for cookie store
func (c *CookieStore) Delete(id string) error {
	return nil
}

// Touch updates the session
func (c *CookieStore) Touch(id string) error {
	return nil
}

// Cleanup is a no-op for cookie store
func (c *CookieStore) Cleanup() error {
	return nil
}

// Encode encodes a session to cookie format
func (c *CookieStore) Encode(session *Session) (string, error) {
	// Marshal to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	
	// Encode to base64
	return base64.StdEncoding.EncodeToString(data), nil
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
