package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Rules: rules/platform/ffi.md — §48 "Generics", §52 "Sema requirements".
func TestExternRejectsUnresolvedGenericDeclaration(t *testing.T) {
	source := `module main

extern "C" fn Convert[T](value: T) T
extern "system" fn PlatformValue[U](value: U) void
extern "C" fn Plain(value: int32) int32
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, source)
	if len(errors) != 2 {
		t.Fatalf("errors = %+v, want one diagnostic per generic extern", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.UnresolvedGenericExtern || !strings.Contains(diagnostic.Message, "unresolved generic parameter") || diagnostic.Line == 0 || diagnostic.Column == 0 {
			t.Errorf("generic extern diagnostic = %+v", diagnostic)
		}
	}
	if len(analyzer.Functions()["Plain"]) != 1 {
		t.Fatal("later concrete extern declaration was not retained")
	}
	if len(analyzer.Functions()["Convert"]) != 0 || len(analyzer.Functions()["PlatformValue"]) != 0 {
		t.Fatal("invalid generic extern declarations entered the callable catalog")
	}
}
