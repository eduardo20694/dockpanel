// Package cost estimates daily spend from container CPU and memory reservations.
// Rates are configurable and default to a published on-demand ballpark, not a cloud bill.
package cost

import (
	"strconv"

	"dockerpilot/internal/envx"
)

// Rates are USD per hour.
type Rates struct {
	CPUPerCoreHour float64 `json:"cpuPerCoreHour"`
	MemPerGBHour   float64 `json:"memPerGBHour"`
}

// Item is one container's estimated daily cost.
type Item struct {
	Name      string  `json:"name"`
	CPUCores  float64 `json:"cpuCores"`
	MemGB     float64 `json:"memGB"`
	USDPerDay float64 `json:"usdPerDay"`
}

// Summary is the API payload.
type Summary struct {
	Rates     Rates   `json:"rates"`
	Items     []Item  `json:"items"`
	USDPerDay float64 `json:"usdPerDay"`
	Note      string  `json:"note"`
}

func DefaultRates() Rates {
	return Rates{CPUPerCoreHour: 0.04, MemPerGBHour: 0.005}
}

// RatesFromEnv reads DOCKERPILOT_CPU_USD_HOUR and DOCKERPILOT_MEM_GB_USD_HOUR.
func RatesFromEnv() Rates {
	r := DefaultRates()
	if v := envx.Get("CPU_USD_HOUR"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 {
			r.CPUPerCoreHour = n
		}
	}
	if v := envx.Get("MEM_GB_USD_HOUR"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 {
			r.MemPerGBHour = n
		}
	}
	return r
}

// EstimateCPUMem returns USD per day for a steady reservation.
func EstimateCPUMem(cpuCores, memGB float64, rates Rates) float64 {
	if cpuCores < 0 {
		cpuCores = 0
	}
	if memGB < 0 {
		memGB = 0
	}
	perHour := cpuCores*rates.CPUPerCoreHour + memGB*rates.MemPerGBHour
	return round2(perHour * 24)
}

// FromReservations builds a summary. nanoCPUs is Docker's NanoCPUs (1e9 = 1 core).
// memoryBytes is the memory limit. Zero reservations contribute nothing but still list the container.
func FromReservations(rows []Reservation, rates Rates) Summary {
	items := make([]Item, 0, len(rows))
	var total float64
	for _, row := range rows {
		cores := float64(row.NanoCPUs) / 1e9
		gb := float64(row.MemoryBytes) / (1024 * 1024 * 1024)
		usd := EstimateCPUMem(cores, gb, rates)
		items = append(items, Item{Name: row.Name, CPUCores: round3(cores), MemGB: round3(gb), USDPerDay: usd})
		total += usd
	}
	if items == nil {
		items = []Item{}
	}
	return Summary{
		Rates:     rates,
		Items:     items,
		USDPerDay: round2(total),
		Note:      "Estimativa a partir de limites de CPU/memória, não de uso real. Ajuste DOCKERPILOT_CPU_USD_HOUR e DOCKERPILOT_MEM_GB_USD_HOUR.",
	}
}

// Reservation is the input extracted from a container HostConfig.
type Reservation struct {
	Name        string
	NanoCPUs    int64
	MemoryBytes int64
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}
