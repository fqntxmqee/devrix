package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// T: D2-S15-A97-T04 — L5-MUPS-TAG-04 golden hash for Observe/Plan/Execute semantic appendix sections.
func TestMUPSSemanticAppendix_GoldenHash(t *testing.T) {
	dims := `{"citation":["file_line"],"severity":["none","p0_p1"],"reject":["planning_meta"]}`
	cases := []struct {
		name string
		fn   func(Locale) string
		loc  Locale
		want string
	}{
		{"observe_zh", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleZH, "4c82843ddb137e5ae02f1ebc95f5d839bb471ae0335c194410c6d9b057c6811c"}, // DM-20260706-011: observational_answer fast-path suffix added
		{"observe_en", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleEN, "f6105f8d1c3903557d9d5d5aff9840cad8caec794047558e60b34bbfae316af6"}, // DM-20260706-011: observational_answer fast-path suffix added
		{"plan_zh", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleZH, "246c4e1affee56a146695a904b085a39e36a28b4f1ac753b607520e15ed6e5c9"},
		{"plan_en", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleEN, "e9d132780e028fc17cefda0c86764540cb00cf47c2bb3bcb6525e6c618575c73"},
		{"execute_zh", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleZH, "54b25bf342bf7e0ebdbf2ca3f2a033a70ba742c6dfcbcadce932db9dfda5a965"},
		{"execute_en", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleEN, "b87ef933103031fc7b0acc4af616674880274cc1dbf905a4e7c6b4d330f4e2be"},
	}
	for _, c := range cases {
		got := hashHex(c.fn(c.loc))
		if got != c.want {
			t.Errorf("%s hash = %s, want %s", c.name, got, c.want)
		}
	}
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
