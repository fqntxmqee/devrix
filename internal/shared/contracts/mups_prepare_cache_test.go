package contracts

import (
	"context"
	"testing"
)

func TestMUPSPrepareBaseCache_roundTrip(t *testing.T) {
	ctx := WithMUPSPrepareCache(context.Background())
	if _, _, ok := TryMUPSPrepareBase(ctx, "sess_a", "directive"); ok {
		t.Fatal("expected miss before store")
	}
	StoreMUPSPrepareBase(ctx, "sess_a", "directive", "core-prompt", map[string]string{"k": "v"})
	gotSys, gotPre, ok := TryMUPSPrepareBase(ctx, "sess_a", "directive")
	if !ok {
		t.Fatal("expected hit after store")
	}
	if gotSys != "core-prompt" {
		t.Fatalf("system = %q", gotSys)
	}
	if gotPre["k"] != "v" {
		t.Fatalf("prepend = %v", gotPre)
	}
	if _, _, ok := TryMUPSPrepareBase(ctx, "sess_a", "other"); ok {
		t.Fatal("expected miss for different message")
	}
}
