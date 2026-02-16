// Package telegram provides a client for sending notifications via Telegram Bot API.
// It formats detected probability changes into human-readable messages and handles
// delivery with retry logic for reliability.
//
// The client supports Markdown formatting and includes error handling for
// common Telegram API issues like rate limiting and network failures.
package telegram

import (
	"fmt"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/poly-oracle/internal/models"
)

// Client handles Telegram notifications
type Client struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// NewClient creates a new Telegram client
func NewClient(botToken, chatID string) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	return &Client{
		bot:    bot,
		chatID: chatIDInt,
	}, nil
}

// Send sends a notification with the detected changes
func (c *Client) Send(changes []models.Change) error {
	message := c.formatMessage(changes)

	// Create message
	msg := tgbotapi.NewMessage(c.chatID, message)
	msg.ParseMode = "MarkdownV2" // Use MarkdownV2 for better escaping support

	// Send with retry
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		_, err := c.bot.Send(msg)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return fmt.Errorf("failed to send message after %d retries: %w", maxRetries, lastErr)
}

// formatMessage formats changes into a Telegram message
func (c *Client) formatMessage(changes []models.Change) string {
	message := "🚨 *Top Probability Changes Detected*\n\n"

	// Show detected time once at the top
	if len(changes) > 0 {
		dateStr := escapeMarkdownV2(changes[0].DetectedAt.Format("2006-01-02 15:04:05"))
		message += fmt.Sprintf("📅 Detected: %s\n\n", dateStr)
	}

	for i, change := range changes {
		// Add emoji for direction
		directionEmoji := "📈"
		if change.Direction == "decrease" {
			directionEmoji = "📉"
		}

		// Format magnitude as percentage
		magnitudePct := change.Magnitude * 100
		oldPct := change.OldProbability * 100
		newPct := change.NewProbability * 100

		// Create clickable hyperlink for event title if URL is available
		var titleLink string
		if change.EventURL != "" {
			// MarkdownV2 hyperlink format: [text](url)
			// Need to escape the title text but not the URL
			escapedQuestion := escapeMarkdownV2(change.EventQuestion)
			titleLink = fmt.Sprintf("[%s](%s)", escapedQuestion, change.EventURL)
		} else {
			// Fallback to plain text if no URL
			escapedQuestion := escapeMarkdownV2(change.EventQuestion)
			titleLink = escapedQuestion
		}

		// Format percentages with escaped periods
		magnitudeStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", magnitudePct))
		oldPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", oldPct))
		newPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", newPct))
		windowStr := escapeMarkdownV2(formatDuration(change.TimeWindow))

		message += fmt.Sprintf("%d\\. %s\n", i+1, titleLink)

		// Add market question if this is a multi-market event
		if change.MarketQuestion != "" && change.MarketQuestion != change.EventQuestion {
			escapedMarketQ := escapeMarkdownV2(change.MarketQuestion)
			message += fmt.Sprintf("   🎯 Market: %s\n", escapedMarketQ)
		}

		message += fmt.Sprintf("   %s Change: *%s* \\(%s → %s\\)\n",
			directionEmoji, magnitudeStr, oldPctStr, newPctStr)
		message += fmt.Sprintf("   ⏱ Window: %s\n\n", windowStr)
	}

	return message
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2
func escapeMarkdownV2(text string) string {
	// Characters that need escaping in MarkdownV2:
	// _ * [ ] ( ) ~ ` > # + - = | { } . !
	// Note: We escape all of them with \ prefix

	result := ""
	for _, char := range text {
		switch char {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
			result += "\\" + string(char)
		default:
			result += string(char)
		}
	}
	return result
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours == 1 {
		return fmt.Sprintf("%dh", hours)
	}
	if hours > 1 {
		return fmt.Sprintf("%dh", hours)
	}

	mins := int(d.Minutes())
	if mins == 1 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm", mins)
}
