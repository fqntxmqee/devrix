package observability

import (
	"sync"
	"testing"
)

// Covers: D5 runtime.fork_session_view_total counter (additive path)
func TestIncForkSessionView_should_count_per_policy(t *testing.T) {
	Reset()
	IncForkSessionViewPolicy(PolicyCow)
	IncForkSessionViewPolicy(PolicyCow)
	IncForkSessionViewPolicy(PolicySnapshot)
	IncForkSessionView("shared")
	if v := ForkSessionViewValue(PolicyCow); v != 2 {
		t.Errorf("cow = %d, want 2", v)
	}
	if v := ForkSessionViewValue(PolicySnapshot); v != 1 {
		t.Errorf("snapshot = %d, want 1", v)
	}
	if v := ForkSessionViewValue(PolicyShared); v != 1 {
		t.Errorf("shared = %d, want 1", v)
	}
}

func TestIncForkSessionView_concurrent_should_be_atomic(t *testing.T) {
	Reset()
	const goroutines = 16
	const perG = 1000
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				IncForkSessionViewPolicy(PolicyCow)
			}
		}()
	}
	wg.Wait()
	want := int64(goroutines * perG)
	if v := ForkSessionViewValue(PolicyCow); v != want {
		t.Errorf("cow = %d, want %d", v, want)
	}
}
