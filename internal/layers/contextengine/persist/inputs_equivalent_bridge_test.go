// T: D2-S15-A02-T28 — InputsEquivalentBridge (persist → surface) integration test.
//
// 验证 bridge 转发正确 (跟 surface.InputsEquivalent 同行为) 并确保
// 没有破坏 ContentReplacementState 已有契约.
package persist

import (
	"testing"
)

func TestInputsEquivalentBridge_ByteIdenticalFastPath(t *testing.T) {
	a := []byte(`{"file_path":"foo.go"}`)
	b := []byte(`{"file_path":"foo.go"}`)
	if !InputsEquivalentBridge("read_file", a, b) {
		t.Error("byte-identical must be equivalent")
	}
}

func TestInputsEquivalentBridge_DelegatesToSurface(t *testing.T) {
	// bridge 必须跟 surface.InputsEquivalent 给出同样的答案 (转发验证).
	cases := []struct {
		tool string
		a, b string
		want bool
	}{
		{"read_file", `{"file_path":"foo.go","limit":50}`, `{"limit":50,"file_path":"foo.go"}`, true},  // reorder ok
		{"read_file", `{"file_path":"foo.go"}`, `{"file_path":"OTHER.go"}`, false},                         // different
		{"bash", `{"command":"ls","cwd":"/tmp"}`, `{"command":"ls","cwd":"/different"}`, true},           // per-tool: command-only
		{"bash", `{"command":"ls"}`, `{"command":"rm -rf /"}`, false},                                     // command diff
		{"write_file", `{"file_path":"a.go","content":"x"}`, `{"file_path":"a.go","content":"x"}`, true}, // same
		{"write_file", `{"file_path":"a.go","content":"x"}`, `{"file_path":"a.go","content":"y"}`, false}, // content diff
	}
	for _, c := range cases {
		got := InputsEquivalentBridge(c.tool, []byte(c.a), []byte(c.b))
		if got != c.want {
			t.Errorf("%s: bridge(%q, %q) = %v, want %v", c.tool, c.a, c.b, got, c.want)
		}
	}
}

func TestInputsEquivalentBridge_EmptyInputs(t *testing.T) {
	if !InputsEquivalentBridge("read_file", nil, nil) {
		t.Error("nil + nil must be equivalent")
	}
	if !InputsEquivalentBridge("read_file", []byte{}, []byte{}) {
		t.Error("empty + empty must be equivalent")
	}
	if InputsEquivalentBridge("read_file", []byte(`{}`), nil) {
		t.Error("non-empty + nil must NOT be equivalent")
	}
}

func TestInputsEquivalentBridge_DoesNotPanicOnInvalidJSON(t *testing.T) {
	// invalid JSON → fail-closed (return false). 不 panic.
	if InputsEquivalentBridge("read_file", []byte(`{broken`), []byte(`{}`)) {
		t.Error("invalid JSON a must fail closed (false)")
	}
}
