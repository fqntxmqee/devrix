package prompt

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
)

// enableDynamicBoundary reports whether the new dynamic-boundary prompt
// structure (§十 spec, post-2026-Q1 layout) should be used instead of the
// legacy static 4-layer concatenation.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from assembler.go
// Build (was 55 LOC god function) into dynamic_build.go.
func (a *SystemPromptAssembler) enableDynamicBoundary() bool {
	return a.cfg.PromptConfig != nil && a.cfg.PromptConfig.EnableDynamicBoundary
}

// wantsDynamicSection returns true if the given section name is configured to
// be emitted as a dynamic boundary section in the new prompt structure.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) wantsDynamicSection(name string) bool {
	if a.cfg.PromptConfig == nil {
		return false
	}
	for _, n := range a.cfg.PromptConfig.DynamicSections {
		if n == name {
			return true
		}
	}
	return false
}

// buildDynamicSections emits the dynamic boundary sections in declaration
// order, honoring per-section cacheability. session_context and git_status
// go through resolveCachedSection (keyed by sessionID), task_notifications is
// live (re-drained every Build), and env_info / loaded_context are computed
// inline.
//
// DM-20260629-002 PR-2: extracted from assembler.go Build's section tail.
func (a *SystemPromptAssembler) buildDynamicSections(sessionID string, in SystemPromptBuildInput, layer3, taskNotif string) ([]string, []string) {
	var parts []string
	var names []string

	sessionCtx := resolveCachedSection(sessionID, "session_context", false, func() string {
		return a.buildSessionContext(in)
	})
	if strings.TrimSpace(sessionCtx) != "" {
		parts = append(parts, sessionCtx)
		names = append(names, "session_context")
	}

	// S4-Gate H-2 fix: task_notifications is a live section (not cacheable),
	// so it gets the latest bus events on every Build. The drain happens at
	// the top of Build, here we only splice / skip empty content.
	if strings.TrimSpace(taskNotif) != "" {
		parts = append(parts, taskNotif)
		names = append(names, "task_notifications")
	}

	if a.wantsDynamicSection("git_status") {
		workDir := in.WorkDir
		if in.Session != nil && in.Session.WorkDir != "" {
			workDir = in.Session.WorkDir
		}
		if gitCtx, ok := resolveGitStatusSection(sessionID, workDir, a.locale); ok {
			parts = append(parts, gitCtx)
			names = append(names, "git_status")
		}
	}

	if a.wantsDynamicSection("env_info") {
		if env := a.buildEnvInfo(in); env != "" {
			parts = append(parts, env)
			names = append(names, "env_info")
		}
	}

	if strings.TrimSpace(layer3) != "" {
		parts = append(parts, layer3)
		names = append(names, "loaded_context")
	}

	return parts, names
}

// resolveGitStatusSection wraps computeGitStatus with per-session caching so
// repeated Build calls within the same session don't re-run `git status`.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func resolveGitStatusSection(sessionID, workDir string, loc i18n.Locale) (string, bool) {
	raw := resolveCachedSection(sessionID, "git_status", false, func() string {
		s, ok := computeGitStatus(workDir, loc)
		if !ok {
			return ""
		}
		return s
	})
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	return raw, true
}

// buildEnvInfo formats the optional "environment" line for the system prompt
// (workdir + model). Returns "" when neither field is set.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) buildEnvInfo(in SystemPromptBuildInput) string {
	workDir := in.WorkDir
	model := ""
	if in.Session != nil {
		if in.Session.WorkDir != "" {
			workDir = in.Session.WorkDir
		}
		model = in.Session.Model
	}
	if workDir == "" && model == "" {
		return ""
	}
	return i18n.EnvInfoHeader(a.locale) + "\n" + i18n.EnvInfoBody(a.locale, workDir, model)
}

// coreTemplate returns the active core template (zh/en) based on locale.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) coreTemplate() string {
	if a.locale == i18n.LocaleEN {
		return a.coreTemplateEN
	}
	return a.coreTemplateZH
}

// guidanceTemplate returns the active guidance template (zh/en) based on locale.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) guidanceTemplate() string {
	if a.locale == i18n.LocaleEN {
		return a.guidanceTemplateEN
	}
	return a.guidanceTemplateZH
}