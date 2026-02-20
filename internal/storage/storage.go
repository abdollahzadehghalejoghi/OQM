package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Storage manages persistent data storage
type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     *Data
}

// NewStorage creates a new storage instance
func NewStorage(filePath string) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
		data: &Data{
			Users:  make([]*User, 0),
			Config: DefaultConfig(),
		},
	}

	// Create directory if not exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load existing data or create new file
	if err := s.Load(); err != nil {
		if os.IsNotExist(err) {
			// Create new file with default data
			if err := s.Save(); err != nil {
				return nil, fmt.Errorf("failed to create data file: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return s, nil
}

// Load reads data from file
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.data)
}

// Save writes data to file
func (s *Storage) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// AddUser adds a new user
func (s *Storage) AddUser(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user with same IP already exists
	for _, u := range s.data.Users {
		if u.IP == user.IP {
			return fmt.Errorf("user with IP %s already exists", user.IP)
		}
	}

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	s.data.Users = append(s.data.Users, user)

	return nil
}

// RemoveUser removes a user by IP
func (s *Storage) RemoveUser(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.data.Users {
		if u.IP == ip {
			s.data.Users = append(s.data.Users[:i], s.data.Users[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("user with IP %s not found", ip)
}

// GetUser retrieves a user by IP
func (s *Storage) GetUser(ip string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.data.Users {
		if u.IP == ip {
			return u, nil
		}
	}

	return nil, fmt.Errorf("user with IP %s not found", ip)
}

// GetUsersByUsername retrieves all users with the same username (multi-device)
func (s *Storage) GetUsersByUsername(username string) []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []*User
	for _, u := range s.data.Users {
		if u.Username == username {
			users = append(users, u)
		}
	}

	return users
}

// GetAllUsers returns all users
func (s *Storage) GetAllUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, len(s.data.Users))
	copy(users, s.data.Users)
	return users
}

// UpdateUser updates an existing user by IP
func (s *Storage) UpdateUser(ip string, updateFn func(*User)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		if u.IP == ip {
			updateFn(u)
			u.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("user with IP %s not found", ip)
}

// UpdateUserByMAC updates an existing user by MAC address
func (s *Storage) UpdateUserByMAC(mac string, updateFn func(*User)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		if u.MAC == mac {
			updateFn(u)
			u.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("user with MAC %s not found", mac)
}

// UpdateUserUsage updates user's data usage
func (s *Storage) UpdateUserUsage(ip string, rxBytes, txBytes int64) error {
	return s.UpdateUser(ip, func(u *User) {
		u.RxBytes = rxBytes
		u.TxBytes = txBytes
	})
}

// BlockUser blocks a user
func (s *Storage) BlockUser(ip string) error {
	return s.UpdateUser(ip, func(u *User) {
		u.IsBlocked = true
	})
}

// UnblockUser unblocks a user
func (s *Storage) UnblockUser(ip string) error {
	return s.UpdateUser(ip, func(u *User) {
		u.IsBlocked = false
	})
}

// ResetUserUsage resets user's usage to zero
func (s *Storage) ResetUserUsage(ip string) error {
	return s.UpdateUser(ip, func(u *User) {
		u.RxBytes = 0
		u.TxBytes = 0
		u.IsBlocked = false
		u.DeviceWarningNotificationSent = false
		u.GroupWarningNotificationSent = false
	})
}

// ResetAllUsage resets usage for all users
func (s *Storage) ResetAllUsage() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		u.RxBytes = 0
		u.TxBytes = 0
		u.IsBlocked = false
		u.DeviceWarningNotificationSent = false
		u.GroupWarningNotificationSent = false
		u.UpdatedAt = time.Now()
	}

	return nil
}

// GetConfig returns the configuration
func (s *Storage) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Config
}

// UpdateConfig updates configuration
func (s *Storage) UpdateConfig(updateFn func(*Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updateFn(s.data.Config)
}

// ExportJSON exports data to a JSON file
func (s *Storage) ExportJSON(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ImportJSON imports data from a JSON file
func (s *Storage) ImportJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return json.Unmarshal(data, &s.data)
}

// GetUsageByUsername calculates total usage for a username (all devices)
func (s *Storage) GetUsageByUsername(username string) (rxBytes, txBytes int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.data.Users {
		if u.Username == username {
			rxBytes += u.RxBytes
			txBytes += u.TxBytes
		}
	}

	return rxBytes, txBytes
}

// GetGroupTelegramChatID returns Telegram chat ID for a group (from first device with chat ID)
func (s *Storage) GetGroupTelegramChatID(username string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.data.Users {
		if u.Username == username && u.TelegramChatID != "" {
			return u.TelegramChatID
		}
	}

	return ""
}

// GetAllUsernames returns list of unique usernames (for grouped users)
func (s *Storage) GetAllUsernames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	usernameMap := make(map[string]bool)
	usernames := make([]string, 0) // Initialize with empty slice, not nil

	for _, u := range s.data.Users {
		if u.Username != "" && u.Username != "-" && !usernameMap[u.Username] {
			usernameMap[u.Username] = true
			usernames = append(usernames, u.Username)
		}
	}

	return usernames
}
