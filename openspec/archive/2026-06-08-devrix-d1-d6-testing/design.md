# Design: D1 & D6 Testing Specifications

**Change ID:** devrix-d1-d6-testing
**Parent Demand:** `demand.md`
**Date:** 2026-06-08

---

## Overview

This document specifies the detailed test cases for D1 Communication and D6 Evolution domains.

---

## D1: Communication Domain Tests

### D1-S3: Commands Module

#### L5-1-3-01: /new Command Parsing

**Test File:** `tests/acceptance/p0/comm_commands_test.go`

**Test Cases:**

| ID | Input | Expected Behavior |
|----|-------|------------------|
| TC-01-01 | `/new` | Command parsed, session terminated, new session created |
| TC-01-02 | `/new arg` | Extra args ignored, same as `/new` |
| TC-01-03 | `/new
` (with newline) | Trim whitespace, parse as `/new` |
| TC-01-04 | `/NEW` (uppercase) | Case-insensitive matching |
| TC-01-05 | `  /new` (leading spaces) | Trim leading spaces |

**Implementation Hint:**
```go
func TestCommand_New_ParsesCorrectly(t *testing.T) {
    cases := []struct {
        name     string
        input    string
        expected CommandType
    }{
        {"/new", CommandNew},
        {"/new arg", CommandNew},
        {"/new
", CommandNew},
        {"/NEW", CommandNew},
        {"  /new", CommandNew},
    }
    // ...
}
```

---

#### L5-1-3-02: /help Command Parsing

**Test File:** `tests/acceptance/p0/comm_commands_test.go`

**Test Cases:**

| ID | Input | Expected Behavior |
|----|-------|------------------|
| TC-02-01 | `/help` | Help text returned, no session interaction |
| TC-02-02 | `/help specific-command` | Help for specific command returned |
| TC-02-03 | `/HELP` | Case-insensitive |
| TC-02-04 | `/help
` | Trim newline |

---

#### L5-1-3-03: /stop Command Parsing

**Test File:** `tests/acceptance/p0/comm_commands_test.go`

**Test Cases:**

| ID | Input | Expected Behavior |
|----|-------|------------------|
| TC-03-01 | `/stop` | LLM call cancelled, partial response preserved |
| TC-03-02 | `/stop
` | Trim newline |
| TC-03-03 | `/STOP` | Case-insensitive |

**Edge Cases:**
- `/stop` during active streaming → Cancel context
- `/stop` when idle → No-op
- `/stop` already called → Idempotent

---

### D1-S1: Gateway Module

#### L5-1-1-01: Session Rejection

**Test File:** `tests/integration/gateway_session_test.go`

**Test Cases:**

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| TC-04-01 | CLI session with invalid requestId | Session rejected with `ErrInvalidRequest` |
| TC-04-02 | CLI session exceeds rate limit | Session rejected with `ErrRateLimited` |
| TC-04-03 | CLI session without auth token | Session rejected with `ErrUnauthorized` |
| TC-04-04 | CLI session with expired token | Session rejected with `ErrTokenExpired` |

**Implementation Hint:**
```go
func TestGateway_RejectCLISession(t *testing.T) {
    tests := []struct {
        name           string
        sessionContext *types.SessionContext
        expectedError  *sharederrors.SentinelError
    }{
        {
            name: "invalid requestId",
            sessionContext: &types.SessionContext{
                RequestID:  "",
                SessionID:  "cli-123",
                SourceType: types.SourceCLI,
            },
            expectedError: sharederrors.ErrInvalidRequest,
        },
        // ...
    }
}
```

---

### D1-S2: Adapters Module

#### L5-1-2-01: Feishu Message Parsing

**Test File:** `internal/layers/communication/adapters/feishu_test.go`

**Test Cases:**

| ID | Payload Type | Expected Behavior |
|----|--------------|------------------|
| TC-05-01 | Text message | `Message` struct correctly extracted |
| TC-05-02 | Image message | Image URL extracted, type set to `Image` |
| TC-05-03 | File message | File metadata extracted |
| TC-05-04 | At-user message | Mentions parsed correctly |
| TC-05-05 | Reply message | Reply-to ID extracted |
| TC-05-06 | Malformed JSON | `ErrInvalidPayload` returned |
| TC-05-07 | Missing required fields | `ErrMissingField` returned |
| TC-05-08 | Unsupported event type | `ErrUnsupportedEventType` returned |

**Test Fixtures:** `tests/fixtures/feishu/`

```
tests/fixtures/feishu/
├── text_message.json
├── image_message.json
├── file_message.json
├── mention_message.json
├── reply_message.json
├── malformed_json.json
└── missing_fields.json
```

---

### D1-S8: Renderers Module

#### L5-1-8-01: ShortId Uniqueness

**Test File:** `internal/shared/types/shortid_test.go`

**Test Cases:**

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| TC-06-01 | Generate 1000 IDs | All unique |
| TC-06-02 | 1000 IDs collision check | Zero collisions |
| TC-06-03 | No异议字符 | Only `[a-z0-9]` |
| TC-06-04 | Length check | Exactly 5 characters |
| TC-06-05 | Excludes confusing chars | No `0/O`, `1/l/I` |

**Implementation Hint:**
```go
func TestShortId_UniqueAndSafe(t *testing.T) {
    const iterations = 1000
    ids := make(map[string]struct{}, iterations)
    
    for i := 0; i < iterations; i++ {
        id := shortid.Generate()
        
        // Uniqueness
        if _, exists := ids[id]; exists {
            t.Fatalf("collision detected: %s", id)
        }
        ids[id] = struct{}{}
        
        // Format
        if len(id) != 5 {
            t.Errorf("expected length 5, got %d", len(id))
        }
        if !shortid.IsValid(id) {
            t.Errorf("invalid characters in: %s", id)
        }
    }
}

func TestShortId_ExcludesConfusingChars(t *testing.T) {
    excluded := []byte{'0', 'O', 'o', '1', 'l', 'I', '|'}
    for i := 0; i < 100; i++ {
        id := shortid.Generate()
        for _, ch := range excluded {
            if bytes.ContainsRune([]byte(id), ch) {
                t.Errorf("confusing char %c in %s", ch, id)
            }
        }
    }
}
```

---

## D6: Evolution Domain Tests

### D6-S1: Version Module

#### L5-6-1-01: Version Detection

**Test File:** `internal/layers/evolution/version/version_test.go`

**Test Cases:**

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| TC-07-01 | Version from git describe | Correct semver string |
| TC-07-02 | Version from ldflags | Build-time version preferred |
| TC-07-03 | Version format check | Matches `v\d+\.\d+\.\d+` |
| TC-07-04 | No version info | Falls back to `dev` |
| TC-07-05 | Dirty git state | Suffix `-dirty` appended |

**Implementation Hint:**
```go
func TestVersion_Detection(t *testing.T) {
    v := version.Get()
    
    // Semver format check
    semverRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`)
    if !semverRegex.MatchString(v.String()) {
        t.Errorf("invalid semver format: %s", v.String())
    }
}

func TestVersion_DirtySuffix(t *testing.T) {
    if v := version.Get(); v.IsDirty() {
        if !strings.HasSuffix(v.String(), "-dirty") {
            t.Errorf("dirty version should end with -dirty: %s", v.String())
        }
    }
}
```

---

### D6-S2: Config Module

#### L5-6-2-01: Config Hot-Reload

**Test File:** `internal/layers/evolution/config/hotreload_test.go`

**⚠️ PREREQUISITE:** `config.LoadAndWatch()` and `config.Changed()` must be implemented before writing these tests. If not yet implemented, defer this test case to a separate change.

**Test Cases:**

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| TC-08-01 | Watch config file | Changes detected within 1s |
| TC-08-02 | Valid config change | New values applied |
| TC-08-03 | Invalid config change | Rollback to previous |
| TC-08-04 | Config file deleted | Graceful degradation |
| TC-08-05 | Nested config update | Deep merge applied |

**Implementation Hint:**
```go
func TestConfig_HotReload_DetectsChange(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "config.yaml")
    
    initial := `timeout: 30s`
    os.WriteFile(configPath, []byte(initial), 0644)
    
    cfg, err := config.LoadAndWatch(configPath)
    if err != nil {
        t.Fatalf("LoadAndWatch: %v", err)
    }
    defer cfg.Stop()
    
    // Modify config
    updated := `timeout: 60s`
    os.WriteFile(configPath, []byte(updated), 0644)
    
    // Wait for detection (max 1s)
    select {
    case <-cfg.Changed():
        if cfg.Timeout() != 60*time.Second {
            t.Errorf("expected 60s, got %v", cfg.Timeout())
        }
    case <-time.After(2 * time.Second):
        t.Fatal("config change not detected within 2s")
    }
}
```

---

## Test File Structure

```
devrix/
tests/
├── acceptance/
│   └── p0/
│       └── comm_commands_test.go      # L5-1-3-01~03
├── integration/
│   └── gateway_session_test.go         # L5-1-1-01
└── fixtures/
    └── feishu/
        ├── text_message.json
        ├── image_message.json
        └── ...

internal/
├── layers/
│   └── communication/
│       └── adapters/
│           └── feishu_test.go          # L5-1-2-01
└── shared/
    └── types/
        └── shortid_test.go             # L5-1-8-01

internal/layers/evolution/
├── version/
│   └── version_test.go                # L5-6-1-01
└── config/
    └── hotreload_test.go              # L5-6-2-01
```

---

## Mock Strategy

| Module | Mock Type | Rationale |
|--------|-----------|-----------|
| Gateway | Interface mock | Avoid real session store |
| Feishu Adapter | HTTP mock | No real Feishu API |
| Config | File-based | Real file system, controlled |

---

## Coverage Goals

| Metric | Target | Measurement |
|--------|--------|-------------|
| D1 Coverage | 100% (5/5) | L5 IDs implemented |
| D6 Coverage | 100% (2/2) | L5 IDs implemented |
| Line Coverage | >70% | `go test -cover` |
| Branch Coverage | >60% | `go test -coverprofile` |

---

## Edge Cases Checklist

### Commands
- [ ] Empty input
- [ ] Whitespace-only input
- [ ] Mixed case command
- [ ] Command with extra whitespace
- [ ] Command in multiline message
- [ ] Unicode in command args

### Gateway
- [ ] Rapid session create/delete
- [ ] Session with special characters in ID
- [ ] Concurrent session access
- [ ] Session store file corruption

### Feishu
- [ ] UTF-8 emoji in message
- [ ] Very long message (>10KB)
- [ ] Binary image data
- [ ] WebSocket reconnection

### ShortId
- [ ] ID generated in parallel
- [ ] ID from multiple goroutines
- [ ] ID storage and retrieval round-trip

### Version
- [ ] No git binary available
- [ ] Git repo not initialized
- [ ] Build without ldflags
- [ ] Version string injection

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial spec |
