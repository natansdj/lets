package lets

import (
	"sync"
	"time"
)

// RateLimiter provides thread-safe rate limiting for operations
type RateLimiter struct {
	mu          sync.Mutex
	lastLog     map[string]time.Time
	minInterval time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified minimum interval between operations
func NewRateLimiter(minInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		lastLog:     make(map[string]time.Time),
		minInterval: minInterval,
	}
}

// Allow checks if an operation with the given key is allowed based on the rate limit
// Returns true if enough time has passed since the last operation with this key
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	lastTime, exists := rl.lastLog[key]

	if !exists || now.Sub(lastTime) >= rl.minInterval {
		rl.lastLog[key] = now
		return true
	}

	return false
}

// Reset clears the rate limiter history for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.lastLog, key)
}

// ResetAll clears all rate limiter history
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.lastLog = make(map[string]time.Time)
}

// Global rate limiter for logging with default interval
var LogRateLimiter = NewRateLimiter(30 * time.Second)

// LogERL (Log Error with Rate Limit) logs an error message with rate limiting
// key: unique identifier for this log type (e.g., "rabbitmq-channel-closed")
// format: printf-style format string
// args: arguments for the format string
func LogERL(key string, format string, args ...interface{}) {
	if LogRateLimiter.Allow(key) {
		LogE(format, args...)
	}
}

// LogWRL (Log Warning with Rate Limit) logs a warning message with rate limiting
func LogWRL(key string, format string, args ...interface{}) {
	if LogRateLimiter.Allow(key) {
		LogW(format, args...)
	}
}

// LogIRL (Log Info with Rate Limit) logs an info message with rate limiting
func LogIRL(key string, format string, args ...interface{}) {
	if LogRateLimiter.Allow(key) {
		LogI(format, args...)
	}
}
