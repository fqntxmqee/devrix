package collaboration

import "github.com/devrix/devrix/internal/layers/multiagent"

// BuildPromptForMode enhances the base system prompt for the given mode.
func BuildPromptForMode(mode multiagent.CollaborationMode, basePrompt string) string {
	switch mode {
	case multiagent.ModeChainOfThought:
		return basePrompt + "\n\n## 推理模式：Chain-of-Thought\n" +
			"请逐步推理，每一步都要明确说明你的理由。\n\n" +
			"### 规范\n" +
			"- 每个推理步骤用「Step N:」标记\n" +
			"- 明确指出每一步的依据（代码观察、文档引用、逻辑推导）\n" +
			"- 如果遇到不确定的信息，标注置信度（高/中/低）\n" +
			"- 最终答案前用「【最终答案】」标记\n\n" +
			"### 示例\n" +
			"Step 1: 分析函数签名 — 发现该函数接受 io.Reader 而非具体类型\n" +
			"Step 2: 检查调用方 — 有 3 处调用，分别传入 os.File 和 bytes.Buffer\n" +
			"Step 3: 推断设计意图 — 接口抽象使得测试时可以传入 bytes.Buffer 模拟\n" +
			"【最终答案】该函数设计使用 io.Reader 接口以实现可测试性"

	case multiagent.ModeIterativeRefinement:
		return basePrompt + "\n\n## 推理模式：Iterative Refinement\n" +
			"请先给出初始答案，然后进行自我批判，最后给出改进版本。\n\n" +
			"### 规范\n" +
			"- 第一步「初始答案」——基于现有信息给出尽可能好的答案\n" +
			"- 第二步「自我批判」——指出上述答案中的不足：遗漏了什么？哪里可能出错？\n" +
			"- 第三步「改进答案」——基于批判改进答案，用「【改进答案】」标记\n\n" +
			"### 示例\n" +
			"【初始答案】使用 sync.Mutex 保护共享状态\n" +
			"【自我批判】未考虑读多写少的场景，sync.RWMutex 在此场景下性能更好\n" +
			"【改进答案】使用 sync.RWMutex：读操作调用 RLock()/RUnlock()，写操作调用 Lock()/Unlock()"

	default:
		return basePrompt
	}
}
