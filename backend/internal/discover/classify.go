// Package discover identifica Docker, Podman, Swarm e Kubernetes a partir do que o daemon e o kubeconfig respondem.
package discover

import "strings"

// Facts é o que se leu de um daemon, sem decidir o tipo.
type Facts struct {
	Reachable  bool
	Err        string
	Runtime    string // docker ou podman
	SwarmState string // inactive, active, pending, error, locked
	Control    bool
	NodeID     string
	SwarmErr   string
}

// Result é a experiência que o produto deve abrir.
type Result struct {
	Kind         string   `json:"kind"`
	DetectedKind string   `json:"detectedKind"`
	Role         string   `json:"role,omitempty"`
	Status       string   `json:"status"`
	Detail       string   `json:"detail"`
	Capabilities []string `json:"capabilities"`
	Manual       bool     `json:"manual"`
}

// Classify decide o tipo. kind vazio ou "auto" segue a detecção.
// Um kind explícito permanece, e a detecção fica em detectedKind.
func Classify(manualKind string, facts Facts) Result {
	manual := explicit(manualKind)
	detected, role, status, detail, caps := fromFacts(facts)
	out := Result{
		DetectedKind: detected,
		Role:         role,
		Status:       status,
		Detail:       detail,
		Capabilities: caps,
		Manual:       manual,
	}
	if manual {
		out.Kind = normalize(manualKind)
		if out.Kind != detected {
			out.Detail = "Configurado manualmente como " + label(out.Kind) + ". A detecção indica " + label(detected) + ". " + detail
		}
		if out.Kind == "swarm" && role != "manager" {
			out.Capabilities = engineCaps(detected)
			out.Status = "attention"
		}
		return out
	}
	out.Kind = detected
	return out
}

func explicit(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "auto":
		return false
	default:
		return true
	}
}

func normalize(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "k8s", "kubernetes":
		return "kubernetes"
	case "podman":
		return "podman"
	case "swarm":
		return "swarm"
	default:
		return "docker"
	}
}

func fromFacts(f Facts) (kind, role, status, detail string, caps []string) {
	if !f.Reachable {
		kind = "docker"
		if f.Runtime == "podman" {
			kind = "podman"
		}
		msg := strings.TrimSpace(f.Err)
		if msg == "" {
			msg = "sem resposta do daemon"
		}
		return kind, "", "offline", "Falha de conexão: " + msg, []string{}
	}
	base := "docker"
	if f.Runtime == "podman" {
		base = "podman"
	}
	state := strings.ToLower(strings.TrimSpace(f.SwarmState))
	switch {
	case state == "active" && f.Control:
		return "swarm", "manager", "online", label(base) + " com Swarm ativo. Este nó é manager: a gestão do Swarm está disponível.", append(engineCaps(base), "swarm")
	case state == "active" && !f.Control && f.NodeID != "":
		return "swarm", "worker", "online", label(base) + " com Swarm ativo. Este nó é worker: a gestão do Swarm fica no manager.", engineCaps(base)
	case state == "error" || state == "locked":
		msg := f.SwarmErr
		if msg == "" {
			msg = "Swarm em estado " + state
		}
		return base, "", "attention", label(base) + " respondeu, mas o Swarm não está utilizável: " + msg, engineCaps(base)
	default:
		return base, "", "online", label(base) + " sem Swarm ativo.", engineCaps(base)
	}
}

func engineCaps(kind string) []string {
	if kind == "podman" {
		return []string{"containers", "compose", "imagens", "podman"}
	}
	return []string{"containers", "compose", "imagens"}
}

func label(kind string) string {
	switch kind {
	case "podman":
		return "Podman"
	case "swarm":
		return "Swarm"
	case "kubernetes":
		return "Kubernetes"
	default:
		return "Docker"
	}
}

// ManageSwarm é verdadeiro só no manager.
func (r Result) ManageSwarm() bool {
	return r.Kind == "swarm" && r.Role == "manager" && r.Status != "offline"
}
