// Package persist — D2-S4 PersistState: 执行后状态持久化。
//
// S4 编排 4 个 A 层:
//   - A01 SaveSnapshot: 会话上下文序列化到磁盘
//   - A02 WriteTranscript: 主线程 + 侧链转录写入
//   - A03 StoreLongTerm: 长期记忆自动存储
//   - A04 CommitWindow: 消息截断 + Active Window 提交
package persist

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// SnapshotPersister serializes and persists session context snapshots.
//
// DSAFT: D2-S4-A01 (SaveSnapshot)
type SnapshotPersister interface {
	Persist(data []byte, sessionID string) error
}

// TranscriptWriter appends messages to the session transcript.
//
// DSAFT: D2-S4-A02 (WriteTranscript)
type TranscriptWriter interface {
	AppendMessages(sessionID string, msgs []byte) error
}

// LongTermStorer stores session summaries to long-term memory.
//
// DSAFT: D2-S4-A03 (StoreLongTerm)
type LongTermStorer interface {
	AutoStore(ctx context.Context, sessionID, query, summary string) error
}

// MessageStore reads and mutates in-memory session messages (S17-A04).
type MessageStore interface {
	Get(sessionID string) (*types.SessionContext, bool)
	AppendFullMessage(sc *types.SessionContext, msg types.Message)
	TrimMessages(sc *types.SessionContext)
}

// CommitWindowRunner executes the A04 CommitWindow logic (D2-S17-A04).
// Implementations typically wrap a compression pipeline + message store.
//
// DSAFT: D2-S17-A04 (CommitWindow)
type CommitWindowRunner interface {
	RunCommitWindow(ctx context.Context, sc *types.SessionContext) (types.CompressionReport, error)
}

// SessionBootstrap lazily initializes a session for first-write persist paths.
type SessionBootstrap func(sessionID string) (*types.SessionContext, error)
