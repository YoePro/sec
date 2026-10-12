package main

import (
	"strings"
	"testing"
)

// TestTryNestedMatchCLI preserves exhaustive returning handlers and rejects
// both genuinely continuing branches and unreachable recovery expressions.
// Rules: rules/errors/errorhandling.md — §§21–21.1;
// rules/control-flow/flowcontrol_match.md — §23.
func TestTryNestedMatchCLI(t *testing.T) {
	for _, test := range []struct {
		file        string
		code, count int
	}{
		{"try_handler_nested_match_valid.sec", 0, 0},
		{"try_handler_nested_match_invalid.sec", 3, 2},
	} {
		_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/"+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if len(doc.Occurrences) != test.count {
			t.Fatal(doc)
		}
		if test.count > 0 && (!strings.Contains(doc.Occurrences[0].Message.Text, "try handler must return") || !strings.Contains(doc.Occurrences[1].Message.Text, "unreachable recovery value")) {
			t.Fatal(doc)
		}
	}
}
