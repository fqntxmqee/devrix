package config

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_StartStop(t *testing.T) {
	path := "/tmp/test_config_watcher.yaml"

	// Create test file
	f, err := createTempFile(path)
	require.NoError(t, err)
	f.Close()

	watcher := NewConfigWatcher(path)

	// Start watcher
	ctx := context.Background()
	err = watcher.Start(ctx)
	require.NoError(t, err)

	// Stop watcher
	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestWatcher_DoubleStart(t *testing.T) {
	path := "/tmp/test_double_start.yaml"
	f, err := createTempFile(path)
	require.NoError(t, err)
	f.Close()

	watcher := NewConfigWatcher(path)
	ctx := context.Background()

	err = watcher.Start(ctx)
	require.NoError(t, err)
	defer watcher.Stop()

	err = watcher.Start(ctx)
	assert.ErrorIs(t, ErrAlreadyStarted, err)
}

func TestWatcher_DoubleStop(t *testing.T) {
	path := "/tmp/test_double_stop.yaml"
	f, err := createTempFile(path)
	require.NoError(t, err)
	f.Close()

	watcher := NewConfigWatcher(path)
	ctx := context.Background()

	err = watcher.Start(ctx)
	require.NoError(t, err)

	err = watcher.Stop()
	require.NoError(t, err)

	err = watcher.Stop()
	assert.ErrorIs(t, ErrAlreadyStopped, err)
}

func TestWatcher_EventDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	path := "/tmp/test_event_detection.yaml"
	f, err := createTempFile(path)
	require.NoError(t, err)
	f.Close()

	watcher := NewConfigWatcher(path)
	ctx := context.Background()

	err = watcher.Start(ctx)
	require.NoError(t, err)
	defer watcher.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Trigger write event
	err = triggerWriteEvent(path)
	require.NoError(t, err)

	// Read event
	select {
	case event := <-watcher.Events():
		assert.Equal(t, path, event.Path)
		assert.True(t, event.Op&WRITE != 0)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestWatcher_NonExistentPath(t *testing.T) {
	watcher := NewConfigWatcher("/nonexistent/path/to/file")
	ctx := context.Background()

	err := watcher.Start(ctx)
	// Should fail to watch non-existent path
	assert.Error(t, err)
}

func TestOpString(t *testing.T) {
	tests := []struct {
		op      Op
		want    string
	}{
		{CREATE, "CREATE"},
		{WRITE, "WRITE"},
		{REMOVE, "REMOVE"},
		{RENAME, "RENAME"},
		{Op(0), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.op.String())
		})
	}
}

func TestFileExists(t *testing.T) {
	// Create temp file
	f, err := createTempFile("/tmp/test_exists.yaml")
	require.NoError(t, err)
	f.Close()

	assert.True(t, FileExists("/tmp/test_exists.yaml"))
	assert.False(t, FileExists("/tmp/nonexistent_file_12345.yaml"))
}

func TestWatcher_ConcurrentAccess(t *testing.T) {
	path := "/tmp/test_concurrent.yaml"
	f, err := createTempFile(path)
	require.NoError(t, err)
	f.Close()

	watcher := NewConfigWatcher(path)
	ctx := context.Background()

	err = watcher.Start(ctx)
	require.NoError(t, err)
	defer watcher.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				watcher.Events()
			}
		}()
	}

	wg.Wait()
}

func createTempFile(path string) (*os.File, error) {
	return os.Create(path)
}

func triggerWriteEvent(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("# modified\n")
	return err
}
