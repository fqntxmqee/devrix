package evaluate

import "context"

func init() {
	RegisterProbe(&ToolAccuracyProbe{})
}

// ToolAccuracyProbe evaluates tool-selection precision/recall/F1 (deterministic, no Judge).
type ToolAccuracyProbe struct{}

func (p *ToolAccuracyProbe) ID() string {
	return "tool_accuracy"
}

func (p *ToolAccuracyProbe) Run(_ context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	expected := stringSliceFromInput(item.Input, "expected_tools")
	actual := stringSliceFromInput(item.Input, "actual_tools")

	precision, recall, f1 := toolSelectionMetrics(expected, actual)

	buckets := map[string]float64{}
	if item.Bucket != "" {
		buckets[item.Bucket] = f1
	}

	return &DomainScore{
		Domain:     "d2",
		Dimension:  p.ID(),
		Score:      f1,
		Confidence: 1.0,
		Buckets:    buckets,
		Details: map[string]float64{
			"precision": precision,
			"recall":    recall,
			"f1":        f1,
		},
	}, nil
}

func toolSelectionMetrics(expected, actual []string) (precision, recall, f1 float64) {
	if len(expected) == 0 && len(actual) == 0 {
		return 1, 1, 1
	}
	if len(expected) == 0 {
		return 0, 1, 0
	}
	if len(actual) == 0 {
		return 1, 0, 0
	}

	expSet := make(map[string]int, len(expected))
	for _, t := range expected {
		expSet[t]++
	}
	actSet := make(map[string]int, len(actual))
	for _, t := range actual {
		actSet[t]++
	}

	tp := 0
	for name, expCount := range expSet {
		actCount := actSet[name]
		if actCount < expCount {
			tp += actCount
		} else {
			tp += expCount
		}
	}

	precision = float64(tp) / float64(len(actual))
	recall = float64(tp) / float64(len(expected))
	f1 = harmonicMean(precision, recall)
	return precision, recall, f1
}

func harmonicMean(a, b float64) float64 {
	if a+b == 0 {
		return 0
	}
	return 2 * a * b / (a + b)
}
