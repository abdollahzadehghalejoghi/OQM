package nft

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	TableName        = "oqm"
	ChainForward     = "forward"
	SetMonitoredRX   = "monitored_rx"    // Download (to user)
	SetMonitoredTX   = "monitored_tx"    // Upload (from user)
	SetBlocked       = "blocked_users"
	SetMonitoredIPv6 = "monitored_users_v6"
	SetBlockedIPv6   = "blocked_users_v6"
)

// Manager handles nftables operations
type Manager struct {
	tableName string
}

// NewManager creates a new nftables manager
func NewManager(tableName string) *Manager {
	if tableName == "" {
		tableName = TableName
	}
	return &Manager{
		tableName: tableName,
	}
}

// execNft executes nft command
func (m *Manager) execNft(args ...string) (string, error) {
	cmd := exec.Command("nft", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nft command failed: %s: %w", string(output), err)
	}
	return string(output), nil
}

// Initialize sets up the nftables table and chains
func (m *Manager) Initialize() error {
	// Create table
	if _, err := m.execNft("add", "table", "inet", m.tableName); err != nil {
		// Ignore error if table already exists
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create RX (download) set with counter (IPv4)
	if _, err := m.execNft(
		"add", "set", "inet", m.tableName, SetMonitoredRX,
		"{", "type", "ipv4_addr;", "counter;", "}",
	); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to create RX set: %w", err)
		}
	}

	// Create TX (upload) set with counter (IPv4)
	if _, err := m.execNft(
		"add", "set", "inet", m.tableName, SetMonitoredTX,
		"{", "type", "ipv4_addr;", "counter;", "}",
	); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to create TX set: %w", err)
		}
	}

	// Create blocked set (IPv4)
	if _, err := m.execNft(
		"add", "set", "inet", m.tableName, SetBlocked,
		"{", "type", "ipv4_addr;", "}",
	); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to create blocked set: %w", err)
		}
	}

	// Create forward chain
	if _, err := m.execNft(
		"add", "chain", "inet", m.tableName, ChainForward,
		"{", "type", "filter", "hook", "forward", "priority", "0;", "policy", "accept;", "}",
	); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("failed to create forward chain: %w", err)
		}
	}

	// Flush chain to ensure clean state
	if _, err := m.execNft("flush", "chain", "inet", m.tableName, ChainForward); err != nil {
		return fmt.Errorf("failed to flush chain: %w", err)
	}

	// Add rules to block users in blocked set
	rules := [][]string{
		// Block outgoing traffic from blocked users
		{"add", "rule", "inet", m.tableName, ChainForward, "ip", "saddr", "@" + SetBlocked, "drop"},
		// Block incoming traffic to blocked users
		{"add", "rule", "inet", m.tableName, ChainForward, "ip", "daddr", "@" + SetBlocked, "drop"},
		// Count TX (upload): traffic FROM user (saddr = source address)
		{"add", "rule", "inet", m.tableName, ChainForward, "ip", "saddr", "@" + SetMonitoredTX},
		// Count RX (download): traffic TO user (daddr = destination address)
		{"add", "rule", "inet", m.tableName, ChainForward, "ip", "daddr", "@" + SetMonitoredRX},
	}

	for _, rule := range rules {
		if _, err := m.execNft(rule...); err != nil {
			return fmt.Errorf("failed to add rule %v: %w", rule, err)
		}
	}

	return nil
}

// AddMonitoredIP adds an IP to both RX and TX monitored sets
func (m *Manager) AddMonitoredIP(ip string) error {
	fmt.Printf("DEBUG: Adding IP %s to nftables (table=%s)\n", ip, m.tableName)

	// Add to RX set (download tracking)
	_, err := m.execNft("add", "element", "inet", m.tableName, SetMonitoredRX, "{", ip, "}")
	if err != nil {
		if strings.Contains(err.Error(), "File exists") {
			fmt.Printf("DEBUG: IP %s already exists in RX set\n", ip)
		} else {
			return fmt.Errorf("failed to add IP %s to RX set: %w", ip, err)
		}
	} else {
		fmt.Printf("DEBUG: Successfully added IP %s to RX set\n", ip)
	}

	// Add to TX set (upload tracking)
	_, err = m.execNft("add", "element", "inet", m.tableName, SetMonitoredTX, "{", ip, "}")
	if err != nil {
		if strings.Contains(err.Error(), "File exists") {
			fmt.Printf("DEBUG: IP %s already exists in TX set\n", ip)
		} else {
			return fmt.Errorf("failed to add IP %s to TX set: %w", ip, err)
		}
	} else {
		fmt.Printf("DEBUG: Successfully added IP %s to TX set\n", ip)
	}

	return nil
}

// RemoveMonitoredIP removes an IP from both RX and TX sets
func (m *Manager) RemoveMonitoredIP(ip string) error {
	// Remove from RX set
	m.execNft("delete", "element", "inet", m.tableName, SetMonitoredRX, "{", ip, "}")
	// Remove from TX set
	m.execNft("delete", "element", "inet", m.tableName, SetMonitoredTX, "{", ip, "}")
	return nil
}

// BlockIP adds an IP to the blocked set
func (m *Manager) BlockIP(ip string) error {
	_, err := m.execNft("add", "element", "inet", m.tableName, SetBlocked, "{", ip, "}")
	if err != nil && !strings.Contains(err.Error(), "File exists") {
		return fmt.Errorf("failed to block IP %s: %w", ip, err)
	}
	return nil
}

// UnblockIP removes an IP from the blocked set
func (m *Manager) UnblockIP(ip string) error {
	_, err := m.execNft("delete", "element", "inet", m.tableName, SetBlocked, "{", ip, "}")
	return err
}

// Counter represents traffic counters for an IP
type Counter struct {
	IP        string
	RxPackets int64 // Download packets
	RxBytes   int64 // Download bytes
	TxPackets int64 // Upload packets
	TxBytes   int64 // Upload bytes
}

// TotalBytes returns total bytes (RX + TX)
func (c *Counter) TotalBytes() int64 {
	return c.RxBytes + c.TxBytes
}

// TotalPackets returns total packets (RX + TX)
func (c *Counter) TotalPackets() int64 {
	return c.RxPackets + c.TxPackets
}

// GetCounters retrieves traffic counters for all monitored IPs (RX and TX separately)
func (m *Manager) GetCounters() (map[string]*Counter, error) {
	counters := make(map[string]*Counter)

	// Get RX (download) counters
	rxOutput, err := m.execNft("list", "set", "inet", m.tableName, SetMonitoredRX)
	if err != nil {
		return nil, fmt.Errorf("failed to list RX set: %w", err)
	}

	// Get TX (upload) counters
	txOutput, err := m.execNft("list", "set", "inet", m.tableName, SetMonitoredTX)
	if err != nil {
		return nil, fmt.Errorf("failed to list TX set: %w", err)
	}

	// Parse RX counters
	rxCount := m.parseCountersInto(counters, rxOutput, true)

	// Parse TX counters
	txCount := m.parseCountersInto(counters, txOutput, false)

	// Debug: log parsed counts
	fmt.Printf("DEBUG: Parsed %d RX counters and %d TX counters (total unique IPs: %d)\n", rxCount, txCount, len(counters))

	// Debug: log which IPs were parsed
	for ip := range counters {
		fmt.Printf("DEBUG: Parsed counter for IP: %s\n", ip)
	}

	return counters, nil
}

// parseCountersInto parses nft output and updates counters map
func (m *Manager) parseCountersInto(counters map[string]*Counter, output string, isRX bool) int {
	// Regex to match: 192.168.1.10 counter packets 1234 bytes 567890
	re := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)\s+counter\s+packets\s+(\d+)\s+bytes\s+(\d+)`)

	count := 0

	// Find ALL matches in the entire output (not line by line)
	// because nftables can put multiple IPs on one line
	allMatches := re.FindAllStringSubmatch(output, -1)

	for _, matches := range allMatches {
		if len(matches) == 4 {
			ip := matches[1]
			packets, _ := strconv.ParseInt(matches[2], 10, 64)
			bytes, _ := strconv.ParseInt(matches[3], 10, 64)

			// Get or create counter for this IP
			counter, exists := counters[ip]
			if !exists {
				counter = &Counter{IP: ip}
				counters[ip] = counter
			}

			// Update RX or TX based on flag
			if isRX {
				counter.RxPackets = packets
				counter.RxBytes = bytes
			} else {
				counter.TxPackets = packets
				counter.TxBytes = bytes
			}
			count++
		}
	}

	return count
}

// GetIPCounter retrieves counter for a specific IP
func (m *Manager) GetIPCounter(ip string) (*Counter, error) {
	counters, err := m.GetCounters()
	if err != nil {
		return nil, err
	}

	if counter, ok := counters[ip]; ok {
		return counter, nil
	}

	// Return zero counter if IP not found
	return &Counter{IP: ip}, nil
}

// ResetCounter resets counter for an IP by removing and re-adding it
func (m *Manager) ResetCounter(ip string) error {
	// Remove from set
	if err := m.RemoveMonitoredIP(ip); err != nil {
		return err
	}

	// Re-add to set (this resets counter)
	return m.AddMonitoredIP(ip)
}

// ListBlockedIPs returns list of blocked IPs
func (m *Manager) ListBlockedIPs() ([]string, error) {
	output, err := m.execNft("list", "set", "inet", m.tableName, SetBlocked)
	if err != nil {
		return nil, err
	}

	var ips []string
	re := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := re.FindStringSubmatch(line); len(matches) > 0 {
			ips = append(ips, matches[1])
		}
	}

	return ips, nil
}

// ListMonitoredIPs returns list of monitored IPs (from RX set)
func (m *Manager) ListMonitoredIPs() ([]string, error) {
	output, err := m.execNft("list", "set", "inet", m.tableName, SetMonitoredRX)
	if err != nil {
		return nil, err
	}

	var ips []string
	re := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := re.FindStringSubmatch(line); len(matches) > 0 {
			// Avoid duplicates
			ip := matches[1]
			found := false
			for _, existingIP := range ips {
				if existingIP == ip {
					found = true
					break
				}
			}
			if !found {
				ips = append(ips, ip)
			}
		}
	}

	return ips, nil
}

// Cleanup removes the nftables table and all rules
func (m *Manager) Cleanup() error {
	_, err := m.execNft("delete", "table", "inet", m.tableName)
	return err
}

// Show displays the current nftables configuration
func (m *Manager) Show() (string, error) {
	return m.execNft("list", "table", "inet", m.tableName)
}
