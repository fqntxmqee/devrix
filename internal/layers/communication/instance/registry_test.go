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
