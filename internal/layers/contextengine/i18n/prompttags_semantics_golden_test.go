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
		{"observe_zh", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleZH, "30167b0fd8d933e899acabe93cea938bd1af6f869a8a368665496617c03953e9"}, // DM-20260705-003: SemanticBlock JSON-lines + condition glossary
		{"observe_en", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleEN, "37683d9238a85ecc2157242a6b90e221fbec330c2cbd46fed051da9cc6480445"}, // DM-20260705-003: SemanticBlock JSON-lines + condition glossary
		{"plan_zh", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleZH, "b2ee3d311adfc0f7b7dd11faa77427a89afb43d7864ca8a528b22f305f665f68"},
		{"plan_en", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleEN, "3f6a629c2141059b6b5eace7bd4d991ba938d8e8660b30b7046b69912cf3bc76"},
		{"execute_zh", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleZH, "4a52d28773b5367f03580beca3cf720c560e04c507cd73da44201dc369f5cad1"},
		{"execute_en", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleEN, "d4ca892fa809204ccbf18cfb89cbc930fd28d7f978c790e01f21531c1946e8f8"},
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
