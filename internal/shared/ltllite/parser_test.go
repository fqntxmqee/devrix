package ltllite

import (
	"errors"
	"strings"
	"testing"
)

// T: W14-PERMISSION-GATE-1-T01 — ParseStruct 正确解析 valid invariant。
func TestParse_ValidInvariant(t *testing.T) {
	type Sample struct {
		Field1 string `invariant:"is_read_only => no_destructive"`
	}
	set, err := ParseStruct(Sample{})
	if err != nil {
		t.Fatalf("ParseStruct: %v", err)
	}
	if len(set.Invariants) != 1 {
		t.Fatalf("len = %d, want 1", len(set.Invariants))
	}
	got := set.Invariants[0]
	if got.Name != "Field1" {
		t.Errorf("Name = %q, want Field1", got.Name)
	}
	if got.Pre != "is_read_only" {
		t.Errorf("Pre = %q, want is_read_only", got.Pre)
	}
	if got.Post != "no_destructive" {
		t.Errorf("Post = %q, want no_destructive", got.Post)
	}
	if got.Source != "Sample" {
		t.Errorf("Source = %q, want Sample", got.Source)
	}
}

// T: W14 — 缺少 => 操作符但语法合法 (隐式恒等: pre = post = tag)。
func TestParse_ImplicitIdentity(t *testing.T) {
	type S struct {
		F string `invariant:"always_true"`
	}
	set, err := ParseStruct(S{})
	if err != nil {
		t.Fatalf("ParseStruct: %v", err)
	}
	if len(set.Invariants) != 1 {
		t.Fatalf("len = %d, want 1", len(set.Invariants))
	}
	if set.Invariants[0].Pre != "always_true" || set.Invariants[0].Post != "always_true" {
		t.Errorf("implicit identity failed: pre=%q post=%q", set.Invariants[0].Pre, set.Invariants[0].Post)
	}
}

// T: W14 — 无 invariant tag 的字段被忽略。
func TestParse_NoTagIgnored(t *testing.T) {
	type S struct {
		NoTag   string
		Tagged  string `invariant:"a => b"`
	}
	set, err := ParseStruct(S{})
	if err != nil {
		t.Fatalf("ParseStruct: %v", err)
	}
	if len(set.Invariants) != 1 {
		t.Errorf("len = %d, want 1 (no-tag field should be skipped)", len(set.Invariants))
	}
	if set.Invariants[0].Name != "Tagged" {
		t.Errorf("Name = %q, want Tagged", set.Invariants[0].Name)
	}
}

// T: W14 — 多字段 (multi-invariants)。
func TestParse_MultipleFields(t *testing.T) {
	type Multi struct {
		A string `invariant:"is_typed => typed_only"`
		B string `invariant:"destructive => permission_gate"`
		C string `invariant:"long_running => interrupt_cancel"`
	}
	set, err := ParseStruct(Multi{})
	if err != nil {
		t.Fatalf("ParseStruct: %v", err)
	}
	if len(set.Invariants) != 3 {
		t.Errorf("len = %d, want 3", len(set.Invariants))
	}
}

// T: W14 — 空 tag 返回 ErrInvalidInvariant (wrapped)。
func TestParse_Invalid_EmptyTag(t *testing.T) {
	type S struct {
		F string `invariant:""`
	}
	_, err := ParseStruct(S{})
	if err == nil {
		t.Fatal("expected error on empty tag, got nil")
	}
	if !errors.Is(err, ErrInvalidInvariant) {
		t.Errorf("expected ErrInvalidInvariant, got %v", err)
	}
}

// T: W14 — 有 => 但 pre 或 post 为空 返回 ErrInvalidInvariant。
func TestParse_Invalid_EmptyPreOrPost(t *testing.T) {
	type S1 struct {
		F string `invariant:" => something"`
	}
	type S2 struct {
		F string `invariant:"something => "`
	}
	for _, sample := range []any{S1{}, S2{}} {
		_, err := ParseStruct(sample)
		if !errors.Is(err, ErrInvalidInvariant) {
			t.Errorf("expected ErrInvalidInvariant for %T, got %v", sample, err)
		}
	}
}

// T: W14 — 解析 pointer-to-struct 也工作 (e.g. *Surface)。
func TestParse_PointerToStruct(t *testing.T) {
	type S struct {
		F string `invariant:"a => b"`
	}
	ps := &S{}
	set, err := ParseStruct(ps)
	if err != nil {
		t.Fatalf("ParseStruct ptr: %v", err)
	}
	if len(set.Invariants) != 1 {
		t.Errorf("ptr to struct: len = %d, want 1", len(set.Invariants))
	}
}

// T: W14 — non-struct 输入返回错误。
func TestParse_NotStruct_ReturnsError(t *testing.T) {
	_, err := ParseStruct(42)
	if err == nil {
		t.Fatal("expected error for non-struct input")
	}
	if !strings.Contains(err.Error(), "struct") {
		t.Errorf("error should mention struct, got %q", err.Error())
	}
}

// T: W14 — Invariant.String 输出含 Source.Name + Raw。
func TestInvariant_String(t *testing.T) {
	type S struct {
		F string `invariant:"a => b"`
	}
	set, _ := ParseStruct(S{})
	got := set.Invariants[0].String()
	if !strings.Contains(got, "S.F") {
		t.Errorf("String should contain S.F, got %q", got)
	}
	if !strings.Contains(got, "a => b") {
		t.Errorf("String should contain raw 'a => b', got %q", got)
	}
}
