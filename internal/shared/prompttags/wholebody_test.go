package prompttags

import "testing"

func TestParseWholeBody_BareJSON(t *testing.T) {
	type payload struct {
		Findings []string `json:"findings"`
	}
	got, ok := ParseWholeBody[payload](`{"findings":["a"]}`)
	if !ok || len(got.Findings) != 1 || got.Findings[0] != "a" {
		t.Fatalf("got = %+v ok=%v", got, ok)
	}
}

func TestParseWholeBody_FencedJSON(t *testing.T) {
	type payload struct {
		Scope string `json:"scope"`
	}
	raw := "```json\n{\"scope\":\"internal/pkg\"}\n```"
	got, ok := ParseWholeBody[payload](raw)
	if !ok || got.Scope != "internal/pkg" {
		t.Fatalf("got = %+v ok=%v", got, ok)
	}
}

func TestParseWholeBody_Array(t *testing.T) {
	got, ok := ParseWholeBody[[]string](`["a","b"]`)
	if !ok || len(got) != 2 || got[1] != "b" {
		t.Fatalf("got = %+v ok=%v", got, ok)
	}
}

func TestParseWholeBody_RejectsPlainText(t *testing.T) {
	_, ok := ParseWholeBody[map[string]string]("not json")
	if ok {
		t.Fatal("expected reject for plain text")
	}
}
