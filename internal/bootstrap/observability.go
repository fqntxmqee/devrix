package bootstrap

// W5 — D5-S24-A02 (alias A2) DebugFilter 通过 CLI flag 启动时过滤日志 wiring。
//
// AC12:
//   - --debug=api,hooks 启动后仅 api/hooks 组件的 DEBUG 通过
//   - 非 DEBUG 级别不受 filter 影响 (passthroughNonDebug)
//
// 实现策略：
//   1. 解析 --debug=api,hooks flag → categories
//   2. 构造 debugfilter.Filter (用于获取 categories 的语义来源)
//   3. 把 Filter 包装成 slog.Handler (slogFilterAdapter) 替换 slog.Default() 的 handler
//   4. slogFilterAdapter 在 DEBUG 级别按 component whitelist 过滤，非 DEBUG 透传

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/instrument/logger"
	"github.com/devrix/devrix/internal/layers/observability/instrument/logger/debugfilter"
)

// ParseDebugFlag 扫描 argv 找 --debug=category1,category2 形式的 flag，
// 返回 categories 切片。空/不存在时返回 nil。
//
// DM-20260617-002 W5 (AC12)
func ParseDebugFlag(argv []string) []string {
	const prefix = "--debug="
	for _, a := range argv {
		if !strings.HasPrefix(a, prefix) {
			continue
		}
		raw := strings.TrimPrefix(a, prefix)
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// slogFilterAdapter 把 debugfilter 逻辑转回 slog.Handler，让 filter 挂在 slog.Default() 路径上。
type slogFilterAdapter struct {
	inner     slog.Handler
	cats      map[string]bool
	passthru  bool // 非 DEBUG 级别透传
}

func (a *slogFilterAdapter) Enabled(ctx context.Context, level slog.Level) bool {
	if a == nil || a.inner == nil {
		return false
	}
	return a.inner.Enabled(ctx, level)
}

func (a *slogFilterAdapter) Handle(ctx context.Context, r slog.Record) error {
	if a == nil || a.inner == nil {
		return nil
	}
	// 非 DEBUG 级别 → passthrough
	if a.passthru && r.Level != slog.LevelDebug {
		return a.inner.Handle(ctx, r)
	}
	// 提取 component attr
	var component string
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "component" || attr.Key == "Component" {
			component = attr.Value.String()
		}
		return true
	})
	if component == "" {
		// 无 component 标记的 debug entry 默认不通过
		return nil
	}
	if a.cats[component] {
		return a.inner.Handle(ctx, r)
	}
	return nil
}

func (a *slogFilterAdapter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogFilterAdapter{
		inner:    a.inner.WithAttrs(attrs),
		cats:     a.cats,
		passthru: a.passthru,
	}
}

func (a *slogFilterAdapter) WithGroup(name string) slog.Handler {
	return &slogFilterAdapter{
		inner:    a.inner.WithGroup(name),
		cats:     a.cats,
		passthru: a.passthru,
	}
}

// categoriesOf 从 debugfilter.Filter 拿 categories（仅用于初始化时构造 adapter）。
func categoriesOf(f *debugfilter.Filter) map[string]bool {
	out := make(map[string]bool)
	for _, c := range f.Categories() {
		out[c] = true
	}
	return out
}

// InstallDebugFilter 把 debugfilter 套到 slog.Default() 路径上。
// categories 为空时等价 no-op (slog.Default() 保持原 handler)。
//
// DM-20260617-002 W5 (AC12)
func InstallDebugFilter(categories []string) {
	if len(categories) == 0 {
		return
	}
	inner := slog.Default().Handler()
	if inner == nil {
		return
	}
	// 用 debugfilter.New 构造 filter 以保证 categories 校验逻辑与 logger 路径一致；
	// logger.Handler 的 inner 用 noop，因为我们只取其 categories。
	filter := debugfilter.New(noopHandler{}, categories)
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     categoriesOf(filter),
		passthru: true,
	}
	slog.SetDefault(slog.New(adapter))
	slog.Info("debug filter installed", "categories", categories)
}

// noopHandler logger.Handler 空实现，让 debugfilter.New 不报错。
type noopHandler struct{}

func (noopHandler) Handle(*logger.LogEntry) error { return nil }
func (noopHandler) SetLevel(logger.LogLevel)     {}
func (noopHandler) SetOutput(io.Writer)           {}
