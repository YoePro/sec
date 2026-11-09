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

// TestThreadConfigurationEnums verifies closed non-error identities, ordinary
// defaults, exhaustive matching and distinct start-mode/priority types.
// Rules: rules/concurrency/threads.md — §§5–6;
// rules/types/default_values.md — "Enums".
func TestThreadConfigurationEnums(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_configuration_enums_valid.sec", 0}, {"thread_configuration_enums_invalid.sec", 4},
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
			"ThreadStartMode": {"Eager", "Deferred"},
			"ThreadPriority":  {"Lowest", "Low", "Normal", "High", "Highest"},
		} {
			typ := a.Types()[name]
			if !typ.Named || !typ.Intrinsic || typ.ErrorAssignable || typ.Module != "core" || typ.EnumDefault != want[0] || !reflect.DeepEqual(typ.EnumValues, want) || len(typ.EnumConsts) != len(want) {
				t.Fatal(name, typ)
			}
		}
	}
}

// TestThreadConfigurationEnumsCore checks canonical source refinement, exact
// variant order and error-marker validation, and untrusted redeclarations.
// Rules: rules/concurrency/threads.md — §§5–6;
// rules/library/core-library.md — compiler-known core declarations.
func TestThreadConfigurationEnumsCore(t *testing.T) {
	const file = "../../testdata/sema/thread_configuration_enums_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ old, replacement string }{
		{"", ""}, {"Eager", "Immediate"}, {"\n    Lowest\n", "\n    Realtime\n"},
		{"ThreadStartMode {", "ThreadStartMode error {"}, {"ThreadPriority {", "ThreadPriority error {"},
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
			for _, name := range []string{"ThreadStartMode", "ThreadPriority"} {
				typ := a.Types()[name]
				if typ.EnumConsts[typ.EnumValues[0]].Token.File != file {
					t.Fatal("lost source identity", typ)
				}
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "exact canonical core enum") {
			t.Fatal(tc, errors)
		}
		for _, name := range []string{"ThreadStartMode", "ThreadPriority"} {
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
