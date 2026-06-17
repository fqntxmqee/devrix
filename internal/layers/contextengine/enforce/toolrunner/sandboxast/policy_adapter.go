package sandboxast

// PolicyAnalyzer 把 sandboxast.Analyzer 适配成 toolrunner.CommandPolicy.ASTAnalyzer 接口。
type PolicyAnalyzer struct {
	A *Analyzer
}

// NewPolicyAnalyzer 构造适配器。
func NewPolicyAnalyzer() *PolicyAnalyzer {
	return &PolicyAnalyzer{A: NewAnalyzer()}
}

// Analyze 返回 (allow, reason)。sandbox.go 期望的签名。
func (p *PolicyAnalyzer) Analyze(cmd string) (bool, string) {
	v := p.A.Analyze(cmd)
	return v.Allow, v.Reason
}
