package main

import "testing"

func TestEnsureWritable(t *testing.T) {
	t.Setenv("DOCKERPILOT_READONLY", "1")
	if err := ensureWritable(); err == nil {
		t.Fatal("expected block")
	}
	t.Setenv("DOCKERPILOT_READONLY", "")
	t.Setenv("DOCKPANEL_READONLY", "")
	if err := ensureWritable(); err != nil {
		t.Fatal(err)
	}
}
