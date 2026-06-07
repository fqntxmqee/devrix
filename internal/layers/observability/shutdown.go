package observability

import (
	"context"
	"sync"
	"time"
)

// ShutdownManager manages graceful shutdown of observability components
type ShutdownManager struct {
	timeout time.Duration
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &ShutdownManager{timeout: timeout}
}

// ShutdownWithContext shuts down the observability layer with a timeout
func (m *ShutdownManager) ShutdownWithContext(ctx context.Context, obs *Observability) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- obs.Shutdown(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// BatchShutdown shuts down multiple observability instances
func (m *ShutdownManager) BatchShutdown(ctx context.Context, instances []*Observability) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(instances))

	for _, obs := range instances {
		wg.Add(1)
		go func(o *Observability) {
			defer wg.Done()
			if err := o.Shutdown(ctx); err != nil {
				errChan <- err
			}
		}(obs)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
