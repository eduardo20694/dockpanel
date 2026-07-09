package api

import (
	"net/http"
	"net/url"
	"os"

	"dockpanel/internal/backup"
	"dockpanel/internal/scan"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listImages(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := dc.CLI.ImageList(r.Context(), image.ListOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (s *Server) scanImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}
	report, err := scan.ScanImage(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, report)
}

func (s *Server) removeImage(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"
	res, err := dc.CLI.ImageRemove(r.Context(), id, image.RemoveOptions{Force: force})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

func (s *Server) pullImage(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		http.Error(w, "informe ?ref=imagem:tag", http.StatusBadRequest)
		return
	}
	out, err := dc.CLI.ImagePull(r.Context(), ref, image.PullOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	defer out.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	buf := make([]byte, 4096)
	for {
		n, err := out.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) listVolumes(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := dc.CLI.VolumeList(r.Context(), volume.ListOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list.Volumes)
}

func (s *Server) backupVolume(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dest := r.URL.Query().Get("dest")
	if dest == "" {
		dest = os.Getenv("DOCKPANEL_BACKUP_DIR")
	}
	res, err := backup.BackupVolume(r.Context(), name, dest, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

func (s *Server) removeVolume(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	force := r.URL.Query().Get("force") == "true"
	backupFirst := r.URL.Query().Get("backup") == "true"

	var backupResult any
	if backupFirst {
		dest := os.Getenv("DOCKPANEL_BACKUP_DIR")
		res, err := backup.BackupVolume(r.Context(), name, dest, "")
		if err != nil {
			writeErr(w, err)
			return
		}
		backupResult = res
	}

	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := dc.CLI.VolumeRemove(r.Context(), name, force); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "removed", "backup": backupResult})
}

func (s *Server) listNetworks(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := dc.CLI.NetworkList(r.Context(), types.NetworkListOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (s *Server) inspectNetwork(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	net, err := dc.CLI.NetworkInspect(r.Context(), id, types.NetworkInspectOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, net)
}

func (s *Server) removeNetwork(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := dc.CLI.NetworkRemove(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "removed"})
}

func (s *Server) systemInfo(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	info, err := dc.CLI.Info(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	version, _ := dc.CLI.ServerVersion(r.Context())
	writeJSON(w, map[string]interface{}{"info": info, "version": version})
}

func (s *Server) dfUsage(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	du, err := dc.CLI.DiskUsage(r.Context(), types.DiskUsageOptions{})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, du)
}
