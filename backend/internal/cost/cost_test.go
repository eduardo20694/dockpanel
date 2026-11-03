package cost

import "testing"

func TestEstimate(t *testing.T) {
	rates := Rates{CPUPerCoreHour: 0.04, MemPerGBHour: 0.005}
	// 1 core + 2 GB for 24h = 0.04*24 + 0.005*2*24 = 0.96 + 0.24 = 1.20
	got := EstimateCPUMem(1, 2, rates)
	if got != 1.2 {
		t.Fatalf("got %v", got)
	}
	sum := FromReservations([]Reservation{{Name: "api", NanoCPUs: 1e9, MemoryBytes: 2 << 30}}, rates)
	if sum.USDPerDay != 1.2 || len(sum.Items) != 1 || sum.Items[0].Name != "api" {
		t.Fatalf("%+v", sum)
	}
}

func TestRatesFromEnvLegacy(t *testing.T) {
	t.Setenv("DOCKERPILOT_CPU_USD_HOUR", "")
	t.Setenv("DOCKPANEL_CPU_USD_HOUR", "0.1")
	t.Setenv("DOCKERPILOT_MEM_GB_USD_HOUR", "0.02")
	r := RatesFromEnv()
	if r.CPUPerCoreHour != 0.1 || r.MemPerGBHour != 0.02 {
		t.Fatalf("%+v", r)
	}
}
