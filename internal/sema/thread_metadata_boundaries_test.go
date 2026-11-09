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

// TestThreadMetadataBoundaries checks exact portable termination metadata and
// rejects hidden observer wait/select without changing metadata-only uses.
// Rules: rules/concurrency/threads.md — §§23,40,43.
func TestThreadMetadataBoundaries(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_termination_valid.sec", 0}, {"thread_termination_invalid.sec", 3},
		{"thread_observer_wait_valid.sec", 0}, {"thread_observer_wait_invalid.sec", 3},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		kind := a.Types()["ThreadTerminationKind"]
		metadata := a.Types()["ThreadTermination"]
		if kind.ErrorAssignable || !kind.Named || kind.Module != "core" || !reflect.DeepEqual(kind.EnumValues, []string{"Requested", "External", "Fault", "Unknown"}) || len(metadata.Fields) != 1 || metadata.Fields[0].Name != "Kind" || !sameConcreteType(metadata.Fields[0].Type, kind) {
			t.Fatal(kind, metadata)
		}
		if strings.Contains(tc.file, "wait_invalid") {
			for _, err := range errors {
				if !strings.Contains(err.Message, "ThreadObserver has no wait or selectable completion capability") || !strings.Contains(err.Help, "owning Thread[T]") {
					t.Fatal(err)
				}
			}
		}
	}
}

// TestThreadTerminationCore validates canonical source refinement, exact
// metadata fields and rejection of noncanonical native status representations.
// Rules: rules/concurrency/threads.md — §23(1–4).
func TestThreadTerminationCore(t *testing.T) {
	const file = "../../testdata/sema/thread_termination_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	enumDeclaration := source[strings.Index(source, "enum ThreadTerminationKind"):strings.Index(source, "type ThreadTermination struct")]
	for _, tc := range []struct{ old, replacement string }{
		{"", ""}, {enumDeclaration, "interface ThreadTerminationKind {}\n\n"}, {"Fault\n", "Native\n"}, {"Kind: ThreadTerminationKind", "Kind: int"},
		{"Kind: ThreadTerminationKind", "Kind: ref ThreadTerminationKind"},
		{"Kind: ThreadTerminationKind,", "Kind: ThreadTerminationKind,\n    NativeCode: int,"},
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
			if a.Types()["ThreadTermination"].Fields[0].Token.File != file {
				t.Fatal("lost field source identity")
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical core") {
			t.Fatal(tc, errors)
		}
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, strings.Replace(source, "module core", "module main", 1))
	if len(errors) != 2 {
		t.Fatal(errors)
	}
}
