package adapters

import (
	"encoding/json"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/core"
)

// BuildCardJSON builds a Feishu interactive card JSON string from core.Card
func BuildCardJSON(card *core.Card) string {
	result := map[string]interface{}{
		"schema": "2.0",
		"config": map[string]interface{}{
			"wide_screen_mode": true,
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

// renderElement renders a core.CardElement to a Feishu card element map
func renderElement(elem core.CardElement) map[string]interface{} {
	switch e := elem.(type) {
	case core.CardMarkdown:
		return map[string]interface{}{
			"tag":     "markdown",
			"content": e.Content,
		}
	case core.CardDivider:
		return map[string]interface{}{
			"tag": "hr",
		}
	case core.CardActions:
		return renderCardActions(e)
	case core.CardListItem:
		return renderCardListItem(e)
	case core.CardSelect:
		return renderCardSelect(e)
	case core.CardNote:
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
func renderCardActions(actions core.CardActions) map[string]interface{} {
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
		if actions.Layout == core.CardActionLayoutEqualColumns {
			action["width"] = "fill"
		}
		renderedActions = append(renderedActions, action)
	}

	if len(renderedActions) == 0 {
		return nil
	}

	if actions.Layout == core.CardActionLayoutEqualColumns {
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
func renderCardListItem(item core.CardListItem) map[string]interface{} {
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
func renderCardSelect(sel core.CardSelect) map[string]interface{} {
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
	card := core.NewCard().
		Markdown(content).
		Note(footer).
		Build()
	return BuildCardJSON(card)
}

// NewCard creates a new card builder using core.CardBuilder
func NewCard() *core.CardBuilder {
	return core.NewCard()
}

// plainText creates a plain_text element
func plainText(content string) map[string]interface{} {
	return map[string]interface{}{
		"tag":    "plain_text",
		"content": content,
	}
}

// PreprocessMarkdown converts markdown to Feishu markdown
func PreprocessMarkdown(content string) string {
	// Convert common markdown to Feishu markdown
	// Headers: ### -> **  (but replace only first occurrence per line)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "### ") {
			lines[i] = "**" + line[4:]
		} else if strings.HasPrefix(line, "## ") {
			lines[i] = "**" + line[3:]
		} else if strings.HasPrefix(line, "# ") {
			lines[i] = "**" + line[2:]
		} else if strings.Contains(line, "**") {
			// Bold - replace inline **text** with <strong>text</strong>
			// Only for lines that are not headers
			lines[i] = replaceInlineBold(line)
		}
	}
	content = strings.Join(lines, "\n")

	// Code blocks - keep as-is for markdown element
	return content
}

// replaceInlineBold replaces **text** with <strong>text</strong> on a single line
// It skips the header prefix ** that was already added
func replaceInlineBold(line string) string {
	// Find ** that is NOT at the start of the line (header marker)
	idx := strings.Index(line[1:], "**")
	if idx < 0 {
		return line
	}
	// idx is relative to line[1:], so actual position is idx+1
	actualIdx := idx + 1
	end := strings.Index(line[actualIdx+2:], "**")
	if end < 0 {
		return line
	}
	actualEnd := actualIdx + 2 + end
	return line[:actualIdx] + "<strong>" + line[actualIdx+2:actualEnd] + "</strong>" + line[actualEnd+2:]
}
