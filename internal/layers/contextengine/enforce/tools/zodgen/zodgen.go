// Package zodgen converts Go struct types into a JSON Schema subset,
// used by the tool_search runtime to surface the parameter shape of
// deferred tools in a model-friendly format.
//
// Supported features (subset):
//   - type (object)
//   - properties (string / integer / number / boolean / array / object)
//   - required (computed from `json:"-"` and `jsonschema:"required"` tag)
//   - enum (from `jsonschema:"enum=a,b,c"` tag)
//   - description (from `jsonschema:"description=..."` tag)
//
// Out of scope for v1.0 (deferred to v1.1):
//   - $ref, oneOf, anyOf, allOf
//   - nested struct references (we recurse but don't deduplicate)
//
// DSAFT: TOOL-SURFACE-1-A02 + T30 (DM-20260618-003 devrix-surface-lazy-loading).
package zodgen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Schema returns a JSON Schema (map[string]any) for the given Go struct
// type. The argument MUST be a struct (or pointer to struct); other
// types return an error.
func Schema(target any) (map[string]any, error) {
	t := reflect.TypeOf(target)
	if t == nil {
		return nil, fmt.Errorf("zodgen: nil target")
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("zodgen: target must be struct, got %s", t.Kind())
	}
	return schemaForStruct(t), nil
}

func schemaForStruct(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, omit, forceRequired := parseJSONTag(f.Tag.Get("json"))
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		desc, enumValues := parseJSONSchemaTag(f.Tag.Get("jsonschema"))
		prop := schemaForType(f.Type)
		if desc != "" {
			prop["description"] = desc
		}
		if len(enumValues) > 0 {
			prop["enum"] = enumValues
		}
		props[name] = prop
		if !omit || forceRequired {
			required = append(required, name)
		}
	}
	out := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func schemaForType(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Slice, reflect.Array:
		return map[string]any{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": schemaForType(t.Elem()),
		}
	case reflect.Struct:
		return schemaForStruct(t)
	}
	return map[string]any{"type": "string"} // unknown → string fallback
}

// parseJSONTag returns (name, omitempty, jsonschema-required).
// Examples:
//
//	`json:"foo"`                 -> ("foo", false, false)
//	`json:"foo,omitempty"`       -> ("foo", true,  false)
//	`json:"foo,required"`        -> ("foo", false, true)
//	`json:"-,required"`          -> ("-",   false, true)   // caller should skip
//	`json:"-"`                   -> ("-",   false, false)
func parseJSONTag(tag string) (name string, omitempty, required bool) {
	if tag == "" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			omitempty = true
		case "required":
			required = true
		}
	}
	return name, omitempty, required
}

// parseJSONSchemaTag extracts description + enum values from the
// `jsonschema:"..."` struct tag. Syntax:
//
//	`jsonschema:"description=foo bar,enum=a|b|c"`
//
// Description and enum are both optional. Enum values are joined with
// either `|` or `,` (commas are tricky inside struct tags because they're
// the delimiter; we accept both but prefer `|`).
func parseJSONSchemaTag(tag string) (description string, enumValues []string) {
	if tag == "" {
		return "", nil
	}
	for _, opt := range strings.Split(tag, ",") {
		switch {
		case strings.HasPrefix(opt, "description="):
			description = strings.TrimPrefix(opt, "description=")
		case strings.HasPrefix(opt, "enum="):
			raw := strings.TrimPrefix(opt, "enum=")
			for _, v := range strings.FieldsFunc(raw, func(r rune) bool {
				return r == '|' || r == ','
			}) {
				v = strings.TrimSpace(v)
				if v != "" {
					enumValues = append(enumValues, v)
				}
			}
		}
	}
	return description, enumValues
}

// SchemaString is a convenience that marshals the result of Schema()
// to a JSON string and returns "" + error on failure.
func SchemaString(target any) (string, error) {
	s, err := Schema(target)
	if err != nil {
		return "", err
	}
	// Manual JSON marshal is overkill; use a stable struct shape.
	return marshalIndent(s), nil
}

// marshalIndent does a deterministic JSON encoding of a
// map[string]any (no external deps). Keys are sorted at each level.
func marshalIndent(v any) string {
	var b strings.Builder
	writeIndent(&b, v, 0)
	return b.String()
}

func writeIndent(b *strings.Builder, v any, depth int) {
	switch x := v.(type) {
	case map[string]any:
		b.WriteString("{")
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		// simple insertion sort; small maps
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
				keys[j-1], keys[j] = keys[j], keys[j-1]
			}
		}
		for i, k := range keys {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n")
			writeIndent(b, k, depth+1)
			b.WriteString(": ")
			writeIndent(b, x[k], depth+1)
		}
		if len(keys) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("}")
	case []any:
		b.WriteString("[")
		for i, e := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			writeIndent(b, e, depth)
		}
		b.WriteString("]")
	case string:
		b.WriteString(strconv.Quote(x))
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64:
		b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case nil:
		b.WriteString("null")
	default:
		// Fallback: %v
		fmt.Fprintf(b, "%v", v)
	}
}
