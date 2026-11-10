package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndLoadShip(t *testing.T) {
	dir := t.TempDir()
	rec := ShipRecord{
		At:          time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC),
		Tag:         "1.0.8",
		PreviousTag: "1.0.7",
		Image:       "redecoop/dockpilot:1.0.8",
		Project:     "dockpilot",
		HostID:      "local",
		Action:      "up",
		Actor:       "ana@org.dev",
	}
	if err := SaveShip(dir, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "last-ship.json")); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShipFromHost(dir, "local")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "1.0.8" || got.PreviousTag != "1.0.7" || got.Actor != "ana@org.dev" {
		t.Fatalf("%+v", got)
	}
	if _, err := LoadShipFromHost(dir, "other"); err == nil {
		t.Fatal("expected missing host")
	}
}

func TestDecideRollbackWhenRecentShipAndCritical(t *testing.T) {
	ship := ShipRecord{At: time.Now().Add(-20 * time.Minute), Tag: "1.2.0", PreviousTag: "1.1.0", HostID: "local"}
	sig := Signals{Now: time.Now(), CriticalCount: 2, RestartCount: 6, Health: "unhealthy", OOMKilled: true}
	sug, reason := Decide(ship, sig)
	if sug != "rollback" {
		t.Fatalf("suggestion %s (%s)", sug, reason)
	}
	if !strings.Contains(reason, "1.1.0") {
		t.Fatalf("reason %s", reason)
	}
	rep := Build(ship, sig)
	if rep.Suggestion != "rollback" || rep.LastShip == nil || !strings.Contains(rep.Story, "rollback") {
		t.Fatalf("%+v", rep)
	}
}

func TestDecideNoneWhenQuiet(t *testing.T) {
	ship := ShipRecord{At: time.Now().Add(-2 * time.Hour), Tag: "1.2.0", PreviousTag: "1.1.0"}
	sug, _ := Decide(ship, Signals{Now: time.Now(), Health: "healthy"})
	if sug != "none" {
		t.Fatalf("got %s", sug)
	}
}

func TestDecideRestartWithoutPreviousTag(t *testing.T) {
	ship := ShipRecord{At: time.Now().Add(-10 * time.Minute), Tag: "9.9.9"}
	sug, _ := Decide(ship, Signals{Now: time.Now(), CriticalCount: 1})
	if sug != "restart" {
		t.Fatalf("got %s", sug)
	}
}

func TestLegacySingleRecord(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"at":"2026-09-22T18:00:00Z","tag":"1.0.8","hostId":"local","previousTag":"1.0.7"}`)
	if err := os.WriteFile(filepath.Join(dir, "last-ship.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShipFromHost(dir, "local")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "1.0.8" {
		t.Fatalf("%+v", got)
	}
}
