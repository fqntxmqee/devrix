package adapters

import (
	"encoding/json"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
)

// BuildStreamingReplyCardJSON builds a JSON 2.0 reply card for cardkit streaming.
func BuildStreamingReplyCardJSON(content string, streaming bool) string {
	result := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode":   streaming,
			"update_multi":     true,
			"width_mode":       "fill",
			"wide_screen_mode": true,
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"element_id": replyTextElementID,
					"content":    content,
				},
			},
		},
	}
	b, _ := json.Marshal(result)
	return string(b)
}

// BuildCardJSON builds a Feishu interactive card JSON string from kernel.Card
func BuildCardJSON(card *kernel.Card) string {
	result := map[string]interface{}{
		"schema": "2.0",
		"config": map[string]interface{}{
			"wide_screen_mode": true,
			"width_mode":       "fill",
		},
	}

	if card.Header != nil && card.Header.Title != "" {
		color := card.Header.Color
		if color == "" {
			color = "blue"
		}
		result["header"] = map[string]interface{}{
			"title":    plainText(card.Header.Title),
			"template": color,
		}
	}

	var elements []map[string]interface{}
	for _, elem := range card.Elements {
		elements = append(elements, renderElement(elem))
	}

	if len(elements) == 0 {
		elements = []map[string]interface{}{
			{"tag": "markdown", "content": " "},
		}
	}

	result["body"] = map[string]interface{}{
		"elements": elements,
	}

	b, _ := json.Marshal(result)
	return string(b)
}

// renderElement renders a kernel.CardElement to a Feishu card element map
func renderElement(elem kernel.CardElement) map[string]interface{} {
	switch e := elem.(type) {
	case kernel.CardMarkdown:
		return map[string]interface{}{
			"tag":     "markdown",
			"content": PreprocessMarkdown(e.Content),
		}
	case kernel.CardDivider:
		return map[string]interface{}{
			"tag": "hr",
		}
	case kernel.CardActions:
		return renderCardActions(e)
	case kernel.CardListItem:
		return renderCardListItem(e)
	case kernel.CardSelect:
		return renderCardSelect(e)
	case kernel.CardNote:
		return map[string]interface{}{
			"tag": "note",
			"elements": []map[string]interface{}{
				{"tag": "plain_text", "content": e.Text},
			},
		}
	default:
		return map[string]interface{}{
			"tag":     "markdown",
			"content": " ",
		}
	}
}

// renderCardActions renders CardActions to Feishu action elements
func renderCardActions(actions kernel.CardActions) map[string]interface{} {
	var renderedActions []map[string]interface{}

	for _, btn := range actions.Buttons {
		btnType := btn.Type
		if btnType == "" {
			btnType = "default"
		}
		valueMap := map[string]string{"action": btn.Value}
		for k, v := range btn.Extra {
			valueMap[k] = v
		}
		action := map[string]interface{}{
			"tag":   "button",
			"text":  plainText(btn.Text),
			"type":  btnType,
			"value": valueMap,
		}
		if actions.Layout == kernel.CardActionLayoutEqualColumns {
			action["width"] = "fill"
		}
		renderedActions = append(renderedActions, action)
	}

	if len(renderedActions) == 0 {
		return nil
	}

	if actions.Layout == kernel.CardActionLayoutEqualColumns {
		// Use column_set for equal columns layout
		columns := make([]map[string]interface{}, 0, len(renderedActions))
		for _, action := range renderedActions {
			columns = append(columns, map[string]interface{}{
				"tag":            "column",
				"width":          "weighted",
				"weight":         1,
				"vertical_align":  "center",
				"horizontal_align": "center",
				"elements":        []map[string]interface{}{action},
			})
		}
		columnSet := map[string]interface{}{
			"tag":     "column_set",
			"columns": columns,
		}
		if len(renderedActions) == 2 {
			columnSet["flex_mode"] = "bisect"
		}
		return columnSet
	}

	// Default row layout
	return map[string]interface{}{
		"tag":      "action",
		"actions":  renderedActions,
	}
}

// renderCardListItem renders CardListItem to Feishu column_set
func renderCardListItem(item kernel.CardListItem) map[string]interface{} {
	btnType := item.BtnType
	if btnType == "" {
		btnType = "default"
	}
	valueMap := map[string]string{"action": item.BtnValue}
	for k, v := range item.Extra {
		valueMap[k] = v
	}

	return map[string]interface{}{
		"tag":      "column_set",
		"flex_mode": "none",
		"columns": []map[string]interface{}{
			{
				"tag":            "column",
				"width":          "weighted",
				"weight":         5,
				"vertical_align":  "center",
				"elements": []map[string]interface{}{
					{"tag": "markdown", "content": item.Text},
				},
			},
			{
				"tag":            "column",
				"width":          "auto",
				"vertical_align":  "center",
				"elements": []map[string]interface{}{
					{
						"tag":   "button",
						"text":  plainText(item.BtnText),
						"type":  btnType,
						"value": valueMap,
					},
				},
			},
		},
	}
}

// renderCardSelect renders CardSelect to Feishu select_static
func renderCardSelect(sel kernel.CardSelect) map[string]interface{} {
	var options []map[string]interface{}
	for _, opt := range sel.Options {
		options = append(options, map[string]interface{}{
			"text":  plainText(opt.Text),
			"value": opt.Value,
		})
	}

	selectElem := map[string]interface{}{
		"tag":         "select_static",
		"placeholder": plainText(sel.Placeholder),
		"options":     options,
	}
	if sel.InitValue != "" {
		selectElem["initial_option"] = sel.InitValue
	}

	return map[string]interface{}{
		"tag":     "action",
		"actions": []map[string]interface{}{selectElem},
	}
}

// BuildCardJSONWithStatus builds a card with status footer
func BuildCardJSONWithStatus(content, footer string) string {
	card := kernel.NewCard().
		Markdown(content).
		Note(footer).
		Build()
	return BuildCardJSON(card)
}

// NewCard creates a new card builder using kernel.CardBuilder
func NewCard() *kernel.CardBuilder {
	return kernel.NewCard()
}

// plainText creates a plain_text element
func plainText(content string) map[string]interface{} {
	return map[string]interface{}{
		"tag":    "plain_text",
		"content": content,
	}
}

// flattenMarkdownTablesForFeishu converts pipe-table rows to plain text so Feishu
// markdown cards do not hit the per-card table element limit during streaming.
func flattenMarkdownTablesForFeishu(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			cells := strings.Split(trimmed, "|")
			parts := make([]string, 0, len(cells))
			for _, cell := range cells {
				cell = strings.TrimSpace(cell)
				if cell == "" || strings.Contains(cell, "---") {
					continue
				}
				parts = append(parts, cell)
			}
			if len(parts) > 0 {
				out = append(out, strings.Join(parts, " · "))
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// PreprocessMarkdown prepares markdown for Feishu JSON 2.0 rich-text components.
// Schema 2.0 supports standard Markdown (headings, tables, code blocks, bold, lists),
// so we pass content through unchanged instead of converting to legacy lark_md syntax.
func PreprocessMarkdown(content string) string {
	return content
}
