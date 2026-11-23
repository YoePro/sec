package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestGenericImplComposition checks complete substituted surfaces independently
// of the primary/extension source order and of early concrete type requests.
// Rules: rules/declarations/impl.md §§21–22;
// rules/declarations/generics.md §§8–9,17.
func TestGenericImplComposition(t *testing.T) {
	for _, order := range [][]string{{"primary", "extension"}, {"extension", "primary"}} {
		program := &ast.Program{}
		for _, name := range order {
			path := "../../testdata/sema/generic_impl_composition/" + name + ".sec"
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(source), path))
			part := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			program.Statements = append(program.Statements, part.Statements...)
		}
		a := NewAnalyzer()
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatalf("%v: %v", order, errors)
		}
		for _, name := range []string{"int", "bool"} {
			typ := a.types["Box"]
			typ.TypeArgs = []Type{a.types[name]}
			instance := a.instantiateGenericType(typ)
			if len(instance.Properties) != 1 || typeDisplayName(instance.Properties[0].Type) != name || len(instance.Events) != 1 || typeDisplayName(instance.Events[0].Payload) != name {
				t.Fatalf("%v: incomplete %s instance: %+v", order, name, instance)
			}
		}
		identities := map[string]string{}
		for expression, plan := range a.resolvedConstructions {
			if expression.Type.Name != "Box" && expression.Type.Name != "Fallible" {
				continue
			}
			argument := expression.Type.TypeArgs[0].Name
			if typeDisplayName(plan.Initializer.Parameters[0].Type) != argument || len(plan.Target.TypeArgs) == 0 || typeDisplayName(plan.Target.TypeArgs[0]) != argument {
				t.Fatalf("lost constructed owner substitution: %+v", plan)
			}
			preview := a.ConstructionSignaturesOf(expression)
			if len(preview) == 0 || typeDisplayName(preview[0].Parameters[0].Type) != argument {
				t.Fatalf("unsubstituted preview: %+v", preview)
			}
			if expression.Type.Name == "Fallible" {
				want := expression.Type.TypeArgs[1].Name
				if plan.ErrorType == nil || plan.Initializer.ConstructionError == nil || typeDisplayName(*plan.ErrorType) != want || typeDisplayName(*plan.Initializer.ConstructionError) != want {
					t.Fatalf("unsubstituted construction error: %+v", plan)
				}
			}
			if argument == "int" || argument == "bool" {
				identity, ok := a.InstantiationIdentityOf(plan.Initializer)
				if !ok || identity.Role != "initializer" || len(identity.OwnerArguments) == 0 {
					t.Fatalf("missing init specialization identity: %+v", identity)
				}
				identities[expression.Type.Name+argument] = identity.Key
			}
		}
		if identities["Boxint"] == "" || identities["Boxbool"] == "" || identities["Boxint"] == identities["Boxbool"] {
			t.Fatalf("conflated constructor instances: %v", identities)
		}
		if got := typeDisplayName(a.functions[initFunctionName("Box")][0].Parameters[0].Type); got != "T" {
			t.Fatalf("template mutated to %s", got)
		}
		demands, err := a.InstantiationDemands()
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, demand := range demands {
			seen[demand.Identity.Key] = true
		}
		if !seen[identities["Boxint"]] || !seen[identities["Boxbool"]] {
			t.Fatalf("constructor specializations lost from canonical demand graph: %+v", demands)
		}
	}
}
