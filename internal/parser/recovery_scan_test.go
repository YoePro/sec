package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Specialized skips must expose the real source they discard and keep later
// declarations. EOF and lookahead boundaries never belong to a skipped range.
// Rules: rules/compiler/parser_recovery.md — "Recovery event", "Enclosing-boundary safety", Appendix A.10.
func TestSpecializedRecoveryRecordsSourceRanges(t *testing.T) {
	const fixture = "../../testdata/parser/specialized_skip_ranges_invalid.sec"
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("invalid fixture accepted")
	}
	names := map[string]bool{}
	var header *ast.FunctionDeclaration
	for _, statement := range result.Program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok {
			names[function.Name.Value] = true
			if function.Name.Value == "Header" {
				header = function
			}
		}
	}
	for _, name := range []string{"Chained", "Pattern", "Arms", "Attributed", "After", "Header"} {
		if !names[name] {
			t.Errorf("lost following function %s", name)
		}
	}
	if header == nil || header.Body == nil || len(header.Body.Statements) != 1 {
		t.Fatalf("lost recovered header: %#v", header)
	}
	branch, ok := header.Body.Statements[0].(*ast.IfStatement)
	if !ok || branch.Consequence == nil || len(branch.Consequence.Statements) != 1 {
		t.Fatalf("nested literal was mistaken for the if body: %#v", header.Body.Statements)
	}
	if _, ok := branch.Consequence.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("lost true if body: %#v", branch.Consequence)
	}
	// Each malformed region below uses a different specialized helper.
	wantSkipLines := map[int]bool{3: false, 4: false, 7: false, 12: false, 19: false, 24: false}
	tokens := lexSourceTokens(string(source))
	for _, event := range result.Recovery {
		if event.Kind != RecoverySkipTokens {
			continue
		}
		if _, covered := wantSkipLines[event.Start.Line]; covered {
			wantSkipLines[event.Start.Line] = true
		}
		if event.Skipped == 0 || event.Start.Type == lexer.EOF || event.End.Type == lexer.EOF || event.End.ByteEnd < event.Start.ByteStart {
			t.Errorf("fabricated skip: %+v", event)
		}
		count := 0
		for _, token := range tokens {
			if token.start >= event.Start.ByteStart && token.end <= event.End.ByteEnd {
				count++
			}
		}
		if count != event.Skipped {
			t.Errorf("skip count = %d, real source tokens = %d: %+v", event.Skipped, count, event)
		}
	}
	for line, seen := range wantSkipLines {
		if !seen {
			t.Errorf("line %d has no specialized skip event: %+v", line, result.Recovery)
		}
	}
}

// A mismatched closer remains with its owner, and an unfinished attribute list
// must not consume the next top-level function while hunting for a ')'.
// Rules: rules/compiler/parser_recovery.md — "Enclosing-boundary safety", "Bounded damage".
func TestSpecializedRecoveryPreservesForeignBoundary(t *testing.T) {
	const fixture = "../../testdata/parser/specialized_skip_boundaries_invalid.sec"
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("invalid fixture accepted")
	}
	found := false
	for _, statement := range result.Program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "After" {
			found = true
		}
	}
	if !found {
		t.Fatalf("attribute recovery lost After: %v", result.Diagnostics)
	}
	for _, event := range result.Recovery {
		if event.Kind == RecoverySkipTokens && event.Start.Line == 4 && event.End.Type == lexer.RBRACE {
			t.Errorf("mismatched outer brace was consumed by chained-assignment recovery: %+v", event)
		}
		if event.Kind == RecoverySkipTokens && event.Start.Line == 7 && event.End.Line >= 8 {
			t.Errorf("attribute recovery consumed next declaration: %+v", event)
		}
	}
}
