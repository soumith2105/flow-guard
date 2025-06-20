package config

import (
	"encoding/json"
	"net/http"

	"flowguard/internal/limiter"
	"flowguard/internal/types"

	"github.com/gorilla/mux"
)

// RESTServer provides REST API endpoints for FlowGuard configuration
type RESTServer struct {
	rateLimiter *limiter.Manager
	router      *mux.Router
}

// NewRESTServer creates a new REST API server
func NewRESTServer(rateLimiter *limiter.Manager) *RESTServer {
	server := &RESTServer{
		rateLimiter: rateLimiter,
		router:      mux.NewRouter(),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the REST API routes
func (s *RESTServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Client configuration endpoints
	api.HandleFunc("/clients", s.listClients).Methods("GET")
	api.HandleFunc("/clients", s.createClient).Methods("POST")
	api.HandleFunc("/clients/{client_id}", s.getClient).Methods("GET")
	api.HandleFunc("/clients/{client_id}", s.updateClient).Methods("PUT")
	api.HandleFunc("/clients/{client_id}", s.deleteClient).Methods("DELETE")

	// Client statistics endpoints
	api.HandleFunc("/clients/{client_id}/stats", s.getClientStats).Methods("GET")
	api.HandleFunc("/stats", s.getAllStats).Methods("GET")

	// Health check
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")

	// Add CORS middleware
	s.router.Use(corsMiddleware)
}

// ServeHTTP implements http.Handler
func (s *RESTServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// listClients returns all client configurations
func (s *RESTServer) listClients(w http.ResponseWriter, r *http.Request) {
	clients := s.rateLimiter.GetAllClients()
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	})
}

// createClient creates a new client configuration
func (s *RESTServer) createClient(w http.ResponseWriter, r *http.Request) {
	var config types.ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	if config.ClientID == "" {
		s.writeError(w, http.StatusBadRequest, "missing_field", "client_id is required")
		return
	}

	s.rateLimiter.SetClientConfig(&config)
	s.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Client configuration created successfully",
		"config":  config,
	})
}

// getClient returns a specific client configuration
func (s *RESTServer) getClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	
	config, exists := s.rateLimiter.GetClientConfig(clientID)
	if !exists {
		s.writeError(w, http.StatusNotFound, "client_not_found", "Client not found")
		return
	}

	s.writeJSON(w, http.StatusOK, config)
}

// updateClient updates a client configuration
func (s *RESTServer) updateClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	
	var config types.ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	// Ensure the client ID matches the URL parameter
	config.ClientID = clientID

	s.rateLimiter.SetClientConfig(&config)
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Client configuration updated successfully",
		"config":  config,
	})
}

// deleteClient removes a client configuration
func (s *RESTServer) deleteClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	
	if deleted := s.rateLimiter.DeleteClient(clientID); !deleted {
		s.writeError(w, http.StatusNotFound, "client_not_found", "Client not found")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Client configuration deleted successfully",
	})
}

// getClientStats returns statistics for a specific client
func (s *RESTServer) getClientStats(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	
	stats, exists := s.rateLimiter.GetClientStats(clientID)
	if !exists {
		s.writeError(w, http.StatusNotFound, "client_not_found", "Client not found")
		return
	}

	s.writeJSON(w, http.StatusOK, stats)
}

// getAllStats returns statistics for all clients
func (s *RESTServer) getAllStats(w http.ResponseWriter, r *http.Request) {
	stats := s.rateLimiter.GetAllStats()
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"stats": stats,
		"count": len(stats),
	})
}

// healthCheck returns the service health status
func (s *RESTServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "flowguard",
		"version": "1.0.0",
	})
}

// writeJSON writes a JSON response
func (s *RESTServer) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response
func (s *RESTServer) writeError(w http.ResponseWriter, statusCode int, errorType, message string) {
	s.writeJSON(w, statusCode, types.RateLimitError{
		Type:    errorType,
		Message: message,
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
} 