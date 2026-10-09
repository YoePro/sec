package interfacecatalog_test

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func parse(t *testing.T, name string) *ast.Program {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/sema/interface_catalog/" + name)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), name))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

// TestCanonicalIteratorRegistry drives declarations, ordinary generic
// constraints and concrete loop dispatch from the same compiler-owned contract.
// Rules: rules/declarations/interfaces.md — §9.1;
// rules/control-flow/flowcontrol_for.md — §37.
func TestCanonicalIteratorRegistry(t *testing.T) {
	entries := sema.CompilerKnownInterfaces()
	if len(entries) != 1 {
		t.Fatalf("unexpected registry: %#v", entries)
	}
	iface := entries[0]
	if iface.Name != "Iterator" || !iface.Intrinsic || iface.Kind != sema.InterfaceType || len(iface.GenericParameters) != 1 || len(iface.InterfaceMethods) != 1 {
		t.Fatalf("wrong interface: %#v", iface)
	}
	required := iface.InterfaceMethods[0]
	if required.CompilerKnownID != "CKM-ITERATOR-NEXT" || required.Name != "Next" || !required.ReceiverMutable || required.ReturnType.Name != "Option" || required.ReturnType.TypeArgs[0].Name != "T" {
		t.Fatalf("wrong requirement: %#v", required)
	}
	program := parse(t, "iterator.sec")
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	found := false
	if err := astwalk.Inspect(program, func(node any) error {
		if loop, ok := node.(*ast.ForStatement); ok {
			plan, ok := a.ResolvedForIterationOf(loop)
			if !ok || plan.Next.CompilerKnownID != required.CompilerKnownID || plan.Next.Name != "Counter.Next" || plan.ElementType.Name != "int" || plan.Conformance.Name != iface.Name {
				t.Fatalf("registry identity lost: %#v", plan)
			}
			found = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("no resolved concrete iteration")
	}
}

// TestCompilerInterfaceRegistryIsolation prevents caller edits or conformance
// source anchors from poisoning shared registry requirements in another analyzer.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/declarations/interfaces.md — §9.1 compiler-owned requirements.
func TestCompilerInterfaceRegistryIsolation(t *testing.T) {
	first := sema.NewAnalyzer()
	interfaces := sema.CompilerKnownInterfaces()
	interfaces[0].GenericParameters[0] = "Changed"
	interfaces[0].InterfaceMethods[0].ReturnType.TypeArgs[0].Name = "Changed"
	typ := first.Types()["Iterator"]
	typ.InterfaceMethods[0].Name = "Changed"
	second := sema.NewAnalyzer().Types()["Iterator"]
	if second.InterfaceMethods[0].Name != "Next" || second.InterfaceMethods[0].ReturnType.TypeArgs[0].Name != "T" || sema.CompilerKnownInterfaces()[0].GenericParameters[0] != "T" {
		t.Fatalf("registry state leaked: %#v", second)
	}
}

// TestCanonicalIteratorConformanceDiagnostics verifies standard conformance
// and reservation diagnostics, with separate source anchors for each clause.
// Rules: rules/declarations/interfaces.md — §§6, 9.1, 12.
func TestCanonicalIteratorConformanceDiagnostics(t *testing.T) {
	for _, name := range []string{"wrong_result_invalid.sec", "redeclared_invalid.sec", "missing_invalid.sec"} {
		t.Run(name, func(t *testing.T) {
			a := sema.NewAnalyzer()
			errs := a.Analyze(parse(t, name))
			if len(errs) == 0 {
				t.Fatal("invalid compiler interface declaration accepted")
			}
			for _, err := range errs {
				if err.File != name || err.Line == 0 {
					t.Fatalf("unmapped failure: %#v", err)
				}
			}
			if name == "missing_invalid.sec" {
				if len(errs) != 2 {
					t.Fatal(errs)
				}
				owners := map[int]bool{}
				for _, err := range errs {
					if !strings.Contains(err.Message, "missing method Next") || err.PreviousLine == 0 {
						t.Fatal(errs)
					}
					owners[err.PreviousLine] = true
				}
				if len(owners) != 2 {
					t.Fatalf("implements clause anchors merged: %v", errs)
				}
			}
		})
	}
}

// Rules: rules/declarations/interfaces.md §§6,9.1;
// rules/compiler/compiler_known_members.md — Stable member identity, LSP.
func TestInterfaceRequirementToolingFacts(t *testing.T) {
	for _, name := range []string{"iterator.sec", "ordinary_next.sec", "next_arity_invalid.sec"} {
		t.Run(name, func(t *testing.T) {
			program := parse(t, name)
			a := sema.NewAnalyzer()
			errors := a.Analyze(program)
			if (len(errors) > 0) != (name == "next_arity_invalid.sec") {
				t.Fatalf("errors: %v", errors)
			}
			found := false
			if err := astwalk.Inspect(program, func(node any) error {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return nil
				}
				selector, ok := call.Callee.(*ast.MemberExpression)
				if !ok || selector.Property.Value != "Next" {
					return nil
				}
				found = true
				token := selector.Property.Token
				member, known := a.CompilerKnownSymbolAt(token.File, token.Line, token.Column)
				if known != (name == "iterator.sec") {
					t.Fatalf("authority: %#v, %v", member, known)
				}
				if known {
					target, ok := a.ResolvedCallTarget(call)
					if !ok || target.Function.CompilerKnownID != member.ID || member.ID != "CKM-ITERATOR-NEXT" || len(member.Result.TypeArgs) != 1 || member.Result.TypeArgs[0].Name != "int" {
						t.Fatalf("identity: %#v / %#v", member, target)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("no requirement call")
			}
		})
	}
}
