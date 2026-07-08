package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Notifier struct {
	telegramToken  string
	telegramChatID string
	discordWebhook string
	http           *http.Client
}

func NewFromEnv() *Notifier {
	return &Notifier{
		telegramToken:  os.Getenv("ALERT_TELEGRAM_BOT_TOKEN"),
		telegramChatID: os.Getenv("ALERT_TELEGRAM_CHAT_ID"),
		discordWebhook: os.Getenv("ALERT_DISCORD_WEBHOOK"),
		http:           &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *Notifier) Enabled() bool {
	return n.telegramToken != "" && n.telegramChatID != "" || n.discordWebhook != ""
}

func (n *Notifier) SendCritical(title, body string) error {
	msg := fmt.Sprintf("🔴 *%s*\n\n%s", escapeMarkdown(title), escapeMarkdown(body))
	var errs []string
	if n.telegramToken != "" && n.telegramChatID != "" {
		if err := n.sendTelegram(msg); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}
	if n.discordWebhook != "" {
		if err := n.sendDiscord(title, body); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (n *Notifier) sendTelegram(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.telegramToken)
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":    n.telegramChatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	resp, err := n.http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendDiscord(title, body string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{{
			"title":       title,
			"description": body,
			"color":       0xE5484D,
		}},
	})
	resp, err := n.http.Post(n.discordWebhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord HTTP %d", resp.StatusCode)
	}
	return nil
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "`", "\\`")
	return replacer.Replace(s)
}
