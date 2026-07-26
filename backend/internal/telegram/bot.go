package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dockpanel/internal/alerts"
	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/metrics"
	"dockpanel/internal/tgmsg"
)

// Bot listens for Telegram commands (getUpdates) and replies in allowlisted chats.
type Bot struct {
	token   string
	allowed map[int64]bool
	pool    *dockerclient.Pool
	notify  *alerts.Notifier
	http    *http.Client
	offset  int64
}

func NewFromEnv(pool *dockerclient.Pool, n *alerts.Notifier) *Bot {
	if n == nil || !n.TelegramConfigured() {
		return nil
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(n.TelegramChatID()), 10, 64)
	if err != nil || chatID == 0 {
		log.Printf("telegram bot: ALERT_TELEGRAM_CHAT_ID inválido")
		return nil
	}
	return &Bot{
		token:   n.TelegramToken(),
		allowed: map[int64]bool{chatID: true},
		pool:    pool,
		notify:  n,
		http:    &http.Client{Timeout: 45 * time.Second},
	}
}

func (b *Bot) Start(ctx context.Context) {
	if b == nil {
		return
	}
	log.Println("telegram bot: ouvindo comandos (/metric, /problems, /help)")
	go b.loop(ctx)
}

func (b *Bot) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		updates, err := b.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram bot: getUpdates: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= b.offset {
				b.offset = u.UpdateID + 1
			}
			b.handleUpdate(ctx, u)
		}
	}
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

func (b *Bot) getUpdates(ctx context.Context) ([]update, error) {
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("offset", strconv.FormatInt(b.offset, 10))
	q.Set("allowed_updates", `["message"]`)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s", b.token, q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		OK     bool     `json:"ok"`
		Result []update `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram ok=false")
	}
	return parsed.Result, nil
}

func (b *Bot) handleUpdate(ctx context.Context, u update) {
	if u.Message == nil || u.Message.Text == "" {
		return
	}
	chatID := u.Message.Chat.ID
	if !b.allowed[chatID] {
		log.Printf("telegram bot: chat %d ignorado (não autorizado)", chatID)
		return
	}
	cmd, _ := splitCommand(u.Message.Text)
	switch cmd {
	case "/start", "/help":
		_ = b.replyHTML(chatID, tgmsg.Help())
	case "/metric", "/metrics":
		_ = b.replyHTML(chatID, tgmsg.Collecting())
		stats, err := metrics.Snapshot(ctx, b.pool)
		if err != nil {
			_ = b.replyHTML(chatID, tgmsg.Error(err.Error()))
			return
		}
		_ = b.replyHTML(chatID, metrics.FormatTelegram(stats))
	case "/problems", "/alerts", "/status":
		_ = b.replyHTML(chatID, b.problemsHTML(ctx))
	default:
		if strings.HasPrefix(cmd, "/") {
			_ = b.replyHTML(chatID, tgmsg.UnknownCommand())
		}
	}
}

func (b *Bot) problemsHTML(ctx context.Context) string {
	var items []tgmsg.ProblemItem
	for _, h := range b.pool.List() {
		cli, err := b.pool.Get(h.ID)
		if err != nil {
			items = append(items, tgmsg.ProblemItem{
				Severity: "critical", Name: h.Label, Reason: "inacessível", Host: h.Label,
			})
			continue
		}
		eng := diagnostics.New(cli.CLI)
		problems, err := eng.ScanProblems(ctx)
		if err != nil {
			items = append(items, tgmsg.ProblemItem{
				Severity: "warning", Name: h.Label, Reason: err.Error(), Host: h.Label,
			})
			continue
		}
		for _, p := range problems {
			items = append(items, tgmsg.ProblemItem{
				Severity: string(p.Severity), Name: p.Name, Reason: p.Reason, Host: h.Label,
			})
		}
	}
	return tgmsg.Problems(items)
}

func (b *Bot) replyHTML(chatID int64, html string) error {
	return b.notify.SendTelegramToHTML(strconv.FormatInt(chatID, 10), html)
}

func splitCommand(text string) (cmd, args string) {
	text = strings.TrimSpace(text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	cmd = parts[0]
	if i := strings.Index(cmd, "@"); i > 0 {
		cmd = cmd[:i]
	}
	cmd = strings.ToLower(cmd)
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}
	return cmd, args
}
