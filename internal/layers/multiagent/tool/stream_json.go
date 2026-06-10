package tool

import (
	"encoding/json"
	"strings"
)

// StreamParseResult is the normalized output of one CLI stdout line.
type StreamParseResult struct {
	Events []Event
	Done   bool // true when the agent turn finished (complete / error)
}

// ParseStreamJSONLine maps Devrix stream-json and Claude Code stream-json lines to events.
func ParseStreamJSONLine(line string) StreamParseResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return StreamParseResult{}
	}

	var simple Event
	if err := json.Unmarshal([]byte(line), &simple); err != nil {
		return StreamParseResult{Events: []Event{{Type: "text", Content: line}}}
	}

	switch simple.Type {
	case "text", "tool_use", "error":
		return StreamParseResult{Events: []Event{simple}}
	case "complete":
		return StreamParseResult{Events: []Event{simple}, Done: true}
	case "assistant":
		return parseClaudeAssistantLine(line)
	case "result":
		return parseClaudeResultLine(line)
	case "system", "user":
		return StreamParseResult{}
	default:
		if simple.Type != "" {
			return StreamParseResult{}
		}
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return StreamParseResult{Events: []Event{{Type: "text", Content: line}}}
	}
	switch envelope.Type {
	case "assistant":
		return parseClaudeAssistantLine(line)
	case "result":
		return parseClaudeResultLine(line)
	}
	return StreamParseResult{}
}

func parseClaudeAssistantLine(line string) StreamParseResult {
	var payload struct {
		Message struct {
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return StreamParseResult{}
	}
	var events []Event
	for _, block := range payload.Message.Content {
		switch block.Type {
		case "text":
			if text := strings.TrimSpace(block.Text); text != "" {
				events = append(events, Event{Type: "text", Content: text})
			}
		case "thinking":
			// Claude Code may emit thinking blocks; surface as text for agent tool output.
			if text := strings.TrimSpace(block.Thinking); text != "" {
				events = append(events, Event{Type: "text", Content: text})
			}
		}
	}
	return StreamParseResult{Events: events}
}

func parseClaudeResultLine(line string) StreamParseResult {
	var payload struct {
		Subtype  string `json:"subtype"`
		IsError  bool   `json:"is_error"`
		Result   string `json:"result"`
		APIError string `json:"api_error_status"`
	}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return StreamParseResult{}
	}
	if payload.IsError || payload.Subtype == "error" {
		msg := strings.TrimSpace(payload.Result)
		if msg == "" {
			msg = strings.TrimSpace(payload.APIError)
		}
		if msg == "" {
			msg = "claude code returned an error"
		}
		return StreamParseResult{
			Events: []Event{{Type: "error", Content: msg}},
			Done:   true,
		}
	}
	return StreamParseResult{
		Events: []Event{{Type: "complete", Content: strings.TrimSpace(payload.Result)}},
		Done:   true,
	}
}
