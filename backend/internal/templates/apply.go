package templates

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Request struct {
	ID          string
	Stack       string
	ProjectPath string
	Vars        map[string]string
}

type Prepared struct {
	Dir     string
	Compose []byte
	Secrets map[string]string
	Env     []string
}

func Prepare(req Request, dataDir string) (Prepared, error) {
	tpl, ok := ByID(req.ID)
	if !ok {
		return Prepared{}, errors.New("template não encontrado")
	}
	if strings.TrimSpace(req.Stack) == "" {
		return Prepared{}, errors.New("stack obrigatória")
	}
	secrets := map[string]string{}
	env := []string{}
	body := tpl.Compose
	for _, v := range tpl.Vars {
		val := ""
		if req.Vars != nil {
			val = req.Vars[v.Key]
		}
		if val == "" {
			val = v.Default
		}
		if val == "" {
			return Prepared{}, errors.New("informe " + v.Key)
		}
		env = append(env, v.Key+"="+val)
		if v.Secret {
			secrets[v.Key] = val
			continue
		}
		body = strings.ReplaceAll(body, "${"+v.Key+"}", val)
	}
	dir := req.ProjectPath
	if dir == "" {
		if dataDir == "" {
			dataDir = "data"
		}
		dir = filepath.Join(dataDir, "stacks", req.Stack)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Prepared{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(body), 0o644); err != nil {
		return Prepared{}, err
	}
	return Prepared{Dir: dir, Compose: []byte(body), Secrets: secrets, Env: env}, nil
}
