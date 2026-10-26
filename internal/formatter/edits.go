package formatter

import "unicode/utf8"

// Edit replaces the half-open byte range [Start, End) of the original source.
type Edit struct {
	Start, End int
	NewText    string
}

// MinimalEdit expresses canonical formatter output as one enclosing edit,
// excluding its unchanged prefix and suffix. Boundaries never split a UTF-8
// scalar or CRLF pair, so editor adapters can convert them to line positions.
// Identical source produces no edit. This changes only how output is applied,
// never the formatting policy or the resulting source bytes.
//
// Rules:
//   - rules/tooling/formatter.md — § 28(1)–(2), § 29(2), (4), (7), (9)
//   - rules/tooling/lsp.md — "Shared formatter", "Formatting"
func MinimalEdit(original, formatted string) (Edit, bool) {
	if original == formatted {
		return Edit{}, false
	}
	start := 0
	for start < len(original) && start < len(formatted) && original[start] == formatted[start] {
		start++
	}
	for start > 0 && (!editBoundary(original, start) || !editBoundary(formatted, start)) {
		start--
	}
	end, replacementEnd := len(original), len(formatted)
	for end > start && replacementEnd > start && original[end-1] == formatted[replacementEnd-1] {
		end--
		replacementEnd--
	}
	for !editBoundary(original, end) || !editBoundary(formatted, replacementEnd) {
		end++
		replacementEnd++
	}
	return Edit{Start: start, End: end, NewText: formatted[start:replacementEnd]}, true
}

// editBoundary recognizes positions representable by an editor without
// splitting a Unicode scalar or physical line ending.
// Rules: rules/foundations/lexical_structure.md — § 1.3, § 2.
func editBoundary(text string, offset int) bool {
	if offset == 0 || offset == len(text) {
		return true
	}
	return utf8.RuneStart(text[offset]) && !(text[offset-1] == '\r' && text[offset] == '\n')
}
