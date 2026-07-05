package i18n

import (
	"strings"
	"testing"
)

// DM-20260705-009 — Golden snapshot for the closed-classifier role declaration
// appended to ObservationTaskAppendix. The markers below are the user-facing
// contract that LLM Observe-node calls must include; drifting them silently
// re-introduces the "review this code" / "return []" / "return markdown"
// regression reported in DM-20260705-009 §2.1.

// T: D7-S5-A99-T10 (NEW, DM-20260705-009) — ObservationTaskAppendix(ZH) declares
// the closed-classifier role and its input/output contract.
func TestObservationTaskAppendix_ClosedClassifier_ZH(t *testing.T) {
	got := ObservationTaskAppendix(LocaleZH)
	markers := []string{
		"封闭式分类助手",        // role label
		"不执行工具",          // negative constraint
		"不评估任务完成度",       // negative constraint
		"不分析任务本身",        // negative constraint
		"不返回 markdown",   // format constraint
		"输入 = directive", // input contract
		"输出 = Obs* 数组",   // output contract
	}
	for _, m := range markers {
		if !strings.Contains(got, m) {
			t.Errorf("zh appendix missing closed-classifier marker %q:\n%s", m, got)
		}
	}
}

// T: D7-S5-A99-T11 (NEW, DM-20260705-009) — ObservationTaskAppendix(ZH) tells the
// LLM to prefer obs_uncertainty over an empty array when signals are insufficient.
func TestObservationTaskAppendix_PreferUncertaintyWhenSignalsInsufficient_ZH(t *testing.T) {
	got := ObservationTaskAppendix(LocaleZH)
	markers := []string{
		"signal 不足",
		"优先 obs_uncertainty",
		"返回 question",
		"directive 模糊",
		"任务需工具时",
	}
	for _, m := range markers {
		if !strings.Contains(got, m) {
			t.Errorf("zh appendix missing signal-insufficient guidance %q:\n%s", m, got)
		}
	}
}

// T: D7-S5-A99-T12 (NEW, DM-20260705-009) — ObservationTaskAppendix(EN) mirrors
// the closed-classifier role and signal-insufficient guidance in English.
func TestObservationTaskAppendix_ClosedClassifierAndUncertainty_EN(t *testing.T) {
	got := ObservationTaskAppendix(LocaleEN)
	markers := []string{
		"closed-set classifier",
		"do not execute tools",
		"do not assess task completion",
		"do not analyze the task itself",
		"prefer obs_uncertainty",
		"signals are insufficient",
		"task needs tools",
		"do not assume its completion status",
	}
	for _, m := range markers {
		if !strings.Contains(got, m) {
			t.Errorf("en appendix missing closed-classifier marker %q:\n%s", m, got)
		}
	}
}

// T: D7-S5-A99-T13 (DM-20260705-004) — observe.node_role must NOT duplicate intro role.
func TestObservationTaskAppendix_NoDuplicateNodeRole(t *testing.T) {
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := ObservationTaskAppendix(loc)
		nodeRole := semanticText(loc, "observe.node_role")
		if strings.Contains(got, nodeRole) {
			t.Fatalf("loc=%s appendix must not repeat observe.node_role:\n%s", loc, got)
		}
		if loc == LocaleZH && !strings.Contains(got, "封闭式分类助手") {
			t.Errorf("loc=ZH appendix should keep intro role, got: %s", got)
		}
		if loc == LocaleEN && !strings.Contains(got, "closed-set classifier") {
			t.Errorf("loc=EN appendix should keep intro role, got: %s", got)
		}
	}
}
