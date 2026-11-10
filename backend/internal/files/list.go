package files

import (
	"bufio"
	"strconv"
	"strings"
)

type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

// ParseLS reads `ls -ln` output. Directories end the permission with d.
func ParseLS(dir, raw string) []Entry {
	var out []Entry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := fields[len(fields)-1]
		if name == "." || name == ".." {
			continue
		}
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		out = append(out, Entry{
			Name: name,
			Path: strings.TrimRight(dir, "/") + "/" + name,
			Dir:  strings.HasPrefix(fields[0], "d"),
			Size: size,
		})
	}
	if out == nil {
		out = []Entry{}
	}
	return out
}
