package freefork

import "sync"

// DM-20260617-002 W7: process-wide Forker 单例,让 freefork_tool.go (toolrunner 层)
// 不直接依赖 bootstrap 注入链路。bootstrap 阶段 SetGlobalForker(...),
// toolrunner.freefork_tool 通过 GlobalForker() 读取。

var (
	globalForkerMu sync.RWMutex
	globalForker   Forker
)

// SetGlobalForker 替换 process-wide Forker。传 nil 时 fork 请求会被拒绝。
func SetGlobalForker(f Forker) {
	globalForkerMu.Lock()
	defer globalForkerMu.Unlock()
	globalForker = f
}

// GlobalForker 返回当前 process-wide Forker。nil 表示未注入。
func GlobalForker() Forker {
	globalForkerMu.RLock()
	defer globalForkerMu.RUnlock()
	return globalForker
}
