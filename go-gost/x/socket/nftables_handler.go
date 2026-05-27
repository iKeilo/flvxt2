//go:build linux

package socket

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/go-gost/x/nftables"
)

type AddNftablesRulesRequest struct {
	Rules []NftablesRulePayload `json:"rules"`
}

type NftablesRulePayload struct {
	ForwardID   int64  `json:"forward_id"`
	NodeID      int64  `json:"node_id"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Target      string `json:"target"`
	SpeedLimit  int    `json:"speed_limit"`
	ChainType   int    `json:"chain_type"`
	NextHopIP   string `json:"next_hop_ip"`
	NextHopPort int    `json:"next_hop_port"`
}

type UpdateNftablesRulesRequest struct {
	Rules []NftablesRulePayload `json:"rules"`
}

type DeleteNftablesRulesRequest struct {
	ForwardIDs []int64  `json:"forward_ids"`
	Protocols  []string `json:"protocols"`
	Ports      []int    `json:"ports"`
}

type GetNftablesCountersRequest struct {
	ForwardIDs []int64 `json:"forward_ids"`
}

func (w *WebSocketReporter) handleAddNftablesRules(data json.RawMessage) error {
	if w.nftablesMgr == nil {
		return fmt.Errorf("nftables manager not initialized")
	}

	var req AddNftablesRulesRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}
	if len(req.Rules) == 0 {
		return fmt.Errorf("rules list cannot be empty")
	}

	for _, rule := range req.Rules {
		target := rule.Target
		if rule.ChainType > 0 && rule.NextHopIP != "" {
			target = net.JoinHostPort(rule.NextHopIP, strconv.Itoa(rule.NextHopPort))
		}
		if err := w.nftablesMgr.AddRule(rule.ForwardID, rule.NodeID, rule.Protocol, rule.Port, target, rule.SpeedLimit); err != nil {
			return fmt.Errorf("add rule for forward %d/%s (target=%q): %w", rule.ForwardID, rule.Protocol, target, err)
		}
	}
	return nil
}

func (w *WebSocketReporter) handleUpdateNftablesRules(data json.RawMessage) error {
	if w.nftablesMgr == nil {
		return fmt.Errorf("nftables manager not initialized")
	}

	var req UpdateNftablesRulesRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}

	for _, rule := range req.Rules {
		target := rule.Target
		if rule.ChainType > 0 && rule.NextHopIP != "" {
			target = net.JoinHostPort(rule.NextHopIP, strconv.Itoa(rule.NextHopPort))
		}
		if err := w.nftablesMgr.UpdateRule(rule.ForwardID, rule.Protocol, rule.Port, target, rule.SpeedLimit); err != nil {
			return fmt.Errorf("update rule for forward %d/%s: %w", rule.ForwardID, rule.Protocol, err)
		}
	}
	return nil
}

func (w *WebSocketReporter) handleDeleteNftablesRules(data json.RawMessage) error {
	if w.nftablesMgr == nil {
		return fmt.Errorf("nftables manager not initialized")
	}

	var req DeleteNftablesRulesRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}

	protocols := req.Protocols
	if len(protocols) == 0 {
		protocols = []string{"tcp", "udp"}
	}

	var errs []error
	for _, forwardID := range req.ForwardIDs {
		for _, protocol := range protocols {
			if len(req.Ports) > 0 {
				for _, port := range req.Ports {
					if err := w.nftablesMgr.DeleteRuleWithPort(forwardID, protocol, port); err != nil {
						errs = append(errs, fmt.Errorf("delete rule forwardID=%d/%s:%d: %w", forwardID, protocol, port, err))
					}
				}
				continue
			}
			if err := w.nftablesMgr.DeleteRule(forwardID, protocol); err != nil {
				errs = append(errs, fmt.Errorf("delete rule forwardID=%d/%s: %w", forwardID, protocol, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("some rules failed to delete: %v", errs)
	}
	return nil
}

func (w *WebSocketReporter) handleGetNftablesCounters(data json.RawMessage) error {
	if w.nftablesMgr == nil {
		return fmt.Errorf("nftables manager not initialized")
	}

	counters := w.nftablesMgr.GetCounters()
	results := make([]nftables.CounterResult, 0, len(counters))
	for _, counter := range counters {
		results = append(results, nftables.CounterResult{
			ForwardID: counter.ForwardID,
			Protocol:  counter.Protocol,
			Port:      counter.Port,
			Packets:   counter.Packets,
			Bytes:     counter.Bytes,
		})
	}

	w.nftablesCounters = results
	return nil
}

func (w *WebSocketReporter) handleResetNftablesCounters(data json.RawMessage) error {
	if w.nftablesMgr == nil {
		return fmt.Errorf("nftables manager not initialized")
	}

	if err := w.nftablesMgr.ResetCounters(); err != nil {
		return fmt.Errorf("reset counters: %w", err)
	}
	w.nftablesCounters = nil
	return nil
}
