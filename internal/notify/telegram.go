package notify

import (
	"fmt"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramNotifier handles Telegram notifications
type TelegramNotifier struct {
	bot         *tgbotapi.BotAPI
	adminChatID string
}

// NewTelegramNotifier creates a new Telegram notifier
// baseURL can be empty (uses default), "https://tapi.bale.ai" for Bale, or custom endpoint
func NewTelegramNotifier(botToken, adminChatID, baseURL string) (*TelegramNotifier, error) {
	if botToken == "" {
		return nil, fmt.Errorf("bot token is empty")
	}

	// Create bot WITHOUT making API call first
	bot := &tgbotapi.BotAPI{
		Token:  botToken,
		Client: &http.Client{},
		Buffer: 100,
	}

	// Set custom base URL BEFORE any API calls (for Bale or custom endpoints)
	if baseURL != "" {
		bot.SetAPIEndpoint(baseURL + "/bot%s/%s")
	} else {
		// Use default Telegram API
		bot.SetAPIEndpoint(tgbotapi.APIEndpoint)
	}

	// Now verify the bot token by calling getMe
	_, err := bot.GetMe()
	if err != nil {
		return nil, fmt.Errorf("failed to verify bot token: %w", err)
	}

	return &TelegramNotifier{
		bot:         bot,
		adminChatID: adminChatID,
	}, nil
}

// SendToAdmin sends a message to the admin chat
func (t *TelegramNotifier) SendToAdmin(message string) error {
	if t.adminChatID == "" {
		return nil // No admin chat ID configured
	}

	msg := tgbotapi.NewMessageToChannel(t.adminChatID, message)
	msg.ParseMode = "Markdown"

	_, err := t.bot.Send(msg)
	return err
}

// SendToUser sends a message to a specific user's chat ID
func (t *TelegramNotifier) SendToUser(chatID, message string) error {
	if chatID == "" {
		return nil // No chat ID provided
	}

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = "Markdown"

	_, err := t.bot.Send(msg)
	return err
}

// NotifyQuotaWarning sends a warning when user reaches quota threshold
func (t *TelegramNotifier) NotifyQuotaWarning(name, ip, userChatID string, usagePercent float64) error {
	message := fmt.Sprintf(
		"⚠️ *Quota Warning*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n"+
			"Usage: `%.1f%%`\n\n"+
			"You are approaching your quota limit.",
		name, ip, usagePercent,
	)

	// Send to user if they have a chat ID
	if userChatID != "" {
		if err := t.SendToUser(userChatID, message); err != nil {
			return err
		}
	}

	// Also notify admin
	adminMsg := fmt.Sprintf(
		"⚠️ *Quota Warning*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n"+
			"Usage: `%.1f%%`",
		name, ip, usagePercent,
	)
	return t.SendToAdmin(adminMsg)
}

// NotifyQuotaExceeded sends notification when quota is exceeded
func (t *TelegramNotifier) NotifyQuotaExceeded(name, ip, userChatID string, usedMB, quotaMB float64) error {
	message := fmt.Sprintf(
		"🚫 *Quota Exceeded*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n"+
			"Used: `%.2f MB`\n"+
			"Quota: `%.2f MB`\n\n"+
			"Your internet access has been blocked due to quota limit.\n"+
			"Please contact administrator to reset or increase your quota.",
		name, ip, usedMB, quotaMB,
	)

	// Send to user if they have a chat ID
	if userChatID != "" {
		if err := t.SendToUser(userChatID, message); err != nil {
			return err
		}
	}

	// Also notify admin
	adminMsg := fmt.Sprintf(
		"🚫 *User Blocked*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n"+
			"Used: `%.2f MB`\n"+
			"Quota: `%.2f MB`\n\n"+
			"Quota exceeded - access blocked.",
		name, ip, usedMB, quotaMB,
	)
	return t.SendToAdmin(adminMsg)
}

// NotifyDailyReport sends daily usage report
func (t *TelegramNotifier) NotifyDailyReport(report string) error {
	message := fmt.Sprintf("📊 *Daily Usage Report*\n\n%s", report)
	return t.SendToAdmin(message)
}

// NotifySystemEvent sends system event notification
func (t *TelegramNotifier) NotifySystemEvent(event string) error {
	message := fmt.Sprintf("ℹ️ *System Event*\n\n%s", event)
	return t.SendToAdmin(message)
}

// NotifyUserReset sends notification when a user's usage is reset
func (t *TelegramNotifier) NotifyUserReset(name, ip, userChatID string) error {
	message := fmt.Sprintf(
		"🔄 *Usage Reset*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n\n"+
			"Your usage has been reset. You now have full quota available.",
		name, ip,
	)

	// Send to user if they have a chat ID
	if userChatID != "" {
		if err := t.SendToUser(userChatID, message); err != nil {
			return err
		}
	}

	// Also notify admin
	adminMsg := fmt.Sprintf(
		"🔄 *User Reset*\n\n"+
			"User: `%s`\n"+
			"IP: `%s`\n\n"+
			"Usage has been reset.",
		name, ip,
	)
	return t.SendToAdmin(adminMsg)
}

// FormatUsageReport formats a usage report for multiple users
func FormatUsageReport(users []UserUsage) string {
	var sb strings.Builder

	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("%-15s %-20s %-10s\n", "IP", "Name", "Usage"))
	sb.WriteString(strings.Repeat("-", 50) + "\n")

	for _, u := range users {
		sb.WriteString(fmt.Sprintf("%-15s %-20s %-10s\n", u.IP, u.Name, u.Usage))
	}

	sb.WriteString("```")
	return sb.String()
}

// UserUsage represents user usage info for reports
type UserUsage struct {
	IP    string
	Name  string
	Usage string
}
