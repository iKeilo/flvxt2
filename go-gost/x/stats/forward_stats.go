package stats

import (
	"sync"
	"time"
)

// ForwardStats tracks aggregated traffic for a single forward rule.
type ForwardStats struct {
	ForwardID   int64     `json:"forward_id"`
	UserID      int64     `json:"user_id"`
	TunnelID    int64     `json:"tunnel_id"`
	NodeID      int64     `json:"node_id"`
	Port        int       `json:"port"`
	ServiceName string    `json:"service_name"`
	InBytes     uint64    `json:"in_bytes"`
	OutBytes    uint64    `json:"out_bytes"`
	InSpeed     uint64    `json:"in_speed"`
	OutSpeed    uint64    `json:"out_speed"`
	Connections int       `json:"connections"`
	LastUpdate  time.Time `json:"last_update"`
	mu          sync.RWMutex
}

// ForwardMetric is the websocket-facing view of a forward rule.
type ForwardMetric struct {
	ForwardID   int64  `json:"forward_id"`
	UserID      int64  `json:"user_id"`
	TunnelID    int64  `json:"tunnel_id"`
	NodeID      int64  `json:"node_id"`
	Port        int    `json:"port"`
	ServiceName string `json:"service_name"`
	InSpeed     uint64 `json:"in_speed"`
	OutSpeed    uint64 `json:"out_speed"`
	Connections int    `json:"connections"`
}

type ForwardStatsManager struct {
	stats map[int64]*ForwardStats
	mu    sync.RWMutex
}

func NewForwardStatsManager() *ForwardStatsManager {
	return &ForwardStatsManager{
		stats: make(map[int64]*ForwardStats),
	}
}

func (m *ForwardStatsManager) GetOrCreate(forwardID, userID, tunnelID int64, serviceName string, nodeID int64, port int) *ForwardStats {
	m.mu.RLock()
	stats, ok := m.stats[forwardID]
	m.mu.RUnlock()

	if !ok {
		stats = &ForwardStats{
			ForwardID:   forwardID,
			UserID:      userID,
			TunnelID:    tunnelID,
			NodeID:      nodeID,
			Port:        port,
			ServiceName: serviceName,
			LastUpdate:  time.Now(),
		}
		m.mu.Lock()
		m.stats[forwardID] = stats
		m.mu.Unlock()
		return stats
	}

	stats.mu.Lock()
	if stats.ServiceName == "" {
		stats.ServiceName = serviceName
	}
	if stats.NodeID == 0 && nodeID > 0 {
		stats.NodeID = nodeID
	}
	if stats.Port == 0 && port > 0 {
		stats.Port = port
	}
	stats.mu.Unlock()

	return stats
}

func (m *ForwardStatsManager) AddTraffic(forwardID, userID, tunnelID int64, serviceName string, nodeID int64, port int, isInbound bool, bytes uint64) {
	stats := m.GetOrCreate(forwardID, userID, tunnelID, serviceName, nodeID, port)

	stats.mu.Lock()
	if isInbound {
		stats.InBytes += bytes
	} else {
		stats.OutBytes += bytes
	}
	stats.LastUpdate = time.Now()
	stats.mu.Unlock()
}

func (m *ForwardStatsManager) AddConnection(forwardID, userID, tunnelID int64, serviceName string, nodeID int64, port int, delta int) {
	stats := m.GetOrCreate(forwardID, userID, tunnelID, serviceName, nodeID, port)

	stats.mu.Lock()
	stats.Connections += delta
	if stats.Connections < 0 {
		stats.Connections = 0
	}
	stats.LastUpdate = time.Now()
	stats.mu.Unlock()
}

func (m *ForwardStatsManager) GetForwardMetrics() []ForwardMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]ForwardMetric, 0, len(m.stats))
	for _, stats := range m.stats {
		stats.mu.RLock()
		metrics = append(metrics, ForwardMetric{
			ForwardID:   stats.ForwardID,
			UserID:      stats.UserID,
			TunnelID:    stats.TunnelID,
			NodeID:      stats.NodeID,
			Port:        stats.Port,
			ServiceName: stats.ServiceName,
			InSpeed:     stats.InSpeed,
			OutSpeed:    stats.OutSpeed,
			Connections: stats.Connections,
		})
		stats.mu.RUnlock()
	}

	return metrics
}

func (m *ForwardStatsManager) CleanupStale(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, stats := range m.stats {
		stats.mu.RLock()
		stale := now.Sub(stats.LastUpdate) > timeout
		stats.mu.RUnlock()
		if stale {
			delete(m.stats, id)
		}
	}
}
