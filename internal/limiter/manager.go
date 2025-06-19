package limiter

import (
	"sync"
	"time"

	"flowguard/internal/types"
)

// Manager handles rate limiting for multiple clients
type Manager struct {
	clients map[string]*ClientLimiter
	stats   map[string]*types.ClientStats
	mutex   sync.RWMutex
}

// ClientLimiter holds the rate limiting state for a single client
type ClientLimiter struct {
	config    *types.ClientConfig
	rpmBucket *types.TokenBucket
	tpmBucket *types.TokenBucket
	mutex     sync.RWMutex
}

// NewManager creates a new rate limiter manager
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*ClientLimiter),
		stats:   make(map[string]*types.ClientStats),
	}
}

// CheckAndConsume checks if a request can proceed and consumes tokens if allowed
func (m *Manager) CheckAndConsume(clientID string, tokenEstimate int64) error {
	m.mutex.RLock()
	client, exists := m.clients[clientID]
	_, statsExists := m.stats[clientID]
	m.mutex.RUnlock()

	if !exists {
		// Auto-create client with no limits if not configured
		m.SetClientConfig(&types.ClientConfig{
			ClientID: clientID,
			Enabled:  true,
		})
		m.mutex.RLock()
		client = m.clients[clientID]
		m.mutex.RUnlock()
	}

	if !statsExists {
		m.mutex.Lock()
		if _, exists := m.stats[clientID]; !exists {
			m.stats[clientID] = &types.ClientStats{
				ClientID:        clientID,
				LastRequestTime: time.Now(),
			}
		}
		m.mutex.Unlock()
	}

	client.mutex.RLock()
	config := client.config
	client.mutex.RUnlock()

	if !config.Enabled {
		// Rate limiting disabled for this client
		m.updateSuccessStats(clientID, tokenEstimate)
		return nil
	}

	// Check RPM limit
	if config.RPM != nil && client.rpmBucket != nil {
		if !client.rpmBucket.TryConsume(1) {
			m.updateDroppedStats(clientID, "rpm")
			return types.ErrRPMExceeded
		}
	}

	// Check TPM limit
	if config.TPM != nil && client.tpmBucket != nil {
		if !client.tpmBucket.TryConsume(tokenEstimate) {
			// Refund the RPM token if TPM check fails
			if config.RPM != nil && client.rpmBucket != nil {
				// Note: In a real implementation, you might want to handle this differently
				// as we can't easily "refund" tokens to a bucket
			}
			m.updateDroppedStats(clientID, "tpm")
			return types.ErrTPMExceeded
		}
	}

	m.updateSuccessStats(clientID, tokenEstimate)
	return nil
}

// SetClientConfig updates or creates a client configuration
func (m *Manager) SetClientConfig(config *types.ClientConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var rpmBucket, tpmBucket *types.TokenBucket

	if config.RPM != nil && *config.RPM > 0 {
		rpmBucket = types.NewTokenBucket(*config.RPM, *config.RPM)
	}

	if config.TPM != nil && *config.TPM > 0 {
		tpmBucket = types.NewTokenBucket(*config.TPM, *config.TPM)
	}

	m.clients[config.ClientID] = &ClientLimiter{
		config:    config,
		rpmBucket: rpmBucket,
		tpmBucket: tpmBucket,
	}

	// Initialize stats if not exists
	if _, exists := m.stats[config.ClientID]; !exists {
		m.stats[config.ClientID] = &types.ClientStats{
			ClientID:        config.ClientID,
			LastRequestTime: time.Now(),
		}
	}
}

// GetClientConfig returns the configuration for a client
func (m *Manager) GetClientConfig(clientID string) (*types.ClientConfig, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	client, exists := m.clients[clientID]
	if !exists {
		return nil, false
	}

	client.mutex.RLock()
	defer client.mutex.RUnlock()

	return client.config, true
}

// GetClientStats returns the statistics for a client
func (m *Manager) GetClientStats(clientID string) (*types.ClientStats, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats, exists := m.stats[clientID]
	if !exists {
		return nil, false
	}

	// Get current bucket levels
	if client, clientExists := m.clients[clientID]; clientExists {
		client.mutex.RLock()
		if client.rpmBucket != nil {
			stats.RPMRemaining = client.rpmBucket.GetRemainingTokens()
		}
		if client.tpmBucket != nil {
			stats.TPMRemaining = client.tpmBucket.GetRemainingTokens()
		}
		client.mutex.RUnlock()
	}

	return stats, true
}

// GetAllClients returns all client configurations
func (m *Manager) GetAllClients() map[string]*types.ClientConfig {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.ClientConfig)
	for clientID, client := range m.clients {
		client.mutex.RLock()
		result[clientID] = client.config
		client.mutex.RUnlock()
	}

	return result
}

// GetAllStats returns all client statistics
func (m *Manager) GetAllStats() map[string]*types.ClientStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.ClientStats)
	for clientID, stats := range m.stats {
		// Update current bucket levels
		if client, exists := m.clients[clientID]; exists {
			client.mutex.RLock()
			if client.rpmBucket != nil {
				stats.RPMRemaining = client.rpmBucket.GetRemainingTokens()
			}
			if client.tpmBucket != nil {
				stats.TPMRemaining = client.tpmBucket.GetRemainingTokens()
			}
			client.mutex.RUnlock()
		}
		result[clientID] = stats
	}

	return result
}

// DeleteClient removes a client configuration
func (m *Manager) DeleteClient(clientID string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	_, exists := m.clients[clientID]
	if exists {
		delete(m.clients, clientID)
		delete(m.stats, clientID)
	}

	return exists
}

// updateSuccessStats updates statistics for a successful request
func (m *Manager) updateSuccessStats(clientID string, tokens int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats := m.stats[clientID]
	stats.TotalRequests++
	stats.SuccessRequests++
	stats.TokensUsed += tokens
	stats.LastRequestTime = time.Now()
}

// updateDroppedStats updates statistics for a dropped request
func (m *Manager) updateDroppedStats(clientID string, reason string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats := m.stats[clientID]
	stats.TotalRequests++
	stats.DroppedRequests++
	stats.LastRequestTime = time.Now()

	switch reason {
	case "rpm":
		stats.RPMDropped++
	case "tpm":
		stats.TPMDropped++
	}
}

// UpdateLatency updates the average latency for a client
func (m *Manager) UpdateLatency(clientID string, latencyMs float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats, exists := m.stats[clientID]
	if !exists {
		return
	}

	// Simple moving average (could be improved with exponential moving average)
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		stats.AvgLatencyMs = (stats.AvgLatencyMs + latencyMs) / 2
	}
} 