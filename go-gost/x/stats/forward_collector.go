package stats

import "time"

type BandwidthCalculator struct {
	interval  time.Duration
	stopChan  chan struct{}
	prevStats map[int64]previousSample
}

type previousSample struct {
	inBytes  uint64
	outBytes uint64
	time     time.Time
}

func NewBandwidthCalculator(interval time.Duration) *BandwidthCalculator {
	return &BandwidthCalculator{
		interval:  interval,
		stopChan:  make(chan struct{}),
		prevStats: make(map[int64]previousSample),
	}
}

func (c *BandwidthCalculator) Start(manager *ForwardStatsManager) {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.calculate(manager)
			case <-c.stopChan:
				return
			}
		}
	}()
}

func (c *BandwidthCalculator) Stop() {
	close(c.stopChan)
}

func (c *BandwidthCalculator) calculate(manager *ForwardStatsManager) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	now := time.Now()
	for id, stats := range manager.stats {
		stats.mu.Lock()

		prev, exists := c.prevStats[id]
		if !exists {
			c.prevStats[id] = previousSample{
				inBytes:  stats.InBytes,
				outBytes: stats.OutBytes,
				time:     now,
			}
			stats.mu.Unlock()
			continue
		}

		delta := now.Sub(prev.time).Seconds()
		if delta > 0 {
			inDelta := int64(stats.InBytes - prev.inBytes)
			outDelta := int64(stats.OutBytes - prev.outBytes)
			if inDelta < 0 {
				inDelta = 0
			}
			if outDelta < 0 {
				outDelta = 0
			}

			stats.InSpeed = uint64(float64(inDelta) / delta)
			stats.OutSpeed = uint64(float64(outDelta) / delta)
		}

		c.prevStats[id] = previousSample{
			inBytes:  stats.InBytes,
			outBytes: stats.OutBytes,
			time:     now,
		}

		stats.mu.Unlock()
	}

	for id, prev := range c.prevStats {
		if _, exists := manager.stats[id]; exists {
			continue
		}
		if time.Since(prev.time) > 5*time.Minute {
			delete(c.prevStats, id)
		}
	}
}
