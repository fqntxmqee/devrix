package workmodel

// ScopeContract captures Goal-level scope convergence before decompose (DM-20260627-003).
type ScopeContract struct {
	GoalStatement      string   `json:"goal_statement,omitempty"`
	InScope            []string `json:"in_scope,omitempty"`
	OutOfScope         []string `json:"out_of_scope,omitempty"`
	Assumptions        []string `json:"assumptions,omitempty"`
	OpenQuestions      []string `json:"open_questions,omitempty"`
	SuccessCriteria    []string `json:"success_criteria,omitempty"`
	SuggestedDecompose []string `json:"suggested_decompose,omitempty"`
}

// HasOpenQuestions reports whether decompose should be blocked.
func (s *ScopeContract) HasOpenQuestions() bool {
	if s == nil {
		return false
	}
	for _, q := range s.OpenQuestions {
		if trimScopeField(q) != "" {
			return true
		}
	}
	return false
}

// IsCompleteEnoughForDecompose returns true when in_scope is set and no blocking questions.
func (s *ScopeContract) IsCompleteEnoughForDecompose() bool {
	if s == nil {
		return false
	}
	if s.HasOpenQuestions() {
		return false
	}
	return len(s.InScope) > 0 || trimScopeField(s.GoalStatement) != ""
}

func trimScopeField(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 {
		last := s[len(s)-1]
		if last != ' ' && last != '\t' && last != '\n' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
