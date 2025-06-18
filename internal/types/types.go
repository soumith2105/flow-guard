package types

import (
	"sync"
	"time"
)

// ClientConfig holds the rate limiting configuration for a specific client
type ClientConfig struct {
	ClientID string  `json:"client_id"`
	RPM      *int64  `json:"rpm,omitempty"`      // Requests per minute (nil means no limit)
	TPM      *int64  `json:"tpm,omitempty"`      // Tokens per minute (nil means no limit)
	Enabled  bool    `json:"enabled"`            // Whether rate limiting is enabled for this client
}

// ClientStats holds runtime statistics for a client
type ClientStats struct {
	ClientID         string    `json:"client_id"`
	TotalRequests    int64     `json:"total_requests"`
	SuccessRequests  int64     `json:"success_requests"`
	DroppedRequests  int64     `json:"dropped_requests"`
	RPMDropped       int64     `json:"rpm_dropped"`
	TPMDropped       int64     `json:"tpm_dropped"`
	TokensUsed       int64     `json:"tokens_used"`
	RPMRemaining     int64     `json:"rpm_remaining"`
	TPMRemaining     int64     `json:"tpm_remaining"`
	LastRequestTime  time.Time `json:"last_request_time"`
	AvgLatencyMs     float64   `json:"avg_latency_ms"`
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	capacity     int64
	tokens       float64
	refillRate   float64       // tokens per second
	lastRefill   time.Time
	mutex        sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity int64, refillPerMinute int64) *TokenBucket {
	refillRate := float64(refillPerMinute) / 60.0 // convert to per second
	return &TokenBucket{
		capacity:   capacity,
		tokens:     float64(capacity),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// TryConsume attempts to consume the specified number of tokens
// Returns true if successful, false if insufficient tokens
func (tb *TokenBucket) TryConsume(tokens int64) bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	tb.refill()
	
	if tb.tokens >= float64(tokens) {
		tb.tokens -= float64(tokens)
		return true
	}
	return false
}

// GetRemainingTokens returns the current number of tokens in the bucket
func (tb *TokenBucket) GetRemainingTokens() int64 {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	
	tb.refill()
	return int64(tb.tokens)
}

// refill adds tokens to the bucket based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	
	if elapsed > 0 {
		tokensToAdd := elapsed * tb.refillRate
		tb.tokens = min(tb.tokens + tokensToAdd, float64(tb.capacity))
		tb.lastRefill = now
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// RateLimitError represents different types of rate limit violations
type RateLimitError struct {
	Type    string `json:"error"`
	Message string `json:"message"`
}

func (e RateLimitError) Error() string {
	return e.Message
}

// Rate limit error types
var (
	ErrRPMExceeded = RateLimitError{Type: "rpm_exceeded", Message: "Request rate limit exceeded"}
	ErrTPMExceeded = RateLimitError{Type: "tpm_exceeded", Message: "Token rate limit exceeded"}
	ErrClientNotFound = RateLimitError{Type: "client_not_found", Message: "Client not configured"}
) 