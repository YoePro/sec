package configedit

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// String renders a TOML basic string, escaping only what TOML requires.
// Rules: rules/projects/projects.md — § 5(2).
func String(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, char := range value {
		switch {
		case char == '"' || char == '\\':
			out.WriteByte('\\')
			out.WriteRune(char)
		case char == '\n':
			out.WriteString(`\n`)
		case char == '\t':
			out.WriteString(`\t`)
		case char < 0x20 || char == 0x7f || char == utf8.RuneError:
			fmt.Fprintf(&out, `\u%04X`, char)
		default:
			out.WriteRune(char)
		}
	}
	out.WriteByte('"')
	return out.String()
}
