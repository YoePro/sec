package main

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type textIDOccurrence struct {
	ID, Text                 string
	Line, Column, Start, End int
}

// textIDOccurrences locates exact textual identifiers in a single source pass,
// preserving byte offsets for context audits and Unicode columns for reports.
// Contract: cmd/yamlstatus/IDEAS.md §§5,8.
func textIDOccurrences(text string, pattern *regexp.Regexp) []textIDOccurrence {
	var refs []textIDOccurrence
	line, lineStart, cursor := 1, 0, 0
	for _, match := range pattern.FindAllStringIndex(text, -1) {
		for cursor < match[0] {
			if text[cursor] == '\n' {
				line++
				lineStart = cursor + 1
			}
			cursor++
		}
		lineEnd := strings.IndexByte(text[match[1]:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += match[1]
		}
		refs = append(refs, textIDOccurrence{ID: text[match[0]:match[1]],
			Text: strings.TrimSuffix(text[lineStart:lineEnd], "\r"), Line: line,
			Column: utf8.RuneCountInString(text[lineStart:match[0]]) + 1,
			Start:  match[0], End: match[1]})
	}
	return refs
}
