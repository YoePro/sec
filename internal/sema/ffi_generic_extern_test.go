package sema

import (
	"os"
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

// TestGenericExternABIBoundary covers unused parameters and preserves concrete
// foreign providers called from generic Sec wrappers.
// Rules: rules/compiler/monomorphization.md — §16.1, §16.4–16.5.
func TestGenericExternABIBoundary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/generic_extern_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 3 {
		t.Fatal(errors)
	}
	for i, diagnostic := range errors {
		if diagnostic.ID != diagnostics.UnresolvedGenericExtern || diagnostic.ProofState != diagnostics.ProofInvalid || diagnostic.PreviousLine != i+3 || diagnostic.RelatedLabel != "generic extern declaration" || diagnostic.EndColumn <= diagnostic.Column {
			t.Fatal(diagnostic)
		}
	}
	if !strings.Contains(errors[0].Help, "Unresolved parameters: T, U.") {
		t.Fatal(errors[0])
	}
	for _, name := range []string{"Convert", "PlatformValue", "Phantom"} {
		if len(a.Functions()[name]) != 0 {
			t.Fatal("unresolved extern entered callable catalog", name)
		}
	}
	if len(a.Functions()["Plain"]) != 1 {
		t.Fatal("concrete extern lost during recovery")
	}
	data, err = os.ReadFile("../../testdata/tooling/generic_extern_wrapper_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if _, errors := analyzeSourceWithAnalyzer(t, string(data)); len(errors) != 0 {
		t.Fatal(errors)
	}
}
