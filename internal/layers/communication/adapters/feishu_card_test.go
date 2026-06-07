package adapters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/core"
)

func parseCardJSON(t *testing.T, jsonStr string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	return result
}

func getCardElements(t *testing.T, result map[string]interface{}) []map[string]interface{} {
	body, ok := result["body"].(map[string]interface{})
	if !ok {
		t.Fatal("expected body to be map")
	}
	elementsRaw, ok := body["elements"].([]interface{})
	if !ok {
		t.Fatal("expected elements to be array")
	}
	elements := make([]map[string]interface{}, len(elementsRaw))
	for i, e := range elementsRaw {
		elements[i], ok = e.(map[string]interface{})
		if !ok {
			t.Fatalf("element %d is not a map", i)
		}
	}
	return elements
}

func TestBuildCardJSON_BasicCard(t *testing.T) {
	card := core.NewCard().
		Title("Test Title", "blue").
		Markdown("Hello **world**").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	if result["schema"] != "2.0" {
		t.Errorf("expected schema 2.0, got %v", result["schema"])
	}

	header, ok := result["header"].(map[string]interface{})
	if !ok {
		t.Fatal("expected header to be map")
	}
	if header["template"] != "blue" {
		t.Errorf("expected header template blue, got %v", header["template"])
	}

	elements := getCardElements(t, result)
	if len(elements) != 1 {
		t.Errorf("expected 1 element, got %d", len(elements))
	}
}

func TestBuildCardJSON_AllColors(t *testing.T) {
	colors := []string{"blue", "green", "red", "orange", "purple", "grey"}

	for _, color := range colors {
		card := core.NewCard().
			Title("Test", color).
			Build()

		jsonStr := BuildCardJSON(card)
		result := parseCardJSON(t, jsonStr)

		header := result["header"].(map[string]interface{})
		if header["template"] != color {
			t.Errorf("expected color %s, got %v", color, header["template"])
		}
	}
}

func TestBuildCardJSON_DefaultColor(t *testing.T) {
	card := core.NewCard().
		Title("Test", "").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	header := result["header"].(map[string]interface{})
	if header["template"] != "blue" {
		t.Errorf("expected default color blue, got %v", header["template"])
	}
}

func TestBuildCardJSON_EmptyCard(t *testing.T) {
	card := core.NewCard().Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	if result["header"] != nil {
		t.Error("expected nil header for card without title")
	}

	elements := getCardElements(t, result)
	if len(elements) != 1 {
		t.Errorf("expected 1 empty element, got %d", len(elements))
	}
	if elements[0]["tag"] != "markdown" || elements[0]["content"] != " " {
		t.Errorf("unexpected empty element: %v", elements[0])
	}
}

func TestBuildCardJSON_WithDivider(t *testing.T) {
	card := core.NewCard().
		Title("Test", "green").
		Markdown("Content before").
		Divider().
		Markdown("Content after").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	if len(elements) != 3 {
		t.Errorf("expected 3 elements, got %d", len(elements))
	}
	if elements[1]["tag"] != "hr" {
		t.Errorf("expected hr tag for divider, got %v", elements[1]["tag"])
	}
}

func TestBuildCardJSON_WithButtons(t *testing.T) {
	card := core.NewCard().
		Title("Actions", "blue").
		Buttons(
			core.PrimaryBtn("Confirm", "confirm"),
			core.DefaultBtn("Cancel", "cancel"),
		).
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	if len(elements) != 1 {
		t.Errorf("expected 1 action element, got %d", len(elements))
	}

	action := elements[0]
	if action["tag"] != "action" {
		t.Errorf("expected action tag, got %v", action["tag"])
	}

	actionsRaw := action["actions"].([]interface{})
	if len(actionsRaw) != 2 {
		t.Errorf("expected 2 buttons, got %d", len(actionsRaw))
	}
}

func TestBuildCardJSON_WithButtonsEqualColumns(t *testing.T) {
	card := core.NewCard().
		Title("Actions", "blue").
		ButtonsEqual(
			core.DangerBtn("Delete", "delete"),
			core.PrimaryBtn("Save", "save"),
		).
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	columnSet := elements[0]
	if columnSet["tag"] != "column_set" {
		t.Errorf("expected column_set tag, got %v", columnSet["tag"])
	}

	columnsRaw := columnSet["columns"].([]interface{})
	if len(columnsRaw) != 2 {
		t.Errorf("expected 2 columns, got %d", len(columnsRaw))
	}
}

func TestBuildCardJSON_WithListItem(t *testing.T) {
	card := core.NewCard().
		Title("List", "purple").
		ListItem("Read file: /tmp/test.txt", "View", "view_file").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	columnSet := elements[0]
	if columnSet["tag"] != "column_set" {
		t.Errorf("expected column_set tag, got %v", columnSet["tag"])
	}
}

func TestBuildCardJSON_WithSelect(t *testing.T) {
	card := core.NewCard().
		Title("Select", "blue").
		Select("Choose an option", []core.CardSelectOption{
			{Text: "Option A", Value: "a"},
			{Text: "Option B", Value: "b"},
		}, "a").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	action := elements[0]
	actionsRaw := action["actions"].([]interface{})
	selectElem := actionsRaw[0].(map[string]interface{})

	if selectElem["tag"] != "select_static" {
		t.Errorf("expected select_static tag, got %v", selectElem["tag"])
	}
}

func TestBuildCardJSON_WithNote(t *testing.T) {
	card := core.NewCard().
		Title("Note Test", "green").
		Markdown("Some content").
		Note("This is a footnote").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	note := elements[1]
	if note["tag"] != "note" {
		t.Errorf("expected note tag, got %v", note["tag"])
	}
}

func TestBuildCardJSON_WithTaggedNote(t *testing.T) {
	card := core.NewCard().
		TaggedNote("action:approve", "Approved by admin").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	if len(elements) != 1 || elements[0]["tag"] != "note" {
		t.Errorf("expected note element, got %v", elements)
	}
}

func TestBuildCardJSON_WideScreenMode(t *testing.T) {
	card := core.NewCard().
		Title("Wide Screen", "blue").
		Build()

	jsonStr := BuildCardJSON(card)
	result := parseCardJSON(t, jsonStr)

	config := result["config"].(map[string]interface{})
	if config["wide_screen_mode"] != true {
		t.Errorf("expected wide_screen_mode true, got %v", config["wide_screen_mode"])
	}
}

func TestPreprocessMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h3 header",
			input:    "### Hello World",
			expected: "**Hello World",
		},
		{
			name:     "h2 header",
			input:    "## Hello World",
			expected: "**Hello World",
		},
		{
			name:     "h1 header",
			input:    "# Hello World",
			expected: "**Hello World",
		},
		{
			name:     "bold text",
			input:    "This is **bold** text",
			expected: "This is <strong>bold</strong> text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessMarkdown(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPreprocessMarkdown_MultipleLines(t *testing.T) {
	input := `### Header Line
## Another Header
Some **bold** text`

	result := PreprocessMarkdown(input)

	if !strings.Contains(result, "**Header Line") {
		t.Error("expected h3 to be converted")
	}
	if !strings.Contains(result, "**Another Header") {
		t.Error("expected h2 to be converted")
	}
	if !strings.Contains(result, "<strong>bold</strong>") {
		t.Error("expected bold to be converted")
	}
}

func TestBuildCardJSONWithStatus(t *testing.T) {
	jsonStr := BuildCardJSONWithStatus("Hello content", "Footer note")
	result := parseCardJSON(t, jsonStr)

	elements := getCardElements(t, result)
	if len(elements) != 2 {
		t.Errorf("expected 2 elements, got %d", len(elements))
	}

	if elements[0]["tag"] != "markdown" {
		t.Errorf("expected markdown tag, got %v", elements[0]["tag"])
	}

	if elements[1]["tag"] != "note" {
		t.Errorf("expected note tag, got %v", elements[1]["tag"])
	}
}

func TestPlainText(t *testing.T) {
	result := plainText("Hello")

	expected := map[string]interface{}{
		"tag":     "plain_text",
		"content": "Hello",
	}

	if result["tag"] != expected["tag"] || result["content"] != expected["content"] {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestBuildCardJSON_CardHasButtons(t *testing.T) {
	cardWithButtons := core.NewCard().
		Buttons(core.PrimaryBtn("OK", "ok")).
		Build()
	if !cardWithButtons.HasButtons() {
		t.Error("card with buttons should report HasButtons=true")
	}

	cardWithoutButtons := core.NewCard().
		Markdown("Just text").
		Build()
	if cardWithoutButtons.HasButtons() {
		t.Error("card without buttons should report HasButtons=false")
	}
}

func TestBuildCardJSON_RenderText(t *testing.T) {
	card := core.NewCard().
		Title("Test Title", "blue").
		Markdown("Hello **world**").
		Buttons(core.PrimaryBtn("OK", "ok")).
		Build()

	text := card.RenderText()

	if !strings.Contains(text, "**Test Title**") {
		t.Error("expected rendered text to contain title")
	}
	if !strings.Contains(text, "Hello") {
		t.Error("expected rendered text to contain markdown content")
	}
	if !strings.Contains(text, "[OK]") {
		t.Error("expected rendered text to contain button")
	}
}