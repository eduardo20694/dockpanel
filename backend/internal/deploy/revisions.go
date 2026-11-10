package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Revision is a stored compose snapshot used for update and rollback.
type Revision struct {
	ID      string    `json:"id"`
	Project string    `json:"project"`
	Tag     string    `json:"tag,omitempty"`
	At      time.Time `json:"at"`
	Actor   string    `json:"actor,omitempty"`
	Path    string    `json:"path,omitempty"`
	Compose string    `json:"compose"`
	Action  string    `json:"action,omitempty"`
}

func revisionsDir(dataDir string) string {
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Join(dataDir, "stack-revisions")
}

// SaveRevision writes one snapshot and returns it with an id.
func SaveRevision(dataDir string, rev Revision) (Revision, error) {
	if strings.TrimSpace(rev.Compose) == "" || strings.TrimSpace(rev.Project) == "" {
		return Revision{}, errors.New("projeto e compose são obrigatórios")
	}
	if rev.ID == "" {
		rev.ID = newRevID()
	}
	if rev.At.IsZero() {
		rev.At = time.Now().UTC()
	}
	if rev.Tag == "" {
		_, rev.Tag = FirstImageTag(rev.Compose)
	}
	dir := revisionsDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Revision{}, err
	}
	raw, err := json.MarshalIndent(rev, "", "  ")
	if err != nil {
		return Revision{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, rev.ID+".json"), raw, 0o644); err != nil {
		return Revision{}, err
	}
	return rev, nil
}

// ListRevisions returns snapshots for a project (or all when project is empty), newest first.
func ListRevisions(dataDir, project string) ([]Revision, error) {
	dir := revisionsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Revision{}, nil
		}
		return nil, err
	}
	var out []Revision
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		rev, err := readRevision(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if project != "" && rev.Project != project {
			continue
		}
		out = append(out, rev)
	}
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j].At.After(out[j-1].At) {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	if out == nil {
		out = []Revision{}
	}
	return out, nil
}

// LoadRevision reads one snapshot by id.
func LoadRevision(dataDir, id string) (Revision, error) {
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return Revision{}, errors.New("id inválido")
	}
	return readRevision(filepath.Join(revisionsDir(dataDir), id+".json"))
}

func readRevision(path string) (Revision, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Revision{}, err
	}
	var rev Revision
	if err := json.Unmarshal(raw, &rev); err != nil {
		return Revision{}, err
	}
	return rev, nil
}

// WriteComposeFile replaces the compose file in projectPath with the snapshot body.
func WriteComposeFile(projectPath, compose string) (string, error) {
	if strings.TrimSpace(projectPath) == "" {
		return "", errors.New("projectPath vazio")
	}
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(projectPath, "docker-compose.yml")
	if existing, err := findComposeFile(projectPath); err == nil && existing != "" {
		name = filepath.Join(projectPath, existing)
	}
	if err := os.WriteFile(name, []byte(compose), 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// FirstImageTag returns the first image reference and its tag.
func FirstImageTag(compose string) (image, tag string) {
	for _, line := range strings.Split(compose, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "image:") {
			continue
		}
		image = strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "image:")), `"'`)
		if i := strings.LastIndex(image, ":"); i > 0 {
			tail := image[i+1:]
			if !strings.Contains(tail, "/") {
				return image, tail
			}
		}
		if image != "" {
			return image, "latest"
		}
	}
	return "", ""
}

func newRevID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}
