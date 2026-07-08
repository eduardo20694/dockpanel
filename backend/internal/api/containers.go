package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
)

type ContainerSummary struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Created int64             `json:"created"`
	Ports   []types.Port      `json:"ports"`
	Labels  map[string]string `json:"labels"`
}

func (s *Server) listContainers(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ctx := r.Context()
	list, err := dc.CLI.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		writeErr(w, err)
		return
	}

	out := make([]ContainerSummary, 0, len(list))
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0][1:]
		}
		out = append(out, ContainerSummary{
			ID: c.ID, Name: name, Image: c.Image, Status: c.Status,
			State: c.State, Created: c.Created, Ports: c.Ports, Labels: c.Labels,
		})
	}
	writeJSON(w, out)
}

func (s *Server) inspectContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	info, err := dc.CLI.ContainerInspect(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, info)
}

func (s *Server) startContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := dc.CLI.ContainerStart(r.Context(), id, container.StartOptions{}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func (s *Server) stopContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := dc.CLI.ContainerStop(r.Context(), id, container.StopOptions{}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (s *Server) restartContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := dc.CLI.ContainerRestart(r.Context(), id, container.StopOptions{}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "restarted"})
}

func (s *Server) removeContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"
	if err := dc.CLI.ContainerRemove(r.Context(), id, container.RemoveOptions{Force: force}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "removed"})
}

func (s *Server) logsContainer(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	follow := r.URL.Query().Get("follow") == "true"

	reader, err := dc.CLI.ContainerLogs(r.Context(), id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: follow, Tail: "300", Timestamps: true,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				f.Flush()
			}
			if err != nil {
				return
			}
		}
	} else {
		io.Copy(w, reader)
	}
}

func (s *Server) execCreate(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Cmd []string `json:"cmd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "corpo inválido", http.StatusBadRequest)
		return
	}
	if len(body.Cmd) == 0 {
		body.Cmd = []string{"/bin/sh"}
	}
	execID, err := dc.CLI.ContainerExecCreate(r.Context(), id, types.ExecConfig{
		Cmd: body.Cmd, AttachStdin: true, AttachStdout: true, AttachStderr: true, Tty: true,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"execId": execID.ID})
}

func (s *Server) liveContainerStats(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ctx := r.Context()
	list, err := dc.CLI.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}

	var ids []string
	for _, c := range list {
		if c.State == "running" {
			ids = append(ids, c.ID)
		}
	}

	out := make([]StatFrame, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			stats, err := dc.CLI.ContainerStats(ctx, id, false)
			if err != nil {
				return
			}
			var raw dockerStatsRaw
			if err := json.NewDecoder(stats.Body).Decode(&raw); err != nil {
				stats.Body.Close()
				return
			}
			stats.Body.Close()
			out[i] = toStatFrame(id, raw)
		}(i, id)
	}
	wg.Wait()

	result := make([]StatFrame, 0, len(ids))
	for _, f := range out {
		if f.ID != "" {
			result = append(result, f)
		}
	}
	writeJSON(w, result)
}
