package prompt

import (
	"sync"
)

// dynamicSectionCache holds session-scoped dynamic section content (ClawCode
// systemPromptSections registry equivalent). Cleared on session reset or /clear.
type dynamicSectionCache struct {
	mu       sync.RWMutex
	sessions map[string]map[string]string
}

var globalDynamicSectionCache = &dynamicSectionCache{
	sessions: make(map[string]map[string]string),
}

func (c *dynamicSectionCache) get(sessionID, name string) (string, bool) {
	if sessionID == "" {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if sess, ok := c.sessions[sessionID]; ok {
		v, ok := sess[name]
		return v, ok
	}
	return "", false
}

func (c *dynamicSectionCache) set(sessionID, name, value string) {
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessions[sessionID] == nil {
		c.sessions[sessionID] = make(map[string]string)
	}
	c.sessions[sessionID][name] = value
}

// ClearDynamicSectionCache drops cached dynamic sections for one session.
func ClearDynamicSectionCache(sessionID string) {
	if sessionID == "" {
		return
	}
	globalDynamicSectionCache.mu.Lock()
	defer globalDynamicSectionCache.mu.Unlock()
	delete(globalDynamicSectionCache.sessions, sessionID)
}

// ClearAllDynamicSectionCache drops all session dynamic section caches.
func ClearAllDynamicSectionCache() {
	globalDynamicSectionCache.mu.Lock()
	defer globalDynamicSectionCache.mu.Unlock()
	globalDynamicSectionCache.sessions = make(map[string]map[string]string)
}

func resolveCachedSection(sessionID, name string, cacheBreak bool, compute func() string) string {
	if cacheBreak || sessionID == "" {
		return compute()
	}
	if v, ok := globalDynamicSectionCache.get(sessionID, name); ok {
		return v
	}
	v := compute()
	globalDynamicSectionCache.set(sessionID, name, v)
	return v
}
