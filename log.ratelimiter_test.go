package lets

import (
	"sync"
	"testing"
)

func TestAllowOnce(t *testing.T) {
	const key = "test-allow-once-single"

	if !allowOnce(key) {
		t.Fatalf("allowOnce(%q) = false on first call, want true", key)
	}
	for i := range 5 {
		if allowOnce(key) {
			t.Errorf("allowOnce(%q) = true on repeat call %d, want false", key, i)
		}
	}
}

func TestAllowOnceDistinctKeys(t *testing.T) {
	keys := []string{"test-allow-once-a", "test-allow-once-b", "test-allow-once-c"}
	for _, key := range keys {
		if !allowOnce(key) {
			t.Errorf("allowOnce(%q) = false on first call, want true", key)
		}
	}
}

// TestAllowOnceConcurrent verifies that exactly one caller observes the first
// occurrence of a key, even under concurrent access.
func TestAllowOnceConcurrent(t *testing.T) {
	const (
		key     = "test-allow-once-concurrent"
		callers = 100
	)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		granted int
	)
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			if allowOnce(key) {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if granted != 1 {
		t.Errorf("allowOnce granted first-call to %d goroutines, want exactly 1", granted)
	}
}
