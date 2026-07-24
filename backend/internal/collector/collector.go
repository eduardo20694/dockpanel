package collector

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"dockpanel/internal/dockerclient"
	"dockpanel/internal/store"

	"github.com/docker/docker/api/types/container"
)

type Collector struct {
	Pool     *dockerclient.Pool
	Store    *store.Store
	Interval time.Duration
	mu       sync.Mutex
	lastRest map[string]int
}

func New(pool *dockerclient.Pool, st *store.Store) *Collector {
	return &Collector{
		Pool:     pool,
		Store:    st,
		Interval: 30 * time.Second,
		lastRest: make(map[string]int),
	}
}

func (c *Collector) Start(ctx context.Context) {
	if c.Store == nil {
		return
	}
	log.Printf("histórico: coletor ativo a cada %s", c.Interval)
	t := time.NewTicker(c.Interval)
	go func() {
		c.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				c.runOnce(ctx)
			}
		}
	}()

	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := c.Store.PruneOlderThan(store.DefaultRetention()); err != nil {
					log.Printf("histórico: prune falhou: %v", err)
				}
			}
		}
	}()
}

func (c *Collector) runOnce(ctx context.Context) {
	for _, h := range c.Pool.Baseline() {
		cli, err := c.Pool.Get(h.ID)
		if err != nil {
			continue
		}
		list, err := cli.CLI.ContainerList(ctx, container.ListOptions{})
		if err != nil {
			continue
		}
		for _, ct := range list {
			if ct.State != "running" {
				continue
			}
			name := ct.ID[:12]
			if len(ct.Names) > 0 {
				name = ct.Names[0][1:]
			}
			info, err := cli.CLI.ContainerInspect(ctx, ct.ID)
			if err != nil {
				continue
			}
			key := h.ID + ":" + ct.ID
			c.mu.Lock()
			prev := c.lastRest[key]
			if info.RestartCount > prev && prev >= 0 && prev > 0 {
				_ = c.Store.RecordRestart(h.ID, ct.ID, name, info.RestartCount)
			}
			if _, ok := c.lastRest[key]; !ok {
				c.lastRest[key] = info.RestartCount
			} else if info.RestartCount != prev {
				c.lastRest[key] = info.RestartCount
			}
			c.mu.Unlock()

			stream, err := cli.CLI.ContainerStats(ctx, ct.ID, false)
			if err != nil {
				continue
			}
			var raw struct {
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
			if err := json.NewDecoder(stream.Body).Decode(&raw); err == nil {
				cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
				sysDelta := float64(raw.CPUStats.SystemUsage) - float64(raw.PreCPUStats.SystemUsage)
				cpuPct := 0.0
				if sysDelta > 0 && cpuDelta > 0 {
					cpus := float64(raw.CPUStats.OnlineCPUs)
					if cpus == 0 {
						cpus = 1
					}
					cpuPct = (cpuDelta / sysDelta) * cpus * 100.0
				}
				memPct := 0.0
				if raw.MemoryStats.Limit > 0 {
					memPct = float64(raw.MemoryStats.Usage) / float64(raw.MemoryStats.Limit) * 100.0
				}
				_ = c.Store.RecordMetric(h.ID, ct.ID, name, cpuPct, memPct, raw.MemoryStats.Usage, info.RestartCount)
			}
			stream.Body.Close()
		}
	}
}
