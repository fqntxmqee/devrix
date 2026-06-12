package safety

// defaultPatterns returns the built-in safety patterns.
//
// Inspired by Claude Fable-5's refusal_handling and harmful_content_safety.
// Patterns are grouped by severity: critical patterns reject the request,
// while lower-severity patterns allow the request through with warnings.
func defaultPatterns() []Pattern {
	return []Pattern{
		// ── Critical: Malicious Code Generation ──
		{
			Name:        "malware_generation",
			Description: "Direct request to generate malware, ransomware, or viruses",
			Patterns: []string{
				"generate malware",
				"write ransomware",
				"create a virus",
				"develop a trojan",
				"build a worm",
				"ransomware code",
				"malware source code",
				"keylogger",
				"write a rootkit",
				"create a botnet",
			},
			Action:    ActionReject,
			Severity:  "critical",
			Locations: []string{"all"},
		},

		// ── Critical: Exploit Generation ──
		{
			Name:        "exploit_generation",
			Description: "Direct request to generate security exploits",
			Patterns: []string{
				"0-day exploit",
				"zero-day exploit",
				"buffer overflow exploit",
				"write a shellcode",
				"generate shellcode",
				"exploit for cve",
				"remote code execution exploit",
				"privilege escalation exploit",
			},
			Action:    ActionReject,
			Severity:  "critical",
			Locations: []string{"all"},
		},

		// ── High: Unauthorized Access Tools ──
		{
			Name:        "unauthorized_access",
			Description: "Tools for unauthorized system access",
			Patterns: []string{
				"sql injection generator",
				"generate sqli payload",
				"bypass authentication code",
				"crack password hash",
				"brute force script",
				"write a crackme",
			},
			Action:    ActionReject,
			Severity:  "high",
			Locations: []string{"all"},
		},

		// ── Medium: Credential / Secret Leakage in Prompt ──
		{
			Name:        "hardcoded_credential",
			Description: "Hardcoded credentials in the prompt that may leak to LLM provider",
			Patterns: []string{
				"sk-",        // OpenAI API key prefix
				"-----begin rsa private key-----",
				"-----begin openssh private key-----",
				"ghp_",       // GitHub personal access token prefix
				"gho_",       // GitHub OAuth access token prefix
				"xoxb-",      // Slack bot token
				"xoxp-",      // Slack user token
			},
			Action:    ActionWarn,
			Severity:  "medium",
			Locations: []string{"message"},
		},

		// ── Medium: Prompt Injection Attempt ──
		{
			Name:        "prompt_injection",
			Description: "Attempt to override system prompt instructions",
			Patterns: []string{
				"ignore previous instructions",
				"ignore all previous instructions",
				"ignore all instructions above",
				"disregard all previous",
				"forget your instructions",
				"you are now",
				"new system prompt",
				"override system prompt",
				"you have been replaced",
			},
			Action:    ActionWarn,
			Severity:  "medium",
			Locations: []string{"message"},
		},

		// ── Medium: Data Exfiltration ──
		{
			Name:        "data_exfiltration",
			Description: "Attempt to exfiltrate data via the generated output",
			Patterns: []string{
				"send this data to",
				"post to webhook",
				"exfiltrate",
				"send to my server",
				"upload to remote",
			},
			Action:    ActionWarn,
			Severity:  "medium",
			Locations: []string{"message"},
		},
	}
}

// IsAllowed reports whether a result is allowed (helper for clean checks).
func (r *Result) IsAllowed() bool {
	return r != nil && r.Allowed
}

// HasRejections reports whether any matches were at reject level.
func (r *Result) HasRejections() bool {
	if r == nil {
		return false
	}
	for _, m := range r.Matches {
		if m.Action == ActionReject {
			return true
		}
	}
	return false
}

// HasWarnings reports whether any matches were at warn level.
func (r *Result) HasWarnings() bool {
	if r == nil {
		return false
	}
	for _, m := range r.Matches {
		if m.Action == ActionWarn {
			return true
		}
	}
	return false
}
