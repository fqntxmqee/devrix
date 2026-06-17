// Package faultinject — A4 故障注入,仅在 `testbuild` build tag 下生效。
//
// 生产 binary 不携带注入逻辑:tag guard 让 Hook() 编译为 no-op。
//
// 启用方式:`DEVRIX_FAULT_INJECT=llmgateway.dispatch.invoke=error:simulated_failure`。
// 格式:target=mode:param[,target=mode:param]
//   - mode=error:Hook 返回带 param 文本的 error
//   - mode=latency:Hook 睡眠 param 解析的毫秒数,返回 nil
//   - mode=truncate:Hook 返回 param 作为 error(用 truncated 上下文)
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.12
//go:build testbuild
// +build testbuild

package faultinject

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Rule 一条注入规则。
type Rule struct {
	Target string
	Mode   string // "error"|"latency"|"truncate"
	Param  string
	Once   bool
}

// Injector 故障注入器。
type Injector struct {
	enabled atomic.Bool
	mu      sync.Mutex
	rules   map[string][]Rule
	// onceCounter 记录 once rule 触发次数
	onceCounter map[string]int
}

// New 构造 injector,自动从 DEVRIX_FAULT_INJECT 解析规则。
func New() *Injector {
	inj := &Injector{
		rules:       make(map[string][]Rule),
		onceCounter: make(map[string]int),
	}
	raw := os.Getenv("DEVRIX_FAULT_INJECT")
	if raw == "" {
		return inj
	}
	inj.parseAndStore(raw)
	inj.enabled.Store(true)
	return inj
}

// Enabled 报告是否启用。
func (i *Injector) Enabled() bool {
	if i == nil {
		return false
	}
	return i.enabled.Load()
}

// AddRule 编程式添加规则(用于测试)。
func (i *Injector) AddRule(r Rule) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rules[r.Target] = append(i.rules[r.Target], r)
	i.enabled.Store(true)
}

// Reset 清空所有规则和 once 计数(测试用)。
func (i *Injector) Reset() {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rules = make(map[string][]Rule)
	i.onceCounter = make(map[string]int)
	i.enabled.Store(false)
}

// Hook 在 target 处检查并触发注入。
// 返回:error / nil;若 mode=latency 还会 sleep。
func (i *Injector) Hook(target string) error {
	if !i.Enabled() {
		return nil
	}
	i.mu.Lock()
	rules := append([]Rule(nil), i.rules[target]...)
	i.mu.Unlock()
	if len(rules) == 0 {
		return nil
	}
	// first matching rule wins(简单语义,支持 once)
	for _, r := range rules {
		if r.Once {
			i.mu.Lock()
			idx := i.onceCounter[target]
			if idx > 0 {
				i.mu.Unlock()
				continue
			}
			i.onceCounter[target] = 1
			i.mu.Unlock()
		}
		return applyRule(r)
	}
	return nil
}

func applyRule(r Rule) error {
	switch r.Mode {
	case "error":
		if r.Param == "" {
			return errors.New("injected error")
		}
		return fmt.Errorf("injected: %s", r.Param)
	case "latency":
		ms, err := strconv.Atoi(r.Param)
		if err != nil || ms < 0 {
			return nil
		}
		// 实现:用 time.Sleep(ms * time.Millisecond)
		// 这里 import 会被 minimal 化,直接走标准 sleep
		sleepMillis(ms)
		return nil
	case "truncate":
		return fmt.Errorf("truncated: %s", r.Param)
	default:
		return nil
	}
}

// parseAndStore 解析 `target=mode:param,target=mode:param` 格式。
func (i *Injector) parseAndStore(raw string) {
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// 拆 target=mode:param
		eq := strings.IndexByte(entry, '=')
		if eq < 0 {
			continue
		}
		target := strings.TrimSpace(entry[:eq])
		rest := entry[eq+1:]
		colon := strings.IndexByte(rest, ':')
		var mode, param string
		if colon < 0 {
			mode = strings.TrimSpace(rest)
		} else {
			mode = strings.TrimSpace(rest[:colon])
			param = rest[colon+1:]
		}
		once := false
		if strings.HasSuffix(target, ":once") {
			once = true
			target = strings.TrimSuffix(target, ":once")
		}
		i.rules[target] = append(i.rules[target], Rule{
			Target: target,
			Mode:   mode,
			Param:  param,
			Once:   once,
		})
	}
}
