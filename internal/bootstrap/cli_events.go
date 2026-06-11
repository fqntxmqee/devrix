package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/renderers"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// CLIProgressHandler renders worker_progress to stdout and delegates other events.
type CLIProgressHandler struct {
	Base     gateway.EventHandler
	Renderer *renderers.CLIRenderer
}

// NewCLIProgressHandler wraps base with CLI worker progress rendering.
func NewCLIProgressHandler(base gateway.EventHandler, ansi config.ANSIConfig) *CLIProgressHandler {
	return &CLIProgressHandler{
		Base:     base,
		Renderer: renderers.NewCLIRenderer(ansi),
	}
}

func (h *CLIProgressHandler) OnMessage(msg *types.OutboundMessage) {
	if h == nil {
		return
	}
	if msg != nil && msg.Metadata["event_type"] == "worker_progress" && h.Renderer != nil {
		h.Renderer.RenderWorkerProgress(msg)
		return
	}
	if h.Base != nil {
		h.Base.OnMessage(msg)
	}
}

func (h *CLIProgressHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	if h == nil || h.Base == nil {
		return false
	}
	return h.Base.OnPermissionRequest(req)
}

func (h *CLIProgressHandler) OnError(err error, sessionID string) {
	if h == nil || h.Base == nil {
		return
	}
	h.Base.OnError(err, sessionID)
}

func (h *CLIProgressHandler) OnStatus(sessionID string, state types.SessionState) {
	if h == nil || h.Base == nil {
		return
	}
	h.Base.OnStatus(sessionID, state)
}
