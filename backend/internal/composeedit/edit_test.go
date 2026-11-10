package composeedit

import (
	"strings"
	"testing"
)

func TestParseRender(t *testing.T) {
	in := "services:\n  api:\n    image: api:1\n    ports:\n      - \"8080:80\"\n    environment:\n      LOG: info\n"
	svcs, err := Parse(in)
	if err != nil || len(svcs) != 1 || svcs[0].Image != "api:1" || len(svcs[0].Env) != 1 {
		t.Fatalf("%+v %v", svcs, err)
	}
	svcs[0].Image = "api:2"
	out := Render(svcs)
	if !strings.Contains(out, "image: api:2") || !strings.Contains(out, "8080:80") {
		t.Fatal(out)
	}
}
