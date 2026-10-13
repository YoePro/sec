package project

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// ProjectUUID is the machine identity, independent of the display name.
// Rules: rules/projects/projects.md — §3.3,§7(6).
type ProjectUUID [16]byte

func (uuid ProjectUUID) String() string {
	encoded := hex.EncodeToString(uuid[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

// ParseProjectUUID requires the canonical stored RFC 9562 spelling and rejects
// Nil. All defined ordinary versions and the special Max UUID are accepted.
// Rules: rules/projects/projects.md — §3.3(2–5),§7(6); RFC 9562 §§4,5.9,5.10.
func ParseProjectUUID(value string) (ProjectUUID, error) {
	var uuid ProjectUUID
	invalid := func() (ProjectUUID, error) {
		return uuid, fmt.Errorf("project.uuid %q must be a non-nil RFC 9562 UUID in lowercase hyphenated form", value)
	}
	if len(value) != 36 || value != strings.ToLower(value) {
		return invalid()
	}
	for _, index := range []int{8, 13, 18, 23} {
		if value[index] != '-' {
			return invalid()
		}
	}
	raw := strings.ReplaceAll(value, "-", "")
	if len(raw) != 32 {
		return invalid()
	}
	if _, err := hex.Decode(uuid[:], []byte(raw)); err != nil {
		return invalid()
	}
	if uuid == (ProjectUUID{}) {
		return invalid()
	}
	max := true
	for _, b := range uuid {
		max = max && b == 255
	}
	if !max && (uuid[8]&0xc0 != 0x80 || uuid[6]>>4 < 1 || uuid[6]>>4 > 8) {
		return invalid()
	}
	return uuid, nil
}

// Identity is the parsed [project] section. Warnings retain suspicious name
// whitespace without changing the human-readable value.
// Rules: rules/projects/projects.md — §7(1–8).
type Identity struct {
	Name     string
	UUID     ProjectUUID
	Module   string
	Warnings []ManifestError
}

// ValidModuleName uses the source parser's single-identifier declaration form,
// including its NFC and keyword rules, rather than a second identifier model.
// Rules: rules/projects/modules.md — §2.1,§4; rules/foundations/grammar.md — ModuleDeclaration.
func ValidModuleName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name {
		return false
	}
	l := lexer.New("module " + name + "\n")
	p := parser.New(l)
	program := p.Parse().Program
	if len(p.Errors()) != 0 || len(l.Diagnostics()) != 0 || len(program.Statements) != 1 {
		return false
	}
	module, ok := program.Statements[0].(*ast.ModuleStatement)
	return ok && module.Path == name && !strings.Contains(name, ".")
}

// ParseManifestIdentity validates the required project section and its typed
// fields. Other sections and fields remain owned by their respective readers.
// Unsupported multiline strings are diagnosed and skipped, never reinterpreted.
// Rules: rules/projects/projects.md — §3.3,§7,§48(1–2).
func ParseManifestIdentity(text string) (Identity, []ManifestError) {
	var identity Identity
	var errors []ManifestError
	fields := map[string]bool{}
	section := false
	projectLine := 0
	skipped := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNumber := 0
	add := func(message string) { errors = append(errors, ManifestError{Line: lineNumber, Message: message}) }
	for scanner.Scan() {
		lineNumber++
		if skipped != "" {
			if unsupportedMultilineClosed(scanner.Text(), skipped) {
				skipped = ""
			}
			continue
		}
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			header := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if parsed, err := parseTOMLString(header); err == nil {
				header = parsed
			}
			section = strings.HasSuffix(line, "]") && !strings.HasPrefix(line, "[[") && header == "project"
			if section {
				if projectLine != 0 {
					add("a manifest contains exactly one [project] table; this is a second one")
				} else {
					projectLine = lineNumber
				}
			}
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		raw = strings.TrimSpace(raw)
		// Skip multiline values in every section, so their contents cannot forge [project].
		if delimiter, message := unsupportedVersionString(raw); delimiter != "" {
			if section {
				field := strings.TrimSpace(key)
				if parsed, err := parseTOMLString(field); err == nil {
					field = parsed
				}
				add("project." + field + ": " + message)
			}
			if !unsupportedMultilineClosed(strings.TrimPrefix(raw, delimiter), delimiter) {
				skipped = delimiter
			}
			continue
		}
		if !section {
			continue
		}
		if !found {
			add("expected project key = value")
			continue
		}
		key = strings.TrimSpace(key)
		if parsed, err := parseTOMLString(key); err == nil {
			key = parsed
		}
		if fields[key] {
			add("duplicate project field " + key)
		}
		fields[key] = true
		if key != "name" && key != "uuid" && key != "module" {
			continue
		}
		value, err := parseIdentityString(raw)
		if err != nil {
			add("project." + key + ": " + err.Error())
			continue
		}
		switch key {
		case "name":
			identity.Name = value
			if value == "" {
				add("project.name must be a non-empty string")
			}
			if value != strings.TrimSpace(value) {
				identity.Warnings = append(identity.Warnings, ManifestError{Line: lineNumber, Message: "project.name has leading or trailing whitespace; it is preserved as part of the name"})
			}
		case "uuid":
			identity.UUID, err = ParseProjectUUID(value)
			if err != nil {
				add(err.Error())
			}
		case "module":
			identity.Module = value
			if !ValidModuleName(value) {
				add("project.module must be a single valid ModuleName identifier")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		add("cannot read project section: " + err.Error())
	}
	if projectLine == 0 {
		errors = append(errors, ManifestError{Message: "the manifest must contain exactly one [project] table"})
	} else {
		for _, field := range []string{"name", "uuid"} {
			if !fields[field] {
				errors = append(errors, ManifestError{Line: projectLine, Message: "[project] is missing the required field " + field})
			}
		}
	}
	return identity, errors
}

// parseIdentityString accepts single-line TOML strings without interpreting Go-only
// escape forms or embedded literal-string delimiters as manifest values.
// Rules: rules/projects/projects.md — §7(4),§48(2).
func parseIdentityString(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("TOML strings must be valid UTF-8")
	}
	if len(raw) < 2 {
		return "", fmt.Errorf("expected a string")
	}
	if raw[0] == '\'' && strings.Contains(raw[1:len(raw)-1], "'") {
		return "", fmt.Errorf("invalid TOML literal string")
	}
	if raw[0] == '"' {
		for i := 1; i < len(raw)-1; i++ {
			if raw[i] == '\\' {
				i++
				if i >= len(raw)-1 || !strings.ContainsRune("btnfr\"\\uU", rune(raw[i])) {
					return "", fmt.Errorf("invalid TOML string escape")
				}
			}
		}
	}
	for _, r := range raw {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return "", fmt.Errorf("invalid control character in TOML string")
		}
	}
	return parseTOMLString(raw)
}
