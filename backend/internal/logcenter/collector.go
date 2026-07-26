package logcenter

import (
	"context"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

type Collector struct {
	Pool     *dockerclient.Pool
	Store    *Store
	mu       sync.Mutex
	watching map[string]context.CancelFunc // host:containerID
}

func NewCollector(pool *dockerclient.Pool, st *Store) *Collector {
	return &Collector{
		Pool:     pool,
		Store:    st,
		watching: make(map[string]context.CancelFunc),
	}
}

func (c *Collector) Start(ctx context.Context) {
	if c == nil || c.Store == nil || c.Pool == nil {
		return
	}
	log.Println("logcenter: coletor ativo")
	c.reconcile(ctx)

	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				c.stopAll()
				return
			case <-t.C:
				c.reconcile(ctx)
			}
		}
	}()

	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		c.pruneOnce()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.pruneOnce()
			}
		}
	}()
}

func (c *Collector) pruneOnce() {
	days := c.Store.GetRetentionDays()
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	n, err := c.Store.PruneOlderThan(cutoff)
	if err != nil {
		log.Printf("logcenter: prune falhou: %v", err)
		return
	}
	if n > 0 {
		log.Printf("logcenter: prune removeu %d linhas (retenção %dd)", n, days)
	}
}

func (c *Collector) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, cancel := range c.watching {
		cancel()
		delete(c.watching, k)
	}
}

func (c *Collector) reconcile(parent context.Context) {
	alive := map[string]struct{}{}
	for _, h := range c.Pool.List() {
		cli, err := c.Pool.Get(h.ID)
		if err != nil {
			continue
		}
		list, err := cli.CLI.ContainerList(parent, container.ListOptions{All: false})
		if err != nil {
			continue
		}
		for _, ct := range list {
			if ct.State != "running" {
				continue
			}
			name := ct.ID
			if len(name) > 12 {
				name = name[:12]
			}
			if len(ct.Names) > 0 {
				name = strings.TrimPrefix(ct.Names[0], "/")
			}
			key := h.ID + ":" + ct.ID
			alive[key] = struct{}{}
			c.mu.Lock()
			_, ok := c.watching[key]
			c.mu.Unlock()
			if ok {
				continue
			}
			child, cancel := context.WithCancel(parent)
			c.mu.Lock()
			c.watching[key] = cancel
			c.mu.Unlock()
			go c.follow(child, h.ID, ct.ID, name)
		}
	}
	c.mu.Lock()
	for key, cancel := range c.watching {
		if _, ok := alive[key]; !ok {
			cancel()
			delete(c.watching, key)
		}
	}
	c.mu.Unlock()
}

func (c *Collector) follow(ctx context.Context, hostID, containerID, name string) {
	defer func() {
		c.mu.Lock()
		delete(c.watching, hostID+":"+containerID)
		c.mu.Unlock()
	}()

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := c.streamOnce(ctx, hostID, containerID, name)
		if ctx.Err() != nil {
			return
		}
		if err != nil && err != io.EOF && err != context.Canceled {
			log.Printf("logcenter: stream %s: %v (reconnect %s)", name, err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Collector) streamOnce(ctx context.Context, hostID, containerID, name string) error {
	cli, err := c.Pool.Get(hostID)
	if err != nil {
		return err
	}
	since := ""
	if last := c.Store.LastSeen(hostID, containerID); last > 0 {
		// small overlap avoided by Insert dedupe; start just after last
		since = time.UnixMilli(last).UTC().Format(time.RFC3339Nano)
	}
	reader, err := cli.CLI.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      since,
		Tail:       "100",
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	stdout := &lineWriter{hostID: hostID, containerID: containerID, name: name, stream: "stdout", store: c.Store}
	stderr := &lineWriter{hostID: hostID, containerID: containerID, name: name, stream: "stderr", store: c.Store}
	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	stdout.flush()
	stderr.flush()
	return err
}

type lineWriter struct {
	hostID, containerID, name, stream string
	store                              *Store
	buf                                string
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		w.handleLine(line)
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if strings.TrimSpace(w.buf) != "" {
		w.handleLine(w.buf)
		w.buf = ""
	}
}

func (w *lineWriter) handleLine(raw string) {
	raw = strings.TrimRight(raw, "\r")
	raw = cleanLine(raw)
	if raw == "" {
		return
	}
	ts, msg := splitTimestamp(raw)
	if msg == "" {
		return
	}
	sev := string(diagnostics.ClassifyLogLine(msg))
	_, _ = w.store.Insert(Entry{
		HostID:        w.hostID,
		ContainerID:   w.containerID,
		ContainerName: w.name,
		Stream:        w.stream,
		TimestampMs:   ts.UnixMilli(),
		TimestampNano: ts.UnixNano(),
		Message:       msg,
		Severity:      sev,
	})
}

func cleanLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

func splitTimestamp(line string) (time.Time, string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return time.Now().UTC(), line
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, parts[0]); err == nil {
			return t.UTC(), parts[1]
		}
	}
	return time.Now().UTC(), line
}
