package updates

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/envx"
)

type Item struct {
	Container string `json:"container"`
	Image     string `json:"image"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	NewerTag  string `json:"newerTag,omitempty"`
	Outdated  bool   `json:"outdated"`
	Error     string `json:"error,omitempty"`
}

type Policy struct {
	Mode            string `json:"mode"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Notify          bool   `json:"notify"`
}

type Result struct {
	Container string `json:"container"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Image     string `json:"image,omitempty"`
}

type Running struct {
	Name   string
	Image  string
	Digest string
}

type Engine interface {
	List(ctx context.Context) ([]Running, error)
	Pull(ctx context.Context, image string) error
	Replace(ctx context.Context, container, image string) error
	Rollback(ctx context.Context, container, previous string) error
	Healthy(ctx context.Context, container string) (bool, string)
}

type Service struct {
	mu       sync.Mutex
	path     string
	policy   Policy
	items    []Item
	history  []Result
	Registry Registry
	Engine   Engine
	Notify   func(title, body string)
}

func Open(dir string) (*Service, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{path: filepath.Join(dir, "updates.json"), policy: Policy{Mode: "approval", Notify: true}}
	raw, err := os.ReadFile(s.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(raw) > 0 {
		var doc struct {
			Policy  Policy   `json:"policy"`
			History []Result `json:"history"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		if doc.Policy.Mode != "" {
			s.policy = doc.Policy
		}
		s.history = doc.History
	}
	return s, nil
}

func (s *Service) Policy() Policy {
	if s == nil {
		return Policy{Mode: "approval"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

func (s *Service) SetPolicy(p Policy) error {
	if s == nil {
		return errors.New("atualizações indisponíveis")
	}
	if p.Mode != "auto" && p.Mode != "approval" {
		p.Mode = "approval"
	}
	if p.IntervalSeconds < 0 {
		p.IntervalSeconds = 0
	}
	s.mu.Lock()
	s.policy.Mode = p.Mode
	s.policy.IntervalSeconds = p.IntervalSeconds
	s.policy.Notify = p.Notify
	err := s.flush()
	s.mu.Unlock()
	return err
}

func (s *Service) Snapshot() ([]Item, []Result, Policy) {
	if s == nil {
		return []Item{}, []Result{}, Policy{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Item{}, s.items...), append([]Result{}, s.history...), s.policy
}

func (s *Service) Check(ctx context.Context) ([]Item, error) {
	if s == nil || s.Engine == nil {
		return nil, errors.New("docker indisponível")
	}
	running, err := s.Engine.List(ctx)
	if err != nil {
		return nil, err
	}
	reg := s.registry()
	user, pass, token := envx.Get("REGISTRY_USER"), envx.GetRaw("REGISTRY_PASSWORD"), envx.GetRaw("REGISTRY_TOKEN")
	var items []Item
	for _, c := range running {
		it := Item{Container: c.Name, Image: c.Image, Current: c.Digest}
		d, err := reg.Latest(ctx, c.Image, user, pass, token)
		if err != nil {
			it.Error = err.Error()
			items = append(items, it)
			continue
		}
		it.Latest = d.Digest
		it.NewerTag = d.NewerTag
		it.Outdated = (c.Digest != "" && d.Digest != "" && c.Digest != d.Digest) || d.NewerTag != ""
		items = append(items, it)
	}
	if items == nil {
		items = []Item{}
	}
	s.mu.Lock()
	s.items = items
	s.mu.Unlock()
	if s.Notify != nil {
		n := 0
		for _, it := range items {
			if it.Outdated {
				n++
			}
		}
		if n > 0 {
			s.Notify("Imagens desatualizadas", fmtCount(n))
		}
	}
	return items, nil
}

func (s *Service) Apply(ctx context.Context, container, actor string, aud *audit.Log) (Result, error) {
	if s == nil || s.Engine == nil {
		return Result{}, errors.New("docker indisponível")
	}
	if envx.ReadOnly() {
		return Result{}, errors.New("modo somente leitura")
	}
	s.mu.Lock()
	var item Item
	found := false
	for _, it := range s.items {
		if it.Container == container {
			item = it
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found || !item.Outdated {
		return Result{Container: container, Status: "current", Detail: "já atualizado"}, nil
	}
	target := item.Image
	if item.NewerTag != "" {
		ref := Parse(item.Image)
		target = ref.Repo + ":" + item.NewerTag
		if ref.Host != "docker.io" {
			target = ref.Host + "/" + target
		} else if len(ref.Repo) > 8 && ref.Repo[:8] == "library/" {
			target = ref.Repo[8:] + ":" + item.NewerTag
		}
	}
	if err := s.Engine.Pull(ctx, target); err != nil {
		res := Result{Container: container, Status: "failed", Detail: err.Error(), Image: target}
		s.push(res, actor, aud)
		return res, err
	}
	if err := s.Engine.Replace(ctx, container, target); err != nil {
		res := Result{Container: container, Status: "failed", Detail: err.Error(), Image: target}
		s.push(res, actor, aud)
		return res, err
	}
	ok, detail := s.Engine.Healthy(ctx, container)
	if !ok {
		_ = s.Engine.Rollback(ctx, container, item.Image)
		if detail == "" {
			detail = "healthcheck falhou"
		}
		res := Result{Container: container, Status: "rolled_back", Detail: detail, Image: item.Image}
		s.push(res, actor, aud)
		if s.Notify != nil {
			s.Notify("Rollback de imagem", container+" "+detail)
		}
		return res, errors.New(detail)
	}
	res := Result{Container: container, Status: "updated", Detail: "aplicado", Image: target}
	s.push(res, actor, aud)
	if s.Notify != nil {
		s.Notify("Imagem atualizada", container+" → "+target)
	}
	return res, nil
}

func (s *Service) push(res Result, actor string, aud *audit.Log) {
	s.mu.Lock()
	s.history = append(s.history, res)
	if len(s.history) > 50 {
		s.history = s.history[len(s.history)-50:]
	}
	_ = s.flush()
	s.mu.Unlock()
	if aud != nil {
		_ = aud.Append(audit.Event{
			At: time.Now().UTC(), Actor: actor, Action: "image " + res.Status + " " + res.Container,
			Detail: res.Image, Status: 200,
		})
	}
}

func (s *Service) Start(ctx context.Context, aud *audit.Log) {
	if s == nil {
		return
	}
	go func() {
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		var last time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				p := s.Policy()
				if p.Mode != "auto" || p.IntervalSeconds <= 0 {
					continue
				}
				if !last.IsZero() && time.Since(last) < time.Duration(p.IntervalSeconds)*time.Second {
					continue
				}
				last = time.Now()
				items, err := s.Check(ctx)
				if err != nil {
					continue
				}
				for _, it := range items {
					if it.Outdated {
						_, _ = s.Apply(ctx, it.Container, "updates", aud)
					}
				}
			}
		}
	}()
}

func (s *Service) registry() Registry {
	if s.Registry != nil {
		return s.Registry
	}
	return &HTTPRegistry{}
}

func (s *Service) flush() error {
	raw, err := json.MarshalIndent(struct {
		Policy  Policy   `json:"policy"`
		History []Result `json:"history"`
	}{s.policy, s.history}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func fmtCount(n int) string {
	return strconvItoa(n) + " container(s) com imagem mais nova"
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
