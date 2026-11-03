package gitops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockerpilot/internal/audit"
)

type Link struct {
	Stack       string   `json:"stack"`
	OrgID       string   `json:"orgId"`
	Enabled     bool     `json:"enabled"`
	RepoURL     string   `json:"repoUrl"`
	Branch      string   `json:"branch"`
	ComposePath string   `json:"composePath"`
	Credential  string   `json:"credential,omitempty"`
	Mode        string   `json:"mode"`
	Commit      string   `json:"commit,omitempty"`
	Applied     string   `json:"applied,omitempty"`
	Pending     *Pending `json:"pending,omitempty"`
}

type Pending struct {
	Commit  string `json:"commit"`
	Compose string `json:"compose"`
	Diff    string `json:"diff"`
}

type Event struct {
	At      time.Time `json:"at"`
	Stack   string    `json:"stack"`
	Trigger string    `json:"trigger"`
	Status  string    `json:"status"`
	Commit  string    `json:"commit,omitempty"`
	Diff    string    `json:"diff,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Actor   string    `json:"actor,omitempty"`
}

type StackPublic struct {
	Stack         string `json:"stack"`
	OrgID         string `json:"orgId"`
	Enabled       bool   `json:"enabled"`
	RepoURL       string `json:"repoUrl"`
	Branch        string `json:"branch"`
	ComposePath   string `json:"composePath"`
	CredentialSet bool   `json:"credentialSet"`
	Mode          string `json:"mode"`
	Commit        string `json:"commit,omitempty"`
	Pending       bool   `json:"pending"`
	PendingDiff   string `json:"pendingDiff,omitempty"`
}

type Status struct {
	Enabled           bool          `json:"enabled"`
	PollSeconds       int           `json:"pollSeconds"`
	WebhookConfigured bool          `json:"webhookConfigured"`
	Stacks            []StackPublic `json:"stacks"`
	History           []Event       `json:"history"`
}

type Remote struct {
	Commit  string
	Compose []byte
}

type Fetcher interface {
	Fetch(ctx context.Context, link Link) (Remote, error)
}

type Applier interface {
	Apply(ctx context.Context, stack string, compose []byte) error
	Rollback(ctx context.Context, stack string, previous []byte) error
	Healthy(ctx context.Context, stack string) (bool, string)
}

type document struct {
	Enabled       bool             `json:"enabled"`
	PollSeconds   int              `json:"pollSeconds"`
	WebhookSecret string           `json:"webhookSecret,omitempty"`
	Stacks        map[string]*Link `json:"stacks"`
	History       []Event          `json:"history"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	root    string
	doc     document
	Fetch   Fetcher
	ApplyTo Applier
	aud     *audit.Log
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "gitops.json"), root: dir}
	s.doc.Stacks = map[string]*Link{}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.doc); err != nil {
			return nil, err
		}
	}
	if s.doc.Stacks == nil {
		s.doc.Stacks = map[string]*Link{}
	}
	return s, nil
}

func (s *Store) Status() Status {
	if s == nil {
		return Status{Stacks: []StackPublic{}, History: []Event{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publicLocked()
}

func (s *Store) publicLocked() Status {
	st := Status{
		Enabled: s.doc.Enabled, PollSeconds: s.doc.PollSeconds,
		WebhookConfigured: s.doc.WebhookSecret != "",
		Stacks:            []StackPublic{}, History: append([]Event{}, s.doc.History...),
	}
	for _, l := range s.doc.Stacks {
		st.Stacks = append(st.Stacks, StackPublic{
			Stack: l.Stack, OrgID: l.OrgID, Enabled: l.Enabled, RepoURL: l.RepoURL,
			Branch: l.Branch, ComposePath: l.ComposePath, CredentialSet: l.Credential != "",
			Mode: l.Mode, Commit: l.Commit, Pending: l.Pending != nil,
			PendingDiff: pendingDiff(l),
		})
	}
	if st.History == nil {
		st.History = []Event{}
	}
	return st
}

func pendingDiff(l *Link) string {
	if l.Pending == nil {
		return ""
	}
	return l.Pending.Diff
}

type GlobalInput struct {
	Enabled       *bool
	PollSeconds   *int
	WebhookSecret *string
}

func (s *Store) SetGlobal(in GlobalInput) error {
	if s == nil {
		return errors.New("gitops indisponível")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Enabled != nil {
		s.doc.Enabled = *in.Enabled
	}
	if in.PollSeconds != nil {
		sec := *in.PollSeconds
		if sec < 0 {
			sec = 0
		}
		s.doc.PollSeconds = sec
	}
	if in.WebhookSecret != nil && *in.WebhookSecret != "" {
		s.doc.WebhookSecret = *in.WebhookSecret
	}
	return s.flush()
}

type LinkInput struct {
	Stack       string
	OrgID       string
	Enabled     *bool
	RepoURL     string
	Branch      string
	ComposePath string
	Credential  *string
	Mode        string
}

func (s *Store) Upsert(in LinkInput) error {
	if s == nil {
		return errors.New("gitops indisponível")
	}
	if in.Stack == "" {
		return errors.New("stack obrigatória")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.doc.Stacks[in.Stack]
	if cur == nil {
		cur = &Link{Stack: in.Stack, OrgID: "default", Enabled: true, Branch: "main", ComposePath: "docker-compose.yml", Mode: "approval"}
	}
	if in.OrgID != "" {
		cur.OrgID = in.OrgID
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.RepoURL != "" {
		cur.RepoURL = in.RepoURL
	}
	if in.Branch != "" {
		cur.Branch = in.Branch
	}
	if in.ComposePath != "" {
		cur.ComposePath = in.ComposePath
	}
	if in.Credential != nil && *in.Credential != "" {
		cur.Credential = *in.Credential
	}
	if in.Mode == "auto" || in.Mode == "approval" {
		cur.Mode = in.Mode
	}
	if cur.Mode == "" {
		cur.Mode = "approval"
	}
	s.doc.Stacks[in.Stack] = cur
	return s.flush()
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) copy(stack string) (Link, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.doc.Stacks[stack]
	if l == nil {
		return Link{}, false
	}
	cp := *l
	if l.Pending != nil {
		p := *l.Pending
		cp.Pending = &p
	}
	return cp, true
}
