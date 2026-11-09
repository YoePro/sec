package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestThreadSetting uses ordinary generic union construction and matching;
// neither plain values nor mismatched payloads acquire preference semantics.
// Rules: rules/concurrency/threads.md — §4(1–4).
func TestThreadSetting(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_setting_valid.sec", 0}, {"thread_setting_invalid.sec", 3},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		typ := a.Types()["ThreadSetting"]
		if typ.Kind != UnionType || typ.Module != "core" || !typ.Intrinsic || !typ.Named || typ.ErrorAssignable || len(typ.GenericParameters) != 1 || len(typ.UnionVariants) != 3 {
			t.Fatal(typ)
		}
		for i, name := range []string{"Default", "Preferred", "Required"} {
			variant := typ.UnionVariants[i]
			if variant.Name != name || (variant.Payload != nil) != (i != 0) {
				t.Fatal(variant)
			}
		}
	}
}

// TestThreadSettingCore verifies exact trusted union refinement, declaration
// source identities and rejection of compiler-known name replacement.
// Rules: rules/concurrency/threads.md — §4;
// rules/library/core-library.md — compiler-known core declarations.
func TestThreadSettingCore(t *testing.T) {
	const file = "../../testdata/sema/thread_setting_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ old, replacement string }{
		{"", ""}, {"Preferred(T)", "Preferred(int)"}, {"Required(T)", "Required(ref T)"},
		{"Default\n", "Default(T)\n"}, {" union {", " union error {"},
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
			if a.Types()["ThreadSetting"].UnionVariants[1].Token.File != file {
				t.Fatal("lost member source identity")
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "exact canonical core union") {
			t.Fatal(tc, errors)
		}
	}
	for _, input := range []string{source, strings.Replace(source, "module core", "module main", 1)} {
		_, errors := analyzeSourceWithAnalyzerRaw(t, input)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "compiler-known and cannot be redeclared") {
			t.Fatal(errors)
		}
	}
}
