package evalcli

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/evolution/evaluate"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/shared/config"
)

type judgeStack struct {
	client evaluate.LLMClient
	config evaluate.JudgeConfig
}

func buildJudgeStack(mockJudge bool, configPath, judgeModel string) (judgeStack, error) {
	if mockJudge {
		return judgeStack{
			client: evaluate.NewStaticLLMClient(),
			config: evaluate.JudgeConfig{Model: "mock", Temperature: 0},
		}, nil
	}

	path := configPath
	if path == "" {
		path = config.FindConfigFile()
	}
	llmCfg, err := config.LoadLLMGatewayConfig(path)
	if err != nil {
		return judgeStack{}, fmt.Errorf("load llm gateway config: %w", err)
	}

	gw, err := stream.NewFromConfig(llmCfg, nil)
	if err != nil {
		return judgeStack{}, fmt.Errorf("create llm gateway: %w", err)
	}

	model := judgeModel
	if model == "" {
		model = llmCfg.DefaultModel
	}
	provider := llmCfg.DefaultProvider

	return judgeStack{
		client: evaluate.NewGatewayLLMClient(gw, model, provider),
		config: evaluate.JudgeConfig{
			Provider:    provider,
			Model:       model,
			Temperature: 0,
		},
	}, nil
}
