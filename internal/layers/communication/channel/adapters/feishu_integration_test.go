package adapters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
)

// TestFeishuIntegration_CardRendering 验证卡片渲染的 JSON 结构（完整覆盖）
func TestFeishuIntegration_CardRendering(t *testing.T) {
	tests := []struct {
		name        string
		card        *kernel.Card
		wantSchema  string
		wantHeader  string
		wantColor   string
		wantElement string
	}{
		{
			name: "thinking card",
			card: kernel.NewCard().
				Title("🤔 思考中...", "blue").
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "🤔 思考中...",
			wantColor:   "blue",
			wantElement: "markdown",
		},
		{
			name: "tool call card",
			card: kernel.NewCard().
				Title("🔧 调用工具", "orange").
				Markdown("**工具:** `read`").
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "🔧 调用工具",
			wantColor:   "orange",
			wantElement: "markdown",
		},
		{
			name: "complete card",
			card: kernel.NewCard().
				Title("✅ 完成!", "green").
				Markdown("任务已完成").
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "✅ 完成!",
			wantColor:   "green",
			wantElement: "markdown",
		},
		{
			name: "error card",
			card: kernel.NewCard().
				Title("❌ 错误", "red").
				Markdown("出错了").
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "❌ 错误",
			wantColor:   "red",
			wantElement: "markdown",
		},
		{
			name: "milestone card",
			card: kernel.NewCard().
				Title("📊 任务进度", "purple").
				Markdown("**进度:** 50%").
				Markdown("**任务:** Step 1").
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "📊 任务进度",
			wantColor:   "purple",
			wantElement: "markdown",
		},
		{
			name: "card with buttons",
			card: kernel.NewCard().
				Title("操作", "blue").
				Buttons(
					kernel.PrimaryBtn("确认", "confirm"),
					kernel.DefaultBtn("取消", "cancel"),
				).
				Build(),
			wantSchema:  "2.0",
			wantHeader:  "操作",
			wantColor:   "blue",
			wantElement: "action",
		},
		{
			name: "card with note",
			card: kernel.NewCard().
				Markdown("内容").
				Note("脚注").
				Build(),
			wantSchema:  "2.0",
			wantElement: "note",
		},
		{
			name: "card with danger button",
			card: kernel.NewCard().
				Buttons(kernel.DangerBtn("删除", "delete")).
				Build(),
			wantSchema:  "2.0",
			wantElement: "action",
		},
		{
			name: "card with equal columns buttons",
			card: kernel.NewCard().
				ButtonsEqual(
					kernel.PrimaryBtn("A", "a"),
					kernel.PrimaryBtn("B", "b"),
				).
				Build(),
			wantSchema:  "2.0",
			wantElement: "column_set",
		},
		{
			name: "card with list item",
			card: kernel.NewCard().
				ListItem("Description", "Button", "action").
				Build(),
			wantSchema:  "2.0",
			wantElement: "column_set",
		},
		{
			name: "card with select",
			card: kernel.NewCard().
				Select("Choose", []kernel.CardSelectOption{
					{Text: "A", Value: "a"},
					{Text: "B", Value: "b"},
				}, "a").
				Build(),
			wantSchema:  "2.0",
			wantElement: "action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr := BuildCardJSON(tt.card)

			var result map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			// Verify schema
			if result["schema"] != tt.wantSchema {
				t.Errorf("schema = %v, want %v", result["schema"], tt.wantSchema)
			}

			// Verify header if expected
			if tt.wantHeader != "" {
				header := result["header"].(map[string]interface{})
				titleContent := header["title"].(map[string]interface{})["content"].(string)
				if titleContent != tt.wantHeader {
					t.Errorf("header title = %v, want %v", titleContent, tt.wantHeader)
				}
				if header["template"] != tt.wantColor {
					t.Errorf("header color = %v, want %v", header["template"], tt.wantColor)
				}
			}

			// Verify element type
			if tt.wantElement != "" {
				body := result["body"].(map[string]interface{})
				elements := body["elements"].([]interface{})
				found := false
				for _, e := range elements {
					elem := e.(map[string]interface{})
					if elem["tag"] == tt.wantElement {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("element type %s not found in card", tt.wantElement)
				}
			}
		})
	}
}

// TestFeishuIntegration_RenderTextFallback 验证 RenderText 回退机制
func TestFeishuIntegration_RenderTextFallback(t *testing.T) {
	tests := []struct {
		name    string
		card    *kernel.Card
		wantStr []string
		notWant []string
	}{
		{
			name: "card with title and markdown",
			card: kernel.NewCard().
				Title("Test Title", "blue").
				Markdown("Hello **world**").
				Build(),
			wantStr: []string{"**Test Title**", "Hello", "world"},
		},
		{
			name: "card with buttons",
			card: kernel.NewCard().
				Buttons(kernel.PrimaryBtn("OK", "ok")).
				Build(),
			wantStr: []string{"[OK]"},
		},
		{
			name: "card with code block preserves format",
			card: kernel.NewCard().
				Markdown("```go\nfunc main() {}\n```").
				Build(),
			wantStr: []string{"func main()"},
		},
		{
			name: "card with list item",
			card: kernel.NewCard().
				ListItem("Description", "Button", "action").
				Build(),
			wantStr: []string{"[Button]"},
		},
		{
			name: "card with divider",
			card: kernel.NewCard().
				Markdown("Before").
				Divider().
				Markdown("After").
				Build(),
			wantStr: []string{"Before", "---", "After"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.card.RenderText()

			for _, want := range tt.wantStr {
				if !strings.Contains(text, want) {
					t.Errorf("RenderText() should contain %q, got: %s", want, text)
				}
			}

			if tt.notWant != nil {
				for _, notWant := range tt.notWant {
					if strings.Contains(text, notWant) {
						t.Errorf("RenderText() should NOT contain %q, got: %s", notWant, text)
					}
				}
			}
		})
	}
}

// TestFeishuIntegration_AllColors 验证所有颜色支持
func TestFeishuIntegration_AllColors(t *testing.T) {
	colors := []string{"blue", "green", "red", "orange", "purple", "grey"}

	for _, color := range colors {
		t.Run(color, func(t *testing.T) {
			card := kernel.NewCard().
				Title("Test", color).
				Build()

			jsonStr := BuildCardJSON(card)

			var result map[string]interface{}
			json.Unmarshal([]byte(jsonStr), &result)

			header := result["header"].(map[string]interface{})
			if header["template"] != color {
				t.Errorf("expected color %s, got %v", color, header["template"])
			}
		})
	}
}

// TestFeishuIntegration_CardHasButtons 验证 HasButtons 方法
func TestFeishuIntegration_CardHasButtons(t *testing.T) {
	tests := []struct {
		name      string
		card      *kernel.Card
		wantButtons bool
	}{
		{
			name: "card with buttons",
			card: kernel.NewCard().
				Buttons(kernel.PrimaryBtn("OK", "ok")).
				Build(),
			wantButtons: true,
		},
		{
			name: "card with list item",
			card: kernel.NewCard().
				ListItem("desc", "btn", "action").
				Build(),
			wantButtons: true,
		},
		{
			name: "card with select",
			card: kernel.NewCard().
				Select("choose", []kernel.CardSelectOption{{Text: "A", Value: "a"}}, "").
				Build(),
			wantButtons: true,
		},
		{
			name: "card with only markdown",
			card: kernel.NewCard().
				Markdown("Hello").
				Build(),
			wantButtons: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.card.HasButtons() != tt.wantButtons {
				t.Errorf("HasButtons() = %v, want %v", tt.card.HasButtons(), tt.wantButtons)
			}
		})
	}
}

// TestFeishuIntegration_PreprocessMarkdown 验证 Markdown 预处理为 JSON 2.0 透传
func TestFeishuIntegration_PreprocessMarkdown(t *testing.T) {
	input := `### Header
**bold** and | A | B | table`
	result := PreprocessMarkdown(input)
	if result != input {
		t.Errorf("PreprocessMarkdown() = %q, want passthrough", result)
	}
}

// TestFeishuIntegration_WideScreenMode 验证宽屏模式
func TestFeishuIntegration_WideScreenMode(t *testing.T) {
	card := kernel.NewCard().
		Title("Wide Screen Test", "blue").
		Build()

	jsonStr := BuildCardJSON(card)

	var result map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &result)

	config := result["config"].(map[string]interface{})
	if config["wide_screen_mode"] != true {
		t.Errorf("expected wide_screen_mode true, got %v", config["wide_screen_mode"])
	}
}