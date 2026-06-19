package orchtypes

// IntentKind is the routing decision produced by ClassifyIntent.
type IntentKind string

const (
	IntentFast        IntentKind = "fast"
	IntentCommand     IntentKind = "command"
	IntentOrchestrate IntentKind = "orchestrate"
	IntentSkip        IntentKind = "skip"
)

// IntentClassification is the result of ClassifyIntent.
type IntentClassification struct {
	Kind       IntentKind
	Confidence int
	Reason     string
	Command    string
}
