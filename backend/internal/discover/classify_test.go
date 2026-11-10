package discover

import "testing"

func TestClassifyMatrix(t *testing.T) {
	cases := []struct {
		name   string
		manual string
		facts  Facts
		kind   string
		role   string
		status string
		manage bool
	}{
		{
			name:   "docker sem swarm",
			facts:  Facts{Reachable: true, Runtime: "docker", SwarmState: "inactive"},
			kind:   "docker",
			status: "online",
		},
		{
			name:   "podman sem swarm",
			facts:  Facts{Reachable: true, Runtime: "podman", SwarmState: "inactive"},
			kind:   "podman",
			status: "online",
		},
		{
			name:   "swarm manager",
			facts:  Facts{Reachable: true, Runtime: "docker", SwarmState: "active", Control: true, NodeID: "n1"},
			kind:   "swarm",
			role:   "manager",
			status: "online",
			manage: true,
		},
		{
			name:   "swarm worker",
			facts:  Facts{Reachable: true, Runtime: "docker", SwarmState: "active", NodeID: "n2"},
			kind:   "swarm",
			role:   "worker",
			status: "online",
		},
		{
			name:   "falha de conexão",
			facts:  Facts{Err: "dial unix:///tmp/ausente.sock: connect: no such file"},
			kind:   "docker",
			status: "offline",
		},
		{
			name:   "podman inacessível pelo socket",
			facts:  Facts{Runtime: "podman", Err: "connection refused"},
			kind:   "podman",
			status: "offline",
		},
		{
			name:   "manual preservado quando diverge",
			manual: "docker",
			facts:  Facts{Reachable: true, Runtime: "docker", SwarmState: "active", Control: true, NodeID: "n1"},
			kind:   "docker",
			role:   "manager",
			status: "online",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.manual, tc.facts)
			if got.Kind != tc.kind || got.Role != tc.role || got.Status != tc.status || got.ManageSwarm() != tc.manage {
				t.Fatalf("%+v", got)
			}
			if got.Status == "offline" && got.Detail == "" {
				t.Fatal("offline sem motivo")
			}
			if tc.role == "worker" && got.ManageSwarm() {
				t.Fatal("worker não gere swarm")
			}
			if tc.manual == "docker" && got.DetectedKind != "swarm" {
				t.Fatalf("detected %+v", got)
			}
		})
	}
}
