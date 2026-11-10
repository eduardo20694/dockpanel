// Package kube fala com a API do Kubernetes (kubeconfig ou service account),
// sem passar por um proxy genérico.
package kube

import (
	"fmt"
	"os"
	"strings"
	"time"

	"dockerpilot/internal/discover"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Cluster struct {
	ID     string
	Label  string
	Status string
	Detail string
	Config *rest.Config
	Client kubernetes.Interface
}

type Set struct {
	clusters []*Cluster
	byID     map[string]*Cluster
}

func NewSet(clusters []*Cluster) *Set {
	s := &Set{byID: map[string]*Cluster{}}
	for _, c := range clusters {
		if c == nil || c.ID == "" {
			continue
		}
		s.clusters = append(s.clusters, c)
		s.byID[c.ID] = c
	}
	return s
}

func (s *Set) List() []Cluster {
	if s == nil {
		return nil
	}
	out := make([]Cluster, 0, len(s.clusters))
	for _, c := range s.clusters {
		out = append(out, Cluster{ID: c.ID, Label: c.Label, Status: c.Status, Detail: c.Detail})
	}
	return out
}

func (s *Set) Get(id string) (*Cluster, error) {
	if s == nil || len(s.clusters) == 0 {
		return nil, fmt.Errorf("nenhum cluster Kubernetes configurado")
	}
	if id == "" || id == "default" {
		return s.clusters[0], nil
	}
	c, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("cluster desconhecido: %q", id)
	}
	return c, nil
}

type Source struct {
	ID         string
	Label      string
	Kubeconfig string
	Context    string
	InCluster  bool
}

func Open(sources []Source) *Set {
	var clusters []*Cluster
	for _, src := range sources {
		c, err := openOne(src)
		if err != nil {
			detail := "Falha de conexão: " + err.Error()
			if src.Context != "" {
				detail = "Contexto " + src.Context + ": " + detail
			}
			clusters = append(clusters, &Cluster{ID: src.ID, Label: src.Label, Status: "offline", Detail: detail})
			continue
		}
		clusters = append(clusters, c)
	}
	return NewSet(clusters)
}

func openOne(src Source) (*Cluster, error) {
	var cfg *rest.Config
	var err error
	if src.InCluster {
		cfg, err = rest.InClusterConfig()
	} else if src.Kubeconfig != "" {
		rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: src.Kubeconfig}
		overrides := &clientcmd.ConfigOverrides{}
		if src.Context != "" {
			overrides.CurrentContext = src.Context
		}
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	} else {
		return nil, fmt.Errorf("sem kubeconfig")
	}
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	probeCfg := rest.CopyConfig(cfg)
	probeCfg.Timeout = 4 * time.Second
	prober, err := kubernetes.NewForConfig(probeCfg)
	if err != nil {
		return nil, err
	}
	if _, err := prober.Discovery().ServerVersion(); err != nil {
		return nil, err
	}
	label := src.Label
	if label == "" {
		label = src.ID
	}
	status, detail := "online", "Cluster acessível."
	if src.InCluster {
		detail = "Conta de serviço do pod."
	} else if src.Context != "" {
		detail = "Contexto " + src.Context + " do kubeconfig."
	}
	return &Cluster{ID: src.ID, Label: label, Status: status, Detail: detail, Config: cfg, Client: cs}, nil
}

// SourcesFromEnv lê DOCKERPILOT_KUBECONFIG (caminho ou lista separada por vírgula)
// e DOCKERPILOT_K8S_INCLUSTER=1.
func SourcesFromEnv() []Source {
	var out []Source
	raw := strings.TrimSpace(os.Getenv("DOCKERPILOT_KUBECONFIG"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("KUBECONFIG"))
	}
	if raw != "" {
		parts := strings.Split(raw, ",")
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			id := "k8s"
			if len(parts) > 1 {
				id = fmt.Sprintf("k8s-%d", i+1)
			}
			targets, _ := discover.ExpandKube(id, id, p, "")
			for _, target := range targets {
				out = append(out, Source{ID: target.ID, Label: target.Label, Kubeconfig: target.Kubeconfig, Context: target.Context})
			}
		}
	}
	if os.Getenv("DOCKERPILOT_K8S_INCLUSTER") == "1" || discover.ServiceAccountPresent() {
		out = append(out, Source{ID: "incluster", Label: "Cluster local", InCluster: true})
	}
	return out
}
