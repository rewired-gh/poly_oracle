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
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/poly-oracle/internal/models"
)

// Client handles Telegram notifications
type Client struct {
	bot            *tgbotapi.BotAPI
	chatID         int64
	maxRetries     int
	retryDelayBase time.Duration
}

// NewClient creates a new Telegram client
func NewClient(botToken, chatID string, maxRetries int, retryDelayBase time.Duration) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	if maxRetries <= 0 {
		maxRetries = 3
	}
	if retryDelayBase <= 0 {
		retryDelayBase = time.Second
	}

	return &Client{
		bot:            bot,
		chatID:         chatIDInt,
		maxRetries:     maxRetries,
		retryDelayBase: retryDelayBase,
	}, nil
}

// Send sends a notification with the detected event groups
func (c *Client) Send(groups []models.Event) error {
	message := c.formatMessage(groups)

	// Create message
	msg := tgbotapi.NewMessage(c.chatID, message)
	msg.ParseMode = "MarkdownV2" // Use MarkdownV2 for better escaping support

	// Send with retry
	var lastErr error

	for i := 0; i < c.maxRetries; i++ {
		_, err := c.bot.Send(msg)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(c.retryDelayBase * time.Duration(i+1))
	}

	return fmt.Errorf("failed to send message after %d retries: %w", c.maxRetries, lastErr)
}

// formatMessage formats event groups into a Telegram MarkdownV2 message.
// Each group is one numbered entry; markets within the group appear as sub-bullets.
func (c *Client) formatMessage(groups []models.Event) string {
	message := "🚨 *Notable Odds Movements*\n\n"

	// Show detected time once at the top (from the first market of the first group)
	if len(groups) > 0 && len(groups[0].Markets) > 0 {
		dateStr := escapeMarkdownV2(groups[0].Markets[0].DetectedAt.Format("2006-01-02 15:04:05"))
		message += fmt.Sprintf("📅 Detected: %s\n\n", dateStr)
	}

	for i, group := range groups {
		// Create clickable hyperlink for event title
		var titleLink string
		if group.URL != "" {
			escapedQuestion := escapeMarkdownV2(group.Title)
			titleLink = fmt.Sprintf("[%s](%s)", escapedQuestion, group.URL)
		} else {
			titleLink = escapeMarkdownV2(group.Title)
		}

		message += fmt.Sprintf("%d\\. %s\n", i+1, titleLink)

		for _, change := range group.Markets {
			directionEmoji := "📈"
			if change.Direction == "decrease" {
				directionEmoji = "📉"
			}

			magnitudePct := change.Magnitude * 100
			oldPct := change.OldProbability * 100
			newPct := change.NewProbability * 100

			magnitudeStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", magnitudePct))
			oldPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", oldPct))
			newPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", newPct))
			windowStr := escapeMarkdownV2(formatDuration(change.TimeWindow))

			// Show market question as sub-bullet when it differs from the event question
			if change.MarketQuestion != "" && change.MarketQuestion != group.Title {
				escapedMarketQ := escapeMarkdownV2(change.MarketQuestion)
				message += fmt.Sprintf("   🎯 %s\n", escapedMarketQ)
			}

			message += fmt.Sprintf("   %s *%s* \\(%s → %s\\) ⏱ %s\n",
				directionEmoji, magnitudeStr, oldPctStr, newPctStr, windowStr)
		}

		message += "\n"
	}

	return message
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2.
// Characters that need escaping: _ * [ ] ( ) ~ ` > # + - = | { } . !
func escapeMarkdownV2(text string) string {
	var b strings.Builder
	b.Grow(len(text) + len(text)/4) // pre-allocate with room for escapes
	for _, char := range text {
		switch char {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
			b.WriteByte('\\')
		}
		b.WriteRune(char)
	}
	return b.String()
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if hours := int(d.Hours()); hours >= 1 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
