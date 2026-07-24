package api

import (
	"fmt"
	"sync"
	"testing"
)

func TestRateLimitLocalConcurrent(t *testing.T) {
	const goroutines = 64
	const perG = 100
	key := fmt.Sprintf("race-exact-%p", &struct{}{})

	localRatesMu.Lock()
	delete(localRates, key)
	localRatesMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				_ = rateLimitLocal(key)
			}
		}()
	}
	wg.Wait()

	localRatesMu.Lock()
	got := localRates[key].n
	localRatesMu.Unlock()
	want := int64(goroutines * perG)
	if got != want {
		t.Fatalf("rateLimitLocal concurrent count = %d, want %d", got, want)
	}
}
