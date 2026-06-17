package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

// TestShortStack_NilError — nil 输入返回空字符串。
func TestShortStack_NilError(t *testing.T) {
	if got := ShortStack(nil, 5); got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}
}

// TestShortStack_TruncatesToN — 截取到 N 帧。
func TestShortStack_TruncatesToN(t *testing.T) {
	err := stderrors.New("boom")
	got := captureShortStack3(err)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	for _, line := range lines {
		if !strings.Contains(line, "()") {
			t.Fatalf("line missing func() format: %q", line)
		}
	}
	// 最顶层应为 captureShortStack3（避免被 testing/runtime 过滤）
	if !strings.Contains(got, "captureShortStack3()") {
		t.Fatalf("expected captureShortStack3() in stack, got %q", got)
	}
}

func captureShortStack3(err error) string {
	return captureShortStack2(err)
}

func captureShortStack2(err error) string {
	return captureShortStack(err)
}

func captureShortStack(err error) string {
	return ShortStack(err, 3)
}

// TestShortStack_FiltersRuntimeNoise — runtime/testing/reflect 帧被过滤。
func TestShortStack_FiltersRuntimeNoise(t *testing.T) {
	err := stderrors.New("boom")
	got := ShortStack(err, 20)
	if strings.Contains(got, "runtime.") {
		t.Fatalf("runtime frames not filtered: %q", got)
	}
	if strings.Contains(got, "testing.") {
		t.Fatalf("testing frames not filtered: %q", got)
	}
	if strings.Contains(got, "reflect.") {
		t.Fatalf("reflect frames not filtered: %q", got)
	}
}

// TestShortStack_IncludesTestFunctionName — 测试函数自身应在栈里。
func TestShortStack_IncludesTestFunctionName(t *testing.T) {
	err := stderrors.New("boom")
	got := ShortStack(err, 20)
	if !strings.Contains(got, "TestShortStack_IncludesTestFunctionName") {
		t.Fatalf("expected test func name in stack, got %q", got)
	}
}

// TestWithShortStack_PreservesErrorMessage — 包装后 Error() 仍含原 message。
func TestWithShortStack_PreservesErrorMessage(t *testing.T) {
	orig := stderrors.New("root cause")
	wrapped := WithShortStack(orig, 5)
	if !strings.Contains(wrapped.Error(), "root cause") {
		t.Fatalf("wrapped.Error missing root cause: %q", wrapped.Error())
	}
}

// TestWithShortStack_AppendsStack — 包装后 Error() 含调用栈。
func TestWithShortStack_AppendsStack(t *testing.T) {
	orig := stderrors.New("boom")
	wrapped := WithShortStack(orig, 5)
	if !strings.Contains(wrapped.Error(), "TestWithShortStack_AppendsStack") {
		t.Fatalf("wrapped.Error missing stack frame: %q", wrapped.Error())
	}
}

// TestWithShortStack_ErrorsIs — 包装后 errors.Is 仍可识别 sentinel。
func TestWithShortStack_ErrorsIs(t *testing.T) {
	wrapped := WithShortStack(ErrSessionNotFound, 3)
	if !stderrors.Is(wrapped, ErrSessionNotFound) {
		t.Fatalf("errors.Is broken after WithShortStack")
	}
}

// TestWithShortStack_NilInput — nil 输入返回 nil。
func TestWithShortStack_NilInput(t *testing.T) {
	if got := WithShortStack(nil, 5); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

// TestWithShortStack_FormatVerb — %+v 展开原 err + 栈。
func TestWithShortStack_FormatVerb(t *testing.T) {
	orig := stderrors.New("boom")
	wrapped := WithShortStack(orig, 5)
	out := fmt.Sprintf("%+v", wrapped)
	if !strings.Contains(out, "boom") {
		t.Fatalf("%%+v missing original error: %q", out)
	}
	if !strings.Contains(out, "TestWithShortStack_FormatVerb") {
		t.Fatalf("%%+v missing stack frame: %q", out)
	}
}

// TestFormatStack_NoError — FormatStack 直接从当前点截取栈。
func TestFormatStack_NoError(t *testing.T) {
	got := FormatStack(5)
	lines := strings.Split(got, "\n")
	if len(lines) < 1 {
		t.Fatalf("expected >=1 line, got %d: %q", len(lines), got)
	}
	if !strings.Contains(got, "TestFormatStack_NoError") {
		t.Fatalf("expected test func in stack, got %q", got)
	}
}
