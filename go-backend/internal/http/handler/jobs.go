package handler

import (
	"context"
	"log"
	"strings"
	"time"
)

func (h *Handler) StartBackgroundJobs() {
	if h == nil || h.repo == nil {
		return
	}

	h.jobsMu.Lock()
	if h.jobsStarted {
		h.jobsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.jobsCancel = cancel
	h.jobsStarted = true
	h.jobsWG.Add(7)
	h.jobsMu.Unlock()

	go h.runHourlyStatsLoop(ctx)
	go h.runDailyMaintenanceLoop(ctx)
	go h.runNodeRenewalCycleLoop(ctx)
	go h.runMetricsIngestion(ctx)
	go h.runHealthChecks(ctx)
	go h.runTunnelQualityProber(ctx)
	go h.runNftablesDomainRefreshLoop(ctx)
}

func (h *Handler) StopBackgroundJobs() {
	if h == nil {
		return
	}

	h.jobsMu.Lock()
	if !h.jobsStarted {
		h.jobsMu.Unlock()
		return
	}
	cancel := h.jobsCancel
	h.jobsCancel = nil
	h.jobsStarted = false
	h.jobsMu.Unlock()

	if cancel != nil {
		cancel()
	}
	h.jobsWG.Wait()
}

func (h *Handler) runMetricsIngestion(ctx context.Context) {
	defer h.jobsWG.Done()
	if h.metrics != nil {
		h.metrics.Start(ctx)
	}
}

func (h *Handler) runHealthChecks(ctx context.Context) {
	defer h.jobsWG.Done()
	if h.healthCheck != nil {
		h.healthCheck.Start(ctx)
	}
}

func (h *Handler) runTunnelQualityProber(ctx context.Context) {
	defer h.jobsWG.Done()
	if h == nil || h.qualityProber == nil {
		return
	}

	h.qualityProber.Start(ctx)
}

func (h *Handler) runHourlyStatsLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextHour(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runStatisticsFlowJob(time.Now())
		}
	}
}

func (h *Handler) runDailyMaintenanceLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextDailyMaintenance(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runResetAndExpiryJob(time.Now())
		}
	}
}

func durationUntilNextHour(now time.Time) time.Duration {
	next := now.Truncate(time.Hour).Add(time.Hour)
	return next.Sub(now)
}

func durationUntilNextDailyMaintenance(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 5, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (h *Handler) runStatisticsFlowJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	nowMs := now.UnixMilli()
	cutoffMs := nowMs - int64((48*time.Hour)/time.Millisecond)
	_ = h.repo.PurgeOldStatisticsFlows(cutoffMs)

	hourMark := now.Truncate(time.Hour)
	hourText := hourMark.Format("15:04")
	createdTime := hourMark.UnixMilli()

	users, err := h.repo.ListAllUserFlowSnapshots()
	if err != nil {
		return
	}

	for _, user := range users {
		currentTotal := user.InFlow + user.OutFlow
		increment := currentTotal

		lastTotal, err := h.repo.GetLastStatisticsFlowTotal(user.UserID)
		if err == nil && lastTotal.Valid {
			increment = currentTotal - lastTotal.Int64
			if increment < 0 {
				increment = currentTotal
			}
		}

		_ = h.repo.CreateStatisticsFlow(user.UserID, increment, currentTotal, hourText, createdTime)
	}
}

func (h *Handler) runResetAndExpiryJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	h.resetMonthlyFlow(now)
	h.resetUserQuotaWindows(now)
	h.disableExpiredUsers(now.UnixMilli())
	h.disableExpiredUserTunnels(now.UnixMilli())
}

func (h *Handler) resetMonthlyFlow(now time.Time) {
	currentDay := now.Day()
	lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()

	_ = h.repo.ResetUserMonthlyFlow(currentDay, lastDay)
	_ = h.repo.ResetUserTunnelMonthlyFlow(currentDay, lastDay)
}

func (h *Handler) disableExpiredUsers(nowMs int64) {
	userIDs, err := h.repo.ListExpiredActiveUserIDs(nowMs)
	if err != nil {
		return
	}

	for _, userID := range userIDs {
		forwards, err := h.listActiveForwardsByUser(userID)
		if err == nil {
			h.pauseForwardRecords(forwards, nowMs)
		}
		_ = h.repo.DisableUser(userID)
	}
}

func (h *Handler) disableExpiredUserTunnels(nowMs int64) {
	items, err := h.repo.ListExpiredActiveUserTunnels(nowMs)
	if err != nil {
		return
	}

	for _, item := range items {
		forwards, err := h.listActiveForwardsByUserTunnel(item.UserID, item.TunnelID)
		if err == nil {
			h.pauseForwardRecords(forwards, nowMs)
		}
		_ = h.repo.DisableUserTunnel(item.ID)
	}
}

func (h *Handler) runNodeRenewalCycleLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextNodeRenewalCycle(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runNodeRenewalCycleJob(time.Now())
		}
	}
}

func durationUntilNextNodeRenewalCycle(now time.Time) time.Duration {
	next := now.Truncate(6 * time.Hour).Add(6 * time.Hour)
	return next.Sub(now)
}

func (h *Handler) runNodeRenewalCycleJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	advanced, err := h.repo.AdvanceNodeRenewalCycles(now.UnixMilli())
	if err != nil {
		return
	}

	_ = advanced
}

func (h *Handler) runNftablesDomainRefreshLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Minute):
			h.runNftablesDomainRefreshJob()
		}
	}
}

func (h *Handler) runNftablesDomainRefreshJob() {
	if h == nil || h.repo == nil {
		return
	}

	forwards, err := h.repo.ListActiveNftablesForwards()
	if err != nil {
		log.Printf("[nftables-dns] list active nftables forwards failed: %v", err)
		return
	}
	if len(forwards) == 0 {
		return
	}

	h.nftablesDomainMu.Lock()
	defer h.nftablesDomainMu.Unlock()

	seen := make(map[int64]struct{}, len(forwards))
	for _, f := range forwards {
		seen[f.ID] = struct{}{}

		targets := splitRemoteTargets(f.RemoteAddr)
		resolvedTargets := make([]string, len(targets))
		hasDomain := false
		for i, t := range targets {
			resolved := resolveTargetIP(t)
			resolvedTargets[i] = resolved
			if resolved != t {
				hasDomain = true
			}
		}

		if !hasDomain {
			delete(h.nftablesDomainCache, f.ID)
			continue
		}

		joined := strings.Join(resolvedTargets, ",")
		if cached, ok := h.nftablesDomainCache[f.ID]; ok && cached == joined {
			continue
		}

		forwardRec, err := h.getForwardRecord(f.ID)
		if err != nil {
			log.Printf("[nftables-dns] getForwardRecord(%d) failed: %v", f.ID, err)
			continue
		}
		tunnel, err := h.getTunnelRecord(f.TunnelID)
		if err != nil {
			log.Printf("[nftables-dns] getTunnelRecord(%d) failed: %v", f.TunnelID, err)
			continue
		}
		ports, err := h.listForwardPorts(f.ID)
		if err != nil {
			log.Printf("[nftables-dns] listForwardPorts(%d) failed: %v", f.ID, err)
			continue
		}
		userTunnelID, _, speed, err := h.resolveUserTunnelAndLimiter(forwardRec.UserID, forwardRec.TunnelID)
		if err != nil {
			log.Printf("[nftables-dns] resolveUserTunnelAndLimiter(%d,%d) failed: %v", forwardRec.UserID, forwardRec.TunnelID, err)
			continue
		}
		if err := h.syncNftablesRules(forwardRec, tunnel, ports, userTunnelID, speed); err != nil {
			log.Printf("[nftables-dns] refresh forward %d failed: %v", f.ID, err)
			continue
		}

		h.nftablesDomainCache[f.ID] = joined
	}

	for id := range h.nftablesDomainCache {
		if _, ok := seen[id]; !ok {
			delete(h.nftablesDomainCache, id)
		}
	}
}
