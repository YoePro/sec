package interfaces_test

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

// analyzedCalls obtains actual source selections without reconstructing lookup.
// Rules: rules/declarations/functions.md — §§19,20,24; interfaces.md — §§5,6.
func analyzedCalls(t *testing.T, fixture string) (*sema.Analyzer, map[string]*ast.CallExpression, []sema.Error) {
	t.Helper()
	path := "../../../testdata/sema/interface_calls/" + fixture
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	a := sema.NewAnalyzer()
	errors := a.Analyze(parsed.Program)
	calls := map[string]*ast.CallExpression{}
	for _, stmt := range parsed.Program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		_ = astwalk.Inspect(fn.Body, func(node any) error {
			if call, ok := node.(*ast.CallExpression); ok {
				calls[fn.Name.Value] = call
			}
			return nil
		})
	}
	return a, calls, errors
}

// TestSelectedInterfaceOverloadFacts preserves direct/inherited type, arity,
// nominal identity and ownership distinctions in the selected invocation fact.
// Rules: rules/declarations/functions.md — §§19,20,24,37;
// rules/declarations/interfaces.md — §§5,6; correction10-20260823.md.
func TestSelectedInterfaceOverloadFacts(t *testing.T) {
	a, calls, errors := analyzedCalls(t, "selected.sec")
	if len(errors) > 0 {
		t.Fatal(errors)
	}
	expected := map[string][]string{"Integer": {"int"}, "Text": {"string"}, "Named": {"Tag"}, "Arity": {"int", "int"}, "Boolean": {"bool"}, "Fixed": {"int32"}, "Borrow": {"ref int"}, "Change": {"ref mut int"}, "Take": {"Box"}, "Variadic": {"int"}, "GenericInteger": {"int32"}, "GenericBoolean": {"bool"}, "InheritedGeneric": {"int32"}, "StaticInterface": {"int"}}
	identities := map[sema.CallableContractID]string{}
	for name, params := range expected {
		fact, ok := a.ResolvedCallTarget(calls[name])
		if !ok || fact.Kind != sema.ResolvedInterfaceMethodCall || fact.InterfaceContract == nil {
			t.Fatalf("%s: %+v %v", name, fact, ok)
		}
		contract := fact.InterfaceContract
		if contract.ID == "" || contract.Receiver == "" || contract.Operation != fact.Function.Name || len(fact.Function.Parameters) != len(params) || len(contract.Parameters) != len(params) || fact.Function.Token.File == "" {
			t.Fatalf("%s: %+v", name, fact)
		}
		if previous, duplicate := identities[contract.ID]; duplicate {
			t.Fatalf("%s and %s collapsed", previous, name)
		}
		identities[contract.ID] = name
		for i, param := range fact.Function.Parameters {
			if param.Type.Name != params[i] {
				t.Fatalf("%s: parameter %+v", name, param)
			}
			if contract.Parameters[i].Ref != (param.Ref || param.Type.Kind == sema.ReferenceType) || contract.Parameters[i].MutableRef != (param.MutableRef || (param.Type.Kind == sema.ReferenceType && param.Type.ReferenceMutable)) || contract.Parameters[i].Consuming != param.Consuming || contract.Parameters[i].Variadic != param.Variadic {
				t.Fatal(contract, param)
			}
		}
		if name == "Change" && !contract.ReceiverMutable {
			t.Fatal(contract)
		}
		if name == "Variadic" && !contract.Parameters[0].Variadic {
			t.Fatal(contract)
		}
		if name == "Borrow" && !contract.Parameters[0].Ref {
			t.Fatal(contract)
		}
		if name == "Take" && !contract.Parameters[0].Consuming {
			t.Fatal(contract)
		}
		// Public snapshots cannot replace a recorded selection or its contract.
		fact.Function.Parameters[0].Type.Name = "corrupted"
		contract.Parameters[0].TypeIdentity = "corrupted"
		contract.Operation = "corrupted"
		again, _ := a.ResolvedCallTarget(calls[name])
		if again.Function.Parameters[0].Type.Name != params[0] || again.InterfaceContract.Parameters[0].TypeIdentity == "corrupted" || again.InterfaceContract.Operation == "corrupted" {
			t.Fatal(again)
		}
	}
	fact, ok := a.ResolvedCallTarget(calls["DirectCall"])
	if !ok || fact.Kind != sema.ResolvedDirectCall || fact.InterfaceContract != nil {
		t.Fatal(fact, ok)
	}
	fact, ok = a.ResolvedCallTarget(calls["ConcreteCall"])
	if !ok || fact.Kind == sema.ResolvedInterfaceMethodCall || fact.InterfaceContract != nil {
		t.Fatal(fact, ok)
	}
	// Both consumers must use the identical invocation identity for each call.
	var sites []sema.CallSite
	graph := a.CallGraph()
	for _, node := range graph.Nodes() {
		sites = append(sites, graph.Outgoing(node.ID)...)
	}
	interfaceSites := 0
	for _, site := range sites {
		if site.Dispatch != sema.CallDispatchInterface {
			continue
		}
		interfaceSites++
		found := false
		for _, call := range calls {
			member, ok := call.Callee.(*ast.MemberExpression)
			if !ok || member.Property.Token.Line != site.Source.Line {
				continue
			}
			fact, ok := a.ResolvedCallTarget(call)
			if ok && fact.InterfaceContract != nil && fact.InterfaceContract.ID == site.TargetSet.OpenContract {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("graph contract diverged: %+v", site)
		}
	}
	if interfaceSites != len(expected) {
		t.Fatalf("interface graph sites = %d, want %d", interfaceSites, len(expected))
	}
}

// TestRejectedInterfaceCallsHaveNoSelection never publishes a guessed target
// when arguments or the receiver contract prevent one valid overload.
// Rules: rules/declarations/functions.md — §20; interfaces.md — §6.
func TestRejectedInterfaceCallsHaveNoSelection(t *testing.T) {
	a, calls, errors := analyzedCalls(t, "rejected_invalid.sec")
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for name, call := range calls {
		if fact, ok := a.ResolvedCallTarget(call); ok {
			t.Fatalf("%s retained %+v", name, fact)
		}
	}
	if !strings.Contains(errors[1].Message, "mutable receiver") {
		t.Fatal(errors)
	}
}

// TestAmbiguousInterfaceCallHasNoSelection preserves the ordinary ambiguity
// rule and does not invent a dispatch identity from an unordered overload set.
// Rules: rules/declarations/functions.md — §20; interfaces.md — §6.
func TestAmbiguousInterfaceCallHasNoSelection(t *testing.T) {
	a, calls, errors := analyzedCalls(t, "ambiguous_invalid.sec")
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "ambiguous call") {
		t.Fatal(errors)
	}
	if fact, ok := a.ResolvedCallTarget(calls["Ambiguous"]); ok {
		t.Fatal(fact)
	}
}
