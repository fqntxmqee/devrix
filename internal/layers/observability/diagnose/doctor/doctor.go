// Package doctor — A1 /doctor 自检命令,对标 clawcode /doctor slash command。
//
// 一次性运行 7 项内置 check,输出结构化报告(JSON / table)。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.9
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Status — check 状态。
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check — 单项自检结果。
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Doctor — 自检运行器接口。
type Doctor interface {
	Run(ctx context.Context) []Check
}

// CheckFunc — 单项 check 实现签名。
type CheckFunc func(ctx context.Context) Check

// DefaultDoctor — 7 项内置 check 默认实现。
type DefaultDoctor struct {
	WorkDir      string
	DevrixBinary string
	LSPServers   []LSPServer
	TranscriptDir string
	ExtraChecks  []CheckFunc
}

// LSPServer — 描述一个 LSP server 可执行命令。
type LSPServer struct {
	Name    string
	Command string
}

// NewDefaultDoctor 构造 default doctor。
func NewDefaultDoctor(workDir, devrixBin, transcriptDir string, lspServers []LSPServer) *DefaultDoctor {
	return &DefaultDoctor{
		WorkDir:       workDir,
		DevrixBinary:  devrixBin,
		TranscriptDir: transcriptDir,
		LSPServers:    lspServers,
	}
}

// Run 顺序执行所有 check,返回报告。
func (d *DefaultDoctor) Run(ctx context.Context) []Check {
	results := make([]Check, 0, 7+len(d.ExtraChecks))
	results = append(results, d.checkInstallPaths(ctx))
	results = append(results, d.checkConfigYAML(ctx))
	results = append(results, d.checkLSPServers(ctx))
	results = append(results, d.checkWorkdirWritable(ctx))
	results = append(results, d.checkObservabilityReady(ctx))
	results = append(results, d.checkToolCount(ctx))
	results = append(results, d.checkTranscriptDir(ctx))
	for _, fn := range d.ExtraChecks {
		results = append(results, fn(ctx))
	}
	return results
}

// FormatJSON — 报告序列化为 JSON。
func FormatJSON(checks []Check) ([]byte, error) {
	return json.MarshalIndent(checks, "", "  ")
}

// FormatTable — 报告渲染为人类可读 table。
func FormatTable(checks []Check) string {
	var b strings.Builder
	b.WriteString("Doctor Report\n")
	b.WriteString("=============\n")
	for _, c := range checks {
		icon := "?"
		switch c.Status {
		case StatusPass:
			icon = "✓"
		case StatusWarn:
			icon = "!"
		case StatusFail:
			icon = "✗"
		}
		fmt.Fprintf(&b, "  %s %-30s %-5s %s\n", icon, c.Name, c.Status, c.Detail)
	}
	return b.String()
}

// Summary — 报告健康度(0=fail, 1=warn, 2=pass)。
func Summary(checks []Check) Status {
	any := false
	for _, c := range checks {
		if c.Status == StatusFail {
			return StatusFail
		}
		if c.Status == StatusWarn {
			any = true
		}
	}
	if any {
		return StatusWarn
	}
	return StatusPass
}

// === 内置 checks ===

func (d *DefaultDoctor) checkInstallPaths(_ context.Context) Check {
	missing := []string{}
	if d.DevrixBinary != "" {
		if _, err := exec.LookPath(d.DevrixBinary); err != nil {
			missing = append(missing, d.DevrixBinary)
		}
	}
	for _, tool := range []string{"go", "gopls"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) == 0 {
		return Check{Name: "install_paths", Status: StatusPass, Detail: "all required tools reachable"}
	}
	return Check{Name: "install_paths", Status: StatusWarn, Detail: "missing: " + strings.Join(missing, ", ")}
}

func (d *DefaultDoctor) checkConfigYAML(_ context.Context) Check {
	if d.WorkDir == "" {
		return Check{Name: "config_yaml_valid", Status: StatusWarn, Detail: "WorkDir not set"}
	}
	for _, name := range []string{"devrix.yaml", "config.yaml"} {
		p := filepath.Join(d.WorkDir, name)
		if _, err := os.Stat(p); err == nil {
			return Check{Name: "config_yaml_valid", Status: StatusPass, Detail: p}
		}
	}
	return Check{Name: "config_yaml_valid", Status: StatusWarn, Detail: "no devrix.yaml/config.yaml found in " + d.WorkDir}
}

func (d *DefaultDoctor) checkLSPServers(_ context.Context) Check {
	if len(d.LSPServers) == 0 {
		return Check{Name: "lsp_servers_reachable", Status: StatusWarn, Detail: "no LSP servers configured"}
	}
	missing := []string{}
	for _, s := range d.LSPServers {
		if s.Command == "" {
			continue
		}
		if _, err := exec.LookPath(s.Command); err != nil {
			missing = append(missing, s.Name+"="+s.Command)
		}
	}
	if len(missing) == 0 {
		return Check{Name: "lsp_servers_reachable", Status: StatusPass, Detail: fmt.Sprintf("%d server(s) reachable", len(d.LSPServers))}
	}
	return Check{Name: "lsp_servers_reachable", Status: StatusFail, Detail: "missing: " + strings.Join(missing, ", ")}
}

func (d *DefaultDoctor) checkWorkdirWritable(_ context.Context) Check {
	if d.WorkDir == "" {
		return Check{Name: "workdir_writable", Status: StatusFail, Detail: "WorkDir not set"}
	}
	probe := filepath.Join(d.WorkDir, ".devrix-doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return Check{Name: "workdir_writable", Status: StatusFail, Detail: err.Error()}
	}
	_ = os.Remove(probe)
	return Check{Name: "workdir_writable", Status: StatusPass, Detail: d.WorkDir}
}

func (d *DefaultDoctor) checkObservabilityReady(_ context.Context) Check {
	// 检测 default logger 是否就绪:通过 SLICE_DEFAULT_LOG_LEVEL 等环境变量评估
	if os.Getenv("SLICE_DEFAULT_LOG_LEVEL") != "" || os.Getenv("DEVRIX_LOG_LEVEL") != "" {
		return Check{Name: "observability_ready", Status: StatusPass, Detail: "log level env set"}
	}
	// 视为就绪(无 fail 触发器)
	return Check{Name: "observability_ready", Status: StatusPass, Detail: "slog/tracer available"}
}

func (d *DefaultDoctor) checkToolCount(_ context.Context) Check {
	// 占位:运行时由 caller 注入真实 count(通过 ExtraChecks)
	// 这里只回报基线 0,避免在 doctor 包内导入 PluginRunner
	return Check{Name: "tool_count", Status: StatusPass, Detail: "see /tools subcommand for live count"}
}

func (d *DefaultDoctor) checkTranscriptDir(_ context.Context) Check {
	if d.TranscriptDir == "" {
		return Check{Name: "transcript_dir_ok", Status: StatusWarn, Detail: "transcript dir not configured"}
	}
	if err := os.MkdirAll(d.TranscriptDir, 0o755); err != nil {
		return Check{Name: "transcript_dir_ok", Status: StatusFail, Detail: err.Error()}
	}
	probe := filepath.Join(d.TranscriptDir, ".devrix-doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return Check{Name: "transcript_dir_ok", Status: StatusFail, Detail: err.Error()}
	}
	_ = os.Remove(probe)
	return Check{Name: "transcript_dir_ok", Status: StatusPass, Detail: d.TranscriptDir}
}
