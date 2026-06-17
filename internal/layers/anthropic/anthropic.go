// Package anthropic is a compile-only stub for v1.0 of
// devrix-surface-lazy-loading. The full Anthropic API client (tool
// search tool, prompt caching, native tool_use streaming) lands in v1.1.
//
// DSAFT: TOOL-SURFACE-1-A02 v1.1 follow-up (DM-20260618-003).
//
// v1.1 items (placeholder, do not implement before S3-Gate):
//   - Client: HTTP client for messages endpoint
//   - ListTools: query the tool catalog the API knows about
//   - ToolUse: parse streamed tool_use blocks from messages stream
//   - ToolSearch: native `tool_search_tool_20251119` wire-up
package anthropic

import "errors"

// ErrNotImplemented is returned by every stub function. v1.1 will replace
// these with real implementations.
var ErrNotImplemented = errors.New("anthropic: not implemented in v1.0; planned for v1.1 (DM-20260618-003)")

// Client is a placeholder for the real HTTP client.
type Client struct{}

// NewClient returns a placeholder client. Calling any method on it
// returns ErrNotImplemented.
func NewClient(_ string) *Client { return &Client{} }

// ListTools returns ErrNotImplemented. v1.1 will hit the Anthropic API.
func (c *Client) ListTools() ([]string, error) {
	return nil, ErrNotImplemented
}

// ToolSearch returns ErrNotImplemented. v1.1 will dispatch to the
// native tool_search_tool_20251119 tool.
func (c *Client) ToolSearch(_ string) ([]string, error) {
	return nil, ErrNotImplemented
}
