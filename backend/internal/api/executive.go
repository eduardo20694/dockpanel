package api

import (
	"net/http"
	"time"

	"dockpanel/internal/diagnostics"
	"dockpanel/internal/insights"
	"dockpanel/internal/scan"
	"dockpanel/internal/stacks"

	"github.com/docker/docker/api/types"
)

type ExecutiveSummary struct {
	GeneratedAt    int64                    `json:"generatedAt"`
	Hosts          []HostExecutive          `json:"hosts"`
	TotalCritical  int                      `json:"totalCritical"`
	TotalWarning   int                      `json:"totalWarning"`
	StacksCritical int                      `json:"stacksCritical"`
	SecurityCritical int                    `json:"securityCritical"`
	LatestTagTotal int                      `json:"latestTagTotal"`
	DiskPressure   bool                     `json:"diskPressure"`
	RecentAlerts   []interface{}            `json:"recentAlerts"`
	TopProblems    []diagnostics.Problem    `json:"topProblems"`
}

type HostExecutive struct {
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	Online         bool    `json:"online"`
	Critical       int     `json:"critical"`
	Warning        int     `json:"warning"`
	StacksCritical int     `json:"stacksCritical"`
	SecurityCritical int   `json:"securityCritical"`
	DiskReclaimGB  float64 `json:"diskReclaimGB"`
	TrivyAvailable bool    `json:"trivyAvailable"`
}

func (s *Server) executiveSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	summary := ExecutiveSummary{GeneratedAt: time.Now().UnixMilli()}

	if s.Store != nil {
		alerts, _ := s.Store.RecentAlerts(10)
		for _, a := range alerts {
			summary.RecentAlerts = append(summary.RecentAlerts, a)
		}
	}

	for _, h := range s.Hosts.List() {
		cli, err := s.Hosts.Get(h.ID)
		he := HostExecutive{ID: h.ID, Label: h.Label, TrivyAvailable: scan.TrivyAvailable()}
		if err != nil {
			he.Online = false
			summary.Hosts = append(summary.Hosts, he)
			continue
		}
		if err := cli.Ping(ctx); err != nil {
			he.Online = false
			summary.Hosts = append(summary.Hosts, he)
			continue
		}
		he.Online = true

		eng := diagnostics.New(cli.CLI)
		problems, _ := eng.ScanProblems(ctx)
		for _, p := range problems {
			if p.Severity == diagnostics.SeverityCritical {
				he.Critical++
				summary.TotalCritical++
			} else {
				he.Warning++
				summary.TotalWarning++
			}
			if len(summary.TopProblems) < 8 {
				summary.TopProblems = append(summary.TopProblems, p)
			}
		}

		stackList, _ := stacks.ListStacks(ctx, cli.CLI, h.ID, h.Label)
		for _, st := range stackList {
			if st.Severity == diagnostics.SeverityCritical {
				he.StacksCritical++
				summary.StacksCritical++
			}
		}

		sec, _ := insights.Audit(ctx, cli.CLI, h.ID, h.Label)
		if sec != nil {
			he.SecurityCritical = sec.CriticalCount
			summary.SecurityCritical += sec.CriticalCount
			summary.LatestTagTotal += sec.LatestTagCount
		}

		du, err := cli.CLI.DiskUsage(ctx, types.DiskUsageOptions{})
		if err == nil {
			var reclaim int64
			for _, img := range du.Images {
				if len(img.RepoTags) == 0 {
					reclaim += img.Size
				}
			}
			if reclaim > 0 {
				gb := float64(reclaim) / 1e9
				he.DiskReclaimGB = gb
				if gb > 5 {
					summary.DiskPressure = true
				}
			}
		}

		summary.Hosts = append(summary.Hosts, he)
	}

	writeJSON(w, summary)
}
