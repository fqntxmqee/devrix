# Design: MUPS semantics schema alignment

**Change ID:** `mups-semantics-schema-alignment`  
**Demand ID:** DM-20260705-003  
**Status:** S4_Development

---

## 1. Types

```go
type SemanticCondition string  // machine codes: scope_unclear, task_directive, ...

type SemanticRule struct {
    Target   string
    Plane    PromptPlane
    WhenUse  SemanticCondition
    WhenNot  SemanticCondition
    Enforced bool
    Gate     string
}

func (r SemanticRule) MachineLine() string  // compact JSON, omitempty enforced/gate
func SemanticBlock(phase MUPSPhase) string  // JSON-lines of output rules
func InputRulesForFrame(frame FrameName) []SemanticRule  // derived from LineFrameRegistry
```

## 2. i18n overlay

| Map | Keys | Purpose |
|-----|------|---------|
| `semanticGlossary{ZH,EN}` | `SemanticCondition` | Condition code → human label |
| `semanticNodeRole{ZH,EN}` | node role keys only | One-line node role |
| `semanticPlaneGuide{ZH,EN}` | const | control/data plane intro |

`RenderSemanticAppendix` composes:

1. Node role (prose)
2. `Semantic rules (machine-readable):` header
3. `SemanticBlock(phase)` JSON-lines
4. Condition glossary (codes referenced by output rules)

## 3. Input semantics derivation

`observeInputSemantics` / `planInputSemantics` maps keyed by `TagName` hold `WhenUse` condition only. `InputRulesForFrame` walks `LineFrameRegistry[frame].Fields` in order, attaches `Target` + `Plane` from `FrameFieldPlane`.

## 4. Invariants

1. Output rules in `semantics.go` remain locale-neutral
2. Glossary must cover every `SemanticCondition` referenced by rules used in prompts
3. `Enforced: true` rules must name existing Go gate (unchanged from v3)
4. `RegisterFrameFieldGuide` init registers all `InputRulesForFrame` tags

## 5. File layout

```
internal/shared/prompttags/
  semantic_condition.go   # SemanticCondition constants
  semantic_rule.go        # SemanticRule, InputRulesForFrame
  semantic_block.go       # SemanticBlock, SemanticConditionsForPhase
  prompt_plane.go         # PromptPlane enum
  semantics.go            # PhaseSemantics output rules only

internal/layers/contextengine/i18n/
  prompttags_semantics_{zh,en}.go   # glossary + node role only
  prompttags_semantics_render.go    # RenderSemanticAppendix
  prompttags_semantics_init.go      # RegisterFrameFieldGuide from InputRulesForFrame
```
