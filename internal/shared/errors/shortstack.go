package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// ShortStack 截取调用栈前 N 帧，去掉 runtime/testing/reflect 噪声。
//
// 输出格式：每行一帧 "<file>:<line> <func>()"，对应 clawcode 错误
// 截断能力（参见 clawcode/src/utils/errors.ts 的 shortStack）。
// 过滤前缀：runtime.* / testing.* / reflect.* / 本包 internal/*
// (避免递归)。
func ShortStack(err error, maxFrames int) string {
	if err == nil {
		return ""
	}
	if maxFrames <= 0 {
		maxFrames = 5
	}

	pcs := make([]uintptr, maxFrames+8)
	n := runtime.Callers(2, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])

	out := make([]string, 0, maxFrames)
	for {
		frame, more := frames.Next()
		if !shouldIncludeFrame(frame) {
			if !more {
				break
			}
			continue
		}
		out = append(out, formatFrame(frame))
		if len(out) >= maxFrames {
			break
		}
		if !more {
			break
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n")
}

// WithShortStack 在错误包装链上挂上截短的栈，可被 fmt.Sprintf("%+v", err) 渲染。
// 实现类型 *stackedError，Unwrap 暴露底层 err 以便 errors.Is/As 仍可工作。
func WithShortStack(err error, maxFrames int) error {
	if err == nil {
		return nil
	}
	return &stackedError{
		err:   err,
		stack: ShortStack(err, maxFrames),
	}
}

// FormatStack 直接从当前调用点截取栈（无需 error），便于日志附加。
func FormatStack(maxFrames int) string {
	return ShortStack(fmt.Errorf("stack"), maxFrames)
}

type stackedError struct {
	err   error
	stack string
}

func (e *stackedError) Error() string {
	if e.stack == "" {
		return e.err.Error()
	}
	return e.err.Error() + "\n" + e.stack
}

func (e *stackedError) Unwrap() error { return e.err }

func (e *stackedError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n%s", e.err, e.stack)
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

func shouldIncludeFrame(frame runtime.Frame) bool {
	fn := frame.Function
	if fn == "" {
		return false
	}
	switch {
	case strings.HasPrefix(fn, "runtime."):
		return false
	case strings.HasPrefix(fn, "testing."):
		return false
	case strings.HasPrefix(fn, "reflect."):
		return false
	}
	// devrix/internal/shared/errors.ShortStack 自身
	if strings.Contains(fn, "shared/errors.ShortStack") ||
		strings.Contains(fn, "shared/errors.FormatStack") {
		return false
	}
	return true
}

func formatFrame(frame runtime.Frame) string {
	file := frame.File
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		// 截断路径前缀，保留最后两段路径，与 clawcode 风格一致
		if second := strings.LastIndex(file[:idx], "/"); second >= 0 {
			file = "..." + file[second:]
		}
	}
	return fmt.Sprintf("%s:%d %s()", file, frame.Line, frame.Function)
}
