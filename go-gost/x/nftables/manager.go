//go:build linux

package nftables

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const (
	TableName        = "flvx"
	TableFamily      = nftables.TableFamilyINet
	PreroutingChain  = "prerouting"
	PostroutingChain = "postrouting"
)

type Manager struct {
	conn  *nftables.Conn
	table *nftables.Table
	rules map[string]*RuleState
	mu    sync.RWMutex
}

type RuleState struct {
	ForwardID  int64
	NodeID     int64
	Protocol   string
	Port       int
	Target     string
	SpeedLimit int
	Chain      *nftables.Chain
	Rule       *nftables.Rule
}

type CounterResult struct {
	ForwardID int64  `json:"forward_id"`
	Protocol  string `json:"protocol"`
	Port      int    `json:"port"`
	Packets   uint64 `json:"packets"`
	Bytes     uint64 `json:"bytes"`
}

func NewManager() (*Manager, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("open nftables: %w", err)
	}
	manager := &Manager{
		conn:  conn,
		rules: make(map[string]*RuleState),
	}
	if err := manager.initTable(); err != nil {
		return nil, fmt.Errorf("init table: %w", err)
	}
	if err := manager.clearStaleRules(); err != nil {
		fmt.Printf("clear stale rules failed: %v\n", err)
	}
	enableIPForwarding()
	return manager, nil
}

func enableIPForwarding() {
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
	_ = exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()
}

func (m *Manager) initTable() error {
	table := &nftables.Table{
		Name:   TableName,
		Family: TableFamily,
	}
	m.conn.AddTable(table)
	if err := m.conn.Flush(); err != nil {
		return fmt.Errorf("add table: %w", err)
	}
	m.table = table
	return m.initChains()
}

func (m *Manager) initChains() error {
	chains := []struct {
		name     string
		hook     *nftables.ChainHook
		priority *nftables.ChainPriority
	}{
		{
			name:     PreroutingChain,
			hook:     nftables.ChainHookPrerouting,
			priority: nftables.ChainPriorityNATDest,
		},
		{
			name:     PostroutingChain,
			hook:     nftables.ChainHookPostrouting,
			priority: nftables.ChainPriorityNATSource,
		},
	}

	for _, chain := range chains {
		m.conn.AddChain(&nftables.Chain{
			Name:     chain.name,
			Table:    m.table,
			Hooknum:  chain.hook,
			Priority: chain.priority,
			Type:     nftables.ChainTypeNAT,
		})
	}

	postroutingChain := &nftables.Chain{Name: PostroutingChain, Table: m.table}
	rules, err := m.conn.GetRules(m.table, postroutingChain)
	if err != nil {
		return fmt.Errorf("get postrouting rules: %w", err)
	}

	hasMasq := false
	for _, rule := range rules {
		for _, expression := range rule.Exprs {
			if _, ok := expression.(*expr.Masq); ok {
				hasMasq = true
				break
			}
		}
		if hasMasq {
			break
		}
	}

	if !hasMasq {
		m.conn.AddRule(&nftables.Rule{
			Table: m.table,
			Chain: postroutingChain,
			Exprs: []expr.Any{&expr.Masq{}},
		})
	}

	return m.conn.Flush()
}

func (m *Manager) AddRule(forwardID, nodeID int64, protocol string, port int, target string, speedLimit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ruleKey(forwardID, protocol)
	if _, exists := m.rules[key]; exists {
		return fmt.Errorf("rule already exists: %s", key)
	}

	dnatAddr, dnatPort := parseTarget(target)
	if dnatAddr == "" || dnatPort <= 0 {
		return fmt.Errorf("invalid target: %q", target)
	}

	preroutingChain := &nftables.Chain{Name: PreroutingChain, Table: m.table}
	ruleExprs, err := buildRuleExpressions(protocol, port, dnatAddr, dnatPort, speedLimit)
	if err != nil {
		return err
	}

	rule := &nftables.Rule{
		Table: m.table,
		Chain: preroutingChain,
		Exprs: ruleExprs,
	}
	m.conn.AddRule(rule)

	if err := m.conn.Flush(); err != nil {
		return fmt.Errorf("add rule: %w", err)
	}

	m.rules[key] = &RuleState{
		ForwardID:  forwardID,
		NodeID:     nodeID,
		Protocol:   protocol,
		Port:       port,
		Target:     target,
		SpeedLimit: speedLimit,
		Chain:      preroutingChain,
		Rule:       rule,
	}
	return nil
}

func buildRuleExpressions(protocol string, port int, dnatAddr string, dnatPort int, speedLimit int) ([]expr.Any, error) {
	var protoNum uint32
	switch protocol {
	case "tcp":
		protoNum = unix.IPPROTO_TCP
	case "udp":
		protoNum = unix.IPPROTO_UDP
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	ruleExprs := []expr.Any{
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{byte(protoNum)}},
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseTransportHeader,
			Offset:       2,
			Len:          2,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{byte(port >> 8), byte(port & 0xFF)},
		},
	}

	if speedLimit > 0 {
		ruleExprs = append(ruleExprs, &expr.Limit{
			Type: expr.LimitTypePkts,
			Rate: uint64(speedLimit),
		})
	}

	ruleExprs = append(ruleExprs, &expr.Counter{})

	ip := net.ParseIP(dnatAddr)
	if ip == nil {
		return nil, fmt.Errorf("invalid target IP: %s", dnatAddr)
	}

	var natFamily uint32
	var ipBytes []byte
	if ip4 := ip.To4(); ip4 != nil {
		natFamily = unix.NFPROTO_IPV4
		ipBytes = ip4
	} else {
		natFamily = unix.NFPROTO_IPV6
		ipBytes = ip.To16()
	}

	ruleExprs = append(ruleExprs,
		&expr.Immediate{Register: 1, Data: ipBytes},
		&expr.Immediate{Register: 2, Data: []byte{byte(dnatPort >> 8), byte(dnatPort & 0xFF)}},
		&expr.NAT{
			Type:        expr.NATTypeDestNAT,
			Family:      natFamily,
			RegAddrMin:  1,
			RegProtoMin: 2,
		},
	)

	return ruleExprs, nil
}

func (m *Manager) UpdateRule(forwardID int64, protocol string, port int, target string, speedLimit int) error {
	if err := m.DeleteRule(forwardID, protocol); err != nil {
		return err
	}
	return m.AddRule(forwardID, 0, protocol, port, target, speedLimit)
}

func (m *Manager) DeleteRule(forwardID int64, protocol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ruleKey(forwardID, protocol)
	delete(m.rules, key)
	return m.deleteRuleFromKernel(forwardID, protocol, 0, false)
}

func (m *Manager) DeleteRuleWithPort(forwardID int64, protocol string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ruleKey(forwardID, protocol)
	delete(m.rules, key)
	return m.deleteRuleFromKernel(forwardID, protocol, port, true)
}

func (m *Manager) deleteRuleFromKernel(forwardID int64, protocol string, port int, matchPort bool) error {
	preroutingChain := &nftables.Chain{Name: PreroutingChain, Table: m.table}
	rules, err := m.conn.GetRules(m.table, preroutingChain)
	if err != nil {
		return fmt.Errorf("get prerouting rules: %w", err)
	}

	protoNum := uint8(unix.IPPROTO_TCP)
	if protocol == "udp" {
		protoNum = uint8(unix.IPPROTO_UDP)
	}
	portBytes := []byte{byte(port >> 8), byte(port & 0xFF)}

	deleted := false
	for _, rule := range rules {
		if isMasqueradeRule(rule) || !matchProtoInRule(rule, protoNum) {
			continue
		}
		if matchPort && !matchPortInRule(rule, portBytes) {
			continue
		}
		m.conn.DelRule(rule)
		deleted = true
	}

	if !deleted {
		return nil
	}
	return m.conn.Flush()
}

func (m *Manager) GetCounters() []CounterResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]CounterResult, 0, len(m.rules))
	for _, ruleState := range m.rules {
		if ruleState.Rule == nil {
			continue
		}
		for _, expression := range ruleState.Rule.Exprs {
			counter, ok := expression.(*expr.Counter)
			if !ok {
				continue
			}
			results = append(results, CounterResult{
				ForwardID: ruleState.ForwardID,
				Protocol:  ruleState.Protocol,
				Port:      ruleState.Port,
				Packets:   counter.Packets,
				Bytes:     counter.Bytes,
			})
		}
	}
	return results
}

func (m *Manager) ResetCounters() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ruleState := range m.rules {
		if ruleState.Rule == nil {
			continue
		}
		for index, expression := range ruleState.Rule.Exprs {
			if _, ok := expression.(*expr.Counter); ok {
				ruleState.Rule.Exprs[index] = &expr.Counter{}
			}
		}
		m.conn.ReplaceRule(ruleState.Rule)
	}
	return m.conn.Flush()
}

func CheckNftablesSupport() (bool, error) {
	conn, err := nftables.New()
	if err != nil {
		return false, fmt.Errorf("nftables not available: %w", err)
	}
	conn.CloseLasting()
	return true, nil
}

func (m *Manager) clearStaleRules() error {
	preroutingChain := &nftables.Chain{Name: PreroutingChain, Table: m.table}
	rules, err := m.conn.GetRules(m.table, preroutingChain)
	if err != nil {
		return fmt.Errorf("get prerouting rules: %w", err)
	}

	for _, rule := range rules {
		if isMasqueradeRule(rule) {
			continue
		}
		m.conn.DelRule(rule)
	}

	return m.conn.Flush()
}

func ruleKey(forwardID int64, protocol string) string {
	return fmt.Sprintf("%d_%s", forwardID, protocol)
}

func parseTarget(target string) (string, int) {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(target))
	if err != nil {
		return "", 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func isMasqueradeRule(rule *nftables.Rule) bool {
	for _, expression := range rule.Exprs {
		if _, ok := expression.(*expr.Masq); ok {
			return true
		}
	}
	return false
}

func matchProtoInRule(rule *nftables.Rule, protoByte uint8) bool {
	for _, expression := range rule.Exprs {
		cmp, ok := expression.(*expr.Cmp)
		if !ok || cmp.Register != 1 || len(cmp.Data) != 1 {
			continue
		}
		if cmp.Data[0] == protoByte {
			return true
		}
	}
	return false
}

func matchPortInRule(rule *nftables.Rule, portBytes []byte) bool {
	if len(portBytes) != 2 {
		return false
	}
	for _, expression := range rule.Exprs {
		cmp, ok := expression.(*expr.Cmp)
		if !ok || cmp.Register != 1 || len(cmp.Data) != 2 {
			continue
		}
		if cmp.Data[0] == portBytes[0] && cmp.Data[1] == portBytes[1] {
			return true
		}
	}
	return false
}
