# Bash AST Security Policy Specification

**Surface ID:** TOOL-SEC-2-A02
**Phase:** 1 (P0)
**Status:** S3_Designed

<!-- T: TOOL-SEC-2-A02-T01, TOOL-SEC-2-A02-T02, TOOL-SEC-2-A02-T03, TOOL-SEC-2-A02-T04, TOOL-SEC-2-A02-T05, TOOL-SEC-2-A02-T06 -->

## ADDED

### Requirement: AST-Based Command Auditing

#### Scenario: Safe Command Allowed
- GIVEN a bash command `ls -la /tmp`
- WHEN the BashASTPolicy audits the command
- THEN the decision is `Allow: true`
- AND the audit takes < 100ms

<!-- T: TOOL-SEC-2-A02-T01 -->

#### Scenario: Dangerous Command Denied (rm -rf)
- GIVEN a bash command `rm -rf /tmp/*`
- WHEN the BashASTPolicy audits the command
- THEN the decision is `Allow: false`
- AND the reason is "rm -rf pattern + path wildcard"
- AND a span `d3.bash.denied` is emitted

<!-- T: TOOL-SEC-2-A02-T02 -->

#### Scenario: AST Parse Failure (Fail-Closed)
- GIVEN an unparseable bash command
- WHEN the BashASTPolicy audits the command
- THEN the decision is `Allow: false`
- AND the reason is "AST parse failed: <error>"

### Requirement: Heredoc Audit

#### Scenario: Simple Heredoc Audited
- GIVEN a bash command with heredoc `cat <<EOF\nhello\nEOF`
- WHEN the BashASTPolicy audits the command
- THEN heredoc content is extracted and checked against zsh attack rules
- AND safe heredoc content (e.g., JSON, plain text) is allowed

<!-- T: TOOL-SEC-2-A02-T03 -->

#### Scenario: Nested Heredoc Denied
- GIVEN a bash command with nested heredoc (depth > 2)
- WHEN the BashASTPolicy audits the command
- THEN the decision is `Allow: false`
- AND the reason is "nested heredoc denied"

#### Scenario: Heredoc with Command Substitution Denied
- GIVEN a bash command `cat <<EOF\n$(rm -rf /)\nEOF`
- WHEN the BashASTPolicy audits the command
- THEN the decision is `Allow: false`
- AND the reason is "heredoc contains command substitution"

### Requirement: Zsh Attack Pattern Detection

#### Scenario: 20+ Known Attack Patterns Blocked
- GIVEN the zsh rule set contains 20+ attack patterns
- WHEN any matching command is audited
- THEN the decision is `Allow: false`
- AND the matched rule name is returned as reason

<!-- T: TOOL-SEC-2-A02-T04 -->

Patterns include (non-exhaustive):
- `rm -rf /` (recursive root delete)
- `:(){:|:&};:` (fork bomb)
- `mkfs.ext4 /dev/sda` (format disk)
- `dd if=/dev/zero of=/dev/sda` (disk overwrite)
- `wget ... | bash` (remote code execution)
- `curl ... | sh` (remote code execution)
- `chmod -R 777 /` (permission escalation)
- ... (20+ total)

### Requirement: LTL-Lite Invariants

<!-- T: TOOL-SEC-2-A02-F*-T05 -->

The BashASTPolicy MUST satisfy:

- `parse_failure => deny` (fail-closed)
- `nested_heredoc => deny` (heredoc depth limit)
- `matched_rule => deny` (any rule match is binding)
- `audit_latency <= 100ms` (audit performance bound)

### Requirement: Integration with Bash Surface

#### Scenario: Pre-Execution Audit Hook
- GIVEN a bash command about to be executed via BashSurface
- WHEN the surface's `Execute` method is called
- THEN the BashASTPolicy.Audit is called first
- AND if denied, Execute returns ErrBashASTDenied with reason

<!-- T: TOOL-SEC-2-A02-T06 -->

#### Scenario: Sandbox Bypass Prevention
- GIVEN the AST analysis identifies a dangerous pattern
- WHEN the bash sandbox is configured
- THEN the dangerous command is blocked before reaching the sandbox
- AND a span `d3.bash.blocked_pre_sandbox` is emitted

## MODIFIED

(None - this is a new policy layer, not a modification of existing surfaces)

## REMOVED

(None)
