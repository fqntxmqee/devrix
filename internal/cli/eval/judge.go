package evalcli

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/evolution/eval"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/shared/config"
)

type judgeStack struct {
	client eval.LLMClient
	config eval.JudgeConfig
}

func buildJudgeStack(mockJudge bool, configPath, judgeModel string) (judgeStack, error) {
	if mockJudge {
		return judgeStack{
			client: eval.NewStaticLLMClient(),
			config: eval.JudgeConfig{Model: "mock", Temperature: 0},
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
		client: eval.NewGatewayLLMClient(gw, model, provider),
		config: eval.JudgeConfig{
			Provider:    provider,
			Model:       model,
			Temperature: 0,
		},
	}, nil
}
