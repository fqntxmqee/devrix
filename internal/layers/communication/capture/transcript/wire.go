package transcript

import "sync"

// globalWriter 进程级默认 Writer。bootstrap 阶段可 SetGlobalWriter 注入自定义。
//
// 用途:为 notify.CompletionEvent、tool_call 等结构化事件提供独立 JSONL 落盘通道,
// 与 contextengine 主线 transcript 并存。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.11
var (
	globalMu sync.RWMutex
	globalW  *Writer
)

// SetGlobalWriter 注入 process-wide writer;nil 时清空。
func SetGlobalWriter(w *Writer) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalW = w
}

// GlobalWriter 取当前 writer。nil-safe。
func GlobalWriter() *Writer {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalW
}

// Append 快捷方法:把 event 写到 global writer;global 为 nil 时 no-op。
func Append(sessionID string, ev Event) error {
	w := GlobalWriter()
	if w == nil {
		return nil
	}
	return w.Append(sessionID, ev)
}
