package webui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DHCPLease represents a DHCP lease entry
type DHCPLease struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
	Expires  string `json:"expires"`
}

// TotalTraffic represents total network traffic
type TotalTraffic struct {
	Interface    string `json:"interface"`
	RxBytes      int64  `json:"rx_bytes"`
	TxBytes      int64  `json:"tx_bytes"`
	TotalBytes   int64  `json:"total_bytes"`
	RxBytesStr   string `json:"rx_bytes_str"`
	TxBytesStr   string `json:"tx_bytes_str"`
	TotalBytesStr string `json:"total_bytes_str"`
}

// scanDHCPLeases reads DHCP leases from file
func (s *Server) scanDHCPLeases() ([]DHCPLease, error) {
	leasesFile := "/tmp/dhcp.leases"

	file, err := os.Open(leasesFile)
	if err != nil {
		// If file doesn't exist, return empty list (not an error)
		if os.IsNotExist(err) {
			return []DHCPLease{}, nil
		}
		return nil, fmt.Errorf("failed to open DHCP leases: %w", err)
	}
	defer file.Close()

	var leases []DHCPLease
	scanner := bufio.NewScanner(file)
	now := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Format: <expiry> <mac> <ip> <hostname> <client-id>
		if len(fields) < 4 {
			continue
		}

		var expiryTs int64
		fmt.Sscanf(fields[0], "%d", &expiryTs)
		expires := time.Unix(expiryTs, 0)

		// Only include active leases
		if expires.Before(now) {
			continue
		}

		lease := DHCPLease{
			IP:       fields[2],
			MAC:      fields[1],
			Hostname: fields[3],
			Expires:  expires.Format("2006-01-02 15:04:05"),
		}

		leases = append(leases, lease)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan DHCP leases: %w", err)
	}

	return leases, nil
}

// getTotalTraffic gets total traffic from main WAN interface
func (s *Server) getTotalTraffic() (*TotalTraffic, error) {
	// Try to detect WAN interface
	iface := s.detectWANInterface()

	if iface == "" {
		// Fallback to common names
		for _, candidate := range []string{"eth0", "eth1", "wan", "pppoe-wan", "wwan0"} {
			if s.interfaceExists(candidate) {
				iface = candidate
				break
			}
		}
	}

	if iface == "" {
		return nil, fmt.Errorf("could not detect WAN interface")
	}

	// Read traffic from /sys/class/net
	rxBytes, err := s.readInterfaceCounter(iface, "rx_bytes")
	if err != nil {
		return nil, err
	}

	txBytes, err := s.readInterfaceCounter(iface, "tx_bytes")
	if err != nil {
		return nil, err
	}

	traffic := &TotalTraffic{
		Interface:     iface,
		RxBytes:       rxBytes,
		TxBytes:       txBytes,
		TotalBytes:    rxBytes + txBytes,
		RxBytesStr:    formatBytes(rxBytes),
		TxBytesStr:    formatBytes(txBytes),
		TotalBytesStr: formatBytes(rxBytes + txBytes),
	}

	return traffic, nil
}

// detectWANInterface tries to detect the WAN interface using UCI
func (s *Server) detectWANInterface() string {
	// Try UCI first (OpenWrt specific)
	cmd := exec.Command("uci", "get", "network.wan.device")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// Try alternative UCI path
	cmd = exec.Command("uci", "get", "network.wan.ifname")
	output, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

// interfaceExists checks if network interface exists
func (s *Server) interfaceExists(iface string) bool {
	path := fmt.Sprintf("/sys/class/net/%s", iface)
	_, err := os.Stat(path)
	return err == nil
}

// readInterfaceCounter reads a counter from /sys/class/net
func (s *Server) readInterfaceCounter(iface, counter string) (int64, error) {
	path := fmt.Sprintf("/sys/class/net/%s/statistics/%s", iface, counter)

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s for %s: %w", counter, iface, err)
	}

	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter: %w", err)
	}

	return value, nil
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Alternative: Get total traffic from iptables (if nftables counters not available)
func (s *Server) getTotalTrafficFromIPTables() (map[string]int64, error) {
	cmd := exec.Command("iptables", "-L", "FORWARD", "-v", "-n", "-x")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse iptables output
	re := regexp.MustCompile(`(\d+)\s+(\d+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)

	var totalPackets, totalBytes int64
	for _, match := range matches {
		if len(match) >= 3 {
			packets, _ := strconv.ParseInt(match[1], 10, 64)
			bytes, _ := strconv.ParseInt(match[2], 10, 64)
			totalPackets += packets
			totalBytes += bytes
		}
	}

	return map[string]int64{
		"packets": totalPackets,
		"bytes":   totalBytes,
	}, nil
}
