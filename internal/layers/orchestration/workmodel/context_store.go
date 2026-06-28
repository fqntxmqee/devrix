package workmodel

import "sync"

type contextSessionData struct {
	mu         sync.RWMutex
	links      []ContextLinkRecord
	scopesByWI map[string]*ContextScope
	cohorts    map[string]*CohortScope
	downlinks  map[string]ChildDownlink // childWorkItemID -> downlink
	peerStatus map[string][]PeerStatusSignal // cohortKey -> signals
}

// CohortScope holds shared metadata for sibling work items under one parent.
//
// CG2′ (ADR-001): siblings share cohort domain signals (ScopeContract, PeerStatus)
// but NOT WorkItemPrivate transcript chains — transcript isolation is per wi:<sid>:<wi_id>.
type CohortScope struct {
	SessionID      string
	ParentWorkItem string
	ScopeContract  *ScopeContract
}

// DefaultCohortSignalBudgetMax is the soft cap for cohort signal metadata (OQ-LC-9).
const DefaultCohortSignalBudgetMax = 8 * 1024

// contextSessions holds per-session ContextGraph state (F4/F5).
var contextSessions sync.Map // sessionID -> *contextSessionData

func (m *TaskManager) contextData(sessionID string) *contextSessionData {
	if m == nil {
		return nil
	}
	v, _ := contextSessions.LoadOrStore(sessionID, &contextSessionData{
		scopesByWI: make(map[string]*ContextScope),
		cohorts:    make(map[string]*CohortScope),
		downlinks:  make(map[string]ChildDownlink),
		peerStatus: make(map[string][]PeerStatusSignal),
	})
	return v.(*contextSessionData)
}

// ResetContextGraphState clears in-memory link/scope state (tests).
func ResetContextGraphState() {
	contextSessions = sync.Map{}
}

// ListContextLinks returns all link records for a session.
func (m *TaskManager) ListContextLinks(sessionID string) []ContextLinkRecord {
	sd := m.contextData(sessionID)
	if sd == nil {
		return nil
	}
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	out := make([]ContextLinkRecord, len(sd.links))
	copy(out, sd.links)
	return out
}

// LinksForWorkItem returns inbound and outbound links touching a work item.
func (m *TaskManager) LinksForWorkItem(sessionID, workItemID string) []ContextLinkRecord {
	all := m.ListContextLinks(sessionID)
	var out []ContextLinkRecord
	for _, l := range all {
		if l.FromWorkItemID == workItemID || l.ToWorkItemID == workItemID {
			out = append(out, l)
		}
	}
	return out
}

func (m *TaskManager) appendContextLink(sessionID string, rec ContextLinkRecord) {
	sd := m.contextData(sessionID)
	if sd == nil {
		return
	}
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.links = append(sd.links, rec)
}

// EnsureContextScope binds a non-ephemeral WorkItem to a ContextScope (F5).
func (m *TaskManager) EnsureContextScope(sessionID string, item *WorkItem) *ContextScope {
	if m == nil || item == nil || item.Ephemeral || item.Kind == WorkKindChecklist {
		return nil
	}
	sd := m.contextData(sessionID)
	if sd == nil {
		return nil
	}
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if scope, ok := sd.scopesByWI[item.ID]; ok {
		return scope
	}
	scope := NewContextScope(sessionID, item.ID, DefaultPersistScopeForKind(item.Kind))
	sd.scopesByWI[item.ID] = scope
	_ = m.tree.SetContextScopeID(sessionID, item.ID, scope.ID)
	item.ContextScopeID = scope.ID
	return scope
}

// ContextScopeForWorkItem returns the bound scope if present.
func (m *TaskManager) ContextScopeForWorkItem(sessionID, workItemID string) (*ContextScope, bool) {
	sd := m.contextData(sessionID)
	if sd == nil {
		return nil, false
	}
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	s, ok := sd.scopesByWI[workItemID]
	return s, ok
}

func linkRecordExists(existing []ContextLinkRecord, rec ContextLinkRecord) bool {
	for _, e := range existing {
		if e.FromWorkItemID == rec.FromWorkItemID && e.ToWorkItemID == rec.ToWorkItemID && e.Kind == rec.Kind {
			return true
		}
	}
	return false
}

// EnsureCohortScope registers cohort domain for siblings under parentWorkItemID.
func (m *TaskManager) EnsureCohortScope(sessionID, parentWorkItemID string) *CohortScope {
	if m == nil || parentWorkItemID == "" {
		return nil
	}
	sd := m.contextData(sessionID)
	if sd == nil {
		return nil
	}
	key := cohortKey(sessionID, parentWorkItemID)
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if c, ok := sd.cohorts[key]; ok {
		return c
	}
	c := &CohortScope{SessionID: sessionID, ParentWorkItem: parentWorkItemID}
	if parent, ok := m.GetWorkItem(sessionID, parentWorkItemID); ok && parent != nil && parent.ScopeContract != nil {
		sc := *parent.ScopeContract
		c.ScopeContract = &sc
	}
	sd.cohorts[key] = c
	return c
}

func cohortKey(sessionID, parentID string) string {
	return sessionID + ":" + parentID
}

func (m *TaskManager) storeChildDownlink(sessionID string, dl ChildDownlink) {
	sd := m.contextData(sessionID)
	if sd == nil || dl.ChildWorkItemID == "" {
		return
	}
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.downlinks[dl.ChildWorkItemID] = dl
}

// ChildDownlinkFor returns the persisted downlink for a child work item.
func (m *TaskManager) ChildDownlinkFor(sessionID, childWorkItemID string) (ChildDownlink, bool) {
	sd := m.contextData(sessionID)
	if sd == nil {
		return ChildDownlink{}, false
	}
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	dl, ok := sd.downlinks[childWorkItemID]
	return dl, ok
}
