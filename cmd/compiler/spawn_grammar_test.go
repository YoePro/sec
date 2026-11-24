package main

import (
	"strings"
	"testing"
)

// TestSpawnGrammarDiagnosticsCLI distinguishes malformed operands from valid
// but unsupported spawned blocks at the compiler's source boundary.
// Rules: rules/concurrency/spawn.md — §2;
// rules/foundations/grammar.md — §89.
func TestSpawnGrammarDiagnosticsCLI(t *testing.T) {
	for _, tc := range []struct {
		file, message string
		code          int
	}{
		{"../../testdata/parser/spawn_missing_operand_invalid.sec", "spawn requires an expression or block", 2},
		{"../../testdata/sema/spawn_blocks_invalid.sec", "compiler capability unavailable: spawned block body", 3},
	} {
		_, output, code := runCLIForDiagnostics(t, "check", tc.file, "--diagnostic-format=json")
		if code != tc.code {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 4 || !strings.Contains(output, tc.message) || strings.Contains(output, "deprecated") {
			t.Fatal(output)
		}
	}
}

// Void spawn blocks are a source capability; emission still follows the one
// lowering track's existing boundary. Rules: rules/concurrency/spawn.md §3A.
func TestVoidSpawnBlocksCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/spawn_blocks/void.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 0 {
		t.Fatal(output)
	}
}
