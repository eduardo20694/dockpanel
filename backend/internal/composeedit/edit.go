package composeedit

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type Service struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Ports []string `json:"ports"`
	Env   []string `json:"env"`
}

type file struct {
	Services map[string]struct {
		Image       string    `yaml:"image"`
		Ports       []string  `yaml:"ports"`
		Environment yaml.Node `yaml:"environment"`
	} `yaml:"services"`
}

func Parse(text string) ([]Service, error) {
	var doc file
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	var out []Service
	for name, svc := range doc.Services {
		env := envList(svc.Environment)
		ports := svc.Ports
		if ports == nil {
			ports = []string{}
		}
		out = append(out, Service{Name: name, Image: svc.Image, Ports: ports, Env: env})
	}
	if out == nil {
		out = []Service{}
	}
	return out, nil
}

func envList(n yaml.Node) []string {
	if n.Kind == yaml.SequenceNode {
		var out []string
		for _, c := range n.Content {
			if c.Value != "" {
				out = append(out, c.Value)
			}
		}
		return out
	}
	if n.Kind == yaml.MappingNode {
		var out []string
		for i := 0; i+1 < len(n.Content); i += 2 {
			out = append(out, n.Content[i].Value+"="+n.Content[i+1].Value)
		}
		return out
	}
	return []string{}
}

func Render(services []Service) string {
	var b strings.Builder
	b.WriteString("services:\n")
	for _, s := range services {
		b.WriteString("  " + s.Name + ":\n")
		b.WriteString("    image: " + s.Image + "\n")
		if len(s.Ports) > 0 {
			b.WriteString("    ports:\n")
			for _, p := range s.Ports {
				b.WriteString("      - \"" + p + "\"\n")
			}
		}
		if len(s.Env) > 0 {
			b.WriteString("    environment:\n")
			for _, e := range s.Env {
				b.WriteString("      - " + e + "\n")
			}
		}
	}
	return b.String()
}
