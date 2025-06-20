package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"flowguard/internal/limiter"
	"flowguard/internal/types"
)

// Handler handles HTTP requests with rate limiting and proxying
type Handler struct {
	rateLimiter *limiter.Manager
	upstream    *httputil.ReverseProxy
	upstreamURL *url.URL
}

// NewHandler creates a new proxy handler
func NewHandler(upstreamURL string, rateLimiter *limiter.Manager) (*Handler, error) {
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)
	
	// Customize the proxy to preserve headers and handle errors
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Add CORS headers if needed
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	return &Handler{
		rateLimiter: rateLimiter,
		upstream:    proxy,
		upstreamURL: parsedURL,
	}, nil
}

// ServeHTTP handles incoming HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Extract required headers
	clientID := r.Header.Get("X-Client-ID")
	tokenEstimateStr := r.Header.Get("X-Token-Estimate")

	// Validate headers
	if clientID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "missing_header", "X-Client-ID header is required")
		return
	}

	if tokenEstimateStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "missing_header", "X-Token-Estimate header is required")
		return
	}

	tokenEstimate, err := strconv.ParseInt(tokenEstimateStr, 10, 64)
	if err != nil || tokenEstimate < 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid_header", "X-Token-Estimate must be a non-negative integer")
		return
	}

	// Check rate limits
	if err := h.rateLimiter.CheckAndConsume(clientID, tokenEstimate); err != nil {
		if rateLimitErr, ok := err.(types.RateLimitError); ok {
			h.writeErrorResponse(w, http.StatusTooManyRequests, rateLimitErr.Type, rateLimitErr.Message)
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	// Create a custom response writer to capture status code
	wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Forward the request to upstream
	h.upstream.ServeHTTP(wrappedWriter, r)

	// Update latency metrics
	latency := time.Since(startTime)
	h.rateLimiter.UpdateLatency(clientID, float64(latency.Milliseconds()))

	log.Printf("Request from client %s: %s %s - %d (%v)", 
		clientID, r.Method, r.URL.Path, wrappedWriter.statusCode, latency)
}

// writeErrorResponse writes a JSON error response
func (h *Handler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorResp := types.RateLimitError{
		Type:    errorType,
		Message: message,
	}
	
	json.NewEncoder(w).Encode(errorResp)
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
} 