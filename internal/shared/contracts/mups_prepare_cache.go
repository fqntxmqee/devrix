package contracts

import "context"

type mupsPrepareCacheKey struct{}

// MUPSPrepareBaseCache holds one PrepareBase result per MUPS pipeline round so
// Observe and Plan can share the same devrix_core system prompt without
// re-running D2 Prepare.
type MUPSPrepareBaseCache struct {
	SessionID    string
	UserMessage  string
	SystemPrompt string
	Prepend      map[string]string
}

// WithMUPSPrepareCache attaches a round-scoped PrepareBase cache to ctx.
func WithMUPSPrepareCache(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, mupsPrepareCacheKey{}, &MUPSPrepareBaseCache{})
}

// MUPSPrepareCacheFrom returns the round cache when present.
func MUPSPrepareCacheFrom(ctx context.Context) (*MUPSPrepareBaseCache, bool) {
	if ctx == nil {
		return nil, false
	}
	c, ok := ctx.Value(mupsPrepareCacheKey{}).(*MUPSPrepareBaseCache)
	return c, ok && c != nil
}

// TryMUPSPrepareBase returns a cached PrepareBase result for sessionID+message.
func TryMUPSPrepareBase(ctx context.Context, sessionID, message string) (systemPrompt string, prepend map[string]string, ok bool) {
	c, found := MUPSPrepareCacheFrom(ctx)
	if !found || c == nil || c.SessionID != sessionID || c.UserMessage != message || c.SystemPrompt == "" {
		return "", nil, false
	}
	if c.Prepend != nil {
		prepend = make(map[string]string, len(c.Prepend))
		for k, v := range c.Prepend {
			prepend[k] = v
		}
	}
	return c.SystemPrompt, prepend, true
}

// StoreMUPSPrepareBase saves a PrepareBase result into the round cache.
func StoreMUPSPrepareBase(ctx context.Context, sessionID, message, systemPrompt string, prepend map[string]string) {
	c, found := MUPSPrepareCacheFrom(ctx)
	if !found || c == nil || sessionID == "" || message == "" || systemPrompt == "" {
		return
	}
	c.SessionID = sessionID
	c.UserMessage = message
	c.SystemPrompt = systemPrompt
	if len(prepend) == 0 {
		c.Prepend = nil
		return
	}
	c.Prepend = make(map[string]string, len(prepend))
	for k, v := range prepend {
		c.Prepend[k] = v
	}
}
