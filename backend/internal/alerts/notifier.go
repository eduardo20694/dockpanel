package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type Notifier struct {
	telegramToken  string
	telegramChatID string
	discordWebhook string
	emailSMTP      string
	emailFrom      string
	emailTo        string
	emailUser      string
	emailPass      string
	whatsappURL    string
	whatsappToken  string
	whatsappTo     string
	http           *http.Client
}

func NewFromEnv() *Notifier {
	return &Notifier{
		telegramToken:  os.Getenv("ALERT_TELEGRAM_BOT_TOKEN"),
		telegramChatID: os.Getenv("ALERT_TELEGRAM_CHAT_ID"),
		discordWebhook: os.Getenv("ALERT_DISCORD_WEBHOOK"),
		emailSMTP:      os.Getenv("ALERT_EMAIL_SMTP"),
		emailFrom:      os.Getenv("ALERT_EMAIL_FROM"),
		emailTo:        os.Getenv("ALERT_EMAIL_TO"),
		emailUser:      os.Getenv("ALERT_EMAIL_USER"),
		emailPass:      os.Getenv("ALERT_EMAIL_PASSWORD"),
		whatsappURL:    os.Getenv("ALERT_WHATSAPP_API_URL"),
		whatsappToken:  os.Getenv("ALERT_WHATSAPP_TOKEN"),
		whatsappTo:     os.Getenv("ALERT_WHATSAPP_TO"),
		http:           &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *Notifier) Enabled() bool {
	return (n.telegramToken != "" && n.telegramChatID != "") ||
		n.discordWebhook != "" ||
		(n.emailSMTP != "" && n.emailTo != "") ||
		(n.whatsappURL != "" && n.whatsappToken != "")
}

func (n *Notifier) SendCritical(title, body string) error {
	return n.SendChannels(title, body, nil)
}

// SendChannels sends to requested channels (empty = all configured).
func (n *Notifier) SendChannels(title, body string, channels []string) error {
	want := func(name string) bool {
		if len(channels) == 0 {
			return true
		}
		for _, c := range channels {
			if strings.EqualFold(c, name) {
				return true
			}
		}
		return false
	}
	msg := fmt.Sprintf("🔴 *%s*\n\n%s", escapeMarkdown(title), escapeMarkdown(body))
	var errs []string
	if want("telegram") && n.telegramToken != "" && n.telegramChatID != "" {
		if err := n.sendTelegram(msg); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}
	if want("discord") && n.discordWebhook != "" {
		if err := n.sendDiscord(title, body); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}
	if want("email") && n.emailSMTP != "" && n.emailTo != "" {
		if err := n.sendEmail(title, body); err != nil {
			errs = append(errs, "email: "+err.Error())
		}
	}
	if want("whatsapp") && n.whatsappURL != "" && n.whatsappToken != "" {
		if err := n.sendWhatsApp(title, body); err != nil {
			errs = append(errs, "whatsapp: "+err.Error())
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

func (n *Notifier) sendEmail(title, body string) error {
	addr := n.emailSMTP
	from := n.emailFrom
	if from == "" {
		from = n.emailUser
	}
	msg := []byte("To: " + n.emailTo + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + title + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n")
	var auth smtp.Auth
	if n.emailUser != "" {
		host := addr
		if i := strings.LastIndex(addr, ":"); i > 0 {
			host = addr[:i]
		}
		auth = smtp.PlainAuth("", n.emailUser, n.emailPass, host)
	}
	return smtp.SendMail(addr, auth, from, []string{n.emailTo}, msg)
}

func (n *Notifier) sendWhatsApp(title, body string) error {
	// Generic WhatsApp Business / Cloud API style stub
	payload, _ := json.Marshal(map[string]interface{}{
		"to":      n.whatsappTo,
		"type":    "text",
		"text":    map[string]string{"body": title + "\n\n" + body},
		"messaging_product": "whatsapp",
	})
	req, err := http.NewRequest(http.MethodPost, n.whatsappURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.whatsappToken)
	resp, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp HTTP %d", resp.StatusCode)
	}
	return nil
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "`", "\\`")
	return replacer.Replace(s)
}
