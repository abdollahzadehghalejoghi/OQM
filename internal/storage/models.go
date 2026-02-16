package storage

import (
	"time"
)

// User represents a network user with quota limits
type User struct {
	IP             string    `json:"ip"`
	MAC            string    `json:"mac"`
	Name           string    `json:"name"`
	Username       string    `json:"username"`         // For grouping multi-device users
	QuotaMB        int64     `json:"quota_mb"`         // Individual device quota (0 = unlimited)
	GroupQuotaMB   int64     `json:"group_quota_mb"`   // Group quota (0 = unlimited, only used if Username set)
	RxBytes        int64     `json:"rx_bytes"`
	TxBytes        int64     `json:"tx_bytes"`
	IsBlocked      bool      `json:"is_blocked"`
	TelegramChatID string    `json:"telegram_chat_id,omitempty"` // Per-device notification
	GroupTelegramChatID string `json:"group_telegram_chat_id,omitempty"` // Group notification (shared)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TotalBytes returns total upload + download
func (u *User) TotalBytes() int64 {
	return u.RxBytes + u.TxBytes
}

// TotalMB returns total usage in MB
func (u *User) TotalMB() float64 {
	return float64(u.TotalBytes()) / 1024 / 1024
}

// QuotaUsagePercent returns percentage of quota used (individual only)
func (u *User) QuotaUsagePercent() float64 {
	if u.QuotaMB == 0 {
		return 0
	}
	return (u.TotalMB() / float64(u.QuotaMB)) * 100
}

// IsQuotaExceeded checks if user has exceeded their quota (individual only)
func (u *User) IsQuotaExceeded() bool {
	if u.QuotaMB == 0 {
		return false // Unlimited quota
	}
	return u.TotalMB() >= float64(u.QuotaMB)
}

// GroupQuotaUsagePercent returns percentage of quota used for entire group
// Only meaningful when called with group total bytes and quota
func GroupQuotaUsagePercent(totalBytes int64, quotaMB int64) float64 {
	if quotaMB == 0 {
		return 0
	}
	totalMB := float64(totalBytes) / 1024 / 1024
	return (totalMB / float64(quotaMB)) * 100
}

// IsGroupQuotaExceeded checks if group has exceeded quota
func IsGroupQuotaExceeded(totalBytes int64, quotaMB int64) bool {
	if quotaMB == 0 {
		return false // Unlimited quota
	}
	totalMB := float64(totalBytes) / 1024 / 1024
	return totalMB >= float64(quotaMB)
}

// Config holds application configuration
type Config struct {
	CheckInterval         int    `json:"check_interval"`       // Seconds between monitoring checks
	WebPort               int    `json:"web_port"`             // Web UI port
	BotType               string `json:"bot_type"`             // Bot type: telegram or bale
	BotToken              string `json:"bot_token"`            // Bot token for notifications (Telegram or Bale)
	BotAPIBaseURL         string `json:"bot_api_base_url"`     // Custom API base URL (for Bale or custom endpoints)
	AdminChatID           string `json:"admin_chat_id"`        // Admin chat ID for global notifications
	ResetSchedule         string `json:"reset_schedule"`       // daily, weekly, monthly
	NFTablesTable         string `json:"nftables_table"`       // nftables table name
	DataDir               string `json:"data_dir"`             // Data directory path
	LogFile               string `json:"log_file"`             // Log file path
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		CheckInterval:       60,
		WebPort:             8080,
		BotType:             "telegram", // Default to Telegram
		BotToken:            "",
		BotAPIBaseURL:       "",          // Empty means use default (Telegram or Bale based on BotType)
		AdminChatID:         "",
		ResetSchedule:       "monthly",
		NFTablesTable:       "oqm",
		DataDir:             "/etc/oqm",
		LogFile:             "/var/log/oqm.log",
	}
}

// Data represents the entire application state
type Data struct {
	Users  []*User `json:"users"`
	Config *Config `json:"config"`
}
