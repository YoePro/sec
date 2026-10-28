package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP retains mandatory severity, marker/source range, parameter links and
// compiler-owned help; changing an API signature is never an automatic fix.
// Rules: rules/memory/copy_move.md — §28.1; rules/tooling/lsp.md — "Shared diagnostic model".
func TestMoveIntoBorrowedParameterLSP(t *testing.T) {
	const base = "../../testdata/sema/move_borrowed/"
	data, err := os.ReadFile(base + "calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := "file:///tmp/sec-move-borrowed/calls_invalid.sec"
	values := analyze(uri, string(data))
	if len(values) != 8 {
		t.Fatal(values)
	}
	lines := strings.Split(string(data), "\n")
	for i, e := range values {
		if e.Code != diagnostics.MoveIntoBorrowedParameter || e.Severity != 1 || !strings.Contains(e.Message, "without transferring ownership") || !strings.Contains(e.Message, "help: Use '") {
			t.Fatalf("lost mentor metadata: %+v", e)
		}
		source := lines[e.Range.Start.Line][e.Range.Start.Character:e.Range.End.Character]
		if !strings.HasPrefix(source, "<-") || len(source) <= 2 {
			t.Fatalf("wrong primary range %q", source)
		}
		if len(e.RelatedInformation) != 1 || e.RelatedInformation[0].Message != "borrowed parameter declared here" {
			t.Fatalf("lost parameter location: %+v", e)
		}
		if (i < 5 || i == 7) && e.RelatedInformation[0].Location.URI != uri {
			t.Fatalf("wrong local parameter location: %+v", e)
		}
		if (i == 5 || i == 6) && !strings.HasSuffix(e.RelatedInformation[0].Location.URI, "sec/core/string.sec") {
			t.Fatalf("wrong core parameter location: %+v", e)
		}
	}
	if fixes := ownershipCodeActions(uri, string(data), values); len(fixes) != 0 {
		t.Fatalf("unrequested API rewrite: %+v", fixes)
	}
	valid, err := os.ReadFile(base + "calls_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if values := analyze("file:///tmp/sec-move-borrowed/calls_valid.sec", string(valid)); len(values) != 0 {
		t.Fatalf("legal borrows/transfers: %v", values)
	}
}
