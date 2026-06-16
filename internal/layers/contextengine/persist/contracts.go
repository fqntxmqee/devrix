// Package persist — D2-S4 PersistState: 执行后状态持久化。
//
// S4 编排 4 个 A 层:
//   - A01 SaveSnapshot: 会话上下文序列化到磁盘
//   - A02 WriteTranscript: 主线程 + 侧链转录写入
//   - A03 StoreLongTerm: 长期记忆自动存储
//   - A04 CommitWindow: 消息截断 + Active Window 提交
package persist

import "context"

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
