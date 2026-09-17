package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryHandlerShadowDiagnostics verifies that a provably shadowed error
// handler points to its source and the earlier covering handler, or the last
// arm that completed coverage of a closed enum error.
// Rules: rules/errors/errorhandling.md — §18 "Handler order and reachability";
// rules/tooling/diagnostics.txt — "Error-handling diagnostics".
func TestTryHandlerShadowDiagnostics(t *testing.T) {
	path := "../../testdata/sema/try_handler_shadow_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser diagnostics = %+v", parsed.Diagnostics)
	}
	errors := NewAnalyzer().Analyze(parsed.Program)
	if len(errors) != 3 {
		t.Fatalf("errors = %+v, want three shadowed handlers", errors)
	}
	wants := []struct {
		line, coveringLine int
		message            string
	}{
		{15, 14, "earlier Err(NotFound)"},
		{23, 22, "earlier Err catch-all"},
		{31, 30, "earlier Err handlers already cover every variant"},
	}
	for index, want := range wants {
		got := errors[index]
		if got.ID != diagnostics.UnreachableTryHandler || got.Line != want.line ||
			got.PreviousLine != want.coveringLine || got.PreviousFile != path ||
			!strings.Contains(got.Message, want.message) {
			t.Errorf("error %d = %+v, want S1045 at line %d covered by line %d", index, got, want.line, want.coveringLine)
		}
	}
}

// TestTryHandlerPartialEnumCatchAllRemainsReachable ensures that explicit
// variant arms do not shadow a catch-all while an enum error has alternatives.
// Rule: rules/errors/errorhandling.md — §18 "Handler order and reachability".
func TestTryHandlerPartialEnumCatchAllRemainsReachable(t *testing.T) {
	path := "../../testdata/sema/try_handler_partial_enum_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser diagnostics = %+v", parsed.Diagnostics)
	}
	if errors := NewAnalyzer().Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("partial enum errors = %+v", errors)
	}
}
