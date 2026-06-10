package evalcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devrix/devrix/internal/layers/evolution/eval"
)

// Run dispatches eval subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devrix eval run --dataset <path>")
	}
	switch args[0] {
	case "run":
		return RunEval(args[1:])
	default:
		return fmt.Errorf("unknown eval command %q", args[0])
	}
}

// RunEval executes `eval run`.
func RunEval(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	dataset := fs.String("dataset", "openspec/eval-datasets/v1/dataset.yaml", "path to eval dataset YAML")
	output := fs.String("output", "", "output report JSON path (default: stdout)")
	saveBaseline := fs.Bool("save-baseline", false, "save report as baseline alongside dataset")
	mockJudge := fs.Bool("mock-judge", true, "use static mock judge (set false for real LLM-as-Judge)")
	configPath := fs.String("config", "", "path to devrix.yaml (for real judge)")
	judgeModel := fs.String("judge-model", "", "judge model override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	stack, err := buildJudgeStack(*mockJudge, *configPath, *judgeModel)
	if err != nil {
		return err
	}

	jm := eval.NewJudgeManager(stack.client, nil, stack.config)
	jm.RegisterRubric(eval.ScoreRubric{
		Dimension:   "compression_recall",
		Instruction: "Evaluate whether ALL key facts from the original context are preserved in the compressed version.",
		Scale:       "0-1",
	})

	engine := eval.NewEvalEngine(eval.EvalConfig{
		Enabled: true,
		Judge:   stack.config,
	}, jm)

	report, err := engine.Run(context.Background(), eval.EvalOpts{
		DatasetPath:  *dataset,
		SaveBaseline: *saveBaseline,
	})
	if err != nil {
		return fmt.Errorf("eval run failed: %w", err)
	}
	if report == nil {
		return fmt.Errorf("eval disabled or empty report")
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	data = append(data, '\n')

	if *output == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(*output, data, 0o644)
}
