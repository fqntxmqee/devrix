package instance

import (
	"context"
	"testing"
	"time"
)

func TestInstanceRegistry_RegisterHealthCheck(t *testing.T) {
	reg := NewInstanceRegistry(50 * time.Millisecond)
	ctx := context.Background()

	info := &InstanceInfo{ID: "i-1", Name: "devrix-a", Address: "127.0.0.1", Port: 8080}
	if err := reg.Register(ctx, info); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	instances, err := reg.GetInstances(ctx)
	if err != nil {
		t.Fatalf("GetInstances() error = %v", err)
	}
	if len(instances) != 1 || instances[0].Status != "healthy" {
		t.Fatalf("instances = %+v", instances)
	}

	if err := reg.HealthCheck(ctx, "i-1"); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

// T: D1-S1-A01-T02
func TestInstanceRegistry_RegisterUnregister(t *testing.T) {
	reg := NewInstanceRegistry(time.Minute)
	ctx := context.Background()

	info := &InstanceInfo{ID: "devrix-dingtalk", Name: "DingTalk Bot", Address: "127.0.0.1", Port: 8081}
	if err := reg.Register(ctx, info); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if reg.Count() != 1 {
		t.Fatalf("count = %d, want 1", reg.Count())
	}
	if err := reg.Unregister(ctx, "devrix-dingtalk"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if reg.Count() != 0 {
		t.Fatalf("count after unregister = %d", reg.Count())
	}
}

// Covers: CQS — GetInstances read-only
func TestInstanceRegistry_GetInstances_doesNotMutateStatus(t *testing.T) {
	reg := NewInstanceRegistry(10 * time.Millisecond)
	ctx := context.Background()

	if err := reg.Register(ctx, &InstanceInfo{ID: "i-1", Name: "n", Port: 8080}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	list, err := reg.GetInstances(ctx)
	if err != nil {
		t.Fatalf("GetInstances() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].Status != "healthy" {
		t.Fatalf("GetInstances mutated status to %q", list[0].Status)
	}
}
