//go:build ignore

package main

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
)

func main() {
	loader := prompt.NewLoader(nil, i18n.DefaultLocale)
	
	fmt.Println("=== Static Sections ===")
	sections := loader.LoadAsSections("/tmp")
	for i, s := range sections {
		fmt.Printf("\n--- Section %d ---\n%s\n", i+1, s)
	}
	
	fmt.Println("\n=== With Dynamic Boundary ===")
	withDynamic := loader.LoadWithDynamic("/tmp", []string{"# Git Status\n...dynamic content..."})
	for i, s := range withDynamic {
		if s == prompt.DynamicBoundary {
			fmt.Printf("\n--- BOUNDARY (Static → Dynamic) ---\n")
		} else {
			fmt.Printf("\n--- Section %d ---\n%s\n", i+1, s)
		}
	}
	
	fmt.Println("\n=== Cache Stats ===")
	stats := loader.GetCacheStats()
	for name, cached := range stats {
		status := "✓ cached"
		if !cached {
			status = "✗ not cached"
		}
		fmt.Printf("  %s: %s\n", name, status)
	}
}
