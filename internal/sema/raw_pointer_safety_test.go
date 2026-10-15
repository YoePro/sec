package sema

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Tooling fixtures remain honest source programs: canonical examples pass the
// frontend, and every explicitly invalid editing/context fixture produces a
// parser or semantic error, including unmarked operations inside unsafe fn.
// Rules: rules/memory/raw_pointers.md — §§5.2,11–15,35;
// rules/compiler/parser_recovery.md — "Error blocks code generation".
func TestRawPointerSafetyFixtures(t *testing.T) {
	for _, test := range []struct {
		file  string
		valid bool
	}{
		{"raw_pointer_registry_valid.sec", true},
		{"raw_pointer_signature_nested_valid.sec", true},
		{"raw_pointer_safety_context_invalid.sec", false},
		{"raw_pointer_unsafe_body_invalid.sec", false},
		{"raw_pointer_signature_eof_invalid.sec", false},
		{"raw_pointer_signature_statement_invalid.sec", false},
		{"raw_pointer_signature_closer_invalid.sec", false},
		{"raw_pointer_signature_unknown_invalid.sec", false},
		{"raw_pointer_signature_conversion_invalid.sec", false},
	} {
		t.Run(test.file, func(t *testing.T) {
			path := "../../testdata/tooling/" + test.file
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
			if parsed.HasErrors {
				if test.valid {
					t.Fatalf("canonical syntax: %+v", parsed.Diagnostics)
				}
				return
			}
			errors := NewAnalyzer().Analyze(parsed.Program)
			if test.valid && len(errors) > 0 {
				t.Fatalf("canonical semantics: %+v", errors)
			}
			if !test.valid && len(errors) == 0 {
				t.Fatal("invalid fixture was accepted")
			}
		})
	}
}
