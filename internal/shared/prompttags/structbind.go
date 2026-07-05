// Package prompttags — structbind.go
//
// Go-struct-driven I/O contract kernel (DM-20260705-003).
// Replaces the 3-place schema definition (struct + FrameSpec + manual fields map)
// with a single Go struct + `pt:"<name>,<plane>[,flags]"` struct tag as the SoT.
//
// Init-time enforcement (panics on design bug, 0 tolerance):
//   1. pt struct tag present and parseable
//   2. tag.Name appears in LineFrameRegistry[frame] (canonical frame fields)
//   3. tag.Plane matches frameFieldPlanes[frame][tag] (if both defined)
//   4. struct field count == LineFrameRegistry[frame].Fields length
//   5. each tag has an i18n when-use guide (populated by i18n package init)
//
// Hot path is reflection-free: `init()` writes the canonical FrameSpec and
// `BuildLineFrameFromStruct` does only field-value reflection (per round, < 50μs).
package prompttags

import (
	"fmt"
	"reflect"
	"strings"
)

// ptTag is the parsed form of `pt:"<name>,<plane>[,omit_empty][,omit_zero][,join=<sep>]"`.
// `pt:"-"` marks a non-prompt field (e.g., SessionID used only for routing).
type ptTag struct {
	Name      TagName
	Plane     PromptPlane
	OmitEmpty bool
	OmitZero  bool
	Join      string // default ","; only meaningful for []string
	Skip      bool   // `pt:"-"` marker
}

// RegisteredFrame holds reflection metadata for one user-frame struct (DM-20260705-003).
type RegisteredFrame struct {
	FrameName FrameName
	Spec      FrameSpec
	ptTags    []ptTag // in struct field declaration order
}

// frameFieldGuides is populated by i18n init() with (frame, tag) pairs that have
// when-use entries in prompttagsSemantics_{en,zh}.go.
var frameFieldGuides = map[FrameName]map[TagName]struct{}{}

// RegisterFrameFieldGuide marks a (frame, tag) pair as having an i18n when-use guide.
// Called by i18n package init(); safe to call multiple times for the same pair.
func RegisterFrameFieldGuide(frame FrameName, tag TagName) {
	if frameFieldGuides[frame] == nil {
		frameFieldGuides[frame] = map[TagName]struct{}{}
	}
	frameFieldGuides[frame][tag] = struct{}{}
}

// MustRegisterFrame reflects T's `pt:"..."` struct tags, registers in LineFrameRegistry,
// and panics on any consistency violation. Call once at package init() per frame.
//
// Generic parameter T is the user-frame struct (e.g., ObserveSignalInput).
// frameName must be a known FrameName (e.g., FrameObserveUser).
func MustRegisterFrame[T any](frameName FrameName) *RegisteredFrame {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("prompttags: MustRegisterFrame[%s]: T must be struct, got %s", frameName, t.Kind()))
	}
	rf := &RegisteredFrame{FrameName: frameName}

	// Check 2 anchor: existing FrameSpec for this frame.
	existingSpec, hasExisting := LineFrameRegistry[frameName]

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			panic(fmt.Errorf("prompttags: %s.%s: unexported field cannot have pt tag", t.Name(), f.Name))
		}
		tag, err := parseFrameFieldTag(f.Tag)
		if err != nil {
			panic(fmt.Errorf("prompttags: %s.%s: %w", t.Name(), f.Name, err))
		}
		if tag.Skip {
			continue
		}
		// Check 2: tag.Name appears in canonical FrameSpec[frame].
		if !hasExisting || !containsTagName(existingSpec.Fields, tag.Name) {
			panic(fmt.Errorf("prompttags: %s.%s: pt tag %q not in LineFrameRegistry[%s] (struct is SoT; update ObserveUserFrame/PlanUserFrame in linefield.go)",
				t.Name(), f.Name, tag.Name, frameName))
		}
		// Check 3: plane matches frameFieldPlanes (if defined).
		if planes, ok := frameFieldPlanes[frameName]; ok {
			if p, ok := planes[tag.Name]; ok && p != tag.Plane {
				panic(fmt.Errorf("prompttags: %s.%s (tag=%s) plane %q conflicts with frameFieldPlanes %q",
					t.Name(), f.Name, tag.Name, tag.Plane, p))
			}
		}
		rf.ptTags = append(rf.ptTags, tag)
		rf.Spec.Fields = append(rf.Spec.Fields, tag.Name)
	}
	// Check 4: struct field count == FrameSpec fields count.
	if hasExisting && len(existingSpec.Fields) != len(rf.Spec.Fields) {
		panic(fmt.Errorf("prompttags: %s struct has %d fields but LineFrameRegistry[%s] has %d; update FrameSpec in linefield.go",
			t.Name(), len(rf.Spec.Fields), frameName, len(existingSpec.Fields)))
	}
	// Check 5: each tag has i18n guide (only if i18n has registered any for this frame).
	if guides, ok := frameFieldGuides[frameName]; ok {
		for _, tag := range rf.ptTags {
			if _, ok := guides[tag.Name]; !ok {
				panic(fmt.Errorf("prompttags: %s tag=%q has no i18n when-use guide; add to prompttags_semantics_{en,zh}.go",
					t.Name(), tag.Name))
			}
		}
	}
	// Write to LineFrameRegistry (struct is SoT; hand-maintained FrameSpec must match).
	LineFrameRegistry[frameName] = rf.Spec
	return rf
}

// BuildLineFrameFromStruct serializes struct fields via reflection.
// Output is byte-equivalent to BuildAnnotatedLineFrame(frame, spec, structFieldsMap(s)).
//
// Accepts either T or *T; nil pointer returns "".
func BuildLineFrameFromStruct(frame FrameName, s any) string {
	if s == nil {
		return ""
	}
	v := reflect.ValueOf(s)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	spec, ok := LineFrameRegistry[frame]
	if !ok {
		return ""
	}
	fields := map[TagName]any{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, err := parseFrameFieldTag(f.Tag)
		if err != nil || tag.Skip {
			continue
		}
		fv := v.Field(i)
		// Dereference pointer for omit + type-switch; nil pointer -> absent field.
		// Added in M2 (DM-20260705-004) for StrategicPlanFrame.Budget 9 fields
		// which use *int to express "absent when Budget.MaxChildren == 0".
		for fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				fv = reflect.Value{}
				break
			}
			fv = fv.Elem()
		}
		if !fv.IsValid() {
			continue
		}
		// Apply omit rules before type conversion.
		if tag.OmitZero && fv.IsZero() {
			continue
		}
		if tag.OmitEmpty && isEmptyValue(fv) {
			continue
		}
		// Convert reflect.Value to map value (must match writeLineField type switch).
		switch fv.Kind() {
		case reflect.String:
			fields[tag.Name] = fv.String()
		case reflect.Float32, reflect.Float64:
			fields[tag.Name] = fv.Float()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fields[tag.Name] = fv.Int()
		case reflect.Bool:
			// Only true maps to "true"; false is omitted (omit_zero).
			if fv.Bool() {
				fields[tag.Name] = "true"
			}
		case reflect.Slice, reflect.Array:
			ss := make([]string, 0, fv.Len())
			for j := 0; j < fv.Len(); j++ {
				ev := fv.Index(j)
				if ev.Kind() == reflect.String {
					ss = append(ss, ev.String())
				}
			}
			if len(ss) > 0 {
				fields[tag.Name] = ss
			}
		}
	}
	return BuildAnnotatedLineFrame(frame, spec, fields)
}

// DocBlockFromStruct returns a one-line-per-field schema sketch derived from pt tags.
// Useful for i18n DocBlock migration (M1: prints field list; M2: enrich to JSON).
func DocBlockFromStruct[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() != reflect.Struct {
		return ""
	}
	var lines []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, err := parseFrameFieldTag(f.Tag)
		if err != nil || tag.Skip {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] %s", tag.Name, tag.Plane, f.Type.String()))
	}
	return strings.Join(lines, "\n")
}

// parseFrameFieldTag parses `pt:"<name>,<plane>[,omit_empty][,omit_zero][,join=<sep>]"`.
func parseFrameFieldTag(tag reflect.StructTag) (ptTag, error) {
	raw, ok := tag.Lookup("pt")
	if !ok {
		return ptTag{}, fmt.Errorf("missing pt struct tag")
	}
	if raw == "-" {
		return ptTag{Skip: true}, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) < 2 {
		return ptTag{}, fmt.Errorf("pt tag %q: want at least <name>,<plane>", raw)
	}
	out := ptTag{
		Name:  TagName(strings.TrimSpace(parts[0])),
		Plane: PromptPlane(strings.TrimSpace(parts[1])),
		Join:  ",",
	}
	if out.Name == "" {
		return ptTag{}, fmt.Errorf("pt tag %q: name empty", raw)
	}
	if out.Plane != PlaneData && out.Plane != PlaneControl {
		return ptTag{}, fmt.Errorf("pt tag %q: plane %q invalid, want data|control", raw, out.Plane)
	}
	for _, p := range parts[2:] {
		p = strings.TrimSpace(p)
		switch {
		case p == "omit_empty":
			out.OmitEmpty = true
		case p == "omit_zero":
			out.OmitZero = true
		case strings.HasPrefix(p, "join="):
			out.Join = strings.TrimPrefix(p, "join=")
		default:
			return ptTag{}, fmt.Errorf("pt tag %q: unknown flag %q", raw, p)
		}
	}
	return out, nil
}

// containsTagName reports whether a FrameSpec contains a given tag.
func containsTagName(spec []TagName, name TagName) bool {
	for _, n := range spec {
		if n == name {
			return true
		}
	}
	return false
}

// isEmptyValue reports whether a reflect.Value should be skipped under OmitEmpty.
// (Strings of "" and slices/arrays of length 0; maps and ptrs of nil.)
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}
