//go:build !linux

package nftables

import "errors"

type Manager struct{}

type RuleState struct{}

type CounterResult struct{}

func NewManager() (*Manager, error) {
	return nil, errors.New("nftables not supported on this platform")
}

func (m *Manager) initTable() error {
	return errors.New("nftables not supported on this platform")
}

func (m *Manager) initChains() error {
	return errors.New("nftables not supported on this platform")
}

func (m *Manager) AddRule(forwardID, nodeID int64, protocol string, port int, target string, speedLimit int) error {
	return errors.New("nftables not supported on this platform")
}

func (m *Manager) UpdateRule(forwardID int64, protocol string, port int, target string, speedLimit int) error {
	return errors.New("nftables not supported on this platform")
}

func (m *Manager) DeleteRule(forwardID int64, protocol string) error {
	return errors.New("nftables not supported on this platform")
}

func (m *Manager) GetCounters() []CounterResult {
	return nil
}

func (m *Manager) ResetCounters() error {
	return errors.New("nftables not supported on this platform")
}

func CheckNftablesSupport() (bool, error) {
	return false, errors.New("nftables not supported on this platform")
}
