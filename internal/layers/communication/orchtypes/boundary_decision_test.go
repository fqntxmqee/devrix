package orchtypes

import (
	"regexp"
	"testing"
)

func TestBoundaryDecisions_Exist(t *testing.T) {
	want := []string{
		BoundaryD1ToD7OrchestrationEntry,
		BoundaryD1ToD4PermissionGate,
		BoundaryD1ForbiddenOrchestrationImport,
	}
	all := AllBoundaryDecisions()
	if len(all) != len(want) {
		t.Fatalf("AllBoundaryDecisions() returned %d items, want %d", len(all), len(want))
	}
	seen := make(map[string]bool, len(all))
	for _, id := range all {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Errorf("boundary decision %q not in AllBoundaryDecisions()", id)
		}
	}
}

func TestBoundaryDecisions_VersionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`)
	for _, id := range AllBoundaryDecisions() {
		if !pattern.MatchString(id) {
			t.Errorf("boundary decision %q does not match format %s", id, pattern)
		}
	}
}

func TestBoundaryDecisions_Unique(t *testing.T) {
	all := AllBoundaryDecisions()
	seen := make(map[string]string, len(all))
	for _, id := range all {
		if other, ok := seen[id]; ok {
			t.Errorf("duplicate boundary decision %q (also at %q)", id, other)
		}
		seen[id] = id
	}
}