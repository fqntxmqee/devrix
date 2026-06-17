// Package contextanalyze — A5 /context analyze CLI 子命令 wrapper, 包装
// contextengine/token/windowanalyzer.TokenAnalyzer 并支持从 messages.jsonl
// 加载 session history。
//
// DM-20260617-002 W10 (AC2).
package contextanalyze

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/token/windowanalyzer"
	"github.com/devrix/devrix/internal/shared/types"
)

// Run 解析 args 并执行 /context analyze。
//   devrix context-analyze --messages-file=path.jsonl
//   devrix context-analyze --session=<id> --storage-dir=<path>
//
// 至少需要 --messages-file 或 --session(--storage-dir 隐式从 commCfg 读取)。
// 消息文件格式:每行一个 JSON 对象 {"role": "...", "content": "..."}。
func Run(args []string) error {
	fs := flag.NewFlagSet("context-analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	messagesFile := fs.String("messages-file", "", "path to messages jsonl (one JSON object per line)")
	sessionID := fs.String("session", "", "session ID (loads metadata only)")
	storageDir := fs.String("storage-dir", "", "FileSessionStore directory (default: DEVRIX_STORAGE_DIR or ~/.devrix/sessions)")
	asJSON := fs.Bool("json", false, "emit JSON breakdown instead of table")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *messagesFile == "" && *sessionID == "" {
		return fmt.Errorf("--messages-file or --session is required")
	}

	var msgs []types.Message
	if *messagesFile != "" {
		loaded, err := readMessagesJSONL(*messagesFile)
		if err != nil {
			return fmt.Errorf("read messages file: %w", err)
		}
		msgs = loaded
	}

	if *sessionID != "" {
		dir := *storageDir
		if dir == "" {
			dir = os.Getenv("DEVRIX_STORAGE_DIR")
		}
		if dir == "" {
			dir = os.Getenv("HOME") + "/.devrix/sessions"
		}
		store, err := capture.NewFileSessionStore(dir)
		if err != nil {
			return fmt.Errorf("open session store: %w", err)
		}
		sess, err := store.Get(*sessionID)
		if err != nil {
			return fmt.Errorf("load session %q: %w", *sessionID, err)
		}
		if sess == nil {
			return fmt.Errorf("session %q not found in %s", *sessionID, dir)
		}
	}

	if len(msgs) == 0 {
		return fmt.Errorf("no messages to analyze (provide non-empty --messages-file)")
	}

	analyzer := windowanalyzer.NewTokenAnalyzer()
	bd := analyzer.AnalyzeMessages(msgs)

	if *asJSON {
		data, err := json.MarshalIndent(bd, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		_, _ = os.Stdout.Write(append(data, '\n'))
		return nil
	}
	_, _ = os.Stdout.Write([]byte(windowanalyzer.FormatTable(bd)))
	return nil
}

// readMessagesJSONL 解析 jsonl 文件, 每行一个 {role, content} 消息。
// 容忍空行 / 解析失败的单行(跳过)。
func readMessagesJSONL(path string) ([]types.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []types.Message{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		out = append(out, types.Message{Role: types.MessageRole(m.Role), Content: m.Content})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// 用 context.Context 占位以满足 imports。
var _ = context.Background
