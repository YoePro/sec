package project

import (
	"fmt"
	"strings"
)

// unsupportedVersionInteger distinguishes unimplemented TOML integer forms
// from semantic VersionNumber range or decimal-spelling errors.
// Rules: rules/projects/projects.md — §8(3),§48(2) expected form and safe correction.
func unsupportedVersionInteger(raw string) string {
	for _, form := range []struct{ prefix, name string }{{"0x", "hexadecimal"}, {"0o", "octal"}, {"0b", "binary"}} {
		if strings.HasPrefix(raw, form.prefix) {
			return fmt.Sprintf("%s TOML integers are not supported by this manifest reader; use a decimal integer in 0..9223372036854775807 instead", form.name)
		}
	}
	return ""
}

// unsupportedVersionString rejects multiline TOML strings explicitly instead
// of treating triple quotes as a single-line literal or parsing contents as keys.
// Rules: rules/projects/projects.md — §§9,10,48(2).
func unsupportedVersionString(raw string) (string, string) {
	for _, delimiter := range []string{"\"\"\"", "'''"} {
		if strings.HasPrefix(raw, delimiter) {
			return delimiter, "multiline TOML strings are not supported by this manifest reader; use a single-line basic or literal string instead"
		}
	}
	return "", ""
}

// unsupportedMultilineClosed bounds recovery to the rejected string so text
// resembling table headers or version keys cannot become manifest facts.
// Escaped basic-string delimiters stay inside the rejected value.
// Rules: rules/projects/projects.md — §48(2),§49(2) resolved facts.
func unsupportedMultilineClosed(line, delimiter string) bool {
	for offset := 0; offset < len(line); {
		relative := strings.Index(line[offset:], delimiter)
		if relative < 0 {
			return false
		}
		index := offset + relative
		backslashes := 0
		if delimiter == "\"\"\"" {
			for previous := index - 1; previous >= 0 && line[previous] == '\\'; previous-- {
				backslashes++
			}
		}
		if backslashes%2 == 0 {
			return true
		}
		offset = index + len(delimiter)
	}
	return false
}
