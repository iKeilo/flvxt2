package stats

import "time"

var (
	GlobalForwardStatsManager *ForwardStatsManager
	GlobalBandwidthCalculator *BandwidthCalculator
)

func Init() {
	GlobalForwardStatsManager = NewForwardStatsManager()
	GlobalBandwidthCalculator = NewBandwidthCalculator(time.Second)
	GlobalBandwidthCalculator.Start(GlobalForwardStatsManager)

	go cleanupStaleStats(5 * time.Minute)
}

func cleanupStaleStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if GlobalForwardStatsManager != nil {
			GlobalForwardStatsManager.CleanupStale(5 * time.Minute)
		}
	}
}

func GetForwardStatsManager() *ForwardStatsManager {
	return GlobalForwardStatsManager
}

func AddForwardTraffic(forwardID, userID, tunnelID int64, serviceName string, nodeID int64, port int, isInbound bool, bytes uint64) {
	if GlobalForwardStatsManager != nil {
		GlobalForwardStatsManager.AddTraffic(forwardID, userID, tunnelID, serviceName, nodeID, port, isInbound, bytes)
	}
}

func AddForwardConnection(forwardID, userID, tunnelID int64, serviceName string, nodeID int64, port int, delta int) {
	if GlobalForwardStatsManager != nil {
		GlobalForwardStatsManager.AddConnection(forwardID, userID, tunnelID, serviceName, nodeID, port, delta)
	}
}
