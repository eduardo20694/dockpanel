package proxy

import (
	"context"
	"testing"
)

func TestValidatePath(t *testing.T) {
	ok := []string{"/containers/json", "/v1/pods", "/networks/abc"}
	for _, p := range ok {
		if err := ValidatePath(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	bad := []string{"", "containers", "/../etc/passwd", "/x?y=1", "/a#b", `/\x`, "/http://evil"}
	for _, p := range bad {
		if err := ValidatePath(p); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestReadOnlyMethod(t *testing.T) {
	if err := ValidateMethod("GET", true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMethod("POST", true); err != ErrReadOnly {
		t.Fatalf("got %v", err)
	}
	if err := ValidateMethod("DELETE", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMethod("TRACE", false); err == nil {
		t.Fatal("trace should fail")
	}
}

func TestKubernetesUnconfigured(t *testing.T) {
	t.Setenv("DOCKERPILOT_K8S_API", "")
	t.Setenv("DOCKPANEL_K8S_API", "")
	_, _, err := DoKubernetes(context.Background(), "GET", "/api/v1/pods", nil, true)
	if err != ErrK8sUnconfigured {
		t.Fatalf("got %v", err)
	}
}
