package dhcp

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Lease represents a DHCP lease
type Lease struct {
	IP       string
	MAC      string
	Hostname string
	Expires  time.Time
}

// Scanner scans DHCP leases
type Scanner struct {
	leasesFile string
}

// NewScanner creates a new DHCP scanner
func NewScanner() *Scanner {
	return &Scanner{
		leasesFile: "/tmp/dhcp.leases", // OpenWrt default
	}
}

// Scan reads DHCP leases file
func (s *Scanner) Scan() ([]*Lease, error) {
	file, err := os.Open(s.leasesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open DHCP leases file: %w", err)
	}
	defer file.Close()

	var leases []*Lease
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Format: <expiry> <mac> <ip> <hostname> <client-id>
		if len(fields) < 4 {
			continue
		}

		// Parse expiry timestamp
		expiryTs := int64(0)
		fmt.Sscanf(fields[0], "%d", &expiryTs)

		lease := &Lease{
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: fields[3],
			Expires:  time.Unix(expiryTs, 0),
		}

		leases = append(leases, lease)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan DHCP leases: %w", err)
	}

	return leases, nil
}

// GetActiveLeases returns only active (non-expired) leases
func (s *Scanner) GetActiveLeases() ([]*Lease, error) {
	allLeases, err := s.Scan()
	if err != nil {
		return nil, err
	}

	var activeLeases []*Lease
	now := time.Now()

	for _, lease := range allLeases {
		if lease.Expires.After(now) {
			activeLeases = append(activeLeases, lease)
		}
	}

	return activeLeases, nil
}
