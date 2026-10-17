// Package formatconfig resolves the effective formatter options of one
// formatting request from explicit invocation options, the selected project's
// `.sec/sec.toml` `[format]` table and the canonical formatter defaults. The
// CLI and the LSP share it so both read and validate the same configuration.
//
// Rules:
//   - rules/tooling/formatter.md — § 4 "Formatter configuration and format version", § 4(18)–(23)
//   - rules/projects/projects.md — "[format]"
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — § 4
package formatconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/formatter"
	"sec/internal/project"
)

// SupportedFormatVersion is the only format_version this formatter implements.
const SupportedFormatVersion = 1

// maxAlignmentPaddingLimit bounds `max_alignment_padding` so a manifest value
// cannot overflow the formatter's column arithmetic (MD-098).
const maxAlignmentPaddingLimit = 1024

// Overrides are formatter options supplied explicitly for one invocation. A
// nil field was not supplied and never shadows the project setting.
type Overrides struct {
	IndentationWidth    *int
	LanguageCorrections *bool
}

// Settings is the effective, validated configuration. Width settings are
// validated and carried for completeness; the current formatter does not yet
// consume a preferred line width.
type Settings struct {
	Options               formatter.Options
	FormatVersion         uint64
	WidthMode             string
	LineWidth             uint64
	AdaptiveMinWidth      uint64
	AdaptiveMaxWidth      uint64
	AdaptiveFallbackWidth uint64
	// Manifest is the project manifest that supplied settings, or empty for
	// a stand-alone file.
	Manifest string
}

// Defaults returns the canonical formatter defaults of formatter.md § 4(5),
// including language_corrections = false.
func Defaults() Settings {
	return Settings{
		Options:       formatter.DefaultOptions(),
		FormatVersion: SupportedFormatVersion,
		WidthMode:     "fixed", LineWidth: 120,
		AdaptiveMinWidth: 80, AdaptiveMaxWidth: 200, AdaptiveFallbackWidth: 120,
	}
}

// ProjectManifest returns the `.sec/sec.toml` of the project that owns path:
// the nearest ancestor directory holding that manifest, the same project-root
// rule the compiler and LSP use. A stand-alone file has none.
func ProjectManifest(path string) (string, bool) {
	current := filepath.Clean(path)
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		manifest := filepath.Join(current, ".sec", "sec.toml")
		if info, err := os.Stat(manifest); err == nil && !info.IsDir() {
			return manifest, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// ForSource resolves the effective settings for formatting the file at path:
// its project's `[format]` table when the file belongs to a project, the
// defaults otherwise, and the explicit overrides on top. An empty path is a
// stand-alone source such as standard input.
func ForSource(path string, overrides Overrides) (Settings, error) {
	if path == "" {
		return Resolve("", "", overrides)
	}
	manifest, found := ProjectManifest(path)
	if !found {
		return Resolve("", "", overrides)
	}
	text, err := os.ReadFile(manifest)
	if err != nil {
		return Settings{}, fmt.Errorf("%s: %w", manifest, err)
	}
	return Resolve(manifest, string(text), overrides)
}

// Resolve applies, from highest to lowest precedence, the explicit overrides,
// the manifest's `[format]` settings and the canonical defaults, then
// validates the effective configuration. Any manifest error, unknown field,
// invalid value or unsupported effective format_version is an error, so a
// caller can stop before modifying any source file.
func Resolve(manifestPath, manifestText string, overrides Overrides) (Settings, error) {
	settings := Defaults()
	settings.Manifest = manifestPath
	if manifestText != "" {
		table, manifestErrors := project.ParseManifestFormat(manifestText)
		if len(manifestErrors) > 0 {
			messages := make([]string, 0, len(manifestErrors))
			for _, manifestError := range manifestErrors {
				messages = append(messages, manifestError.Error())
			}
			return Settings{}, fmt.Errorf("%s: invalid [format] configuration: %s", manifestPath, strings.Join(messages, "; "))
		}
		if table.FormatVersion != nil {
			settings.FormatVersion = *table.FormatVersion
		}
		if table.VerticalStyle != nil {
			settings.Options.VerticalStyle = formatter.VerticalStyle(*table.VerticalStyle)
		}
		if table.IndentationWidth != nil {
			settings.Options.IndentationWidth = int(*table.IndentationWidth)
		}
		if table.WidthMode != nil {
			settings.WidthMode = *table.WidthMode
		}
		for _, field := range []struct {
			value  *uint64
			target *uint64
		}{
			{table.LineWidth, &settings.LineWidth},
			{table.AdaptiveMinWidth, &settings.AdaptiveMinWidth},
			{table.AdaptiveMaxWidth, &settings.AdaptiveMaxWidth},
			{table.AdaptiveFallbackWidth, &settings.AdaptiveFallbackWidth},
		} {
			if field.value != nil {
				*field.target = *field.value
			}
		}
		if table.MaxAlignmentPadding != nil {
			if *table.MaxAlignmentPadding == 0 || *table.MaxAlignmentPadding > maxAlignmentPaddingLimit {
				return Settings{}, fmt.Errorf("%s: max_alignment_padding must be 1..%d spaces, got %d", manifestPath, maxAlignmentPaddingLimit, *table.MaxAlignmentPadding)
			}
			settings.Options.MaxAlignmentPadding = int(*table.MaxAlignmentPadding)
		}
		if table.LanguageCorrections != nil {
			settings.Options.Fix = *table.LanguageCorrections
		}
	}
	if overrides.IndentationWidth != nil {
		settings.Options.IndentationWidth = *overrides.IndentationWidth
	}
	if overrides.LanguageCorrections != nil {
		settings.Options.Fix = *overrides.LanguageCorrections
	}
	if err := settings.validate(); err != nil {
		if manifestPath != "" {
			return Settings{}, fmt.Errorf("%s: %w", manifestPath, err)
		}
		return Settings{}, err
	}
	return settings, nil
}

func (settings Settings) validate() error {
	if settings.FormatVersion != SupportedFormatVersion {
		return fmt.Errorf("unsupported format_version %d; this formatter implements format_version = %d and leaves sources unchanged", settings.FormatVersion, SupportedFormatVersion)
	}
	if settings.Options.IndentationWidth == 0 {
		return errors.New("indentation_width must be 2, 4, or 6 spaces, got 0")
	}
	if err := formatter.ValidateOptions(settings.Options); err != nil {
		return err
	}
	if settings.WidthMode != "fixed" && settings.WidthMode != "adaptive" {
		return fmt.Errorf("width_mode must be \"fixed\" or \"adaptive\", got %q", settings.WidthMode)
	}
	for name, value := range map[string]uint64{"line_width": settings.LineWidth, "adaptive_min_width": settings.AdaptiveMinWidth, "adaptive_max_width": settings.AdaptiveMaxWidth, "adaptive_fallback_width": settings.AdaptiveFallbackWidth} {
		if value == 0 {
			return fmt.Errorf("%s must be a positive width", name)
		}
	}
	if settings.AdaptiveMinWidth > settings.AdaptiveMaxWidth {
		return fmt.Errorf("adaptive_min_width %d exceeds adaptive_max_width %d", settings.AdaptiveMinWidth, settings.AdaptiveMaxWidth)
	}
	return nil
}
