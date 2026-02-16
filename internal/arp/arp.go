package arp

import (
	"bufio"
	"os"
	"strings"
)

// Entry represents an ARP table entry
type Entry struct {
	IP  string
	MAC string
}

// GetTable reads the system ARP table
func GetTable() ([]Entry, error) {
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)

	// Skip header line
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 4 {
			continue
		}

		ip := fields[0]
		mac := fields[3]

		// Skip incomplete entries (MAC = 00:00:00:00:00:00)
		if mac == "00:00:00:00:00:00" {
			continue
		}

		entries = append(entries, Entry{
			IP:  ip,
			MAC: strings.ToLower(mac),
		})
	}

	return entries, scanner.Err()
}

// LookupIP finds IP address for a given MAC address
func LookupIP(mac string) (string, bool) {
	entries, err := GetTable()
	if err != nil {
		return "", false
	}

	mac = strings.ToLower(mac)

	for _, entry := range entries {
		if entry.MAC == mac {
			return entry.IP, true
		}
	}

	return "", false
}

// LookupMAC finds MAC address for a given IP address
func LookupMAC(ip string) (string, bool) {
	entries, err := GetTable()
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IP == ip {
			return entry.MAC, true
		}
	}

	return "", false
}
