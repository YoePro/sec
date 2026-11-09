package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestThreadLifecycleErrors checks the exact closed enum inventories, nominal
// error identities, exhaustive matches and rejection of obsolete variants.
// Rules: rules/concurrency/threads.md — §§18–19.
func TestThreadLifecycleErrors(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_lifecycle_errors_valid.sec", 0}, {"thread_lifecycle_errors_invalid.sec", 3},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		for name, want := range map[string][]string{
			"ThreadSpawnError": {"OutOfMemory", "ResourceLimit", "StackAllocationFailed", "InvalidConfiguration", "PermissionDenied", "ThreadLocalInitializationFailed", "NativeFailure"},
			"ThreadStartError": {"InvalidState", "ResourceUnavailable", "PermissionDenied", "NativeFailure"},
		} {
			typ := a.Types()[name]
			if !typ.Named || !typ.Intrinsic || !typ.ErrorAssignable || typ.Module != "core" || !reflect.DeepEqual(typ.EnumValues, want) || len(typ.EnumConsts) != len(want) {
				t.Fatal(name, typ)
			}
		}
		for _, err := range errors {
			if !strings.Contains(err.Message, "has never been declared") {
				t.Fatal(err)
			}
		}
	}
}

// TestThreadLifecycleErrorsCore checks real source tokens, exact declaration
// validation and reserved identities without reading or modifying core sources.
// Rules: rules/concurrency/threads.md — §§18–19;
// rules/library/core-library.md — compiler-known core declarations.
func TestThreadLifecycleErrorsCore(t *testing.T) {
	const file = "../../testdata/sema/thread_lifecycle_errors_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ old, replacement string }{
		{"", ""}, {"OutOfMemory", "Unsupported"}, {"InvalidState", "Unsupported"},
		{"ThreadSpawnError error", "ThreadSpawnError"}, {"ThreadStartError error", "ThreadStartError"},
	} {
		input := source
		if tc.old != "" {
			input = strings.Replace(source, tc.old, tc.replacement, 1)
		}
		p := parser.New(lexer.NewWithFile(input, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if tc.old == "" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			for _, name := range []string{"ThreadSpawnError", "ThreadStartError"} {
				typ := a.Types()[name]
				if typ.EnumConsts[typ.EnumValues[0]].Token.File != file {
					t.Fatal("lost source identity", typ)
				}
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "exact canonical core error enum") {
			t.Fatal(tc, errors)
		}
		for _, name := range []string{"ThreadSpawnError", "ThreadStartError"} {
			if !sameConcreteType(a.Types()[name], builtinType(name)) {
				t.Fatal(a.Types()[name])
			}
		}
	}
	for _, input := range []string{source, strings.Replace(source, "module core", "module main", 1)} {
		_, errors := analyzeSourceWithAnalyzerRaw(t, input)
		if len(errors) != 2 {
			t.Fatal(errors)
		}
		for _, err := range errors {
			if !strings.Contains(err.Message, "compiler-known and cannot be redeclared") {
				t.Fatal(err)
			}
		}
	}
}
