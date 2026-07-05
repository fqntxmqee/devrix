package prompttags

import (
	"reflect"
	"strings"
	"testing"
)

// dummyFrame is a small stand-in user-frame struct for kernel unit tests.
// It exercises pt tag parsing, omit_empty / omit_zero, plane validation, and
// reflection-based serialization without depending on the real ObserveSignalInput.
type dummyFrame struct {
	SessionID  string   `pt:"-"`
	WorkItemID string   `pt:"work_item_id,control"`
	Directive  string   `pt:"directive,data"`
	Signal     []string `pt:"signal,data,omit_empty"`
	PriorMean  float64  `pt:"prior_mean,control,omit_zero"`
	HasPrior   bool     `pt:"incremental_only,control,omit_zero"`
}

// dummyFrameAlt has a deliberately wrong plane to verify init panic.
type dummyFrameAlt struct {
	Bad string `pt:"work_item_id,bogus"`
}

func TestParseFrameFieldTag_HappyPath(t *testing.T) {
	tag, err := parseFrameFieldTag(reflect.StructTag(`pt:"work_item_id,control"`))
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != "work_item_id" || tag.Plane != "control" {
		t.Fatalf("got %+v", tag)
	}
}

func TestParseFrameFieldTag_Flags(t *testing.T) {
	cases := []struct {
		raw  string
		want ptTag
	}{
		{`pt:"signal,data,omit_empty"`, ptTag{Name: "signal", Plane: "data", OmitEmpty: true, Join: ","}},
		{`pt:"prior_mean,control,omit_zero"`, ptTag{Name: "prior_mean", Plane: "control", OmitZero: true, Join: ","}},
		{`pt:"prior_observation_ids,control,omit_empty,join=|"`, ptTag{Name: "prior_observation_ids", Plane: "control", OmitEmpty: true, Join: "|"}},
		{`pt:"-"`, ptTag{Skip: true}},
	}
	for _, c := range cases {
		got, err := parseFrameFieldTag(reflect.StructTag(c.raw))
		if err != nil {
			t.Errorf("%s: %v", c.raw, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", c.raw, got, c.want)
		}
	}
}

func TestParseFrameFieldTag_Errors(t *testing.T) {
	cases := []string{
		``,                              // missing
		`json:"foo"`,                    // wrong key
		`pt:""`,                         // empty value
		`pt:"only_name"`,                // missing plane
		`pt:",data"`,                    // empty name
		`pt:"x,bogus"`,                  // bad plane
		`pt:"x,data,unknown_flag"`,      // unknown flag
	}
	for _, raw := range cases {
		if _, err := parseFrameFieldTag(reflect.StructTag(raw)); err == nil {
			t.Errorf("raw %q: expected error, got nil", raw)
		}
	}
}

// T: shared-A99-T01 / L5-MUPS-GSD-01 — MustRegisterFrame registers happy-path struct.
func TestMustRegisterFrame_HappyPath(t *testing.T) {
	// Pre-register a FrameSpec for the dummy frame so MustRegisterFrame can validate.
	dummyFrameName := FrameName("dummy_frame_for_test")
	LineFrameRegistry[dummyFrameName] = FrameSpec{Fields: []TagName{
		TagWorkItemID, TagDirective, TagSignal, TagPriorMean, TagIncrementalOnly,
	}}
	defer delete(LineFrameRegistry, dummyFrameName)

	rf := MustRegisterFrame[dummyFrame]("dummy_frame_for_test")
	if rf == nil {
		t.Fatal("nil registered frame")
	}
	if len(rf.Spec.Fields) != 5 {
		t.Fatalf("got %d fields, want 5", len(rf.Spec.Fields))
	}
	if rf.Spec.Fields[0] != "work_item_id" {
		t.Fatalf("field order: got %v", rf.Spec.Fields)
	}
	// dummy frame should now be in registry with our spec.
	spec, ok := LineFrameRegistry[dummyFrameName]
	if !ok {
		t.Fatal("dummy frame not in registry after MustRegisterFrame")
	}
	if len(spec.Fields) != 5 {
		t.Fatalf("registry has %d fields after register, want 5", len(spec.Fields))
	}
}

// T: shared-A99-T01 — init() in sessionorchestrator already registers ObserveSignalInput
// without panic (smoke test).
func TestMustRegisterFrame_ObserveSignalInputRegistered(t *testing.T) {
	spec, ok := LineFrameRegistry[FrameObserveUser]
	if !ok {
		t.Fatal("FrameObserveUser missing from LineFrameRegistry")
	}
	// 9 fields per ObserveUserFrame definition in linefield.go.
	if len(spec.Fields) != 9 {
		t.Fatalf("FrameObserveUser has %d fields, want 9: %v", len(spec.Fields), spec.Fields)
	}
	expected := []TagName{
		TagWorkItemID, TagDirective, TagPriorParseReject, TagPriorMean,
		TagScopeGoal, TagScopeOpenQuestion, TagSignal, TagPriorObservationIDs, TagIncrementalOnly,
	}
	for i, want := range expected {
		if spec.Fields[i] != want {
			t.Errorf("field[%d] = %q, want %q", i, spec.Fields[i], want)
		}
	}
}

// T: shared-A99-T02 / L5-MUPS-GSD-02 — BuildLineFrameFromStruct produces the
// expected key:value lines for a fully-populated struct (IncrementalOnly bool
// maps to "true", slices to one line per element, PriorMean to %.3f).
func TestBuildLineFrameFromStruct_FullStruct(t *testing.T) {
	in := dummyFrame{
		WorkItemID: "wi-1",
		Directive:  "do thing",
		Signal:     []string{"a", "b"},
		PriorMean:  0.7,
		HasPrior:   true,
	}
	LineFrameRegistry["dummy_frame_test"] = FrameSpec{
		Fields: []TagName{TagWorkItemID, TagDirective, TagSignal, TagPriorMean, TagIncrementalOnly},
	}
	defer delete(LineFrameRegistry, "dummy_frame_test")

	got := BuildLineFrameFromStruct("dummy_frame_test", &in)
	want := "[data] work_item_id: wi-1\n" +
		"[data] directive: do thing\n" +
		"[data] signal: a\n" +
		"[data] signal: b\n" +
		"[data] prior_mean: 0.700\n" +
		"[data] incremental_only: true\n"
	if got != want {
		t.Errorf("byte mismatch:\nstruct:\n%s\nwant:\n%s", got, want)
	}
}

// T: shared-A99-T02 — omit_empty/omit_zero skip fields.
func TestBuildLineFrameFromStruct_OmitEmpty(t *testing.T) {
	in := dummyFrame{
		WorkItemID: "wi-1",
		Directive:  "do thing",
		// Signal: nil → omit_empty
		// PriorMean: 0 → omit_zero
		// HasPrior: false → omit_zero
	}
	LineFrameRegistry["dummy_omit_test"] = FrameSpec{
		Fields: []TagName{TagWorkItemID, TagDirective, TagSignal, TagPriorMean, TagIncrementalOnly},
	}
	defer delete(LineFrameRegistry, "dummy_omit_test")

	got := BuildLineFrameFromStruct("dummy_omit_test", &in)
	// Should contain only work_item_id and directive.
	if strings.Contains(got, "signal:") {
		t.Errorf("signal should be omitted: %q", got)
	}
	if strings.Contains(got, "prior_mean:") {
		t.Errorf("prior_mean should be omitted: %q", got)
	}
	if strings.Contains(got, "incremental_only:") {
		t.Errorf("incremental_only should be omitted: %q", got)
	}
	if !strings.Contains(got, "work_item_id:") || !strings.Contains(got, "directive:") {
		t.Errorf("missing required fields: %q", got)
	}
}

// T: shared-A99-T02 — nil pointer and non-struct inputs return "".
func TestBuildLineFrameFromStruct_EdgeCases(t *testing.T) {
	if got := BuildLineFrameFromStruct("dummy_omit_test", nil); got != "" {
		t.Errorf("nil input: got %q", got)
	}
	var nilp *dummyFrame
	if got := BuildLineFrameFromStruct("dummy_omit_test", nilp); got != "" {
		t.Errorf("nil pointer: got %q", got)
	}
	if got := BuildLineFrameFromStruct("dummy_omit_test", "string"); got != "" {
		t.Errorf("non-struct: got %q", got)
	}
	if got := BuildLineFrameFromStruct("__nonexistent_frame__", &dummyFrame{}); got != "" {
		t.Errorf("unknown frame: got %q", got)
	}
}

// docBlockShape mirrors the real ObserveSignalInput field set (defined here to
// avoid importing sessionorchestrator into the prompttags unit tests). The actual
// registration of the real struct happens in sessionorchestrator.init().
type docBlockShape struct {
	SessionID           string   `pt:"-"`
	WorkItemID          string   `pt:"work_item_id,control"`
	Directive           string   `pt:"directive,data"`
	PriorParseReject    string   `pt:"prior_parse_reject,control,omit_empty"`
	PriorMean           float64  `pt:"prior_mean,control,omit_zero"`
	ScopeGoal           string   `pt:"scope_goal,data,omit_empty"`
	ScopeOpenQuestions  []string `pt:"scope_open_question,data,omit_empty"`
	Signal              []string `pt:"signal,data,omit_empty"`
	PriorObservationIDs []string `pt:"prior_observation_ids,control,omit_empty"`
	IncrementalOnly     bool     `pt:"incremental_only,control,omit_zero"`
}

// T: shared-A99-T03 / L5-MUPS-GSD-03 — DocBlockFromStruct returns one line per field.
func TestDocBlockFromStruct_ShapeMatches(t *testing.T) {
	got := DocBlockFromStruct[docBlockShape]()

	want := []string{
		"- work_item_id [control] string",
		"- directive [data] string",
		"- prior_parse_reject [control] string",
		"- prior_mean [control] float64",
		"- scope_goal [data] string",
		"- scope_open_question [data] []string",
		"- signal [data] []string",
		"- prior_observation_ids [control] []string",
		"- incremental_only [control] bool",
	}
	lines := strings.Split(got, "\n")
	if len(lines) != len(want) {
		t.Fatalf("DocBlockFromStruct lines = %d, want %d:\n%s", len(lines), len(want), got)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

// T: shared-A99-T04 / L5-MUPS-GSD-04 — invalid plane panics.
func TestMustRegisterFrame_InvalidPlanePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on invalid plane")
		}
	}()
	MustRegisterFrame[dummyFrameAlt]("dummy_bad_plane_test")
}

// T: shared-A99-T04 — non-struct generic param panics.
func TestMustRegisterFrame_NonStructPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on non-struct type")
		}
	}()
	MustRegisterFrame[string]("dummy_string_test")
}

// T: shared-A99-T05 — RegisterFrameFieldGuide populates the registry; missing
// tag for a registered frame panics.
func TestRegisterFrameFieldGuide_MissingPanics(t *testing.T) {
	RegisterFrameFieldGuide("dummy_guide_test_frame", TagWorkItemID)
	defer delete(frameFieldGuides, "dummy_guide_test_frame")
	if _, ok := frameFieldGuides["dummy_guide_test_frame"][TagWorkItemID]; !ok {
		t.Fatal("RegisterFrameFieldGuide failed to insert")
	}
	// A struct that has a tag NOT in the registered guides should panic.
	type dummyFrameMissingGuide struct {
		WorkItemID string `pt:"work_item_id,control"`
		Directive  string `pt:"directive,data"`
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing i18n guide")
		}
	}()
	MustRegisterFrame[dummyFrameMissingGuide]("dummy_guide_test_frame")
}
