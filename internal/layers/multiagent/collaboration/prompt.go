package collaboration

import "github.com/devrix/devrix/internal/layers/multiagent"

// BuildPromptForMode enhances the base system prompt for the given mode.
func BuildPromptForMode(mode multiagent.CollaborationMode, basePrompt string) string {
	switch mode {
	case multiagent.ModeChainOfThought:
		return basePrompt + "\n\n## 推理模式：Chain-of-Thought\n" +
			"请逐步推理，每一步都要明确说明你的理由。" +
			"最终答案前用【最终答案】标记。"
	case multiagent.ModeIterativeRefinement:
		return basePrompt + "\n\n## 推理模式：Iterative Refinement\n" +
			"请先给出初始答案，然后进行自我批判（指出不足），" +
			"最后在【改进答案】后给出改进版本。"
	default:
		return basePrompt
	}
}
