# Context Engine Performance Specification

**Change ID:** devrix-context-engine-v4
**Parent Spec:** `openspec/archive/2026-06-07-devrix-context-engine-v3/specs/context-engine/spec.md`
**Status:** Draft
**Version:** 4.0.0

---

## 1. Async Autocompact

```gherkin
Feature: Async Autocompact

  Scenario: Autocompact returns placeholder immediately
    Given a conversation with 20 turns exceeding compression target
    When the compression pipeline reaches step 6 (autocompact)
    Then a placeholder summary message is returned within 50ms
    And the PEV loop continues without blocking

  Scenario: Autocompact generates full summary asynchronously
    Given an async autocompact was triggered
    When the LLM summary completes
    Then IAutocompactObserver.OnAutocompactComplete is called
    And the completed summary replaces the placeholder

  Scenario: Async summary failure degrades gracefully
    Given an async autocompact is in progress
    When the LLM call fails or times out
    Then the placeholder message remains in the conversation
    And the observer is notified of degradation

  Scenario: Multiple async triggers cancel previous
    Given an async autocompact is in progress with token "t1"
    When a new autocompact is triggered with token "t2"
    Then the "t1" goroutine is cancelled
    And only the "t2" summary is written

  Scenario: Async autocompact respects Shutdown
    Given an async autocompact is in progress
    When Shutdown is called with a 2s timeout
    Then all pending goroutines are cancelled
    And Shutdown returns within the timeout

  Scenario: Stale async result is discarded
    Given session "s1" triggers async autocompact with token "t1"
    And another autocompact is triggered for same session with token "t2"
    When the "t1" goroutine eventually completes
    Then the "t1" summary is NOT written (stale)
    And only the "t2" summary is delivered

  Scenario: Sessions do not interfere
    Given session "s1" triggers async autocompact with token "t1"
    And session "s2" triggers async autocompact with token "t2"
    When the "t1" goroutine for session "s1" completes
    Then its summary is written to session "s1"
    And session "s2" is unaffected

  Scenario: Async completion after session end is safe
    Given session "s1" triggers async autocompact with token "t1"
    And the session ends before the goroutine completes
    When the goroutine finishes and calls OnAutocompactComplete
    Then no panic or goroutine leak occurs
```

---

## 2. Snappy Snapshot Compression

```gherkin
Feature: Snappy Snapshot Compression

  Scenario: Snapshot is compressed when compression enabled
    Given snapshot compression is enabled
    When a session with 50 messages is serialized
    Then the output begins with the snappy magic bytes
    And the compressed size is less than 70% of the raw JSON size

  Scenario: Small snapshots skip compression
    Given compression_threshold is 4096 bytes
    When a session with 2 messages is serialized
    Then the output is raw JSON (no magic bytes)
    And deserialization succeeds

  Scenario: Old uncompressed snapshots are still readable
    Given a legacy V3 snapshot file (raw JSON, no magic bytes)
    When Deserialize is called
    Then the session context is restored correctly

  Scenario: Corrupt compressed data returns error
    Given data with snappy magic bytes but corrupt content
    When Deserialize is called
    Then a SnapshotCorruptError is returned

  Scenario: Backup file uses same compression format
    Given compression is enabled
    And a backup directory is configured
    When WriteBackup is called
    Then the backup file also contains compressed data
```
