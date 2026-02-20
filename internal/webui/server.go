package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sort"

	"github.com/abdollahzadehghalejoghi/oqm/internal/nft"
	"github.com/abdollahzadehghalejoghi/oqm/internal/notify"
	"github.com/abdollahzadehghalejoghi/oqm/internal/storage"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/logger"
)

//go:embed templates/*
var templatesFS embed.FS

// Server handles HTTP requests for the web UI
type Server struct {
	storage *storage.Storage
	mux     *http.ServeMux
}

// NewServer creates a new web server
func NewServer(store *storage.Storage) *Server {
	s := &Server{
		storage: store,
		mux:     http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures HTTP routes
func (s *Server) setupRoutes() {
	// Static files
	staticFS, _ := fs.Sub(templatesFS, "templates")
	s.mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// API endpoints
	s.mux.HandleFunc("/api/users", s.corsMiddleware(s.handleUsers))
	s.mux.HandleFunc("/api/users/", s.corsMiddleware(s.handleUser))
	s.mux.HandleFunc("/api/stats", s.corsMiddleware(s.handleStats))
	s.mux.HandleFunc("/api/config", s.corsMiddleware(s.handleConfig))
	s.mux.HandleFunc("/api/dhcp-leases", s.corsMiddleware(s.handleDHCPLeases))
	s.mux.HandleFunc("/api/total-traffic", s.corsMiddleware(s.handleTotalTraffic))
	s.mux.HandleFunc("/api/reset-all", s.corsMiddleware(s.handleResetAll))
	s.mux.HandleFunc("/api/usernames", s.corsMiddleware(s.handleUsernames))
	s.mux.HandleFunc("/api/test-telegram", s.corsMiddleware(s.handleTestTelegram))
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// UserWithOnlineStatus extends User with online status
type UserWithOnlineStatus struct {
	*storage.User
	IsOnline bool `json:"is_online"`
}

// handleUsers handles /api/users endpoint
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Reload from disk to get latest data from daemon
		s.storage.Load()
		users := s.storage.GetAllUsers()

		// Get DHCP leases for online detection
		leases, err := s.scanDHCPLeases()
		if err != nil {
			logger.Error("Failed to scan DHCP leases: %v", err)
			leases = []DHCPLease{} // Use empty list if failed
		}

		// Build lookup maps for fast online detection
		ipMap := make(map[string]bool)
		macMap := make(map[string]bool)
		for _, lease := range leases {
			ipMap[lease.IP] = true
			macMap[lease.MAC] = true
		}

		// Add online status to each user
		usersWithStatus := make([]*UserWithOnlineStatus, len(users))
		for i, user := range users {
			isOnline := false

			// Check if user is online based on DHCP leases
			if user.MAC != "" && user.MAC != "00:00:00:00:00:00" {
				// MAC-based user: check if MAC is in DHCP leases
				isOnline = macMap[user.MAC]
			} else if user.IP != "" && user.IP != "0.0.0.0" {
				// IP-based user: check if IP is in DHCP leases
				isOnline = ipMap[user.IP]
			}

			usersWithStatus[i] = &UserWithOnlineStatus{
				User:     user,
				IsOnline: isOnline,
			}
		}

		s.jsonResponse(w, usersWithStatus)

	case "POST":
		var user storage.User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			s.errorResponse(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.storage.AddUser(&user); err != nil {
			s.errorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.storage.Save(); err != nil {
			s.errorResponse(w, "Failed to save", http.StatusInternalServerError)
			return
		}

		// Trigger immediate daemon check
		s.triggerDaemonCheck()

		s.jsonResponse(w, map[string]string{"status": "success"})

	default:
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUser handles /api/users/{ip} endpoint
func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Path[len("/api/users/"):]

	switch r.Method {
	case "GET":
		// Reload from disk to get latest data from daemon
		s.storage.Load()
		user, err := s.storage.GetUser(ip)
		if err != nil {
			s.errorResponse(w, "User not found", http.StatusNotFound)
			return
		}
		s.jsonResponse(w, user)

	case "PUT":
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			s.errorResponse(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		err := s.storage.UpdateUser(ip, func(u *storage.User) {
			if quota, ok := updates["quota_mb"].(float64); ok {
				u.QuotaMB = int64(quota)
			}
			if groupQuota, ok := updates["group_quota_mb"].(float64); ok {
				u.GroupQuotaMB = int64(groupQuota)
			}
			if name, ok := updates["name"].(string); ok {
				u.Name = name
			}
			if username, ok := updates["username"].(string); ok {
				u.Username = username
			}
			if telegramID, ok := updates["telegram_chat_id"].(string); ok {
				u.TelegramChatID = telegramID
			}
			if groupTelegramID, ok := updates["group_telegram_chat_id"].(string); ok {
				u.GroupTelegramChatID = groupTelegramID
			}
			if blocked, ok := updates["is_blocked"].(bool); ok {
				u.IsBlocked = blocked
			}
		})

		if err != nil {
			s.errorResponse(w, err.Error(), http.StatusNotFound)
			return
		}

		s.storage.Save()

		// Trigger immediate daemon check (especially for block/unblock changes)
		s.triggerDaemonCheck()

		s.jsonResponse(w, map[string]string{"status": "success"})

	case "DELETE":
		// Get nftables manager
		cfg := s.storage.GetConfig()
		nftMgr := nft.NewManager(cfg.NFTablesTable)

		// Remove from nftables first (monitoring and blocking)
		if err := nftMgr.RemoveMonitoredIP(ip); err != nil {
			logger.Error("Failed to remove IP from nftables monitoring: %v", err)
		}
		if err := nftMgr.UnblockIP(ip); err != nil {
			logger.Error("Failed to unblock IP: %v", err)
		}

		// Remove from storage
		if err := s.storage.RemoveUser(ip); err != nil {
			s.errorResponse(w, err.Error(), http.StatusNotFound)
			return
		}

		s.storage.Save()

		// Trigger immediate daemon check
		s.triggerDaemonCheck()

		s.jsonResponse(w, map[string]string{"status": "success"})

	default:
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats handles /api/stats endpoint
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	// Reload from disk to get latest data from daemon
	s.storage.Load()
	users := s.storage.GetAllUsers()

	totalUsers := len(users)
	activeUsers := 0
	blockedUsers := 0
	var totalTraffic int64

	for _, u := range users {
		if u.IsBlocked {
			blockedUsers++
		} else {
			activeUsers++
		}
		totalTraffic += u.TotalBytes()
	}

	// Top users
	sortedUsers := make([]*storage.User, len(users))
	copy(sortedUsers, users)
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].TotalBytes() > sortedUsers[j].TotalBytes()
	})

	topUsers := sortedUsers
	if len(topUsers) > 5 {
		topUsers = topUsers[:5]
	}

	stats := map[string]interface{}{
		"total_users":   totalUsers,
		"active_users":  activeUsers,
		"blocked_users": blockedUsers,
		"total_traffic": totalTraffic,
		"top_users":     topUsers,
	}

	s.jsonResponse(w, stats)
}

// handleConfig handles /api/config endpoint
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Reload from disk to get latest config
		s.storage.Load()
		cfg := s.storage.GetConfig()
		s.jsonResponse(w, cfg)

	case "PUT":
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			s.errorResponse(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		s.storage.UpdateConfig(func(c *storage.Config) {
			if interval, ok := updates["check_interval"].(float64); ok {
				c.CheckInterval = int(interval)
			}
			if port, ok := updates["web_port"].(float64); ok {
				c.WebPort = int(port)
			}
			// New fields
			if botType, ok := updates["bot_type"].(string); ok {
				c.BotType = botType
			}
			if token, ok := updates["bot_token"].(string); ok {
				c.BotToken = token
			}
			if baseURL, ok := updates["bot_api_base_url"].(string); ok {
				c.BotAPIBaseURL = baseURL
			}
			if chatID, ok := updates["admin_chat_id"].(string); ok {
				c.AdminChatID = chatID
			}
			if schedule, ok := updates["reset_schedule"].(string); ok {
				c.ResetSchedule = schedule
			}
			if dataDir, ok := updates["data_dir"].(string); ok {
				c.DataDir = dataDir
			}
			if logFile, ok := updates["log_file"].(string); ok {
				c.LogFile = logFile
			}
		})

		s.storage.Save()
		s.jsonResponse(w, map[string]string{"status": "success"})

	default:
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// jsonResponse sends a JSON response
func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// errorResponse sends an error response
func (s *Server) errorResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// handleDHCPLeases returns list of DHCP leases
func (s *Server) handleDHCPLeases(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	leases, err := s.scanDHCPLeases()
	if err != nil {
		s.errorResponse(w, "Failed to scan DHCP leases", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, leases)
}

// handleTotalTraffic returns total traffic from all interfaces
func (s *Server) handleTotalTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traffic, err := s.getTotalTraffic()
	if err != nil {
		s.errorResponse(w, "Failed to get total traffic", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, traffic)
}

// handleResetAll resets usage for all users
func (s *Server) handleResetAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get nftables manager
	cfg := s.storage.GetConfig()
	nftMgr := nft.NewManager(cfg.NFTablesTable)

	// Create notifier if configured
	var notifier *notify.TelegramNotifier
	if cfg.BotToken != "" && cfg.AdminChatID != "" {
		baseURL := ""
		if cfg.BotType == "bale" {
			baseURL = "https://tapi.bale.ai"
		}
		var err error
		notifier, err = notify.NewTelegramNotifier(cfg.BotToken, cfg.AdminChatID, baseURL)
		if err != nil {
			logger.Error("Failed to create notifier: %v", err)
			// Continue without notifier
		}
	}

	// Get all users to reset their counters in nftables
	users := s.storage.GetAllUsers()
	for _, user := range users {
		// Skip invalid IPs
		if user.IP == "" || user.IP == "0.0.0.0" {
			continue
		}

		// Reset counter in nftables (remove and re-add IP)
		if err := nftMgr.ResetCounter(user.IP); err != nil {
			logger.Error("Failed to reset nftables counter for %s: %v", user.IP, err)
			// Continue with other users even if one fails
		}

		// Send individual user notification
		if notifier != nil {
			chatID := user.TelegramChatID
			if chatID == "" && user.GroupTelegramChatID != "" {
				chatID = user.GroupTelegramChatID
			}
			if err := notifier.NotifyUserReset(user.Name, user.IP, chatID); err != nil {
				logger.Error("Failed to send reset notification to %s: %v", user.Name, err)
			}
		}
	}

	// Reset all users in storage
	if err := s.storage.ResetAllUsage(); err != nil {
		s.errorResponse(w, "Failed to reset usage", http.StatusInternalServerError)
		return
	}

	// Save to disk
	if err := s.storage.Save(); err != nil {
		s.errorResponse(w, "Failed to save", http.StatusInternalServerError)
		return
	}

	// Send system notification for all reset
	if notifier != nil {
		if err := notifier.NotifySystemEvent(fmt.Sprintf("Manual reset completed - %d users reset", len(users))); err != nil {
			logger.Error("Failed to send system notification: %v", err)
		}
	}

	s.jsonResponse(w, map[string]string{
		"status":  "success",
		"message": "All user usage has been reset",
	})
}

// handleUsernames returns list of existing usernames (for dropdown)
func (s *Server) handleUsernames(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Reload from disk to get latest data
	s.storage.Load()
	usernames := s.storage.GetAllUsernames()
	s.jsonResponse(w, usernames)
}

// handleTestTelegram sends a test message to verify bot connection (Telegram/Bale)
func (s *Server) handleTestTelegram(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current config
	cfg := s.storage.GetConfig()

	// Check if bot is configured
	if cfg.BotToken == "" {
		s.errorResponse(w, "Bot token is not configured", http.StatusBadRequest)
		return
	}

	if cfg.AdminChatID == "" {
		s.errorResponse(w, "Admin chat ID is not configured", http.StatusBadRequest)
		return
	}

	// Determine base URL
	var baseURL string
	if cfg.BotAPIBaseURL != "" {
		baseURL = cfg.BotAPIBaseURL
	} else if cfg.BotType == "bale" {
		baseURL = "https://tapi.bale.ai"
	}

	botTypeName := cfg.BotType
	if botTypeName == "" {
		botTypeName = "Telegram"
	}

	// Create temporary notifier
	notifier, err := notify.NewTelegramNotifier(cfg.BotToken, cfg.AdminChatID, baseURL)
	if err != nil {
		s.errorResponse(w, fmt.Sprintf("Failed to create bot: %v", err), http.StatusInternalServerError)
		return
	}

	// Send test message
	testMessage := fmt.Sprintf("✅ *%s Connection Test*\n\nYour OQM system is successfully connected to %s!\n\nYou will receive notifications here for:\n• Quota warnings (80%%)\n• Quota exceeded (100%%)\n• System events", botTypeName, botTypeName)
	err = notifier.SendToAdmin(testMessage)
	if err != nil {
		s.errorResponse(w, fmt.Sprintf("Failed to send test message: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Test message sent successfully! Check your %s.", botTypeName),
	})
}

// triggerDaemonCheck creates a trigger file to force daemon to check immediately
func (s *Server) triggerDaemonCheck() {
	cfg := s.storage.GetConfig()
	triggerFile := cfg.DataDir + "/.trigger"

	// Create empty trigger file
	if err := os.WriteFile(triggerFile, []byte{}, 0644); err != nil {
		logger.Error("Failed to create trigger file: %v", err)
	}
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting web server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}
