package resolved

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkFilterUnmark(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mark(Item{HostID: "local", ContainerID: "abc", Name: "api", Reason: "crash loop", By: "ana"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "resolved_problems.json")); err != nil {
		t.Fatal(err)
	}
	in := []Problem{
		{ContainerID: "abc", Name: "api", Reason: "crash loop"},
		{ContainerID: "abc", Name: "api", Reason: "parou com erro"},
		{ContainerID: "def", Name: "db", Reason: "crash loop"},
	}
	got := s.Filter("local", in)
	if len(got) != 2 {
		t.Fatalf("filtered %+v", got)
	}
	reopen, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopen.List("local")) != 1 {
		t.Fatalf("persisted %+v", reopen.List(""))
	}
	if err := reopen.Unmark("local", "abc", "crash loop"); err != nil {
		t.Fatal(err)
	}
	if len(reopen.Filter("local", in)) != 3 {
		t.Fatal("unmark failed")
	}
}
