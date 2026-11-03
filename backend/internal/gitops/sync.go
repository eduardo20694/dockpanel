package gitops

import (
	"context"
	"errors"
	"strings"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/envx"
)

func (s *Store) Sync(ctx context.Context, stack, trigger, actor string, aud *audit.Log) (Event, error) {
	if s == nil {
		return Event{}, errors.New("gitops indisponível")
	}
	link, ok := s.copy(stack)
	if !ok {
		return Event{}, errors.New("stack não vinculada")
	}
	if !s.globalOn() || !link.Enabled {
		return s.record(stack, trigger, "skipped", "", "", "gitops desativado", actor, aud), nil
	}
	if envx.ReadOnly() && trigger != "diff" {
		return s.record(stack, trigger, "skipped", "", "", "modo somente leitura", actor, aud), errors.New("modo somente leitura")
	}
	if trigger == "approve" {
		if link.Pending == nil {
			return Event{}, errors.New("nada pendente")
		}
		return s.applyContent(ctx, link, link.Pending.Commit, []byte(link.Pending.Compose), link.Pending.Diff, trigger, actor, aud)
	}
	remote, err := s.fetcher().Fetch(ctx, link)
	if err != nil {
		ev := s.record(stack, trigger, "failed", "", "", err.Error(), actor, aud)
		return ev, err
	}
	diff := Diff(link.Applied, string(remote.Compose))
	if diff == "" {
		return s.record(stack, trigger, "unchanged", remote.Commit, "", "sem alteração", actor, aud), nil
	}
	if link.Mode != "auto" {
		s.mu.Lock()
		if cur := s.doc.Stacks[stack]; cur != nil {
			cur.Pending = &Pending{Commit: remote.Commit, Compose: string(remote.Compose), Diff: diff}
		}
		_ = s.flush()
		s.mu.Unlock()
		return s.record(stack, trigger, "pending", remote.Commit, diff, "aguardando aprovação", actor, aud), nil
	}
	return s.applyContent(ctx, link, remote.Commit, remote.Compose, diff, trigger, actor, aud)
}

func (s *Store) Preview(ctx context.Context, stack string) (string, error) {
	link, ok := s.copy(stack)
	if !ok {
		return "", errors.New("stack não vinculada")
	}
	if link.Pending != nil && link.Pending.Diff != "" {
		return link.Pending.Diff, nil
	}
	remote, err := s.fetcher().Fetch(ctx, link)
	if err != nil {
		return "", err
	}
	return Diff(link.Applied, string(remote.Compose)), nil
}

func (s *Store) applyContent(ctx context.Context, link Link, commit string, compose []byte, diff, trigger, actor string, aud *audit.Log) (Event, error) {
	prev := link.Applied
	if err := s.applier().Apply(ctx, link.Stack, compose); err != nil {
		return s.record(link.Stack, trigger, "failed", commit, diff, err.Error(), actor, aud), err
	}
	ok, detail := s.applier().Healthy(ctx, link.Stack)
	if !ok {
		if prev != "" {
			_ = s.applier().Rollback(ctx, link.Stack, []byte(prev))
		}
		if detail == "" {
			detail = "healthcheck falhou"
		}
		s.mu.Lock()
		if cur := s.doc.Stacks[link.Stack]; cur != nil {
			cur.Pending = nil
		}
		_ = s.flush()
		s.mu.Unlock()
		return s.record(link.Stack, trigger, "rolled_back", commit, diff, detail, actor, aud), errors.New(detail)
	}
	s.mu.Lock()
	if cur := s.doc.Stacks[link.Stack]; cur != nil {
		cur.Applied = string(compose)
		cur.Commit = commit
		cur.Pending = nil
	}
	_ = s.flush()
	s.mu.Unlock()
	return s.record(link.Stack, trigger, "synced", commit, diff, "aplicado", actor, aud), nil
}

func (s *Store) record(stack, trigger, status, commit, diff, detail, actor string, aud *audit.Log) Event {
	if len(diff) > 4000 {
		diff = diff[:4000] + "\n…"
	}
	ev := Event{
		At: time.Now().UTC(), Stack: stack, Trigger: trigger, Status: status,
		Commit: commit, Diff: diff, Detail: detail, Actor: actor,
	}
	s.mu.Lock()
	s.doc.History = append(s.doc.History, ev)
	if len(s.doc.History) > 50 {
		s.doc.History = s.doc.History[len(s.doc.History)-50:]
	}
	_ = s.flush()
	s.mu.Unlock()
	if aud != nil {
		_ = aud.Append(audit.Event{
			At: ev.At, Actor: actor, Action: "gitops " + status + " " + stack,
			Status: 200, Detail: "trigger:" + trigger + " commit:" + commit,
		})
	}
	return ev
}

func (s *Store) globalOn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Enabled
}

func (s *Store) fetcher() Fetcher {
	if s.Fetch != nil {
		return s.Fetch
	}
	return GitCLI{}
}

func (s *Store) applier() Applier {
	if s.ApplyTo != nil {
		return s.ApplyTo
	}
	return composeApplier{root: s.root}
}

func (s *Store) Webhook(ctx context.Context, body []byte, githubSig, gitlabSig, gitlabToken, actor string, aud *audit.Log) (int, error) {
	if s == nil {
		return 0, errors.New("gitops indisponível")
	}
	s.mu.Lock()
	secret := s.doc.WebhookSecret
	s.mu.Unlock()
	if err := Verify(body, githubSig, gitlabSig, gitlabToken, secret); err != nil {
		return 0, err
	}
	meta := parseHook(body)
	s.mu.Lock()
	var names []string
	for name, l := range s.doc.Stacks {
		if !l.Enabled {
			continue
		}
		repoOK := false
		for _, r := range meta.Repos {
			if sameRepo(r, l.RepoURL) {
				repoOK = true
				break
			}
		}
		if !repoOK {
			continue
		}
		if meta.Ref != "" && l.Branch != "" && meta.Ref != l.Branch {
			continue
		}
		names = append(names, name)
	}
	s.mu.Unlock()
	for _, name := range names {
		_, _ = s.Sync(ctx, name, "webhook", actor, aud)
	}
	return len(names), nil
}

func (s *Store) Start(ctx context.Context, aud *audit.Log) {
	if s == nil {
		return
	}
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		var last time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				sec, names := s.pollSnapshot()
				if sec <= 0 || len(names) == 0 {
					continue
				}
				if !last.IsZero() && time.Since(last) < time.Duration(sec)*time.Second {
					continue
				}
				last = time.Now()
				for _, name := range names {
					_, _ = s.Sync(ctx, name, "poll", "gitops", aud)
				}
			}
		}
	}()
}

func (s *Store) pollSnapshot() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.doc.Enabled || s.doc.PollSeconds <= 0 {
		return 0, nil
	}
	var names []string
	for name, l := range s.doc.Stacks {
		if l.Enabled {
			names = append(names, name)
		}
	}
	return s.doc.PollSeconds, names
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func sanitizeErr(err error, secret string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "***")
	}
	return clip(msg, 300)
}
