package api

import (
	"net/http"

	"dockpanel/internal/diagnostics"
	"dockpanel/internal/drift"

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
	writeJSON(w, problems)
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
