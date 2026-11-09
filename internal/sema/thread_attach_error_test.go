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

// TestThreadAttachError checks exact nominal identity, error compatibility,
// exhaustive matching and rejection of the removed NotAttached variant.
// Rules: rules/concurrency/thread_local.md — §29(1–3).
func TestThreadAttachError(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_attach_error_valid.sec", 0}, {"thread_attach_error_invalid.sec", 1},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		typ := a.Types()["ThreadAttachError"]
		if !typ.Intrinsic || !typ.Named || !typ.ErrorAssignable || typ.Module != "core" || !reflect.DeepEqual(typ.EnumValues, []string{"AlreadyAttached", "ResourceUnavailable", "NativeFailure"}) || len(typ.EnumConsts) != 3 {
			t.Fatal(typ)
		}
		if tc.errors > 0 && !strings.Contains(errors[0].Message, "ThreadAttachError.NotAttached has never been declared") {
			t.Fatal(errors)
		}
	}
}

// TestThreadAttachErrorCoreDeclaration verifies trusted refinement and fallback
// recovery, while untrusted source cannot redeclare the reserved identity.
// Rules: rules/concurrency/thread_local.md — §29;
// rules/library/core-library.md — compiler-known core declarations.
func TestThreadAttachErrorCoreDeclaration(t *testing.T) {
	const file = "../../testdata/sema/thread_attach_error_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, replacement := range []string{"", "NotAttached", "enum ThreadAttachError { AlreadyAttached, ResourceUnavailable, NativeFailure }", "type ThreadAttachError struct {}"} {
		input := source
		if replacement == "NotAttached" {
			input = strings.Replace(source, "AlreadyAttached", replacement, 1)
		} else if replacement != "" {
			input = "module core\n" + replacement + "\n"
		}
		p := parser.New(lexer.NewWithFile(input, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if replacement == "" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if a.Types()["ThreadAttachError"].EnumConsts["AlreadyAttached"].Token.File != file {
				t.Fatal("lost source identity")
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical core") {
			t.Fatal(replacement, errors)
		}
		if !sameConcreteType(a.Types()["ThreadAttachError"], builtinType("ThreadAttachError")) || len(a.Types()["ThreadAttachError"].EnumValues) != 3 {
			t.Fatal(a.Types()["ThreadAttachError"])
		}
	}
	for _, input := range []string{source, strings.Replace(source, "module core", "module main", 1)} {
		_, errors := analyzeSourceWithAnalyzerRaw(t, input)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "compiler-known and cannot be redeclared") {
			t.Fatal(errors)
		}
	}
}
