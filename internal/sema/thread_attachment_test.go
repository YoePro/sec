package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The source attachment signature returns an opaque move-only token and the
// canonical closed attachment error, never a legacy ThreadContext result.
// Rules: rules/concurrency/thread_local.md §§29–33.
func TestThreadAttachmentSurface(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_attachment/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.types["ThreadAttachment"]
	if !isThreadAttachmentType(typ) || CopyClassificationOf(typ) != CopyMoveOnly || IsDefaultable(typ) || !compilerKnownRequiresDestruction(typ) {
		t.Fatal("wrong attachment traits", typ)
	}
	members := CompilerKnownMembersForType(a.types["Thread"], true)
	found := false
	for _, m := range members {
		if m.Name == "AttachCurrent" {
			found = true
			if m.Result.Name != "Result" || len(m.Result.TypeArgs) != 2 || !isThreadAttachmentType(m.Result.TypeArgs[0]) || m.Result.TypeArgs[1].Name != "ThreadAttachError" {
				t.Fatal(m)
			}
		}
	}
	if !found {
		t.Fatal("attachment absent from canonical member catalog")
	}
	data, err = os.ReadFile("../../testdata/sema/thread_attachment/calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors = analyzeSourceWithAnalyzerRaw(t, string(data))
	for _, want := range []string{"AttachCurrent expects 0 arguments", "does not take type arguments", "Task.AttachCurrent", "ThreadContext", "has no source constructor", "has no source struct constructor", "cannot copy"} {
		hit := false
		for _, e := range errors {
			if strings.Contains(e.Message, want) {
				hit = true
			}
		}
		if !hit {
			t.Fatalf("missing %q: %v", want, errors)
		}
	}
}

// Trusted source may refine only the exact opaque declaration; a lookalike
// never gains the token surface or bypasses canonical ownership traits.
// Rules: rules/concurrency/thread_local.md §30(1–5).
func TestThreadAttachmentCoreIdentity(t *testing.T) {
	const file = "thread-attachment-core.sec"
	data, err := os.ReadFile("../../testdata/sema/thread_attachment/core.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{"@noCopy\ntype ThreadAttachment", "type ThreadAttachment", "@noCopy\ntype ThreadAttachment struct { Value: int }", "enum ThreadAttachment { Fake }"} {
		source := strings.Replace(string(data), "@noCopy\ntype ThreadAttachment", replacement, 1)
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if replacement == "@noCopy\ntype ThreadAttachment" {
			if len(errors) != 0 || a.types["ThreadAttachment"].DeclarationToken.File != file {
				t.Fatal(errors)
			}
		} else if len(errors) == 0 {
			t.Fatal("invalid trusted declaration accepted", replacement)
		}
	}
	if len(threadAttachmentMembers(Type{Name: "ThreadAttachment", Module: "main", Kind: StructType}, false)) != 0 {
		t.Fatal("lookalike acquired intrinsic surface")
	}
}

// No runtime implementation is inferred from a source signature; explicit
// guarantees cannot silently assume unknown attachment effects are absent.
// Rules: rules/concurrency/thread_local.md §§31(5),53–54;
// rules/foundations/attributes.md — Effect guarantees.
func TestThreadAttachmentUnknownEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_attachment/effects_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 3 {
		t.Fatal(errors)
	}
}
