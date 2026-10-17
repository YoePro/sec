package project

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// FormatSettings is the `[format]` table of a manifest: every field is nil
// when the table omits it, so the formatter default applies to that field
// alone. Present reports that the table exists.
type FormatSettings struct {
	Present               bool
	FormatVersion         *uint64
	VerticalStyle         *string
	IndentationWidth      *uint64
	WidthMode             *string
	LineWidth             *uint64
	AdaptiveMinWidth      *uint64
	AdaptiveMaxWidth      *uint64
	AdaptiveFallbackWidth *uint64
	MaxAlignmentPadding   *uint64
	LanguageCorrections   *bool
}

// ParseManifestFormat reads the `[format]` project table of a manifest. It
// checks only the table's own shape: the key names of formatter.md § 4(5),
// their TOML value kinds, duplicate keys and a duplicate table. Value domains
// and the supported format_version are validated by the formatter
// configuration resolver, which owns their meaning. Other tables are skipped.
//
// Rules:
//   - rules/projects/projects.md — § 6 "Manifest tables", "[format]"
//   - rules/tooling/formatter.md — § 4 "Formatter configuration and format version"
func ParseManifestFormat(text string) (FormatSettings, []ManifestError) {
	var settings FormatSettings
	errors := []ManifestError{}
	seen := map[string]bool{}
	inFormat := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			header := strings.TrimSpace(strings.Trim(line, "[]"))
			inFormat = header == "format"
			if inFormat {
				if settings.Present {
					errors = append(errors, ManifestError{Line: lineNumber, Message: "a manifest contains at most one [format] table; this is a second one"})
				}
				settings.Present = true
			}
			continue
		}
		if !inFormat {
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		if !found {
			errors = append(errors, ManifestError{Line: lineNumber, Message: "expected key = value"})
			continue
		}
		key, raw = strings.TrimSpace(key), strings.TrimSpace(raw)
		if seen[key] {
			errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("duplicate format field %s", key)})
			continue
		}
		seen[key] = true
		number := func(target **uint64) {
			value, err := parseVersionNumber(raw)
			if err != nil {
				errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("format field %s must be a non-negative integer: %v", key, err)})
				return
			}
			*target = &value
		}
		text := func(target **string) {
			value, err := parseTOMLString(raw)
			if err != nil {
				errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("format field %s must be a string", key)})
				return
			}
			*target = &value
		}
		switch key {
		case "format_version":
			number(&settings.FormatVersion)
		case "vertical_style":
			text(&settings.VerticalStyle)
		case "indentation_width":
			number(&settings.IndentationWidth)
		case "width_mode":
			text(&settings.WidthMode)
		case "line_width":
			number(&settings.LineWidth)
		case "adaptive_min_width":
			number(&settings.AdaptiveMinWidth)
		case "adaptive_max_width":
			number(&settings.AdaptiveMaxWidth)
		case "adaptive_fallback_width":
			number(&settings.AdaptiveFallbackWidth)
		case "max_alignment_padding":
			number(&settings.MaxAlignmentPadding)
		case "language_corrections":
			value, err := strconv.ParseBool(raw)
			if err != nil || (raw != "true" && raw != "false") {
				errors = append(errors, ManifestError{Line: lineNumber, Message: "format field language_corrections must be true or false"})
				continue
			}
			settings.LanguageCorrections = &value
		default:
			errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("unknown format field %s", key)})
		}
	}
	return settings, errors
}
