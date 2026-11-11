package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCancellationContextSurface checks exact opaque owners and member types,
// including temporal distinctions, borrow result and non-consuming Cancel.
// Rules: rules/concurrency/cancellation.md — §§35–38,43.
func TestCancellationContextSurface(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/context_surface_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if mode == "invalid" {
			if len(errors) != 14 {
				t.Fatalf("invalid diagnostics (%d): %v", len(errors), errors)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, name := range []string{"Context", "ContextSource"} {
			typ := a.Types()[name]
			if typ.Module != "core" || !typ.Intrinsic || CopyClassificationOf(typ) != CopyMoveOnly || TriviallyDestructible(typ) || len(typ.Fields) != 0 {
				t.Fatalf("%s: module=%s intrinsic=%v copy=%v trivial=%v fields=%v", name, typ.Module, typ.Intrinsic, CopyClassificationOf(typ), TriviallyDestructible(typ), typ.Fields)
			}
		}
		// Registration of an infallible signature is not a proof that the
		// future runtime helper cannot panic, allocate or block.
		graph := a.CallGraph()
		for _, name := range []string{"Root", "Child", "Timed", "Deadline", "Request"} {
			if !graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).MayPanic {
				t.Fatal("lost unknown runtime effect", name)
			}
		}
	}
	for _, tc := range []struct {
		owner, name, result string
		static              bool
	}{
		{"Context", "Background", "Context", true},
		{"Context", "WithCancel", "ContextSource", false},
		{"Context", "WithTimeout", "ContextSource", false},
		{"Context", "WithDeadline", "ContextSource", false},
		{"Context", "IsCancelled", "bool", false},
		{"Context", "Error", "Option[ContextError]", false},
		{"Context", "Deadline", "Option[Instant]", false},
		{"ContextSource", "Context", "ref Context", false},
		{"ContextSource", "Cancel", "void", false},
	} {
		member, ok := compilerKnownMember(builtinType(tc.owner), tc.name, tc.static)
		if !ok || typeDisplayName(member.Result) != tc.result || !strings.Contains(member.Rule, "cancellation.md") {
			t.Fatal(tc, member, ok)
		}
		cold, ok := CompilerKnownIntrinsicSymbolByID(member.ID)
		if !ok || cold.Signature != member.Signature {
			t.Fatal("cold member", member, cold)
		}
	}
}

// TestCancellationContextSourceCore requires exact trusted opaque metadata and
// rejects untrusted replacement or a malformed core declaration.
// Rules: rules/concurrency/cancellation.md — §36.
func TestCancellationContextSourceCore(t *testing.T) {
	const file = "../../sec/core/context.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range []string{"@noCopy\ntype ContextSource", "type ContextSource", "@noCopy\ntype ContextSource struct { Value: int }", "enum ContextSource { Value }"} {
		source := strings.Replace(string(data), "@noCopy\ntype ContextSource", declaration, 1)
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if declaration == "@noCopy\ntype ContextSource" {
			if len(errors) != 0 || a.Types()["ContextSource"].DeclarationToken.File != file {
				t.Fatal(errors)
			}
		} else if len(errors) == 0 {
			t.Fatal("accepted malformed source", declaration)
		}
	}
}

// TestCancellationContextBorrowEscape retains the property receiver's origin
// for ordinary local-reference escape checking; it cannot create a new owner.
// Rules: rules/concurrency/cancellation.md — §§35(4),38(1–2).
func TestCancellationContextBorrowEscape(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/context_borrow_escape_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "local") {
		t.Fatal(errors)
	}
}
