package workmodel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/shared/textutil"
)

const defaultSessionBaseDir = "~/.devrix/sessions"

// PartitionKey returns the WorkItemPrivate partition identifier.
func PartitionKey(sessionID, workItemID string) string {
	return fmt.Sprintf("wi:%s:%s", sessionID, workItemID)
}

// CohortPartitionKey returns the LayerCohort partition identifier.
func CohortPartitionKey(sessionID, parentWorkItemID string) string {
	return fmt.Sprintf("cohort:%s:%s", sessionID, parentWorkItemID)
}

// TruncateArtifactSummary caps upstream artifact text for Materialize inject.
func TruncateArtifactSummary(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// PrivateChainMessageCount reads the wi private jsonl line count (best-effort).
func PrivateChainMessageCount(sessionID, workItemID, baseDir string) int {
	if sessionID == "" || workItemID == "" {
		return 0
	}
	dir := baseDir
	if dir == "" {
		dir = defaultSessionBaseDir
	}
	path := filepath.Join(textutil.ExpandPath(dir), sessionID, "wi", workItemID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) > 0 {
			n++
		}
	}
	return n
}

// MaterializeContextResolveHint extends ResolveHint with partition / upstream / cohort lines (F6).
func MaterializeContextResolveHint(sessionID string, tm *TaskManager, focus *WorkItem) string {
	if tm == nil || focus == nil {
		return ""
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("Resolve: partition=%s depth=%d.",
		PartitionKey(sessionID, focus.ID), tm.Tree().Depth(sessionID, focus.ID)))
	if focus.ParentID != "" {
		parts = append(parts, fmt.Sprintf("Resolve: cohort domain=%s.",
			CohortPartitionKey(sessionID, focus.ParentID)))
	}
	if len(focus.BlockedBy) > 0 {
		parts = append(parts, fmt.Sprintf("Resolve: upstream inject from %s (structured bubble, no private chain).",
			strings.Join(focus.BlockedBy, ", ")))
	}
	if n := PrivateChainMessageCount(sessionID, focus.ID, ""); n > 0 {
		parts = append(parts, fmt.Sprintf("Resolve: private chain len=%d.", n))
	}
	if dl, ok := tm.ChildDownlinkFor(sessionID, focus.ID); ok {
		if dl.ExpectedReturn != "" {
			parts = append(parts, fmt.Sprintf("Resolve: expected_return=%q.", dl.ExpectedReturn))
		}
	}
	return strings.Join(parts, " ")
}

// FormatMaterializeContextShow renders partition + inbound signals for /task context show.
func FormatMaterializeContextShow(sessionID string, tm *TaskManager, item *WorkItem) string {
	if item == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  partition: %s\n", PartitionKey(sessionID, item.ID))
	fmt.Fprintf(&b, "  depth: %d\n", tm.Tree().Depth(sessionID, item.ID))
	if item.ParentID != "" {
		fmt.Fprintf(&b, "  cohort: %s\n", CohortPartitionKey(sessionID, item.ParentID))
		if c := tm.EnsureCohortScope(sessionID, item.ParentID); c != nil && c.ScopeContract != nil {
			fmt.Fprintf(&b, "  cohort_scope_in: %s\n", strings.Join(c.ScopeContract.InScope, ", "))
		}
	}
	if n := PrivateChainMessageCount(sessionID, item.ID, ""); n > 0 {
		fmt.Fprintf(&b, "  private_chain_messages: %d\n", n)
	}
	if len(item.BlockedBy) > 0 {
		fmt.Fprintf(&b, "  blocked_by: %s\n", strings.Join(item.BlockedBy, ", "))
		for _, id := range item.BlockedBy {
			up, ok := tm.GetWorkItem(sessionID, id)
			if !ok || up == nil || up.LastRound == nil {
				continue
			}
			fmt.Fprintf(&b, "  upstream_signal: %s\n", StructuredBubbleStatement(id, up.LastRound))
		}
	}
	if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok {
		if dl.Directive != "" {
			fmt.Fprintf(&b, "  downlink_directive: %s\n", dl.Directive)
		}
		if len(dl.ScopeIn) > 0 {
			fmt.Fprintf(&b, "  downlink_scope_in: %s\n", strings.Join(dl.ScopeIn, ", "))
		}
		if dl.ExpectedReturn != "" {
			fmt.Fprintf(&b, "  downlink_expected_return: %s\n", dl.ExpectedReturn)
		}
	}
	if item.ScopeContract != nil {
		if len(item.ScopeContract.InScope) > 0 {
			fmt.Fprintf(&b, "  scope_in: %s\n", strings.Join(item.ScopeContract.InScope, ", "))
		}
		if item.ScopeContract.HasOpenQuestions() {
			fmt.Fprintf(&b, "  open_questions: %s\n", strings.Join(item.ScopeContract.OpenQuestions, "; "))
		}
	}
	return b.String()
}
