// Package triage answers "o que quebrou": it remembers the last ship
// and decides whether to roll back, restart, or leave the host alone.
package triage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const shipFile = "last-ship.json"

// ShipRecord is one deploy (or rollback) written to last-ship.json.
type ShipRecord struct {
	At          time.Time `json:"at"`
	Tag         string    `json:"tag"`
	PreviousTag string    `json:"previousTag,omitempty"`
	Image       string    `json:"image,omitempty"`
	Project     string    `json:"project,omitempty"`
	HostID      string    `json:"hostId,omitempty"`
	Action      string    `json:"action,omitempty"`
	Actor       string    `json:"actor,omitempty"`
	Note        string    `json:"note,omitempty"`
}

type shipFileDoc struct {
	Hosts map[string]ShipRecord `json:"hosts"`
}

// ProblemBrief is the slice of a container problem the story needs.
type ProblemBrief struct {
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	Reason      string `json:"reason"`
	Severity    string `json:"severity"`
}

// IncidentBrief is a Log Center burst inside the triage window.
type IncidentBrief struct {
	StartMs     int64    `json:"startMs"`
	EndMs       int64    `json:"endMs"`
	Containers  []string `json:"containers"`
	EntryCount  int      `json:"entryCount"`
	SampleLines []string `json:"sampleLines"`
}

// Signals are live observations used by Decide. Zero values mean "unknown", not "healthy",
// except counts, which mean none observed.
type Signals struct {
	Now           time.Time
	RunningTag    string
	StartedAt     string
	Health        string
	RestartCount  int
	OOMKilled     bool
	CriticalCount int
	WarningCount  int
	Problems      []ProblemBrief
	Incidents     []IncidentBrief
	DockerNote    string
}

// Report is the JSON body of GET /api/triage.
type Report struct {
	Suggestion       string          `json:"suggestion"`
	SuggestionReason string          `json:"suggestionReason,omitempty"`
	PreviousTag      string          `json:"previousTag,omitempty"`
	Story            string          `json:"story"`
	RunningTag       string          `json:"runningTag,omitempty"`
	LastShip         *ShipRecord     `json:"lastShip,omitempty"`
	StartedAt        string          `json:"startedAt,omitempty"`
	Health           string          `json:"health,omitempty"`
	RestartCount     int             `json:"restartCount"`
	OOMKilled        bool            `json:"oomKilled"`
	CriticalCount    int             `json:"criticalCount"`
	WarningCount     int             `json:"warningCount"`
	Incidents        []IncidentBrief `json:"incidents"`
	Problems         []ProblemBrief  `json:"problems"`
}

func shipPath(dir string) string {
	if dir == "" {
		dir = "data"
	}
	return filepath.Join(dir, shipFile)
}

// SaveShip upserts the record for its host and rewrites last-ship.json.
func SaveShip(dir string, rec ShipRecord) error {
	if rec.At.IsZero() {
		rec.At = time.Now().UTC()
	}
	host := rec.HostID
	if host == "" {
		host = "default"
		rec.HostID = host
	}
	doc, err := readDoc(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if doc.Hosts == nil {
		doc.Hosts = map[string]ShipRecord{}
	}
	doc.Hosts[host] = rec
	if err := os.MkdirAll(dirOr(dir), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(shipPath(dir), raw, 0o644)
}

// LoadShipFromHost reads last-ship.json and returns the record for hostID.
// A file that is a single ShipRecord (older shape) is accepted for every host.
func LoadShipFromHost(dir, hostID string) (ShipRecord, error) {
	raw, err := os.ReadFile(shipPath(dir))
	if err != nil {
		return ShipRecord{}, err
	}
	var single ShipRecord
	if err := json.Unmarshal(raw, &single); err == nil && !single.At.IsZero() && single.Tag != "" && !bytesContainsHosts(raw) {
		if hostID == "" || single.HostID == "" || single.HostID == hostID {
			return single, nil
		}
	}
	doc, err := readDoc(dir)
	if err != nil {
		return ShipRecord{}, err
	}
	if hostID == "" {
		hostID = "default"
	}
	if rec, ok := doc.Hosts[hostID]; ok {
		return rec, nil
	}
	// Fall back to the newest record when the host has never shipped.
	var newest ShipRecord
	for _, rec := range doc.Hosts {
		if newest.At.IsZero() || rec.At.After(newest.At) {
			newest = rec
		}
	}
	if hostID != "default" && newest.HostID != "" && newest.HostID != hostID {
		return ShipRecord{}, os.ErrNotExist
	}
	return newest, nil
}

func bytesContainsHosts(raw []byte) bool {
	return strings.Contains(string(raw), `"hosts"`)
}

func readDoc(dir string) (shipFileDoc, error) {
	raw, err := os.ReadFile(shipPath(dir))
	if err != nil {
		return shipFileDoc{}, err
	}
	var doc shipFileDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return shipFileDoc{}, err
	}
	return doc, nil
}

func dirOr(dir string) string {
	if dir == "" {
		return "data"
	}
	return dir
}

// Decide picks rollback, restart or none from the last ship and live signals.
func Decide(ship ShipRecord, sig Signals) (suggestion, reason string) {
	now := sig.Now
	if now.IsZero() {
		now = time.Now()
	}
	recent := !ship.At.IsZero() && now.Sub(ship.At) <= 6*time.Hour
	unhealthy := sig.OOMKilled || sig.RestartCount >= 3 || sig.CriticalCount > 0 || healthBad(sig.Health)
	warn := sig.WarningCount > 0 || sig.RestartCount > 0 || len(sig.Incidents) > 0 || len(sig.Problems) > 0

	switch {
	case recent && unhealthy && ship.PreviousTag != "" && ship.PreviousTag != ship.Tag:
		return "rollback", fmt.Sprintf(
			"O ship %s (%s) entrou há pouco e a saúde piorou. Volte para %s.",
			or(ship.Tag, "sem tag"), ship.At.Local().Format("02/01 15:04"), ship.PreviousTag,
		)
	case recent && unhealthy:
		return "restart", "O ship recente coincidiu com falha e não há tag anterior para rollback. Reinicie o serviço e leia o Log Center antes de subir de novo."
	case unhealthy:
		return "restart", "Não há ship recente culpado. O container está instável — reinicie e investigue memória ou crash loop."
	case recent && warn && ship.PreviousTag != "" && ship.PreviousTag != ship.Tag:
		return "restart", "Há avisos depois do ship, ainda sem gravidade de rollback. Reinicie e acompanhe os logs por alguns minutos."
	case warn:
		return "restart", "Há sinais de instabilidade sem um ship claramente culpado. Reinicie o container afetado e confira o Log Center."
	default:
		if ship.At.IsZero() {
			return "none", "Nenhum ship registrado e nenhum sinal de quebra neste host."
		}
		return "none", "Nada indica que o último ship tenha quebrado o host."
	}
}

func healthBad(h string) bool {
	h = strings.ToLower(h)
	return strings.Contains(h, "unhealthy") || h == "dead" || h == "restarting"
}

// Build joins Decide with a Portuguese story. It is pure: no Docker, no disk.
func Build(ship ShipRecord, sig Signals) Report {
	suggestion, reason := Decide(ship, sig)
	running := sig.RunningTag
	if running == "" {
		running = ship.Tag
	}
	problems := sig.Problems
	if problems == nil {
		problems = []ProblemBrief{}
	}
	incidents := sig.Incidents
	if incidents == nil {
		incidents = []IncidentBrief{}
	}
	rep := Report{
		Suggestion:       suggestion,
		SuggestionReason: reason,
		PreviousTag:      ship.PreviousTag,
		Story:            writeStory(ship, sig, suggestion),
		RunningTag:       running,
		StartedAt:        sig.StartedAt,
		Health:           sig.Health,
		RestartCount:     sig.RestartCount,
		OOMKilled:        sig.OOMKilled,
		CriticalCount:    sig.CriticalCount,
		WarningCount:     sig.WarningCount,
		Incidents:        incidents,
		Problems:         problems,
	}
	if !ship.At.IsZero() {
		cp := ship
		rep.LastShip = &cp
	}
	return rep
}

func writeStory(ship ShipRecord, sig Signals, suggestion string) string {
	var b strings.Builder
	if ship.At.IsZero() {
		b.WriteString("Ainda não há last-ship neste host. ")
	} else {
		fmt.Fprintf(&b, "Último ship %s em %s", or(ship.Tag, "sem tag"), ship.At.Local().Format("02/01/2006 15:04"))
		if ship.PreviousTag != "" {
			fmt.Fprintf(&b, " (antes: %s)", ship.PreviousTag)
		}
		if ship.Project != "" {
			fmt.Fprintf(&b, ", projeto %s", ship.Project)
		}
		if ship.Actor != "" {
			fmt.Fprintf(&b, ", por %s", ship.Actor)
		}
		b.WriteString(". ")
	}
	if sig.DockerNote != "" {
		b.WriteString(sig.DockerNote)
		b.WriteString(" ")
	}
	fmt.Fprintf(&b, "Sinais agora: %d crítico(s), %d aviso(s), %d restart(s)", sig.CriticalCount, sig.WarningCount, sig.RestartCount)
	if sig.OOMKilled {
		b.WriteString(", OOM")
	}
	if sig.Health != "" {
		fmt.Fprintf(&b, ", health %s", sig.Health)
	}
	b.WriteString(". ")
	if len(sig.Incidents) > 0 {
		fmt.Fprintf(&b, "Log Center: %d incidente(s) nas últimas 6h. ", len(sig.Incidents))
		if len(sig.Incidents[0].SampleLines) > 0 {
			fmt.Fprintf(&b, "Amostra: %s. ", sig.Incidents[0].SampleLines[0])
		}
	} else {
		b.WriteString("Log Center sem incidente warning/critical na janela de 6h. ")
	}
	switch suggestion {
	case "rollback":
		fmt.Fprintf(&b, "O que fazer: rollback para %s e só então investigar a tag %s.", or(ship.PreviousTag, "a tag anterior"), or(ship.Tag, "atual"))
	case "restart":
		b.WriteString("O que fazer: reiniciar o container afetado, ler o Log Center e não subir outra tag até o health voltar.")
	default:
		b.WriteString("O que fazer: nada. O host não mostra quebra ligada ao último ship.")
	}
	return strings.TrimSpace(b.String())
}

func or(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
