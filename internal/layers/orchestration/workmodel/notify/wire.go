package notify

// GlobalBus 默认 process-wide bus,供 workmodel.TaskManager / task_output tool 共享。
//
// 初始化方式:bootstrap 阶段 SetGlobalBus(...) 或使用默认 NewInMemoryBus(64)。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.7
import "sync"

// globalBus 单例 + mutex 保护。
var (
	globalBusMu sync.RWMutex
	globalBus   Bus = NewInMemoryBus(64)
)

// SetGlobalBus 替换 process-wide bus。传 nil 恢复 default in-memory。
func SetGlobalBus(b Bus) {
	globalBusMu.Lock()
	defer globalBusMu.Unlock()
	if b == nil {
		globalBus = NewInMemoryBus(64)
		return
	}
	globalBus = b
}

// GlobalBus 返回当前 process-wide bus。
func GlobalBus() Bus {
	globalBusMu.RLock()
	defer globalBusMu.RUnlock()
	return globalBus
}
