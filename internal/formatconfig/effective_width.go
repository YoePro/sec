package formatconfig

import "sec/internal/formatter"

// EffectiveOptions resolves a validated project's preferred width for one
// presentation context. Zero means no usable observed viewport. Fixed mode
// ignores that context; adaptive mode clamps the observation or fallback to
// the project range. The Settings snapshot and manifest remain unchanged.
// Rules: rules/tooling/formatter.md — §4(9–15,20–23),§10(1).
func (settings Settings) EffectiveOptions(presentationWidth uint64) formatter.Options {
	options := settings.Options
	width := settings.LineWidth
	if settings.WidthMode == "adaptive" {
		width = presentationWidth
		if width == 0 {
			width = settings.AdaptiveFallbackWidth
		}
		if width < settings.AdaptiveMinWidth {
			width = settings.AdaptiveMinWidth
		}
		if width > settings.AdaptiveMaxWidth {
			width = settings.AdaptiveMaxWidth
		}
	}
	options.LineWidth = width
	return options
}
