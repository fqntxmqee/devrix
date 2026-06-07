package memory

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

// WorkingMemory holds ephemeral state for a single Process call.
type WorkingMemory struct {
	ActiveTools  []string
	StreamBuffer strings.Builder
	CurrentPEV   types.PEVState
}

// NewWorkingMemory creates working memory.
func NewWorkingMemory() *WorkingMemory {
	return &WorkingMemory{}
}

// AppendStream adds streaming text chunk.
func (w *WorkingMemory) AppendStream(chunk string) {
	w.StreamBuffer.WriteString(chunk)
}

// FlushStream returns accumulated assistant text and clears buffer.
func (w *WorkingMemory) FlushStream() string {
	s := w.StreamBuffer.String()
	w.StreamBuffer.Reset()
	return s
}
