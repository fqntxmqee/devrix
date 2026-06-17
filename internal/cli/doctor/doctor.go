// Package doctorcli — A1 /doctor CLI 子命令 wrapper, 包装
// observability/diagnose/doctor.DefaultDoctor 并支持 --json / --table 两种输出。
//
// DM-20260617-002 W9 (AC1).
package doctorcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/doctor"
)

// Run 解析 args 并执行 /doctor 自检。
//   devrix doctor                  → 人类可读 table + summary
//   devrix doctor --json           → JSON 报告
//   devrix doctor --workdir=DIR    → 指定工作目录(默认 os.Getwd)
//   devrix doctor --transcript-dir=PATH
func Run(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	asJSON := fs.Bool("json", false, "emit JSON report")
	workDir := fs.String("workdir", "", "working directory (default: current dir)")
	transcriptDir := fs.String("transcript-dir", "", "transcript directory")
	devrixBin := fs.String("devrix-bin", "devrix", "devrix binary name (for PATH check)")
	lspFake := fs.Bool("lsp-fake", false, "force LSP check to fail (test hook)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	wd := *workDir
	if wd == "" {
		wd, _ = os.Getwd()
	}
	td := *transcriptDir
	if td == "" {
		td = os.Getenv("DEVRIX_TRANSCRIPT_DIR")
	}

	lspServers := []doctor.LSPServer{
		{Name: "gopls", Command: "gopls"},
		{Name: "tsc", Command: "tsc"},
	}
	if *lspFake {
		// 注入一个不存在的命令, 强制 LSP check 失败。
		lspServers = append(lspServers, doctor.LSPServer{
			Name:    "fake-lsp-test",
			Command: "this-binary-does-not-exist-xyz",
		})
	}

	d := doctor.NewDefaultDoctor(wd, *devrixBin, td, lspServers)
	checks := d.Run(context.Background())

	if *asJSON {
		data, err := doctor.FormatJSON(checks)
		if err != nil {
			return fmt.Errorf("format json: %w", err)
		}
		_, _ = os.Stdout.Write(append(data, '\n'))
	} else {
		_, _ = os.Stdout.Write([]byte(doctor.FormatTable(checks)))
		_, _ = os.Stdout.Write([]byte("\nSummary: " + strings.ToUpper(string(doctor.Summary(checks))) + "\n"))
	}
	return nil
}
