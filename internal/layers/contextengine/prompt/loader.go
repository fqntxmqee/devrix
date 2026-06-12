package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/shared/config"
)

// Cache manages cached section content.
type Cache struct {
	mu      sync.RWMutex
	content map[string]string
}

var (
	globalCache     *Cache
	globalCacheOnce sync.Once
)

// GetCache returns the global cache.
func GetCache() *Cache {
	globalCacheOnce.Do(func() {
		globalCache = &Cache{
			content: make(map[string]string),
		}
	})
	return globalCache
}

// Get returns cached content for a section.
func (c *Cache) Get(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	content, ok := c.content[name]
	return content, ok
}

// Set caches content for a section.
func (c *Cache) Set(name, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.content[name] = content
}

// Loader loads system prompts from configured sources.
type Loader struct {
	cfg       *config.SystemPromptConfig
	cache     *Cache
	staticMap map[string]string
}

// NewLoader creates a prompt loader.
func NewLoader(cfg *config.SystemPromptConfig) *Loader {
	if cfg == nil {
		cfg = &config.SystemPromptConfig{
			Sources:  []string{"AGENTS.md", ".devrix/AGENTS.md"},
			Fallback: "You are Devrix, a multi-agent development assistant.",
		}
	}

	loader := &Loader{
		cfg:       cfg,
		cache:     GetCache(),
		staticMap: make(map[string]string),
	}

	// Register static sections
	loader.staticMap = map[string]string{
		"intro":               sectionIntro,
		"system":              sectionSystem,
		"doing_tasks":         sectionDoingTasks,
		"actions":             sectionActions,
		"using_tools":         sectionUsingTools,
		"output_efficiency":   sectionOutputEfficiency,
		"tone_and_style":      sectionToneAndStyle,
		"safety_guidelines":   sectionSafetyGuidelines,
		"knowledge_boundaries": sectionKnowledgeBoundaries,
		"todo_write":          sectionTodoWrite,
		"glob":                sectionGlob,
		"grep":                sectionGrep,
		"edit_file":           sectionEditFile,
	}

	// Pre-populate cache with static content
	for name, content := range loader.staticMap {
		loader.cache.Set(name, content)
	}

	return loader
}

// LoadAsSections loads prompt and returns it as a list of sections.
func (l *Loader) LoadAsSections(workDir string) []string {
	sections := make([]string, 0, len(l.staticMap))

	for _, name := range []string{
		"intro", "system", "doing_tasks", "actions",
		"using_tools", "output_efficiency", "tone_and_style",
		"safety_guidelines", "knowledge_boundaries",
		"todo_write", "glob", "grep", "edit_file",
	} {
		if content, ok := l.cache.Get(name); ok {
			sections = append(sections, content)
		}
	}

	return sections
}

// LoadCustom loads custom prompt from workdir.
func (l *Loader) LoadCustom(workDir string) string {
	for _, src := range l.cfg.Sources {
		path := src
		if !filepath.IsAbs(src) && workDir != "" {
			path = filepath.Join(workDir, src)
		}
		data, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// IsCustomPromptAvailable checks if a custom prompt exists.
func (l *Loader) IsCustomPromptAvailable(workDir string) bool {
	for _, src := range l.cfg.Sources {
		path := src
		if !filepath.IsAbs(src) && workDir != "" {
			path = filepath.Join(workDir, src)
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// ClearCache clears the prompt cache.
func (l *Loader) ClearCache() {
	// Re-populate static content
	for name, content := range l.staticMap {
		l.cache.Set(name, content)
	}
}

// GetCacheStats returns cache statistics.
func (l *Loader) GetCacheStats() map[string]bool {
	stats := make(map[string]bool)
	for name := range l.staticMap {
		_, ok := l.cache.Get(name)
		stats[name] = ok
	}
	return stats
}

// Static section contents
const (
	DynamicBoundary = "<!-- DYNAMIC_CONTENT_BOUNDARY -->"

	sectionIntro = `You are an interactive agent that helps users with software engineering tasks.
Use the instructions below and the tools available to you to assist the user.

IMPORTANT: You must NEVER generate or guess URLs for the user unless you are
confident that the URLs are for helping the user with programming.`

	sectionSystem = `# System

- All text you output outside of tool use is displayed to the user.
- Tools are executed in a user-selected permission mode.
- Tool results may include <system-reminder> or other tags.
- The system will automatically compress prior messages as context limits approach.`

	sectionDoingTasks = `# Doing tasks

- Don't add features, refactor code, or make "improvements" beyond what was asked.
- Don't add error handling for scenarios that can't happen.
- Don't create helpers, utilities, or abstractions for one-time operations.
- Default to writing no comments. Only add when the WHY is non-obvious.
- Before reporting task complete, verify it actually works.`

	sectionActions = `# Executing actions with care

Carefully consider the reversibility and blast radius of actions.

Examples of risky actions that warrant user confirmation:
- Destructive: deleting files/branches, rm -rf, dropping tables
- Hard-to-reverse: force-push, git reset --hard
- Shared state: pushing code, PRs, sending messages

When in doubt, ask before acting.`

	sectionUsingTools = `# Using your tools

- Use dedicated read tool instead of cat, head, tail
- Use dedicated edit tool instead of sed or awk
- Use dedicated glob tool instead of find
- Call multiple independent tools in parallel when possible.

CRITICAL: Do NOT use bash when a relevant dedicated tool is provided.`

	sectionOutputEfficiency = `# Output efficiency

IMPORTANT: Go straight to the point. Try the simplest approach first.

Keep your text output brief and direct:
- Lead with the answer or action, not the reasoning
- Skip filler words and unnecessary transitions

If you can say it in one sentence, don't use three.`

	sectionToneAndStyle = `# Tone and style

- Only use emojis if the user explicitly requests it.
- Your responses should be short and concise.
- Include file_path:line_number for code references.
- Be precise and factual.`

	sectionSafetyGuidelines = `# Safety Guidelines

## Code Security
- NEVER hardcode secrets, API keys, passwords, or tokens in source code
- NEVER commit .env files, credentials.json, or any sensitive configuration
- ALWAYS validate user input at system boundaries (API endpoints, file uploads, CLI args)
- ALWAYS use parameterized queries for database operations — never string concatenation
- When modifying authentication/authorization code, flag it for security review

## Output Safety
- Do not reproduce copyrighted code verbatim — always paraphrase or derive
- Do not generate code that appears to be malware, exploits, or malicious tools
- If a request involves security-sensitive code, add appropriate warnings

## Dependency Safety
- Prefer well-maintained libraries over hand-rolled solutions
- Flag dependencies with known security issues when encountered`

	sectionKnowledgeBoundaries = `# Knowledge Boundaries

## What to Verify
- When referencing a library or framework API, prefer checking project's own usage patterns first
- For version-specific behavior, check go.mod or package.json rather than assuming
- When a user reports unexpected behavior, investigate rather than assuming the code is correct

## What to Assume
- The project's existing conventions and patterns are intentional — follow them
- Tests that exist are correct unless evidence shows otherwise
- Configuration files reflect the intended setup`

	sectionTodoWrite = `## Todo List Management (todo_write)

Use the todo_write tool to create and manage a structured task list. This helps track progress, organize complex tasks, and ensure thoroughness.

### When to Use
1. Complex multi-step tasks (3+ distinct steps)
2. Non-trivial tasks requiring careful planning
3. User explicitly requests a todo list
4. User provides multiple tasks (numbered or comma-separated)
5. After receiving new instructions — immediately capture requirements as todos
6. When starting a task — mark in_progress BEFORE beginning work (only one at a time)
7. After completing a task — mark completed and add any follow-up tasks discovered

### When NOT to Use
1. Single, straightforward task
2. Task is trivial with no organizational benefit
3. Task can be completed in less than 3 trivial steps
4. Task is purely conversational or informational

### Task States and Management
- States: pending (not started), in_progress (currently working), completed (finished successfully)
- Each task must have: content (imperative form, e.g. "Run tests") and activeForm (present continuous, e.g. "Running tests")
- Update status in real-time as you work; mark complete IMMEDIATELY after finishing
- Exactly ONE task at a time should be in_progress
- Remove tasks that are no longer relevant
- ONLY mark completed when fully accomplished; if blocked, create a task describing what to resolve
- Never mark completed if tests are failing, implementation is partial, or errors are unresolved`

	sectionGlob = `## Glob Tool

Use the glob tool to find files by name pattern or wildcard. Supports patterns like "**/*.js" or "src/**/*.ts". Returns matching file paths sorted by modification time. Use this tool when you need to find files by name patterns.`

	sectionGrep = `## Grep Tool

Use the grep tool to search file contents using regular expressions. Supports three output modes: content (matching lines with line numbers), files_with_matches (file paths only), and count (match counts per file). Supports case insensitive search with -i, context lines with -C, and pagination with head_limit and offset. Always use Grep for search tasks — never use bash grep or rg directly.`

	sectionEditFile = `## Edit Tool (edit_file)

Use edit_file to modify files by replacing exact text. The tool finds old_string in the file and replaces it with new_string. Always read the file before editing it. Ensure old_string is unique in the file (or use replace_all=true). Preserve exact indentation (tabs/spaces) when matching text. If old_string is not found, check for differences in whitespace or smart quotes. Prefer edit_file over write_file for targeted changes.`
)

// Load resolves system prompt for a work directory.
func (l *Loader) Load(workDir string) string {
	return l.LoadCustom(workDir)
}

// LoadWithDynamic loads static sections plus dynamic ones.
func (l *Loader) LoadWithDynamic(workDir string, dynamicSections []string) []string {
	sections := l.LoadAsSections(workDir)

	// Add dynamic boundary marker if there are dynamic sections
	if len(dynamicSections) > 0 {
		sections = append(sections, DynamicBoundary)
		sections = append(sections, dynamicSections...)
	}

	return sections
}
