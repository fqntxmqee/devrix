// Package ltllite — LTL-Lite 跨切面不变式框架 (DM-20260618-007 W14 — PERMISSION-GATE-1)。
//
// 核心思想: 把"tool surface 的跨切面约束"用 Go struct tag + 简单的 pre=>post 语法表达,
// 运行时由 Check 验证, CI lint 静态扫描 _invariant.go 文件。LTL-Lite 不是 model checker,
// 而是规约语言作为通信媒介 (设计文档 response-gametheory-r2.md §分歧 3)。
//
// 语法:
//
//	type Surface struct {
//	    ReadOnly        string `invariant:"is_read_only => no_destructive"`
//	    PermissionGated string `invariant:"destructive => permission_gate"`
//	}
//
// parser 抽 `pre => post` 两段; Check 评估 pre 和 post 在当前 state 下是否成立;
// pre=true && post=false → Violation。
package ltllite

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// InvariantTag 是 struct tag 键名。
const InvariantTag = "invariant"

// ErrInvalidInvariant — invariant tag 缺少 => 操作符。
var ErrInvalidInvariant = errors.New("ltllite: invalid invariant (missing '=>' operator)")

// Invariant 一条不变式断言。
type Invariant struct {
	Name    string // 来源 struct 字段名
	Pre     string // 前置条件 (e.g. "is_read_only")
	Post    string // 后置条件 (e.g. "no_destructive")
	Raw     string // 原始 tag 值 (含 =>)
	Source  string // 字段所属类型名 (e.g. "LSPToolSurface")
}

// InvariantSet 一次 ParseStruct 的输出。
type InvariantSet struct {
	Invariants []Invariant
}

// ParseStruct 通过 reflect 提取 s 的所有 invariant:"pre => post" tag。
//
// 支持的 tag 形式:
//
//	invariant:"pre => post"
//	invariant:"pre"           // 隐式 post=pre (恒等不变式, 永远成立, 不会违规)
//
// 解析失败 (无 => 且无隐式恒等) 返回 ErrInvalidInvariant。
//
// 反射入口接受 any (Go 1.18+ alias for interface{}); 实际类型必须是 struct 或 *struct。
func ParseStruct(s any) (InvariantSet, error) {
	v := reflect.ValueOf(s)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return InvariantSet{}, fmt.Errorf("ltllite: ParseStruct requires struct, got %s", v.Kind())
	}
	t := v.Type()
	source := t.Name()
	var out InvariantSet
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, ok := f.Tag.Lookup(InvariantTag)
		if !ok {
			continue
		}
		inv, err := parseTag(f.Name, source, tag)
		if err != nil {
			return out, err
		}
		out.Invariants = append(out.Invariants, inv)
	}
	return out, nil
}

func parseTag(name, source, raw string) (Invariant, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Invariant{}, fmt.Errorf("%w: empty tag on %s.%s", ErrInvalidInvariant, source, name)
	}
	if !strings.Contains(raw, "=>") {
		// 隐式恒等: pre = post = raw
		return Invariant{Name: name, Pre: raw, Post: raw, Raw: raw, Source: source}, nil
	}
	parts := strings.SplitN(raw, "=>", 2)
	if len(parts) != 2 {
		return Invariant{}, fmt.Errorf("%w: malformed %q on %s.%s", ErrInvalidInvariant, raw, source, name)
	}
	pre := strings.TrimSpace(parts[0])
	post := strings.TrimSpace(parts[1])
	if pre == "" || post == "" {
		return Invariant{}, fmt.Errorf("%w: empty pre or post in %q on %s.%s", ErrInvalidInvariant, raw, source, name)
	}
	return Invariant{Name: name, Pre: pre, Post: post, Raw: raw, Source: source}, nil
}

// String 便于日志/错误展示。
func (i Invariant) String() string {
	return fmt.Sprintf("%s.%s: %s", i.Source, i.Name, i.Raw)
}
