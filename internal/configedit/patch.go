package configedit

import (
	"fmt"
	"strings"
)

// changePatch emits one deterministic unified hunk with three context lines
// around the changed region. Fixed metadata names prevent arbitrary target
// paths from altering patch syntax; File separately binds the actual target.
// Rules: harness design notes §§5(2)–(4).
func ReviewPatch(before, after, label string) string {
	if before == after {
		return ""
	}
	lines := func(text string) []string {
		if text == "" {
			return nil
		}
		return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	old, next := lines(before), lines(after)
	prefix := 0
	for prefix < len(old) && prefix < len(next) && old[prefix] == next[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && old[len(old)-suffix-1] == next[len(next)-suffix-1] {
		suffix++
	}
	// A newline-only edit must keep the final line inside the changed region.
	if strings.HasSuffix(before, "\n") != strings.HasSuffix(after, "\n") {
		suffix = 0
		if prefix == len(old) && prefix == len(next) && prefix > 0 {
			prefix--
		}
	}
	from := max(0, prefix-3)
	oldEnd, nextEnd := len(old)-max(0, suffix-3), len(next)-max(0, suffix-3)
	var out strings.Builder
	start := func(count int) int {
		if count == 0 {
			return from
		}
		return from + 1
	}
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n@@ -%d,%d +%d,%d @@\n", label, label, start(oldEnd-from), oldEnd-from, start(nextEnd-from), nextEnd-from)
	writeLine := func(marker string, line string, atEnd bool, source string) {
		out.WriteString(marker + line + "\n")
		if atEnd && !strings.HasSuffix(source, "\n") {
			out.WriteString("\\ No newline at end of file\n")
		}
	}
	for i := from; i < prefix; i++ {
		writeLine(" ", old[i], i == len(old)-1, before)
	}
	for i := prefix; i < len(old)-suffix; i++ {
		writeLine("-", old[i], i == len(old)-1, before)
	}
	for i := prefix; i < len(next)-suffix; i++ {
		writeLine("+", next[i], i == len(next)-1, after)
	}
	for i := len(next) - suffix; i < nextEnd; i++ {
		writeLine(" ", next[i], i == len(next)-1, after)
	}
	return out.String()
}
