package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/config"
)

// WireAgentToolRegistry loads agent_tools from config and registers CLI/Cursor agents.
// Returns nil when disabled or on load error (caller may still start without agent tools).
func WireAgentToolRegistry(configFile string) *tool.Registry {
	agentToolsCfg, err := config.LoadAgentToolsConfig(configFile)
	if err != nil {
		slog.Warn("failed to load agent tools config", "error", err)
		return nil
	}
	if !agentToolsCfg.Enabled {
		return nil
	}
	reg := tool.NewRegistry()
	for _, tCfg := range agentToolsCfg.Tools {
		var agt tool.AgentTool
		switch tCfg.Type {
		case "cursor":
			agt = tool.NewCursorAgentTool(tool.CursorConfig{
				Name:         tCfg.Name,
				DisplayName:  tCfg.DisplayName,
				Description:  tCfg.Description,
				Capabilities: tCfg.Capabilities,
				Role:         tCfg.Role,
				Command:      tCfg.Command,
				Model:        tCfg.Model,
				Mode:         tCfg.Mode,
				WorkDir:      tCfg.WorkDir,
				Timeout:      tCfg.Timeout,
			})
		default:
			agt = tool.NewCLIAgentTool(tool.CLIConfig{
				Name:         tCfg.Name,
				DisplayName:  tCfg.DisplayName,
				Description:  tCfg.Description,
				Capabilities: tCfg.Capabilities,
				Role:         tCfg.Role,
				Command:      tCfg.Command,
				Args:         tCfg.Args,
				WorkDir:      tCfg.WorkDir,
				Timeout:      tCfg.Timeout,
				IdleTimeout:  tCfg.IdleTimeout,
			})
		}
		if err := reg.Register(agt); err != nil {
			slog.Error("register agent tool", "name", tCfg.Name, "error", err)
			continue
		}
		slog.Info("agent tool registered", "name", tCfg.Name)
	}
	if len(reg.List()) == 0 {
		return nil
	}
	return reg
}
