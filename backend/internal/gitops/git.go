package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitCLI struct{}

func (GitCLI) Fetch(ctx context.Context, link Link) (Remote, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return Remote{}, fmt.Errorf("git não está instalado")
	}
	branch := link.Branch
	if branch == "" {
		branch = "main"
	}
	path := link.ComposePath
	if path == "" {
		path = "docker-compose.yml"
	}
	dir, err := os.MkdirTemp("", "dockpilot-git-*")
	if err != nil {
		return Remote{}, err
	}
	defer os.RemoveAll(dir)
	url := link.RepoURL
	env := os.Environ()
	if strings.Contains(link.Credential, "BEGIN") {
		key := filepath.Join(dir, "deploy_key")
		if err := os.WriteFile(key, []byte(link.Credential), 0o600); err != nil {
			return Remote{}, err
		}
		env = append(env, "GIT_SSH_COMMAND=ssh -i "+key+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new")
	} else if tok := strings.TrimSpace(link.Credential); tok != "" && strings.HasPrefix(url, "https://") {
		url = strings.Replace(url, "https://", "https://x-access-token:"+tok+"@", 1)
	} else if tok != "" && strings.HasPrefix(url, "http://") {
		url = strings.Replace(url, "http://", "http://x-access-token:"+tok+"@", 1)
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, url, filepath.Join(dir, "src"))
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Remote{}, fmt.Errorf("git clone: %s", sanitizeErr(fmt.Errorf("%w %s", err, out), link.Credential))
	}
	src := filepath.Join(dir, "src")
	body, err := os.ReadFile(filepath.Join(src, filepath.Clean(path)))
	if err != nil {
		return Remote{}, fmt.Errorf("compose %s: %w", path, err)
	}
	rev := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	rev.Dir = src
	commit, err := rev.Output()
	if err != nil {
		return Remote{}, err
	}
	return Remote{Commit: strings.TrimSpace(string(commit)), Compose: body}, nil
}
