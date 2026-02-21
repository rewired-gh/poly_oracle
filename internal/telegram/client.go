// Package telegram provides a client for sending notifications via Telegram Bot API.
package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rewired-gh/polyoracle/internal/models"
)

// Client handles Telegram notifications.
type Client struct {
	bot            *tgbotapi.BotAPI
	chatID         int64
	maxRetries     int
	retryDelayBase time.Duration
}

// NewClient creates a new Telegram client.
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

// ListenForCommands starts a goroutine that polls for Telegram updates and handles bot commands.
// It returns immediately; the goroutine stops when ctx is cancelled.
func (c *Client) ListenForCommands(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := c.bot.GetUpdatesChan(u)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.bot.StopReceivingUpdates()
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				if update.Message != nil && update.Message.IsCommand() {
					c.handleCommand(update.Message)
				}
			}
		}
	}()
}

func (c *Client) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "ping":
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Pong")
		c.bot.Send(reply) //nolint:errcheck
	}
}

// sendMarkdownV2 sends a MarkdownV2 message with linear-backoff retry.
func (c *Client) sendMarkdownV2(text string) error {
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ParseMode = "MarkdownV2"

	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		if _, err := c.bot.Send(msg); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(c.retryDelayBase * time.Duration(i+1))
	}
	return fmt.Errorf("failed after %d retries: %w", c.maxRetries, lastErr)
}

// SendError sends a monitoring error notification.
// Call this only on the first occurrence of a consecutive error sequence.
func (c *Client) SendError(cycleErr error) error {
	text := fmt.Sprintf("⚠️ *Monitoring error*\n`%s`", escapeMarkdownV2(cycleErr.Error()))
	return c.sendMarkdownV2(text)
}

// SendRecovery sends a recovery notification after consecutive failures.
func (c *Client) SendRecovery(failureCount int) error {
	text := fmt.Sprintf("✅ *Monitoring recovered* after %d consecutive failure\\(s\\)", failureCount)
	return c.sendMarkdownV2(text)
}

// Send sends a notification with the detected event groups.
func (c *Client) Send(groups []models.EventGroup) error {
	return c.sendMarkdownV2(c.formatMessage(groups))
}

// formatMessage formats event groups into a Telegram MarkdownV2 message.
func (c *Client) formatMessage(groups []models.EventGroup) string {
	message := "🚨 *Notable Odds Movements*\n\n"

	if len(groups) > 0 && len(groups[0].Markets) > 0 {
		dateStr := escapeMarkdownV2(groups[0].Markets[0].DetectedAt.Format("2006-01-02 15:04:05"))
		message += fmt.Sprintf("📅 Detected: %s\n\n", dateStr)
	}

	for i, group := range groups {
		var titleLink string
		if group.EventURL != "" {
			escapedTitle := escapeMarkdownV2(group.EventTitle)
			titleLink = fmt.Sprintf("[%s](%s)", escapedTitle, group.EventURL)
		} else {
			titleLink = escapeMarkdownV2(group.EventTitle)
		}

		message += fmt.Sprintf("%d\\. %s\n", i+1, titleLink)

		for _, alert := range group.Markets {
			directionEmoji := "📈"
			if alert.NewProb < alert.OldProb {
				directionEmoji = "📉"
			}

			priceDeltaPct := alert.PriceDelta * 100
			oldPct := alert.OldProb * 100
			newPct := alert.NewProb * 100

			deltaStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", priceDeltaPct))
			oldPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", oldPct))
			newPctStr := escapeMarkdownV2(fmt.Sprintf("%.1f%%", newPct))

			if alert.MarketQuestion != "" && alert.MarketQuestion != group.EventTitle {
				escapedMarketQ := escapeMarkdownV2(alert.MarketQuestion)
				message += fmt.Sprintf("   🎯 %s\n", escapedMarketQ)
			}

			message += fmt.Sprintf("   %s *%s* \\(%s → %s\\)\n",
				directionEmoji, deltaStr, oldPctStr, newPctStr)
		}

		message += "\n"
	}

	return message
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2.
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
