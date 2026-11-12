package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPortableThreadTerminationBoundary rejects both legacy chained calls and
// direct/captured termination methods while preserving owner join state and
// ordinary explicitly declared APIs on other receivers.
// Rules: rules/concurrency/threads.md — §§40(6),52–53.
func TestPortableThreadTerminationBoundary(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		file := "../../testdata/sema/thread_termination_boundary_" + mode + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		want := 0
		if mode == "invalid" {
			want = 9
		}
		if len(errors) != want {
			t.Fatal(mode, errors)
		}
		for _, err := range errors {
			if err.ID != diagnostics.ThreadTerminationNotPortable || err.Severity != diagnostics.SeverityError || err.File != file || err.Line == 0 || err.EndColumn <= err.Column || !strings.Contains(err.Help, "RequestCancel()") || !strings.Contains(err.Help, "unsafe target-specific API") {
				t.Fatal(err)
			}
			if !strings.Contains(err.Message, "platform") && !strings.Contains(err.Message, "Terminate") {
				t.Fatal(err)
			}
		}
		// No rejected call acquires an executable compiler-known member ID.
		if mode == "invalid" {
			ast.Visit(program, func(node any) bool {
				if spawn, ok := node.(*ast.SpawnExpression); ok {
					typ, resolved := a.ResolvedTypeOf(spawn)
					if !resolved || typ.Name != "Thread" || !typ.Intrinsic {
						t.Fatal("spawn lost canonical thread identity", typ, resolved)
					}
				}
				member, ok := node.(*ast.MemberExpression)
				if ok && (member.Property.Value == "Terminate" || member.Property.Value == "platform") {
					if _, found := a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]; found {
						t.Fatal("fabricated termination fact", member)
					}
				}
				return true
			})
		}
		for _, name := range []string{"Thread", "ThreadObserver"} {
			typ := a.Types()[name]
			typ.TypeArgs = []Type{a.Types()["void"]}
			for _, static := range []bool{false, true} {
				for _, member := range CompilerKnownMembersForType(typ, static) {
					if member.Name == "Terminate" || member.Name == "platform" {
						t.Fatal("portable termination leaked into catalog", member)
					}
				}
			}
		}
	}
	definition, ok := diagnostics.Lookup(diagnostics.ThreadTerminationNotPortable)
	if !ok || !definition.Mandatory || definition.Name != "thread.termination-not-portable" {
		t.Fatal(definition, ok)
	}
}
