package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"dockpanel/internal/dockerclient"
	"dockpanel/internal/tgmsg"

	"github.com/docker/docker/api/types/container"
)

// ContainerStat is a point-in-time CPU/RAM sample for one container.
type ContainerStat struct {
	HostID    string
	HostLabel string
	ID        string
	Name      string
	State     string
	CPUPct    float64
	MemPct    float64
	MemUsage  uint64
	MemLimit  uint64
}

type statsRaw struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
}

// Snapshot collects live stats for all containers across configured hosts.
func Snapshot(ctx context.Context, pool *dockerclient.Pool) ([]ContainerStat, error) {
	var (
		mu   sync.Mutex
		out  []ContainerStat
		errs []string
		wg   sync.WaitGroup
	)

	for _, h := range pool.List() {
		cli, err := pool.Get(h.ID)
		if err != nil {
			errs = append(errs, h.Label+": "+err.Error())
			continue
		}
		list, err := cli.CLI.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			errs = append(errs, h.Label+": "+err.Error())
			continue
		}
		for _, ct := range list {
			ct := ct
			h := h
			name := ct.ID
			if len(name) > 12 {
				name = name[:12]
			}
			if len(ct.Names) > 0 {
				name = strings.TrimPrefix(ct.Names[0], "/")
			}
			stat := ContainerStat{
				HostID: h.ID, HostLabel: h.Label, ID: ct.ID, Name: name, State: ct.State,
			}
			if ct.State != "running" {
				mu.Lock()
				out = append(out, stat)
				mu.Unlock()
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				stream, err := cli.CLI.ContainerStats(ctx, ct.ID, false)
				if err != nil {
					mu.Lock()
					out = append(out, stat)
					mu.Unlock()
					return
				}
				defer stream.Body.Close()
				var raw statsRaw
				if err := json.NewDecoder(stream.Body).Decode(&raw); err != nil {
					mu.Lock()
					out = append(out, stat)
					mu.Unlock()
					return
				}
				stat.CPUPct = cpuPercent(raw)
				stat.MemUsage = raw.MemoryStats.Usage
				stat.MemLimit = raw.MemoryStats.Limit
				if raw.MemoryStats.Limit > 0 {
					stat.MemPct = float64(raw.MemoryStats.Usage) / float64(raw.MemoryStats.Limit) * 100
				}
				mu.Lock()
				out = append(out, stat)
				mu.Unlock()
			}()
		}
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool {
		if out[i].State == "running" && out[j].State != "running" {
			return true
		}
		if out[i].State != "running" && out[j].State == "running" {
			return false
		}
		if out[i].CPUPct != out[j].CPUPct {
			return out[i].CPUPct > out[j].CPUPct
		}
		return out[i].MemUsage > out[j].MemUsage
	})

	if len(out) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

func cpuPercent(raw statsRaw) float64 {
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(raw.CPUStats.SystemUsage) - float64(raw.PreCPUStats.SystemUsage)
	if sysDelta <= 0 || cpuDelta <= 0 {
		return 0
	}
	cpus := float64(raw.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = 1
	}
	return (cpuDelta / sysDelta) * cpus * 100
}

// FormatTelegram builds a human-readable metrics report (Telegram HTML).
func FormatTelegram(stats []ContainerStat) string {
	running := 0
	items := make([]tgmsg.MetricItem, 0, len(stats))
	for _, s := range stats {
		host := s.HostLabel
		if host == "" {
			host = s.HostID
		}
		isRun := s.State == "running"
		if isRun {
			running++
		}
		items = append(items, tgmsg.MetricItem{
			Name: s.Name, Host: host, State: s.State,
			CPUPct: s.CPUPct, MemPct: s.MemPct,
			MemHuman: FormatBytes(s.MemUsage),
			Running:  isRun,
		})
	}
	return tgmsg.Metrics(len(stats), running, items)
}

// FormatBytes renders byte counts in human units.
func FormatBytes(n uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
