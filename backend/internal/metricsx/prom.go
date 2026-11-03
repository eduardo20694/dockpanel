package metricsx

import (
	"fmt"
	"net/http"
	"time"
)

type Snapshot struct {
	Hosts      int
	Containers int
	Alerts     int
	Deploys    int
}

func Text(s Snapshot) string {
	return fmt.Sprintf(`# HELP dockpilot_hosts Ambientes configurados.
# TYPE dockpilot_hosts gauge
dockpilot_hosts %d
# HELP dockpilot_containers Containers visíveis.
# TYPE dockpilot_containers gauge
dockpilot_containers %d
# HELP dockpilot_alerts Alertas recentes.
# TYPE dockpilot_alerts gauge
dockpilot_alerts %d
# HELP dockpilot_deploys Deploys registrados.
# TYPE dockpilot_deploys gauge
dockpilot_deploys %d
`, s.Hosts, s.Containers, s.Alerts, s.Deploys)
}

func Handler(s Snapshot) http.HandlerFunc {
	body := Text(s)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(body))
	}
}

func Now() int64 { return time.Now().UnixNano() }
