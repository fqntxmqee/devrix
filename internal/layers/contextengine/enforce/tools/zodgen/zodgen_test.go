package zodgen_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/zodgen"
)

// T: TOOL-SURFACE-1-T30 — Schema() handles basic struct → object/properties/required.
func TestSchema_BasicStruct(t *testing.T) {
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	got, err := zodgen.Schema(User{})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if got["type"] != "object" {
		t.Errorf("type = %v, want object", got["type"])
	}
	props, _ := got["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Error("missing name property")
	}
	if _, ok := props["age"]; !ok {
		t.Error("missing age property")
	}
	req, _ := got["required"].([]string)
	if len(req) != 2 {
		t.Errorf("required = %v, want [name age]", req)
	}
}

// T: TOOL-SURFACE-1-T30 — omitempty drops field from required.
func TestSchema_Omitempty(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		Nickname string `json:"nickname,omitempty"`
	}
	got, _ := zodgen.Schema(User{})
	req, _ := got["required"].([]string)
	if len(req) != 1 || req[0] != "name" {
		t.Errorf("required = %v, want [name]", req)
	}
}

// T: TOOL-SURFACE-1-T30 — jsonschema:"required" forces field into required.
func TestSchema_ForceRequired(t *testing.T) {
	type User struct {
		Name     string `json:"name,omitempty"`
		Nickname string `json:"nickname,omitempty,required"`
	}
	got, _ := zodgen.Schema(User{})
	req, _ := got["required"].([]string)
	// nickname is forced required even with omitempty.
	containsNick := false
	for _, r := range req {
		if r == "nickname" {
			containsNick = true
		}
	}
	if !containsNick {
		t.Errorf("required = %v, want nickname forced required", req)
	}
}

// T: TOOL-SURFACE-1-T30 — jsonschema enum values are captured.
func TestSchema_Enum(t *testing.T) {
	type Color struct {
		Name string `json:"name" jsonschema:"enum=red|green|blue"`
	}
	got, _ := zodgen.Schema(Color{})
	props, _ := got["properties"].(map[string]any)
	name, _ := props["name"].(map[string]any)
	enum, _ := name["enum"].([]string)
	want := []string{"red", "green", "blue"}
	if !reflect.DeepEqual(enum, want) {
		t.Errorf("enum = %v, want %v", enum, want)
	}
}

// T: TOOL-SURFACE-1-T30 — jsonschema description populates property.description.
func TestSchema_Description(t *testing.T) {
	type User struct {
		Name string `json:"name" jsonschema:"description=user full name"`
	}
	got, _ := zodgen.Schema(User{})
	props, _ := got["properties"].(map[string]any)
	name, _ := props["name"].(map[string]any)
	if name["description"] != "user full name" {
		t.Errorf("description = %v", name["description"])
	}
}

// T: TOOL-SURFACE-1-T30 — nested struct is recursed.
func TestSchema_NestedStruct(t *testing.T) {
	type Address struct {
		City string `json:"city"`
	}
	type User struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}
	got, _ := zodgen.Schema(User{})
	props, _ := got["properties"].(map[string]any)
	addr, _ := props["address"].(map[string]any)
	if addr["type"] != "object" {
		t.Errorf("nested type = %v, want object", addr["type"])
	}
	addrProps, _ := addr["properties"].(map[string]any)
	if _, ok := addrProps["city"]; !ok {
		t.Error("missing nested city property")
	}
}

// T: TOOL-SURFACE-1-T30 — slice field → array with items.
func TestSchema_SliceField(t *testing.T) {
	type User struct {
		Tags []string `json:"tags"`
	}
	got, _ := zodgen.Schema(User{})
	props, _ := got["properties"].(map[string]any)
	tags, _ := props["tags"].(map[string]any)
	if tags["type"] != "array" {
		t.Errorf("type = %v, want array", tags["type"])
	}
	items, _ := tags["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("items.type = %v, want string", items["type"])
	}
}

// T: TOOL-SURFACE-1-T30 — SchemaString returns parseable JSON.
func TestSchemaString(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}
	s, err := zodgen.SchemaString(User{})
	if err != nil {
		t.Fatalf("SchemaString: %v", err)
	}
	if !strings.Contains(s, `"type"`) || !strings.Contains(s, `"object"`) {
		t.Errorf("SchemaString output = %s, want contains object schema", s)
	}
}
