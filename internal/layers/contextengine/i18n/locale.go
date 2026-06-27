// Package i18n provides locale-aware prompt sections and tool schema localization
// for the D2 context engine (system prompts and LLM-facing tool definitions).
package i18n

import "strings"

// Locale is the language used for LLM-facing prompts and tool schemas.
type Locale string

const (
	LocaleZH Locale = "zh-CN"
	LocaleEN Locale = "en-US"
)

// DefaultLocale is used when no language is configured.
const DefaultLocale = LocaleZH

// ParseLanguage maps a config string to Locale. Unknown values default to zh-CN.
func ParseLanguage(s string) Locale {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en", "en-us", "en_us":
		return LocaleEN
	case "zh", "zh-cn", "zh_cn", "zh-hans":
		return LocaleZH
	case "":
		return DefaultLocale
	default:
		if strings.HasPrefix(strings.ToLower(s), "en") {
			return LocaleEN
		}
		return DefaultLocale
	}
}

func (l Locale) IsChinese() bool { return l != LocaleEN }
