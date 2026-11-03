package updates

import (
	"strconv"
	"strings"
)

type Ref struct {
	Host string
	Repo string
	Tag  string
}

func Parse(image string) Ref {
	image = strings.TrimSpace(image)
	if i := strings.Index(image, "@"); i >= 0 {
		image = image[:i]
	}
	host := "docker.io"
	rest := image
	tag := "latest"
	if i := strings.LastIndex(rest, ":"); i > 0 && !strings.Contains(rest[i:], "/") {
		tag = rest[i+1:]
		rest = rest[:i]
	}
	if strings.Contains(rest, ".") || strings.Contains(rest, ":") || rest == "localhost" || strings.HasPrefix(rest, "localhost/") {
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 {
			host = parts[0]
			rest = parts[1]
		}
	}
	if host == "docker.io" && !strings.Contains(rest, "/") {
		rest = "library/" + rest
	}
	return Ref{Host: host, Repo: rest, Tag: tag}
}

type semver struct {
	major, minor, patch int
	ok                  bool
}

func parseSemver(tag string) semver {
	tag = strings.TrimPrefix(tag, "v")
	parts := strings.Split(tag, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semver{}
	}
	var n [3]int
	for i := 0; i < len(parts); i++ {
		v, err := strconv.Atoi(parts[i])
		if err != nil {
			return semver{}
		}
		n[i] = v
	}
	return semver{major: n[0], minor: n[1], patch: n[2], ok: true}
}

func (a semver) less(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

func newerTag(current string, tags []string) string {
	cur := parseSemver(current)
	if !cur.ok {
		return ""
	}
	best := ""
	var bestV semver
	for _, t := range tags {
		v := parseSemver(t)
		if !v.ok || !cur.less(v) {
			continue
		}
		if best == "" || bestV.less(v) {
			best = t
			bestV = v
		}
	}
	return best
}
