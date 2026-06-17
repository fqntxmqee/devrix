package tracker

// DM-20260617-002 W8: process-wide Tracker 单例,让 tracker_tool.go (toolrunner 层)
// 不直接依赖 bootstrap 注入链路。bootstrap 阶段 SetGlobalTracker(...),
// toolrunner 通过 GlobalTracker() 读取,周期 tick goroutine 由 bootstrap 启动。

import "sync"

var (
	globalTrackerMu sync.RWMutex
	globalTracker   *Tracker
)

// SetGlobalTracker 替换 process-wide Tracker。传 nil 时 query_diagnostics 工具
// 会返回 "tracker not initialized" 错误。
func SetGlobalTracker(t *Tracker) {
	globalTrackerMu.Lock()
	defer globalTrackerMu.Unlock()
	globalTracker = t
}

// GlobalTracker 返回当前 process-wide Tracker。nil 表示未注入。
func GlobalTracker() *Tracker {
	globalTrackerMu.RLock()
	defer globalTrackerMu.RUnlock()
	return globalTracker
}
