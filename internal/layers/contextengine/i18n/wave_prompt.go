package i18n

// WavePromptLabels are locale-aware section headings for Wave worker system prompts.
type WavePromptLabels struct {
	AllowedFileScope     string
	FilesChangedUpstream string
	UpstreamErrorPrefix  string
}

// WavePromptLabelsFor returns Wave system-prompt section headings.
func WavePromptLabelsFor(loc Locale) WavePromptLabels {
	if loc == LocaleEN {
		return WavePromptLabels{
			AllowedFileScope:     "Allowed file scope:",
			FilesChangedUpstream: "Files changed by upstream:",
			UpstreamErrorPrefix:  "Upstream error (for context): ",
		}
	}
	return WavePromptLabels{
		AllowedFileScope:     "允许的文件范围:",
		FilesChangedUpstream: "上游变更的文件:",
		UpstreamErrorPrefix:  "上游错误（供参考）: ",
	}
}
