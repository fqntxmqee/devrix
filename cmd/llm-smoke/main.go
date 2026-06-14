package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func main() {
	if os.Getenv("MINIMAX_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "MINIMAX_API_KEY is required")
		os.Exit(1)
	}

	configFile := config.FindConfigFile()
	if configFile != "" {
		fmt.Printf("config: %s\n", configFile)
	}

	obs := observability.NewNoOp()
	obsBridge := observability.NewBridge(obs)
	llmStack := llmbridge.WireContextLLM(configFile, obsBridge)
	if llmbridge.IsMockGateway(llmStack) {
		fmt.Fprintln(os.Stderr, "LLM gateway fell back to mock — check devrix.yaml llm_gateway section")
		os.Exit(1)
	}
	llmbridge.LogLLMReadiness(configFile)

	commCfg, _, _, ctxCfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	toolCfg, err := config.LoadToolConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load tool config: %v\n", err)
		os.Exit(1)
	}

	userCfg, _ := config.LoadUserConfig()
	permMgr := capture.NewPermissionManager(&commCfg.Permission)
	if userCfg != nil {
		permMgr.SetUserConfig(userCfg)
	}
	engine := bootstrap.NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, nil)

	prompt := "你好，请用一句话介绍你自己。"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}
	fmt.Printf("prompt: %q\n\n", prompt)

	session := &types.Session{
		SessionID:     fmt.Sprintf("llm_smoke_%d", time.Now().UnixMilli()),
		ChatID:        "llm_smoke",
		CreatedAt:     time.Now(),
		LastMessageAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	eventCh := engine.Process(ctx, session, prompt)
	var response strings.Builder
	for ev := range eventCh {
		line := strings.TrimSpace(ev.Content)
		if line != "" {
			fmt.Printf("[%s] %s\n", ev.Type, truncate(line, 300))
		} else {
			fmt.Printf("[%s]\n", ev.Type)
		}
		switch ev.Type {
		case "text":
			response.WriteString(ev.Content)
		case "error":
			fmt.Fprintf(os.Stderr, "engine error: %s\n", ev.Content)
			os.Exit(1)
		}
	}

	final := strings.TrimSpace(response.String())
	if final == "" {
		fmt.Fprintln(os.Stderr, "no text response received")
		os.Exit(1)
	}

	fmt.Println("\n--- final response ---")
	fmt.Println(final)
	fmt.Println("\nSMOKE PASS")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
