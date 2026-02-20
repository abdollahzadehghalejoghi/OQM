package daemon

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abdollahzadehghalejoghi/oqm/internal/arp"
	"github.com/abdollahzadehghalejoghi/oqm/internal/nft"
	"github.com/abdollahzadehghalejoghi/oqm/internal/notify"
	"github.com/abdollahzadehghalejoghi/oqm/internal/storage"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/logger"
)

// Daemon manages the monitoring and quota enforcement
type Daemon struct {
	storage      *storage.Storage
	nftMgr       *nft.Manager
	notifier     *notify.TelegramNotifier
	interval     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	forceCheck   chan bool           // Channel to trigger immediate check
	monitoredIPs map[string]bool     // Track which IPs are already in nftables
	blockedIPs   map[string]bool     // Track which IPs are currently blocked
}

// New creates a new daemon instance
func New(store *storage.Storage) (*Daemon, error) {
	cfg := store.GetConfig()

	// Create nftables manager
	nftMgr := nft.NewManager(cfg.NFTablesTable)

	// Create bot notifier (Telegram/Bale - optional)
	var notifier *notify.TelegramNotifier

	if cfg.BotToken != "" {
		var err error
		var baseURL string

		// Determine base URL based on bot type
		if cfg.BotAPIBaseURL != "" {
			// Custom base URL specified
			baseURL = cfg.BotAPIBaseURL
		} else if cfg.BotType == "bale" {
			// Bale messenger
			baseURL = "https://tapi.bale.ai"
		}
		// Empty baseURL means use Telegram default

		notifier, err = notify.NewTelegramNotifier(cfg.BotToken, cfg.AdminChatID, baseURL)
		if err != nil {
			logger.Error("Failed to create bot notifier: %v", err)
			// Continue without notifier
		} else {
			botTypeName := cfg.BotType
			if botTypeName == "" {
				botTypeName = "telegram"
			}
			logger.Info("%s bot notifier initialized", botTypeName)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		storage:      store,
		nftMgr:       nftMgr,
		notifier:     notifier,
		interval:     time.Duration(cfg.CheckInterval) * time.Second,
		ctx:          ctx,
		cancel:       cancel,
		forceCheck:   make(chan bool, 1), // Buffered channel
		monitoredIPs: make(map[string]bool),
		blockedIPs:   make(map[string]bool),
	}, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	logger.Info("Starting OQM daemon...")

	// Initialize nftables
	if err := d.nftMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize nftables: %w", err)
	}
	logger.Info("nftables initialized")

	// Add all existing users to monitored set
	users := d.storage.GetAllUsers()
	for _, u := range users {
		if err := d.nftMgr.AddMonitoredIP(u.IP); err != nil {
			logger.Error("Failed to add %s to monitored set: %v", u.IP, err)
		}

		// Re-block users who were blocked
		if u.IsBlocked {
			if err := d.nftMgr.BlockIP(u.IP); err != nil {
				logger.Error("Failed to re-block %s: %v", u.IP, err)
			}
		}
	}
	logger.Info("Added %d users to monitoring", len(users))

	// Perform initial check to sync storage with nftables counters
	logger.Info("Performing initial sync with nftables...")
	if err := d.checkAndUpdate(); err != nil {
		logger.Error("Initial sync failed: %v", err)
	} else {
		logger.Info("Initial sync completed")
	}

	// Send startup notification
	if d.notifier != nil {
		d.notifier.NotifySystemEvent(fmt.Sprintf("OQM daemon started - monitoring %d users", len(users)))
	}

	// Start monitoring loop
	go d.monitorLoop()

	// Start reset scheduler
	go d.resetScheduler()

	logger.Info("Daemon started successfully")
	return nil
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	logger.Info("Stopping OQM daemon...")
	d.cancel()

	if d.notifier != nil {
		d.notifier.NotifySystemEvent("OQM daemon stopped")
	}
}

// monitorLoop runs the main monitoring loop
func (d *Daemon) monitorLoop() {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	// Check for trigger file every second
	triggerChecker := time.NewTicker(1 * time.Second)
	defer triggerChecker.Stop()

	cfg := d.storage.GetConfig()
	triggerFile := cfg.DataDir + "/.trigger"

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if err := d.checkAndUpdate(); err != nil {
				logger.Error("Monitor check failed: %v", err)
			}
		case <-triggerChecker.C:
			// Check if trigger file exists
			if d.checkTriggerFile(triggerFile) {
				logger.Info("Trigger file detected - forcing immediate check")
				if err := d.checkAndUpdate(); err != nil {
					logger.Error("Triggered check failed: %v", err)
				}
			}
		}
	}
}

// checkTriggerFile checks if trigger file exists and removes it
func (d *Daemon) checkTriggerFile(path string) bool {
	if _, err := os.Stat(path); err == nil {
		// File exists, remove it
		os.Remove(path)
		return true
	}
	return false
}

// isMACBased checks if user is tracked by MAC address
func (d *Daemon) isMACBased(user *storage.User) bool {
	return user.MAC != "" && user.MAC != "00:00:00:00:00:00"
}

// checkAndUpdate checks counters and updates user usage
func (d *Daemon) checkAndUpdate() error {
	// Reload storage from disk to get latest users (added via Web UI or CLI)
	if err := d.storage.Load(); err != nil {
		logger.Error("Failed to reload storage: %v", err)
		// Continue with cached data
	}

	// Get counters from nftables
	counters, err := d.nftMgr.GetCounters()
	if err != nil {
		return fmt.Errorf("failed to get counters: %w", err)
	}
	logger.Debug("Retrieved %d counters from nftables", len(counters))

	users := d.storage.GetAllUsers()
	logger.Debug("Processing %d users", len(users))
	checkedGroups := make(map[string]bool) // Track which groups we've already checked

	// Resolve MAC-based users to IPs via ARP
	for i, user := range users {
		if d.isMACBased(user) {
			// Lookup IP from ARP table
			ip, found := arp.LookupIP(user.MAC)
			if found {
				// IP found - update if changed
				if user.IP != ip && ip != "0.0.0.0" {
					oldIP := user.IP
					logger.Info("MAC %s resolved to IP: %s (was: %s)", user.MAC, ip, oldIP)

					// Remove old IP from nftables if it was being monitored
					if oldIP != "" && oldIP != "0.0.0.0" && d.monitoredIPs[oldIP] {
						logger.Info("Removing old IP %s from nftables (MAC changed IP)", oldIP)
						d.nftMgr.RemoveMonitoredIP(oldIP)
						delete(d.monitoredIPs, oldIP)
						if d.blockedIPs[oldIP] {
							delete(d.blockedIPs, oldIP)
						}
					}

					// Update user IP in storage using MAC as key
					err := d.storage.UpdateUserByMAC(user.MAC, func(su *storage.User) {
						su.IP = ip
					})
					if err != nil {
						logger.Error("Failed to update IP for MAC %s: %v", user.MAC, err)
						continue
					}

					d.storage.Save()

					// Update local copy
					users[i].IP = ip
				}
			} else {
				// MAC not found in ARP table - device is offline
				logger.Debug("MAC %s not found in ARP table - device offline", user.MAC)
				// Mark as offline (don't save this to storage, it's just for this cycle)
				users[i].IP = "0.0.0.0"
			}
		}
	}

	// Add any new users to nftables monitoring
	newIPsAdded := false
	for _, user := range users {
		// Skip MAC-based users that are offline (no IP resolved)
		if d.isMACBased(user) && (user.IP == "" || user.IP == "0.0.0.0") {
			continue
		}

		// Skip invalid IP addresses
		if user.IP == "" || user.IP == "0.0.0.0" {
			continue
		}

		// Check if this IP is already being monitored
		if !d.monitoredIPs[user.IP] {
			// New user - add to monitoring
			if err := d.nftMgr.AddMonitoredIP(user.IP); err != nil {
				logger.Error("Failed to add new user %s to monitoring: %v", user.IP, err)
			} else {
				logger.Info("Added new user %s to monitoring", user.IP)
				d.monitoredIPs[user.IP] = true
				newIPsAdded = true
			}
		}
	}

	// If new IPs were added, refresh counters to include them
	if newIPsAdded {
		logger.Debug("New IPs were added, refreshing counters from nftables")
		freshCounters, err := d.nftMgr.GetCounters()
		if err != nil {
			logger.Error("Failed to refresh counters: %v", err)
		} else {
			counters = freshCounters
			logger.Debug("Refreshed counters: now have %d IPs", len(counters))
		}
	}

	for _, user := range users {
		// Skip MAC-based users that are offline
		if d.isMACBased(user) && user.IP == "0.0.0.0" {
			continue
		}

		// First check for manual block/unblock changes (from Web UI)
		// This must happen regardless of traffic
		counter, exists := counters[user.IP]

		// Check if block status changed manually
		if user.IsBlocked {
			// User should be blocked - ensure it's blocked in nftables
			if !d.blockedIPs[user.IP] {
				if err := d.nftMgr.BlockIP(user.IP); err != nil {
					logger.Error("Failed to block %s: %v", user.IP, err)
				} else {
					d.blockedIPs[user.IP] = true
				}
			}
		} else {
			// User should NOT be blocked - unblock if currently blocked
			if d.blockedIPs[user.IP] {
				if err := d.nftMgr.UnblockIP(user.IP); err != nil {
					logger.Error("Failed to unblock %s: %v", user.IP, err)
				} else {
					d.blockedIPs[user.IP] = false
				}
			}
		}

		if !exists {
			logger.Debug("No counter found for %s (not in nftables yet)", user.IP)

			// Still check quota for blocked users (they might need auto-unblock)
			if user.IsBlocked {
				if user.Username != "" && user.Username != "-" && user.GroupQuotaMB > 0 {
					if !checkedGroups[user.Username] {
						d.checkQuota(user)
						checkedGroups[user.Username] = true
					}
				} else {
					d.checkQuota(user)
				}
			}

			continue // No traffic for this user yet
		}

		// Update usage in storage
		// nftables now tracks RX (download) and TX (upload) separately
		rxBytes := counter.RxBytes
		txBytes := counter.TxBytes

		// Check quota even if usage hasn't changed (for blocked users that need auto-unblock)
		shouldCheckQuota := false

		// Only update if changed
		if rxBytes != user.RxBytes || txBytes != user.TxBytes {
			logger.Debug("Updating usage for %s: RX=%d, TX=%d (was RX=%d, TX=%d)",
				user.IP, rxBytes, txBytes, user.RxBytes, user.TxBytes)

			err := d.storage.UpdateUserUsage(user.IP, rxBytes, txBytes)
			if err != nil {
				logger.Error("Failed to update usage for %s: %v", user.IP, err)
				continue
			}

			// Save to disk immediately
			if err := d.storage.Save(); err != nil {
				logger.Error("Failed to save storage: %v", err)
			} else {
				logger.Debug("Saved usage for %s to disk", user.IP)
			}

			shouldCheckQuota = true
		} else if user.IsBlocked {
			// Usage hasn't changed but user is blocked - check if they should be unblocked
			shouldCheckQuota = true
		}

		// Check quota if needed (only once per group)
		if shouldCheckQuota {
			updatedUser, _ := d.storage.GetUser(user.IP)
			if updatedUser != nil {
				// For grouped users (with group quota), only check quota once per group
				if updatedUser.Username != "" && updatedUser.Username != "-" && updatedUser.GroupQuotaMB > 0 {
					if !checkedGroups[updatedUser.Username] {
						d.checkQuota(updatedUser)
						checkedGroups[updatedUser.Username] = true
					}
				} else {
					// Individual user (no group quota) - always check
					d.checkQuota(updatedUser)
				}
			}
		}
	}

	return nil
}

// checkQuota checks if user exceeded quota and takes action
// Checks both individual device quota AND group quota (if applicable)
func (d *Daemon) checkQuota(user *storage.User) {
	// Check if user is currently blocked and should be auto-unblocked
	if user.IsBlocked {
		deviceOK := true  // Assume device quota is OK (unlimited or under limit)
		groupOK := true   // Assume group quota is OK (unlimited or under limit)

		// Check device quota if it exists
		if user.QuotaMB > 0 {
			if user.IsQuotaExceeded() {
				deviceOK = false // Device quota exceeded
				logger.Debug("User %s (%s) still over device quota (%.1f%% of %d MB)",
					user.Name, user.IP, user.QuotaUsagePercent(), user.QuotaMB)
			} else {
				logger.Info("User %s (%s) is now under device quota (%.1f%% of %d MB)",
					user.Name, user.IP, user.QuotaUsagePercent(), user.QuotaMB)
			}
		}

		// Check group quota if user is in a group
		if user.Username != "" && user.Username != "-" && user.GroupQuotaMB > 0 {
			groupUsers := d.storage.GetUsersByUsername(user.Username)
			var totalBytes int64
			for _, u := range groupUsers {
				totalBytes += u.TotalBytes()
			}

			if storage.IsGroupQuotaExceeded(totalBytes, user.GroupQuotaMB) {
				groupOK = false // Group quota exceeded
				logger.Debug("Group %s still over quota (%.1f%% of %d MB)",
					user.Username, storage.GroupQuotaUsagePercent(totalBytes, user.GroupQuotaMB),
					user.GroupQuotaMB)
			} else {
				logger.Info("Group %s is now under group quota (%.1f%% of %d MB)",
					user.Username, storage.GroupQuotaUsagePercent(totalBytes, user.GroupQuotaMB),
					user.GroupQuotaMB)
			}
		}

		// Unblock only if BOTH device AND group quotas are OK
		if deviceOK && groupOK {
			d.unblockUser(user, "quota now under limit (quota was increased or usage decreased)")
		}

		return // Don't check for blocking again if already blocked
	}

	// Check individual device quota first
	if user.QuotaMB > 0 {
		deviceUsagePercent := user.QuotaUsagePercent()
		deviceExceeded := user.IsQuotaExceeded()

		if deviceUsagePercent >= 80 && deviceUsagePercent < 100 {
			// Only send warning if not already sent
			if !user.DeviceWarningNotificationSent {
				d.sendQuotaWarning(user, deviceUsagePercent, "device")
				// Mark as sent
				d.storage.UpdateUser(user.IP, func(u *storage.User) {
					u.DeviceWarningNotificationSent = true
				})
				d.storage.Save()
			}
		} else if deviceUsagePercent < 80 {
			// Reset warning flag if usage drops below threshold
			if user.DeviceWarningNotificationSent {
				d.storage.UpdateUser(user.IP, func(u *storage.User) {
					u.DeviceWarningNotificationSent = false
				})
				d.storage.Save()
			}
		}

		if deviceExceeded {
			d.blockUser(user, "device quota exceeded")
			return // Already blocked, no need to check group quota
		}
	}

	// Check group quota if user is part of a group
	if user.Username != "" && user.Username != "-" && user.GroupQuotaMB > 0 {
		// Get all users in the same group
		groupUsers := d.storage.GetUsersByUsername(user.Username)

		// Calculate total usage for the group
		var totalBytes int64
		for _, u := range groupUsers {
			totalBytes += u.TotalBytes()
		}

		// Check group quota
		groupUsagePercent := storage.GroupQuotaUsagePercent(totalBytes, user.GroupQuotaMB)
		groupExceeded := storage.IsGroupQuotaExceeded(totalBytes, user.GroupQuotaMB)

		logger.Debug("Group %s: %.2f%% of %d MB (total: %d bytes)", user.Username, groupUsagePercent, user.GroupQuotaMB, totalBytes)

		if groupUsagePercent >= 80 && groupUsagePercent < 100 {
			// Only send warning if not already sent (check first user in group)
			if len(groupUsers) > 0 && !groupUsers[0].GroupWarningNotificationSent {
				d.sendGroupQuotaWarning(user.Username, groupUsers, groupUsagePercent)
				// Mark as sent for all users in group
				for _, u := range groupUsers {
					d.storage.UpdateUser(u.IP, func(usr *storage.User) {
						usr.GroupWarningNotificationSent = true
					})
				}
				d.storage.Save()
			}
		} else if groupUsagePercent < 80 {
			// Reset warning flag if usage drops below threshold
			if len(groupUsers) > 0 && groupUsers[0].GroupWarningNotificationSent {
				for _, u := range groupUsers {
					d.storage.UpdateUser(u.IP, func(usr *storage.User) {
						usr.GroupWarningNotificationSent = false
					})
				}
				d.storage.Save()
			}
		}

		if groupExceeded {
			logger.Info("Group %s quota EXCEEDED - blocking all devices", user.Username)
			d.blockGroup(user.Username, groupUsers)
		}
	}
}

// sendQuotaWarning sends warning for individual device quota
func (d *Daemon) sendQuotaWarning(user *storage.User, usagePercent float64, quotaType string) {
	logger.Info("Device quota warning: %s (%s) at %.1f%%", user.Name, user.IP, usagePercent)

	if d.notifier == nil {
		return
	}

	cfg := d.storage.GetConfig()
	chatID := user.TelegramChatID

	// Send to device if it has a chat ID
	if chatID != "" {
		err := d.notifier.NotifyQuotaWarning(user.Name, user.IP, chatID, usagePercent)
		if err != nil {
			logger.Error("Failed to send device quota warning: %v", err)
		}
	}

	// ALSO send to admin
	if cfg.AdminChatID != "" && cfg.AdminChatID != chatID {
		err := d.notifier.NotifyQuotaWarning(user.Name+" (device)", user.IP, cfg.AdminChatID, usagePercent)
		if err != nil {
			logger.Error("Failed to send quota warning to admin: %v", err)
		}
	}
}

// sendGroupQuotaWarning sends warning for group quota
func (d *Daemon) sendGroupQuotaWarning(username string, groupUsers []*storage.User, usagePercent float64) {
	logger.Info("Group quota warning: %s at %.1f%% (%d devices)", username, usagePercent, len(groupUsers))

	if d.notifier == nil || len(groupUsers) == 0 {
		return
	}

	cfg := d.storage.GetConfig()
	firstUser := groupUsers[0]

	// Use GroupTelegramChatID if set, otherwise fallback to device's TelegramChatID
	chatID := firstUser.GroupTelegramChatID
	if chatID == "" {
		chatID = d.storage.GetGroupTelegramChatID(username)
	}

	// Send to group if they have a chat ID
	if chatID != "" {
		err := d.notifier.NotifyQuotaWarning(username+" (group)", firstUser.IP, chatID, usagePercent)
		if err != nil {
			logger.Error("Failed to send group quota warning: %v", err)
		}
	}

	// ALSO send to admin
	if cfg.AdminChatID != "" && cfg.AdminChatID != chatID {
		err := d.notifier.NotifyQuotaWarning(username+" (group)", firstUser.IP, cfg.AdminChatID, usagePercent)
		if err != nil {
			logger.Error("Failed to send group quota warning to admin: %v", err)
		}
	}
}

// blockUser blocks an individual device
func (d *Daemon) blockUser(user *storage.User, reason string) {
	logger.Info("Blocking device: %s (%s) - %s", user.Name, user.IP, reason)

	// Block in nftables
	if err := d.nftMgr.BlockIP(user.IP); err != nil {
		logger.Error("Failed to block %s: %v", user.IP, err)
		return
	}

	// Track blocked status
	d.blockedIPs[user.IP] = true

	// Update storage
	if err := d.storage.BlockUser(user.IP); err != nil {
		logger.Error("Failed to update block status for %s: %v", user.IP, err)
		return
	}

	d.storage.Save()

	// Send notification
	if d.notifier != nil {
		cfg := d.storage.GetConfig()
		chatID := user.TelegramChatID

		// Send to device
		if chatID != "" {
			err := d.notifier.NotifyQuotaExceeded(user.Name, user.IP, chatID)
			if err != nil {
				logger.Error("Failed to send quota exceeded notification: %v", err)
			}
		}

		// ALSO send to admin
		if cfg.AdminChatID != "" && cfg.AdminChatID != chatID {
			err := d.notifier.NotifyQuotaExceeded(user.Name+" (device)", user.IP, cfg.AdminChatID)
			if err != nil {
				logger.Error("Failed to send quota exceeded notification to admin: %v", err)
			}
		}
	}
}

// unblockUser unblocks a user (called when quota is increased or usage is reset)
func (d *Daemon) unblockUser(user *storage.User, reason string) {
	logger.Info("Unblocking device: %s (%s) - %s", user.Name, user.IP, reason)

	// Unblock in nftables
	if err := d.nftMgr.UnblockIP(user.IP); err != nil {
		logger.Error("Failed to unblock %s: %v", user.IP, err)
		return
	}

	// Track unblocked status
	d.blockedIPs[user.IP] = false

	// Update storage
	if err := d.storage.UnblockUser(user.IP); err != nil {
		logger.Error("Failed to update unblock status for %s: %v", user.IP, err)
		return
	}

	d.storage.Save()

	// Send notification (optional - you can add a "you're unblocked" notification if desired)
	if d.notifier != nil {
		cfg := d.storage.GetConfig()
		chatID := user.TelegramChatID

		// Send to device
		if chatID != "" {
			message := fmt.Sprintf("✅ *Access Restored*\n\nUser: `%s`\nIP: `%s`\n\nYour internet access has been restored.", user.Name, user.IP)
			err := d.notifier.SendToUser(chatID, message)
			if err != nil {
				logger.Error("Failed to send unblock notification: %v", err)
			}
		}

		// ALSO send to admin
		if cfg.AdminChatID != "" && cfg.AdminChatID != chatID {
			message := fmt.Sprintf("✅ *User Unblocked*\n\nUser: `%s`\nIP: `%s`\n\nReason: %s", user.Name, user.IP, reason)
			err := d.notifier.SendToAdmin(message)
			if err != nil {
				logger.Error("Failed to send unblock notification to admin: %v", err)
			}
		}
	}
}

// blockGroup blocks all devices in a group
func (d *Daemon) blockGroup(username string, groupUsers []*storage.User) {
	logger.Info("Group quota exceeded: %s - blocking all %d devices", username, len(groupUsers))

	// Block all devices in the group
	for _, u := range groupUsers {
		if u.IsBlocked {
			continue // Already blocked
		}

		// Block in nftables
		if err := d.nftMgr.BlockIP(u.IP); err != nil {
			logger.Error("Failed to block %s: %v", u.IP, err)
			continue
		}

		// Track blocked status
		d.blockedIPs[u.IP] = true

		// Update storage
		if err := d.storage.BlockUser(u.IP); err != nil {
			logger.Error("Failed to update block status for %s: %v", u.IP, err)
			continue
		}
	}

	d.storage.Save()

	// Send notification (only once for the group)
	if d.notifier != nil && len(groupUsers) > 0 {
		cfg := d.storage.GetConfig()
		firstUser := groupUsers[0]

		// Use GroupTelegramChatID if set
		chatID := firstUser.GroupTelegramChatID
		if chatID == "" {
			chatID := d.storage.GetGroupTelegramChatID(username)
			if chatID == "" {
				chatID = firstUser.TelegramChatID
			}
		}

		// Send to group
		if chatID != "" {
			err := d.notifier.NotifyQuotaExceeded(username+" (group)", firstUser.IP, chatID)
			if err != nil {
				logger.Error("Failed to send group quota exceeded notification: %v", err)
			}
		}

		// ALSO send to admin
		if cfg.AdminChatID != "" && cfg.AdminChatID != chatID {
			err := d.notifier.NotifyQuotaExceeded(username+" (group)", firstUser.IP, cfg.AdminChatID)
			if err != nil {
				logger.Error("Failed to send group quota exceeded notification to admin: %v", err)
			}
		}
	}
}

// resetScheduler handles scheduled resets
func (d *Daemon) resetScheduler() {
	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	lastReset := time.Now()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			cfg := d.storage.GetConfig()
			shouldReset := false

			switch cfg.ResetSchedule {
			case "daily":
				if time.Since(lastReset) >= 24*time.Hour {
					shouldReset = true
				}
			case "weekly":
				if time.Since(lastReset) >= 7*24*time.Hour {
					shouldReset = true
				}
			case "monthly":
				// Reset on first day of month
				now := time.Now()
				if now.Day() == 1 && lastReset.Month() != now.Month() {
					shouldReset = true
				}
			}

			if shouldReset {
				logger.Info("Scheduled reset triggered (%s)", cfg.ResetSchedule)
				d.resetAllUsers()
				lastReset = time.Now()
			}
		}
	}
}

// resetAllUsers resets usage for all users
func (d *Daemon) resetAllUsers() {
	users := d.storage.GetAllUsers()

	for _, u := range users {
		// Reset counter in nftables
		if err := d.nftMgr.ResetCounter(u.IP); err != nil {
			logger.Error("Failed to reset counter for %s: %v", u.IP, err)
		}

		// Unblock if blocked
		if u.IsBlocked {
			if err := d.nftMgr.UnblockIP(u.IP); err != nil {
				logger.Error("Failed to unblock %s: %v", u.IP, err)
			}
		}

		// Send individual user notification
		if d.notifier != nil {
			chatID := u.TelegramChatID
			if chatID == "" && u.GroupTelegramChatID != "" {
				chatID = u.GroupTelegramChatID
			}
			if err := d.notifier.NotifyUserReset(u.Name, u.IP, chatID); err != nil {
				logger.Error("Failed to send reset notification to %s: %v", u.Name, err)
			}
		}
	}

	// Reset in storage
	if err := d.storage.ResetAllUsage(); err != nil {
		logger.Error("Failed to reset storage: %v", err)
		return
	}

	d.storage.Save()

	logger.Info("Reset completed for all users")

	// Send system notification
	if d.notifier != nil {
		d.notifier.NotifySystemEvent(fmt.Sprintf("Scheduled reset completed - %d users reset", len(users)))
	}
}

// Wait blocks until daemon is stopped
func (d *Daemon) Wait() {
	<-d.ctx.Done()
}
