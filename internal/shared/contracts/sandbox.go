package contracts

import "context"

// WorkerDirSandbox isolates delegate worker filesystem writes from the leader session.
type WorkerDirSandbox interface {
	Enabled() bool
	Enter(ctx context.Context, sessionID, slug, workDir string) (path string, err error)
	Exit(ctx context.Context, path string, keep bool) error
}
