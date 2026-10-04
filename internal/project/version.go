// Package project holds the structured Project model of a `.sec/sec.toml`
// manifest that does not depend on the compiler or the language server.
package project

import (
	"bufio"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Version is a resolved structured Project or Target version.
//
// Rules:
//   - rules/projects/projects.md — § 8 "Project version model", § 9 "User-defined version component"
type Version struct {
	Major          uint64
	Minor          uint64
	Revision       uint64
	Build          uint64
	UserDefined    string
	HasUserDefined bool
	Format         string
}

// VersionOverride is one `[target.<name>.version]` table: every present field
// replaces the inherited Project field.
//
// Rules:
//   - rules/projects/projects.md — § 11 "Target version overrides"
type VersionOverride struct {
	Major       *uint64
	Minor       *uint64
	Revision    *uint64
	Build       *uint64
	UserDefined *string
	Format      *string
}

// ManifestVersions holds the version tables of one manifest.
type ManifestVersions struct {
	Project         Version
	HasProject      bool
	TargetOverrides map[string]VersionOverride
}

// ManifestError is one located manifest version error.
type ManifestError struct {
	Line    int
	Message string
}

func (err ManifestError) Error() string {
	if err.Line > 0 {
		return fmt.Sprintf("sec.toml:%d: %s", err.Line, err.Message)
	}
	return "sec.toml: " + err.Message
}

var (
	userDefinedPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	formatPlaceholderPattern = regexp.MustCompile(`\[[A-Za-z_][A-Za-z0-9_]*\]`)
	versionPlaceholders      = map[string]bool{"[major]": true, "[minor]": true, "[revision]": true, "[build]": true, "[userdefined]": true}
)

// ParseManifestVersions reads the `[version]` table and every
// `[target.<name>.version]` table of a manifest, validates their fields, and
// reports every error found. Other tables are skipped. It accepts the TOML
// subset these tables use: decimal integers (with `_` digit separators),
// basic and literal strings, and comments.
//
// Rules:
//   - rules/projects/projects.md — § 8(1)–(3), § 9(3), § 10, § 11, § 48 "Project validation"
func ParseManifestVersions(text string) (ManifestVersions, []ManifestError) {
	result := ManifestVersions{TargetOverrides: map[string]VersionOverride{}}
	errors := []ManifestError{}
	type pendingVersion struct {
		fields map[string]bool
		line   int
	}
	var project *pendingVersion
	section := ""
	target := ""
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
			section, target = "", ""
			switch {
			case header == "version":
				if project != nil {
					errors = append(errors, ManifestError{Line: lineNumber, Message: "a manifest contains exactly one [version] table; this is a second one"})
				}
				project = &pendingVersion{fields: map[string]bool{}, line: lineNumber}
				result.HasProject = true
				section = "version"
			case strings.HasPrefix(header, "target.") && strings.HasSuffix(header, ".version"):
				name := strings.TrimSuffix(strings.TrimPrefix(header, "target."), ".version")
				if unquoted, err := strconv.Unquote(name); err == nil {
					name = unquoted
				}
				if _, exists := result.TargetOverrides[name]; exists {
					errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("duplicate [target.%s.version] table", name)})
				}
				result.TargetOverrides[name] = VersionOverride{}
				section, target = "target", name
			}
			continue
		}
		if section == "" {
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		if !found {
			errors = append(errors, ManifestError{Line: lineNumber, Message: "expected key = value"})
			continue
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if section == "version" {
			if project.fields[key] {
				errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("duplicate version field %s", key)})
			}
			project.fields[key] = true
		}
		override := result.TargetOverrides[target]
		switch key {
		case "major", "minor", "revision", "build":
			number, err := parseVersionNumber(raw)
			if err != nil {
				errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("version.%s: %v", key, err)})
				continue
			}
			if section == "version" {
				*projectNumberField(&result.Project, key) = number
			} else {
				*overrideNumberField(&override, key) = &number
			}
		case "userdefined", "format":
			value, err := parseTOMLString(raw)
			if err != nil {
				errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("version.%s: %v", key, err)})
				continue
			}
			if key == "userdefined" {
				// A non-empty value must be a valid component; an empty value
				// clears an inherited component and is valid only on a Target.
				if value != "" && !userDefinedPattern.MatchString(value) {
					errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("version.userdefined %q must match [A-Za-z0-9][A-Za-z0-9._-]*", value)})
				}
				if value == "" && section == "version" {
					errors = append(errors, ManifestError{Line: lineNumber, Message: "version.userdefined is empty; omit it instead"})
				}
				if section == "version" {
					result.Project.UserDefined, result.Project.HasUserDefined = value, value != ""
				} else {
					override.UserDefined = &value
				}
			} else {
				if message, ok := formatPlaceholderError(value); !ok {
					errors = append(errors, ManifestError{Line: lineNumber, Message: message})
				}
				if section == "version" {
					result.Project.Format = value
				} else {
					override.Format = &value
				}
			}
		default:
			errors = append(errors, ManifestError{Line: lineNumber, Message: fmt.Sprintf("unknown version field %s", key)})
		}
		if section == "target" {
			result.TargetOverrides[target] = override
		}
	}
	if project == nil {
		errors = append(errors, ManifestError{Message: "the manifest has no [version] table; every Project manifest contains exactly one"})
		return result, errors
	}
	for _, field := range []string{"major", "minor", "revision", "build", "format"} {
		if !project.fields[field] {
			errors = append(errors, ManifestError{Line: project.line, Message: fmt.Sprintf("[version] is missing the required field %s", field)})
		}
	}
	if len(errors) > 0 {
		return result, errors
	}
	if _, err := result.Project.Render(); err != nil {
		errors = append(errors, ManifestError{Line: project.line, Message: err.Error()})
	}
	for name, override := range result.TargetOverrides {
		resolved := result.Project.Resolve(override)
		if _, err := resolved.Render(); err != nil {
			errors = append(errors, ManifestError{Message: fmt.Sprintf("target %s version: %v", name, err)})
		}
	}
	return result, errors
}

// Resolve applies a Target override to the Project version.
//
// Rules:
//   - rules/projects/projects.md — § 11(4)–(9) inheritance, replacement, and userdefined clearing
func (version Version) Resolve(override VersionOverride) Version {
	resolved := version
	for _, pair := range []struct {
		target *uint64
		value  *uint64
	}{{&resolved.Major, override.Major}, {&resolved.Minor, override.Minor}, {&resolved.Revision, override.Revision}, {&resolved.Build, override.Build}} {
		if pair.value != nil {
			*pair.target = *pair.value
		}
	}
	if override.UserDefined != nil {
		resolved.UserDefined, resolved.HasUserDefined = *override.UserDefined, *override.UserDefined != ""
	}
	if override.Format != nil {
		resolved.Format = *override.Format
	}
	return resolved
}

// Render produces the version string from the structured fields: the five
// placeholders are replaced and every other character is literal.
//
// Rules:
//   - rules/projects/projects.md — § 10 "Version format", § 49(3) render from parsed fields
func (version Version) Render() (string, error) {
	if message, ok := formatPlaceholderError(version.Format); !ok {
		return "", fmt.Errorf("%s", message)
	}
	if strings.Contains(version.Format, "[userdefined]") && !version.HasUserDefined {
		return "", fmt.Errorf("version format %q references [userdefined], but the resolved version has no user-defined component", version.Format)
	}
	rendered := strings.NewReplacer(
		"[major]", strconv.FormatUint(version.Major, 10),
		"[minor]", strconv.FormatUint(version.Minor, 10),
		"[revision]", strconv.FormatUint(version.Revision, 10),
		"[build]", strconv.FormatUint(version.Build, 10),
		"[userdefined]", version.UserDefined,
	).Replace(version.Format)
	if rendered == "" {
		return "", fmt.Errorf("version format %q renders an empty version", version.Format)
	}
	return rendered, nil
}

// formatPlaceholderError rejects a bracketed ASCII identifier that is not one
// of the five placeholders (§ 10(6)) and an empty format (§ 10(10)).
func formatPlaceholderError(format string) (string, bool) {
	if format == "" {
		return "version.format is empty; a resolved version must render a non-empty string", false
	}
	for _, placeholder := range formatPlaceholderPattern.FindAllString(format, -1) {
		if !versionPlaceholders[placeholder] {
			return fmt.Sprintf("version.format uses unknown placeholder %s; the placeholders are [major], [minor], [revision], [build], and [userdefined]", placeholder), false
		}
	}
	return "", true
}

// parseVersionNumber accepts a non-negative decimal TOML integer, with `_`
// between digits, in 0..9223372036854775807 (§ 8(3)).
func parseVersionNumber(raw string) (uint64, error) {
	if raw == "" {
		return 0, fmt.Errorf("expected a non-negative integer")
	}
	if strings.HasPrefix(raw, "-") {
		return 0, fmt.Errorf("%s is negative; a VersionNumber is in 0..9223372036854775807", raw)
	}
	digits := strings.TrimPrefix(raw, "+")
	if strings.HasPrefix(digits, "_") || strings.HasSuffix(digits, "_") || strings.Contains(digits, "__") {
		return 0, fmt.Errorf("%s is not a TOML integer", raw)
	}
	digits = strings.ReplaceAll(digits, "_", "")
	if len(digits) > 1 && digits[0] == '0' {
		return 0, fmt.Errorf("%s has a leading zero, which TOML integers do not allow", raw)
	}
	value, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a non-negative decimal integer", raw)
	}
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds 9223372036854775807", raw)
	}
	return value, nil
}

// parseTOMLString accepts a TOML basic ("...") or literal ('...') string.
func parseTOMLString(raw string) (string, error) {
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1], nil
	}
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return strconv.Unquote(raw)
	}
	return "", fmt.Errorf("expected a string")
}

// stripComment removes a `#` comment outside strings.
func stripComment(line string) string {
	quote := rune(0)
	escaped := false
	for index, char := range line {
		switch {
		case escaped:
			escaped = false
		case quote == '"' && char == '\\':
			escaped = true
		case quote != 0 && char == quote:
			quote = 0
		case quote == 0 && (char == '"' || char == '\''):
			quote = char
		case quote == 0 && char == '#':
			return line[:index]
		}
	}
	return line
}

func projectNumberField(version *Version, key string) *uint64 {
	switch key {
	case "major":
		return &version.Major
	case "minor":
		return &version.Minor
	case "revision":
		return &version.Revision
	}
	return &version.Build
}

func overrideNumberField(override *VersionOverride, key string) **uint64 {
	switch key {
	case "major":
		return &override.Major
	case "minor":
		return &override.Minor
	case "revision":
		return &override.Revision
	}
	return &override.Build
}
