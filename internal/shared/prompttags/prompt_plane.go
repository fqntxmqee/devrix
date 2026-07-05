package prompttags

// PromptPlane distinguishes data payload from orchestration control fields in user frames.
type PromptPlane string

const (
	PlaneData    PromptPlane = "data"
	PlaneControl PromptPlane = "control"
)
