package bootstrap

// S4-Gate H-3 fix: prompt assembler 用的 task notification drainer 通过
// function-based DI 注入, prompt 包不 import orchestration/workmodel/notify.
import (
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
)

var taskNotifDrainerOnce sync.Once

// wireTaskNotifDrainer 注入真实 drainer 到 prompt.globalTaskNotifDrainer.
// bootstrap 阶段调用, 之后 assembler.Build 每次都从 prompt 拿这个函数 drain
// notify bus (bus.Drain 是消费性的, 不会重复注入).
func wireTaskNotifDrainer() {
	taskNotifDrainerOnce.Do(func() {
		prompt.SetTaskNotifDrainer(taskNotifDrainerImpl)
	})
}

// taskNotifDrainerImpl 真实 drain 实现: 调 notify.GlobalBus().Drain + FormatReminder.
func taskNotifDrainerImpl(sessionID string) string {
	bus := notify.GlobalBus()
	if bus == nil {
		return ""
	}
	return notify.FormatReminder(bus.Drain(sessionID))
}
