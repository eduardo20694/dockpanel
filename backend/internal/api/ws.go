package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type StatFrame struct {
	ID           string  `json:"id"`
	CPUPct       float64 `json:"cpuPct"`
	MemUsage     uint64  `json:"memUsage"`
	MemLimit     uint64  `json:"memLimit"`
	MemPct       float64 `json:"memPct"`
	NetRx        uint64  `json:"netRx"`
	NetTx        uint64  `json:"netTx"`
	RestartCount int     `json:"restartCount"`
	Time         int64   `json:"time"`
}

func (s *Server) wsStats(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var restartMu sync.Mutex
	var lastRestart int
	var lastInspect time.Time

	pollRestart := func() int {
		restartMu.Lock()
		defer restartMu.Unlock()
		if time.Since(lastInspect) < 3*time.Second {
			return lastRestart
		}
		info, err := dc.CLI.ContainerInspect(r.Context(), id)
		if err == nil {
			lastRestart = info.RestartCount
			lastInspect = time.Now()
		}
		return lastRestart
	}

	stream, err := dc.CLI.ContainerStats(r.Context(), id, true)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer stream.Body.Close()

	dec := json.NewDecoder(stream.Body)
	for {
		var raw dockerStatsRaw
		if err := dec.Decode(&raw); err != nil {
			return
		}
		frame := toStatFrame(id, raw)
		frame.RestartCount = pollRestart()
		if err := conn.WriteJSON(frame); err != nil {
			return
		}
	}
}

type dockerStatsRaw struct {
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
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func toStatFrame(id string, raw dockerStatsRaw) StatFrame {
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
	var rx, tx uint64
	for _, n := range raw.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return StatFrame{
		ID: id, CPUPct: round2(cpuPct), MemUsage: raw.MemoryStats.Usage,
		MemLimit: raw.MemoryStats.Limit, MemPct: round2(memPct),
		NetRx: rx, NetTx: tx, Time: time.Now().UnixMilli(),
	}
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

// wsTerminal — shell interativo via WebSocket + docker exec.
func (s *Server) wsTerminal(w http.ResponseWriter, r *http.Request) {
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	ctx := r.Context()
	execResp, err := dc.CLI.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd:          []string{"/bin/sh"},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		ws.WriteJSON(map[string]string{"error": err.Error()})
		return
	}

	attach, err := dc.CLI.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		ws.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer attach.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := attach.Reader.Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
			if _, err := attach.Conn.Write(msg); err != nil {
				break
			}
		}
	}
	attach.Close()
	<-done
}
