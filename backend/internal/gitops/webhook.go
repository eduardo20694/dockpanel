package gitops

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

var ErrBadSignature = errors.New("assinatura do webhook inválida")

func sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Verify accepts GitHub X-Hub-Signature-256 and GitLab HMAC (X-Gitlab-Signature-256) or the GitLab secret token.
func Verify(body []byte, githubSig, gitlabSig, gitlabToken, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return errors.New("webhook sem segredo")
	}
	mac := sign([]byte(secret), body)
	if githubSig != "" && hmac.Equal([]byte(strings.TrimPrefix(strings.TrimSpace(githubSig), "sha256=")), []byte(mac)) {
		return nil
	}
	if gitlabSig != "" && hmac.Equal([]byte(strings.TrimPrefix(strings.TrimSpace(gitlabSig), "sha256=")), []byte(mac)) {
		return nil
	}
	if gitlabToken != "" && hmac.Equal([]byte(gitlabToken), []byte(secret)) {
		return nil
	}
	return ErrBadSignature
}

type hookMeta struct {
	Ref   string
	Repos []string
}

func parseHook(body []byte) hookMeta {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return hookMeta{}
	}
	meta := hookMeta{Ref: branchOf(str(raw["ref"]))}
	if repo, ok := raw["repository"].(map[string]any); ok {
		for _, k := range []string{"clone_url", "ssh_url", "html_url", "git_url"} {
			if v := str(repo[k]); v != "" {
				meta.Repos = append(meta.Repos, v)
			}
		}
	}
	if proj, ok := raw["project"].(map[string]any); ok {
		for _, k := range []string{"git_http_url", "git_ssh_url", "http_url", "web_url"} {
			if v := str(proj[k]); v != "" {
				meta.Repos = append(meta.Repos, v)
			}
		}
	}
	return meta
}

func branchOf(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func repoKey(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(raw, ".git")
	for _, p := range []string{"https://", "http://", "ssh://", "git@"} {
		raw = strings.TrimPrefix(raw, p)
	}
	raw = strings.ReplaceAll(raw, ":", "/")
	return strings.Trim(raw, "/")
}

func sameRepo(a, b string) bool {
	return repoKey(a) != "" && repoKey(a) == repoKey(b)
}
