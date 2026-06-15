package contracts

import "context"

// WorktreeSandbox isolates worker filesystem writes from the leader session.
type WorktreeSandbox interface {
	Enabled() bool
	Enter(ctx context.Context, sessionID, slug, workDir string) (path string, err error)
	Exit(ctx context.Context, path string, keep bool) error
}
