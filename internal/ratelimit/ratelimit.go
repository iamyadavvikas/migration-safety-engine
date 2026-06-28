// Package ratelimit provides HTTP middleware for API rate limiting.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Config holds rate limiting configuration.
type Config struct {
	RequestsPerSecond float64       // Max requests per second (default: 10)
	BurstSize         int           // Max burst size (default: 20)
	CleanupInterval   time.Duration // How often to clean up (default: 10s)
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond: 10,
		BurstSize:         20,
		CleanupInterval:   10 * time.Second,
	}
}

// Limiter tracks request rates per client.
type Limiter struct {
	config  Config
	clients map[string]*client
	mu      sync.RWMutex
}

type client struct {
	tokens    float64
	lastSeen  time.Time
}

// NewLimiter creates a new rate limiter.
func NewLimiter(cfg Config) *Limiter {
	l := &Limiter{
		config:  cfg,
		clients: make(map[string]*client),
	}
	go l.cleanup()
	return l
}

// Allow checks if a request from the given client is allowed.
func (l *Limiter) Allow(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	c, exists := l.clients[clientID]
	if !exists {
		l.clients[clientID] = &client{
			tokens:   float64(l.config.BurstSize) - 1,
			lastSeen: time.Now(),
		}
		return true
	}

	// Refill tokens based on time passed
	elapsed := time.Since(c.lastSeen).Seconds()
	c.tokens += elapsed * l.config.RequestsPerSecond
	if c.tokens > float64(l.config.BurstSize) {
		c.tokens = float64(l.config.BurstSize)
	}
	c.lastSeen = time.Now()

	if c.tokens < 1 {
		return false
	}

	c.tokens--
	return true
}

// cleanup removes stale clients periodically.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		for id, c := range l.clients {
			if time.Since(c.lastSeen) > time.Minute {
				delete(l.clients, id)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use IP + User-Agent as client identifier
		clientID := r.RemoteAddr + "|" + r.UserAgent()

		if !l.Allow(clientID) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limit exceeded", "retry_after": 1}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
