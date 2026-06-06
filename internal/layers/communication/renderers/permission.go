package renderers

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionRenderer renders permission request cards
type PermissionRenderer struct {
	ansi config.ANSIConfig
}

// NewPermissionRenderer creates a new PermissionRenderer
func NewPermissionRenderer(ansi config.ANSIConfig) *PermissionRenderer {
	return &PermissionRenderer{ansi: ansi}
}

// RenderCard renders a permission request card
func (r *PermissionRenderer) RenderCard(req *types.PermissionRequest) {
	width := 60
	border := strings.Repeat("─", width)

	fmt.Println()
	fmt.Printf("┌%s┐\n", border)
	fmt.Printf("│%s⚠️  Permission Required%s\n", padCenter("⚠️  Permission Required", width-2), r.ansi.Reset)
	fmt.Printf("├%s┤\n", border)
	fmt.Printf("│  Tool: %s%s%s\n", r.ansi.Warning, req.ToolName, r.ansi.Reset)
	fmt.Printf("│  Risk: %s\n", r.formatRiskLevel(req.RiskLevel))
	fmt.Printf("├%s┤\n", border)

	if req.Description != "" {
		fmt.Printf("│  Description:\n")
		fmt.Printf("│  %s\n", wrapText(req.Description, width-4))
		fmt.Printf("│\n")
	}

	if req.InputPreview != "" {
		fmt.Printf("│  Input Preview:\n")
		fmt.Printf("│  %s\n", wrapText(req.InputPreview, width-4))
		fmt.Printf("│\n")
	}

	fmt.Printf("├%s┤\n", border)
	fmt.Printf("│  %s[yes]%s Allow   %s[no]%s Deny   %s[all]%s Allow all\n",
		r.ansi.Assistant, r.ansi.Reset,
		r.ansi.Error, r.ansi.Reset,
		r.ansi.Assistant, r.ansi.Reset)
	fmt.Printf("└%s┘\n", border)
	fmt.Println()
}

// formatRiskLevel formats a risk level with color
func (r *PermissionRenderer) formatRiskLevel(level types.RiskLevel) string {
	var color string
	switch level {
	case types.RiskLevelCritical:
		color = r.ansi.Error
	case types.RiskLevelHigh:
		color = r.ansi.Error
	case types.RiskLevelMedium:
		color = r.ansi.Warning
	default:
		color = r.ansi.Assistant
	}
	return fmt.Sprintf("%s%s%s", color, level, r.ansi.Reset)
}

// padCenter pads a string to center it within a width
func padCenter(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	left := (width - len(s)) / 2
	right := width - len(s) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// wrapText wraps text to fit within a width
func wrapText(s string, width int) string {
	var lines []string
	words := strings.Fields(s)
	currentLine := ""
	widthCount := 0

	for _, word := range words {
		wordLen := len(word)
		if widthCount+wordLen+1 > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
			widthCount = wordLen
		} else {
			if currentLine != "" {
				currentLine += " "
				widthCount++
			}
			currentLine += word
			widthCount += wordLen
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n  ")
}
