package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/abdollahzadehghalejoghi/oqm/internal/nft"
	"github.com/abdollahzadehghalejoghi/oqm/internal/notify"
	"github.com/abdollahzadehghalejoghi/oqm/internal/storage"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/config"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/logger"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users with their usage and status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			users := store.GetAllUsers()
			if len(users) == 0 {
				fmt.Println("No users found.")
				return nil
			}

			table := NewTable("IP", "MAC", "Name", "Username", "Quota", "Used", "Usage%", "Status")
			for _, u := range users {
				status := "Active"
				if u.IsBlocked {
					status = "Blocked"
				}
				quota := fmt.Sprintf("%d MB", u.QuotaMB)
				if u.QuotaMB == 0 {
					quota = "Unlimited"
				}
				table.AddRow(
					u.IP,
					u.MAC,
					u.Name,
					u.Username,
					quota,
					FormatBytes(u.TotalBytes()),
					FormatPercent(u.QuotaUsagePercent()),
					status,
				)
			}

			table.Render(os.Stdout)
			return nil
		},
	}
}

// NewAddUserCommand creates the add-user command
func NewAddUserCommand() *cobra.Command {
	var ip, mac, name, username, telegramChatID string
	var quota int64

	cmd := &cobra.Command{
		Use:   "add-user",
		Short: "Add a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ip == "" || mac == "" {
				return fmt.Errorf("--ip and --mac are required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			user := &storage.User{
				IP:             ip,
				MAC:            mac,
				Name:           name,
				Username:       username,
				QuotaMB:        quota,
				TelegramChatID: telegramChatID,
			}

			if err := store.AddUser(user); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			// Add to nftables
			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			if err := nftMgr.AddMonitoredIP(ip); err != nil {
				logger.Error("Failed to add IP to nftables: %v", err)
			}

			fmt.Printf("User %s (%s) added successfully\n", name, ip)
			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address (required)")
	cmd.Flags().StringVar(&mac, "mac", "", "User MAC address (required)")
	cmd.Flags().StringVar(&name, "name", "", "User display name")
	cmd.Flags().StringVar(&username, "username", "", "Username for grouping devices")
	cmd.Flags().Int64Var(&quota, "quota", 0, "Quota in MB (0 = unlimited)")
	cmd.Flags().StringVar(&telegramChatID, "telegram", "", "Telegram chat ID for notifications")

	return cmd
}

// NewRemoveUserCommand creates the remove-user command
func NewRemoveUserCommand() *cobra.Command {
	var ip string

	cmd := &cobra.Command{
		Use:   "remove-user",
		Short: "Remove a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ip == "" {
				return fmt.Errorf("--ip is required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			// Remove from nftables first
			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			nftMgr.RemoveMonitoredIP(ip)
			nftMgr.UnblockIP(ip)

			if err := store.RemoveUser(ip); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("User %s removed successfully\n", ip)
			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address (required)")
	return cmd
}

// NewUpdateQuotaCommand creates the update-quota command
func NewUpdateQuotaCommand() *cobra.Command {
	var ip string
	var quota int64

	cmd := &cobra.Command{
		Use:   "update-quota",
		Short: "Update user quota",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ip == "" {
				return fmt.Errorf("--ip is required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			err = store.UpdateUser(ip, func(u *storage.User) {
				u.QuotaMB = quota
			})
			if err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("Quota updated for %s: %d MB\n", ip, quota)
			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address (required)")
	cmd.Flags().Int64Var(&quota, "quota", 0, "New quota in MB")
	return cmd
}

// NewUsageCommand creates the usage command
func NewUsageCommand() *cobra.Command {
	var ip string
	var all bool

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Display usage for user(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			if all {
				users := store.GetAllUsers()
				table := NewTable("IP", "Name", "Download", "Upload", "Total", "Quota", "Usage%")
				for _, u := range users {
					quota := fmt.Sprintf("%d MB", u.QuotaMB)
					if u.QuotaMB == 0 {
						quota = "Unlimited"
					}
					table.AddRow(
						u.IP,
						u.Name,
						FormatBytes(u.RxBytes),
						FormatBytes(u.TxBytes),
						FormatBytes(u.TotalBytes()),
						quota,
						FormatPercent(u.QuotaUsagePercent()),
					)
				}
				table.Render(os.Stdout)
			} else {
				if ip == "" {
					return fmt.Errorf("--ip or --all is required")
				}

				user, err := store.GetUser(ip)
				if err != nil {
					return err
				}

				fmt.Printf("Usage for %s (%s):\n", user.Name, user.IP)
				fmt.Printf("  Download: %s\n", FormatBytes(user.RxBytes))
				fmt.Printf("  Upload:   %s\n", FormatBytes(user.TxBytes))
				fmt.Printf("  Total:    %s\n", FormatBytes(user.TotalBytes()))
				if user.QuotaMB > 0 {
					fmt.Printf("  Quota:    %d MB (%.1f%% used)\n", user.QuotaMB, user.QuotaUsagePercent())
				} else {
					fmt.Printf("  Quota:    Unlimited\n")
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address")
	cmd.Flags().BoolVar(&all, "all", false, "Show usage for all users")
	return cmd
}

// NewTopCommand creates the top command
func NewTopCommand() *cobra.Command {
	var rx, tx bool
	var limit int

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show top users by traffic",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			users := store.GetAllUsers()

			// Sort based on flag
			if rx {
				sort.Slice(users, func(i, j int) bool {
					return users[i].RxBytes > users[j].RxBytes
				})
				fmt.Println("Top users by download:")
			} else if tx {
				sort.Slice(users, func(i, j int) bool {
					return users[i].TxBytes > users[j].TxBytes
				})
				fmt.Println("Top users by upload:")
			} else {
				sort.Slice(users, func(i, j int) bool {
					return users[i].TotalBytes() > users[j].TotalBytes()
				})
				fmt.Println("Top users by total traffic:")
			}

			// Apply limit
			if limit > 0 && limit < len(users) {
				users = users[:limit]
			}

			table := NewTable("Rank", "IP", "Name", "Download", "Upload", "Total")
			for i, u := range users {
				table.AddRow(
					fmt.Sprintf("%d", i+1),
					u.IP,
					u.Name,
					FormatBytes(u.RxBytes),
					FormatBytes(u.TxBytes),
					FormatBytes(u.TotalBytes()),
				)
			}

			table.Render(os.Stdout)
			return nil
		},
	}

	cmd.Flags().BoolVar(&rx, "rx", false, "Sort by download")
	cmd.Flags().BoolVar(&tx, "tx", false, "Sort by upload")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of users to show")
	return cmd
}

// NewBlockCommand creates the block command
func NewBlockCommand() *cobra.Command {
	var ip string

	cmd := &cobra.Command{
		Use:   "block",
		Short: "Block a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ip == "" {
				return fmt.Errorf("--ip is required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			// Block in nftables
			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			if err := nftMgr.BlockIP(ip); err != nil {
				return err
			}

			// Update storage
			if err := store.BlockUser(ip); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("User %s blocked successfully\n", ip)
			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address (required)")
	return cmd
}

// NewUnblockCommand creates the unblock command
func NewUnblockCommand() *cobra.Command {
	var ip string

	cmd := &cobra.Command{
		Use:   "unblock",
		Short: "Unblock a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ip == "" {
				return fmt.Errorf("--ip is required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			// Unblock in nftables
			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			if err := nftMgr.UnblockIP(ip); err != nil {
				return err
			}

			// Update storage
			if err := store.UnblockUser(ip); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("User %s unblocked successfully\n", ip)
			return nil
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address (required)")
	return cmd
}

// NewResetUsageCommand creates the reset-usage command
func NewResetUsageCommand() *cobra.Command {
	var ip string
	var all bool

	cmd := &cobra.Command{
		Use:   "reset-usage",
		Short: "Reset usage for user(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)

			// Create notifier if configured
			cfg := store.GetConfig()
			var notifier *notify.TelegramNotifier
			if cfg.BotToken != "" && cfg.AdminChatID != "" {
				baseURL := ""
				if cfg.BotType == "bale" {
					baseURL = "https://tapi.bale.ai"
				}
				notifier, err = notify.NewTelegramNotifier(cfg.BotToken, cfg.AdminChatID, baseURL)
				if err != nil {
					logger.Error("Failed to create notifier: %v", err)
					// Continue without notifier
				}
			}

			if all {
				// Reset all users
				users := store.GetAllUsers()
				for _, u := range users {
					nftMgr.ResetCounter(u.IP)

					// Send individual user notification
					if notifier != nil {
						chatID := u.TelegramChatID
						if chatID == "" && u.GroupTelegramChatID != "" {
							chatID = u.GroupTelegramChatID
						}
						if err := notifier.NotifyUserReset(u.Name, u.IP, chatID); err != nil {
							logger.Error("Failed to send reset notification to %s: %v", u.Name, err)
						}
					}
				}
				if err := store.ResetAllUsage(); err != nil {
					return err
				}

				// Send system notification for all reset
				if notifier != nil {
					if err := notifier.NotifySystemEvent(fmt.Sprintf("Manual reset completed - %d users reset", len(users))); err != nil {
						logger.Error("Failed to send system notification: %v", err)
					}
				}

				fmt.Println("Usage reset for all users")
			} else {
				if ip == "" {
					return fmt.Errorf("--ip or --all is required")
				}

				// Get user info for notification
				user, err := store.GetUser(ip)
				if err != nil {
					return err
				}

				nftMgr.ResetCounter(ip)
				if err := store.ResetUserUsage(ip); err != nil {
					return err
				}

				// Send notification for single user reset
				if notifier != nil && user != nil {
					chatID := user.TelegramChatID
					if chatID == "" && user.GroupTelegramChatID != "" {
						chatID = user.GroupTelegramChatID
					}
					if err := notifier.NotifyUserReset(user.Name, user.IP, chatID); err != nil {
						logger.Error("Failed to send reset notification: %v", err)
					}
				}

				fmt.Printf("Usage reset for %s\n", ip)
			}

			return store.Save()
		},
	}

	cmd.Flags().StringVar(&ip, "ip", "", "User IP address")
	cmd.Flags().BoolVar(&all, "all", false, "Reset usage for all users")
	return cmd
}

// NewConfigCommand creates the config command
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	// config --set <key> <value>
	var setKey, setValue string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set a configuration value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setKey == "" || setValue == "" {
				return fmt.Errorf("both --key and --value are required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			store.UpdateConfig(func(c *storage.Config) {
				switch setKey {
				case "check-interval":
					fmt.Sscanf(setValue, "%d", &c.CheckInterval)
				case "web-port":
					fmt.Sscanf(setValue, "%d", &c.WebPort)
				case "bot-type":
					c.BotType = setValue
				case "bot-token":
					c.BotToken = setValue
				case "bot-api-base-url":
					c.BotAPIBaseURL = setValue
				case "admin-chat-id":
					c.AdminChatID = setValue
				case "reset-schedule":
					c.ResetSchedule = setValue
				case "nftables-table":
					c.NFTablesTable = setValue
				case "data-dir":
					c.DataDir = setValue
				case "log-file":
					c.LogFile = setValue
				default:
					fmt.Printf("Unknown config key: %s\n", setKey)
					return
				}
			})

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("Config updated: %s = %s\n", setKey, setValue)
			return nil
		},
	}
	setCmd.Flags().StringVar(&setKey, "key", "", "Config key")
	setCmd.Flags().StringVar(&setValue, "value", "", "Config value")

	// config --get <key>
	var getKey string
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a configuration value",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			cfg := store.GetConfig()

			if getKey == "" {
				// Show all config
				fmt.Printf("Check Interval:         %d seconds\n", cfg.CheckInterval)
				fmt.Printf("Web Port:               %d\n", cfg.WebPort)
				fmt.Printf("\n--- Bot Settings ---\n")

				// Show bot settings
				if cfg.BotType != "" || cfg.BotToken != "" {
					fmt.Printf("Bot Type:               %s\n", cfg.BotType)
					fmt.Printf("Bot Token:              %s\n", cfg.BotToken)
					if cfg.BotAPIBaseURL != "" {
						fmt.Printf("Bot API Base URL:       %s\n", cfg.BotAPIBaseURL)
					}
					fmt.Printf("Admin Chat ID:          %s\n", cfg.AdminChatID)
				}

				fmt.Printf("\n--- General Settings ---\n")
				fmt.Printf("Reset Schedule:         %s\n", cfg.ResetSchedule)
				fmt.Printf("NFTables Table:         %s\n", cfg.NFTablesTable)
				fmt.Printf("Data Directory:         %s\n", cfg.DataDir)
				fmt.Printf("Log File:               %s\n", cfg.LogFile)
			} else {
				switch getKey {
				case "check-interval":
					fmt.Println(cfg.CheckInterval)
				case "web-port":
					fmt.Println(cfg.WebPort)
				case "bot-type":
					fmt.Println(cfg.BotType)
				case "bot-token":
					fmt.Println(cfg.BotToken)
				case "bot-api-base-url":
					fmt.Println(cfg.BotAPIBaseURL)
				case "admin-chat-id":
					fmt.Println(cfg.AdminChatID)
				case "reset-schedule":
					fmt.Println(cfg.ResetSchedule)
				case "nftables-table":
					fmt.Println(cfg.NFTablesTable)
				case "data-dir":
					fmt.Println(cfg.DataDir)
				case "log-file":
					fmt.Println(cfg.LogFile)
				default:
					return fmt.Errorf("unknown config key: %s", getKey)
				}
			}

			return nil
		},
	}
	getCmd.Flags().StringVar(&getKey, "key", "", "Config key (optional, shows all if not provided)")

	cmd.AddCommand(setCmd, getCmd)
	return cmd
}

// NewDaemonCommand creates the daemon command
func NewDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the OQM daemon",
	}

	// Add run subcommand
	cmd.AddCommand(NewDaemonRunCommand())

	return cmd
}

// NewNftCommand creates the nft command
func NewNftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nft",
		Short: "Manage nftables",
	}

	var showCmd = &cobra.Command{
		Use:   "show",
		Short: "Show nftables configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			output, err := nftMgr.Show()
			if err != nil {
				return err
			}

			fmt.Println(output)
			return nil
		},
	}

	var listMonitoredCmd = &cobra.Command{
		Use:   "list-monitored",
		Short: "List monitored IPs in nftables",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			ips, err := nftMgr.ListMonitoredIPs()
			if err != nil {
				return err
			}

			if len(ips) == 0 {
				fmt.Println("No IPs are being monitored in nftables.")
				return nil
			}

			fmt.Println("Monitored IPs:")
			for _, ip := range ips {
				// Try to get counter for this IP
				counter, err := nftMgr.GetIPCounter(ip)
				if err == nil && counter != nil {
					fmt.Printf("  %s - RX: %s, TX: %s, Total: %s\n",
						ip,
						FormatBytes(counter.RxBytes),
						FormatBytes(counter.TxBytes),
						FormatBytes(counter.TotalBytes()),
					)
				} else {
					fmt.Printf("  %s\n", ip)
				}
			}
			fmt.Printf("\nTotal: %d IPs monitored\n", len(ips))
			return nil
		},
	}

	var listBlockedCmd = &cobra.Command{
		Use:   "list-blocked",
		Short: "List blocked IPs in nftables",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			nftMgr := nft.NewManager(store.GetConfig().NFTablesTable)
			ips, err := nftMgr.ListBlockedIPs()
			if err != nil {
				return err
			}

			if len(ips) == 0 {
				fmt.Println("No IPs are blocked.")
				return nil
			}

			fmt.Println("Blocked IPs:")
			for _, ip := range ips {
				fmt.Printf("  %s\n", ip)
			}
			fmt.Printf("\nTotal: %d IPs blocked\n", len(ips))
			return nil
		},
	}

	cmd.AddCommand(showCmd, listMonitoredCmd, listBlockedCmd)
	return cmd
}

// NewScanDHCPCommand creates the scan-dhcp command
func NewScanDHCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scan-dhcp",
		Short: "Scan DHCP leases and display active clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import dhcp package
			scanner := &dhcpScanner{}
			leases, err := scanner.scan()
			if err != nil {
				return err
			}

			if len(leases) == 0 {
				fmt.Println("No active DHCP leases found.")
				return nil
			}

			table := NewTable("IP", "MAC", "Hostname", "Expires")
			for _, l := range leases {
				table.AddRow(l.IP, l.MAC, l.Hostname, l.Expires.Format("2006-01-02 15:04:05"))
			}

			table.Render(os.Stdout)
			fmt.Printf("\nTotal active leases: %d\n", len(leases))
			return nil
		},
	}
}

// dhcpScanner is a simple DHCP lease scanner
type dhcpScanner struct{}

type dhcpLease struct {
	IP       string
	MAC      string
	Hostname string
	Expires  time.Time
}

func (s *dhcpScanner) scan() ([]*dhcpLease, error) {
	file, err := os.Open("/tmp/dhcp.leases")
	if err != nil {
		return nil, fmt.Errorf("failed to open DHCP leases file: %w", err)
	}
	defer file.Close()

	var leases []*dhcpLease
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 4 {
			continue
		}

		var expiryTs int64
		fmt.Sscanf(fields[0], "%d", &expiryTs)

		lease := &dhcpLease{
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: fields[3],
			Expires:  time.Unix(expiryTs, 0),
		}

		// Only include active leases
		if lease.Expires.After(time.Now()) {
			leases = append(leases, lease)
		}
	}

	return leases, scanner.Err()
}

// NewExportCommand creates the export command
func NewExportCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export data to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = fmt.Sprintf("oqm-export-%s.json", time.Now().Format("20060102-150405"))
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			if err := store.ExportJSON(path); err != nil {
				return err
			}

			fmt.Printf("Data exported to %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "json", "", "Export file path (optional)")
	return cmd
}

// NewImportCommand creates the import command
func NewImportCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import data from JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--json is required")
			}

			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			if err := store.ImportJSON(path); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}

			fmt.Printf("Data imported from %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "json", "", "Import file path (required)")
	return cmd
}
