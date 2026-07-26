package clock

import (
	"os"
	"strings"
	"sync"
	"time"

	_ "time/tzdata" // zoneinfo embutido — funciona sem tzdata no Alpine
)

var (
	once sync.Once
	loc  *time.Location
)

// Location returns the app timezone (TZ / ALERT_TZ, default America/Sao_Paulo).
func Location() *time.Location {
	once.Do(func() {
		name := strings.TrimSpace(os.Getenv("TZ"))
		if name == "" {
			name = strings.TrimSpace(os.Getenv("ALERT_TZ"))
		}
		if name == "" {
			name = "America/Sao_Paulo"
		}
		l, err := time.LoadLocation(name)
		if err != nil {
			l = time.FixedZone("UTC-3", -3*60*60)
		}
		loc = l
	})
	return loc
}

// Now is time.Now() in the app timezone.
func Now() time.Time {
	return time.Now().In(Location())
}
