package instance

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// InstanceInfo represents information about a running instance
type InstanceInfo struct {
	ID        string
	Name      string
	Address   string
	Port      int
	Status    string // "healthy" | "unhealthy"
	StartedAt time.Time
	LastSeen  time.Time
}

// IInstanceRegistry defines the interface for instance registry
type IInstanceRegistry interface {
	// Register registers a new instance
	Register(ctx context.Context, info *InstanceInfo) error
	// Unregister unregisters an instance
	Unregister(ctx context.Context, id string) error
	// GetInstances returns all registered instances
	GetInstances(ctx context.Context) ([]*InstanceInfo, error)
	// HealthCheck updates the health status of an instance
	HealthCheck(ctx context.Context, id string) error
}

// InstanceRegistry implements in-memory instance registry
type InstanceRegistry struct {
	mu        sync.RWMutex
	instances map[string]*InstanceInfo
	timeout   time.Duration // considered unhealthy after this duration
}

// NewInstanceRegistry creates a new instance registry
func NewInstanceRegistry(timeout time.Duration) *InstanceRegistry {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &InstanceRegistry{
		instances: make(map[string]*InstanceInfo),
		timeout:   timeout,
	}
}

// Register registers a new instance
func (r *InstanceRegistry) Register(ctx context.Context, info *InstanceInfo) error {
	if info.ID == "" {
		return fmt.Errorf("instance ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	info.Status = "healthy"
	info.LastSeen = time.Now()

	r.instances[info.ID] = info

	slog.Info("instance registered",
		"id", info.ID,
		"name", info.Name,
		"address", info.Address,
	)

	return nil
}

// Unregister unregisters an instance
func (r *InstanceRegistry) Unregister(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[id]; !exists {
		return fmt.Errorf("instance %s not found", id)
	}

	delete(r.instances, id)

	slog.Info("instance unregistered",
		"id", id,
	)

	return nil
}

// GetInstances returns all registered instances
func (r *InstanceRegistry) GetInstances(ctx context.Context) ([]*InstanceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*InstanceInfo, 0, len(r.instances))
	now := time.Now()

	for _, inst := range r.instances {
		// Update status based on last seen
		if now.Sub(inst.LastSeen) > r.timeout {
			inst.Status = "unhealthy"
		}
		result = append(result, inst)
	}

	return result, nil
}

// HealthCheck updates the health status of an instance
func (r *InstanceRegistry) HealthCheck(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.instances[id]
	if !exists {
		return fmt.Errorf("instance %s not found", id)
	}

	inst.LastSeen = time.Now()
	inst.Status = "healthy"

	return nil
}

// GetInstance returns a single instance
func (r *InstanceRegistry) GetInstance(ctx context.Context, id string) (*InstanceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.instances[id]
	if !exists {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	return inst, nil
}

// Count returns the number of registered instances
func (r *InstanceRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

// GetHealthyInstances returns only healthy instances
func (r *InstanceRegistry) GetHealthyInstances(ctx context.Context) ([]*InstanceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*InstanceInfo, 0)
	now := time.Now()

	for _, inst := range r.instances {
		if now.Sub(inst.LastSeen) <= r.timeout {
			result = append(result, inst)
		}
	}

	return result, nil
}
