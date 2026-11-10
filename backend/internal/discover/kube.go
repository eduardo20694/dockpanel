package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockerpilot/internal/dockerclient"

	"k8s.io/client-go/tools/clientcmd"
)

const saRoot = "/var/run/secrets/kubernetes.io/serviceaccount"

// ServiceAccountPresent diz se este processo está dentro de um cluster.
func ServiceAccountPresent() bool {
	return serviceAccountAt(saRoot)
}

func serviceAccountAt(root string) bool {
	_, err := os.Stat(filepath.Join(root, "token"))
	return err == nil
}

// ExpandKube abre um kubeconfig e devolve um ambiente por contexto.
// Se o contexto vier preenchido, só esse entra. A configuração manual não é descartada.
func ExpandKube(id, label, path, context string) ([]KubeTarget, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sem kubeconfig")
	}
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return []KubeTarget{{
			ID: id, Label: label, Kubeconfig: path, Context: context,
			Status: "offline", Detail: "Falha ao ler kubeconfig: " + err.Error(),
		}}, nil
	}
	if label == "" {
		label = id
	}
	if context != "" {
		if _, ok := cfg.Contexts[context]; !ok {
			return []KubeTarget{{
				ID: id, Label: label, Kubeconfig: path, Context: context,
				Status: "offline", Detail: "Contexto " + context + " não existe neste kubeconfig.",
			}}, nil
		}
		return []KubeTarget{{
			ID: id, Label: label, Kubeconfig: path, Context: context, Status: "online",
			Detail: "Contexto " + context + " escolhido na configuração.",
		}}, nil
	}
	if len(cfg.Contexts) == 0 {
		return []KubeTarget{{
			ID: id, Label: label, Kubeconfig: path, Status: "offline",
			Detail: "O kubeconfig não tem contextos.",
		}}, nil
	}
	var out []KubeTarget
	for name := range cfg.Contexts {
		out = append(out, KubeTarget{
			ID: id + "--" + sanitize(name), Label: label + " · " + name,
			Kubeconfig: path, Context: name, Status: "online",
			Detail: "Contexto " + name + " descoberto no kubeconfig.",
		})
	}
	if len(out) > 1 {
		for i := range out {
			out[i].Detail += " Há mais de um contexto: escolha o ambiente no seletor."
		}
	}
	return out, nil
}

// KubeTargetsFromHosts preserva hosts já marcados como Kubernetes e expande kubeconfigs.
func KubeTargetsFromHosts(hosts []dockerclient.HostConfig) []KubeTarget {
	var out []KubeTarget
	for _, h := range hosts {
		kind := strings.ToLower(strings.TrimSpace(h.Kind))
		if h.Kubeconfig == "" && kind != "kubernetes" && kind != "k8s" {
			continue
		}
		if h.Kubeconfig == "" {
			out = append(out, KubeTarget{
				ID: h.ID, Label: h.Label, Status: "offline",
				Detail: "Host Kubernetes sem kubeconfig e sem conta de serviço.",
			})
			continue
		}
		expanded, _ := ExpandKube(h.ID, h.Label, h.Kubeconfig, h.Context)
		out = append(out, expanded...)
	}
	return out
}

type KubeTarget struct {
	ID         string
	Label      string
	Kubeconfig string
	Context    string
	InCluster  bool
	Status     string
	Detail     string
}

func InClusterTarget() KubeTarget {
	if ServiceAccountPresent() {
		return KubeTarget{ID: "incluster", Label: "Cluster local", InCluster: true, Status: "online", Detail: "Conta de serviço do pod detectada."}
	}
	return KubeTarget{ID: "incluster", Label: "Cluster local", InCluster: true, Status: "offline", Detail: "DOCKERPILOT_K8S_INCLUSTER está ligado, mas a conta de serviço não está montada."}
}

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "ctx"
	}
	return s
}
