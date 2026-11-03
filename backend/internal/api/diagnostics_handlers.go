package api

import (
	"net/http"

	"dockerpilot/internal/diagnostics"
	"dockerpilot/internal/drift"
	"dockerpilot/internal/resolved"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/go-chi/chi/v5"
)

func imageListOptionsDangling() image.ListOptions {
	return image.ListOptions{Filters: filters.NewArgs(filters.Arg("dangling", "true"))}
}

func volumeListOptionsDangling() volume.ListOptions {
	return volume.ListOptions{Filters: filters.NewArgs(filters.Arg("dangling", "true"))}
}

func (s *Server) scanProblems(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	eng := diagnostics.New(dc.CLI)
	problems, err := eng.ScanProblems(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, s.withoutResolved(r, problems))
}

func (s *Server) withoutResolved(r *http.Request, problems []diagnostics.Problem) []diagnostics.Problem {
	if problems == nil {
		problems = []diagnostics.Problem{}
	}
	if s.Resolved == nil || len(problems) == 0 {
		return problems
	}
	host := s.resolveHostID(r)
	briefs := make([]resolved.Problem, 0, len(problems))
	for _, p := range problems {
		briefs = append(briefs, resolved.Problem{ContainerID: p.ContainerID, Name: p.Name, Reason: p.Reason})
	}
	kept := map[string]struct{}{}
	for _, p := range s.Resolved.Filter(host, briefs) {
		kept[p.ContainerID+"|"+p.Reason] = struct{}{}
	}
	out := make([]diagnostics.Problem, 0, len(kept))
	for _, p := range problems {
		if _, ok := kept[p.ContainerID+"|"+p.Reason]; ok {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) diagnoseContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	eng := diagnostics.New(dc.CLI)
	d, err := eng.Diagnose(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, d)
}

func (s *Server) composeDrift(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "informe ?path=pasta/do/compose", http.StatusBadRequest)
		return
	}
	report, err := drift.Check(r.Context(), dc.CLI, path)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, report)
}

type SafePruneReport struct {
	DanglingImages        []PruneItem `json:"danglingImages"`
	UnusedVolumes         []PruneItem `json:"unusedVolumes"`
	StoppedOldContainers  []PruneItem `json:"stoppedOldContainers"`
	EstimatedReclaimBytes int64       `json:"estimatedReclaimBytes"`
}

type PruneItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Bytes  int64  `json:"bytes"`
}

func (s *Server) safePruneReport(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ctx := r.Context()
	report := SafePruneReport{}

	imgs, err := dc.CLI.ImageList(ctx, imageListOptionsDangling())
	if err == nil {
		for _, img := range imgs {
			report.DanglingImages = append(report.DanglingImages, PruneItem{
				ID: img.ID, Name: "<none>:<none>",
				Detail: "imagem intermediária sem tag, sobra de build", Bytes: img.Size,
			})
			report.EstimatedReclaimBytes += img.Size
		}
	}

	volList, err := dc.CLI.VolumeList(ctx, volumeListOptionsDangling())
	if err == nil {
		for _, v := range volList.Volumes {
			report.UnusedVolumes = append(report.UnusedVolumes, PruneItem{
				ID: v.Name, Name: v.Name, Detail: "não está montado em nenhum container",
			})
		}
	}

	list, err := dc.CLI.ContainerList(ctx, container.ListOptions{
		All: true, Filters: filters.NewArgs(filters.Arg("status", "exited")),
	})
	if err == nil {
		for _, c := range list {
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = c.Names[0][1:]
			}
			report.StoppedOldContainers = append(report.StoppedOldContainers, PruneItem{
				ID: c.ID, Name: name, Detail: c.Status,
			})
		}
	}
	writeJSON(w, report)
}
