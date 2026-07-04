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
		{"observe_zh", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleZH, "9cfa3a1053a50d7345c8dd58dd274a93059dfa6327cd3a94cfad81095495b419"},
		{"observe_en", func(l Locale) string { return ObservationTaskAppendix(l) }, LocaleEN, "798e4f4c5374f8fb178a364d13ae56f32479868f1ef8119013738d8752c61abc"},
		{"plan_zh", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleZH, "06c8f2737f58780e81c0146badcf88c120451d4280ec184f1fdce38b9424e52b"},
		{"plan_en", func(l Locale) string { return StrategicPlanAppendix(l, dims) }, LocaleEN, "e62adba50963f659e5b53ed1ba0238a09161587b9105d8412762faf7908bd507"},
		{"execute_zh", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleZH, "5e26a1f8d46932ff234f35c919f6f8b403ae19171ab05af86820b5f4f4d2ac4f"},
		{"execute_en", func(l Locale) string { return WorkItemExecuteOutputHints(l) }, LocaleEN, "774717980446f67c6de81329afa9214b4bdb19306705a207b5faedec26535b53"},
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
