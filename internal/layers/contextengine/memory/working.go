package memory

import (
	"strings"
)

// WorkingMemory holds ephemeral state for a single Process call.
type WorkingMemory struct {
	StreamBuffer strings.Builder
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
