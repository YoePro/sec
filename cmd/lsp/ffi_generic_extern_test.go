package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Rules: rules/platform/ffi.md — §48 "Generics", §53 "Diagnostics";
// rules/tooling/lsp.md — "One semantic source of truth".
func TestAnalyzePublishesUnresolvedGenericExtern(t *testing.T) {
	source := `module main
extern "C" fn Convert[T](value: T) T
`
	items := analyze("file:///tmp/sec-lsp-ffi-generic/main.sec", source)
	if len(items) != 1 {
		t.Fatalf("diagnostics = %+v, want one unresolved generic extern error", items)
	}
	item := items[0]
	if item.Code != diagnostics.UnresolvedGenericExtern || item.Severity != 1 || !strings.Contains(item.Message, "unresolved generic parameter") {
		t.Fatalf("diagnostic = %+v", item)
	}
	if item.Range.Start.Line != 1 || item.Range.End.Character <= item.Range.Start.Character {
		t.Fatalf("generic parameter range = %+v", item.Range)
	}
}
