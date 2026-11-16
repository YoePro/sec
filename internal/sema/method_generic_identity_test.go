package sema

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMethodInstantiationIdentityAndDemand separates enclosing and local type
// arguments, preserves phantom identity, composes forwarded method calls and
// excludes unused templates and speculative cache entries from program demand.
// Rules: rules/compiler/monomorphization.md — §§4,6–9;
// rules/declarations/generics.md — §§9,25–26.
func TestMethodInstantiationIdentityAndDemand(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generics/method_identity.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "method_identity.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				identities := map[string]InstantiationIdentity{}
				for _, statement := range program.Statements {
					fn, ok := statement.(*ast.FunctionDeclaration)
					if !ok {
						continue
					}
					call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
					target, known := a.ResolvedCallTarget(call)
					if !known {
						t.Fatal("missing target", fn.Name.Value)
					}
					identity, known := a.InstantiationIdentityOf(target.Function)
					if !known {
						t.Fatal("missing identity", fn.Name.Value)
					}
					if target.Function.InstantiationIdentity != identity.Key || callableID(target.Function) != CallableID("instantiation|"+identity.Key) {
						t.Fatal("call target and graph disagree with canonical identity", fn.Name.Value)
					}
					identities[fn.Name.Value] = identity
				}
				if identities["First"].Key != identities["Repeat"].Key {
					t.Fatal("explicit and inferred requests differ")
				}
				for _, pair := range [][2]string{{"First", "Other"}, {"FirstStatic", "OtherStatic"}, {"FirstSize", "OtherSize"}, {"FirstForward", "OtherForward"}} {
					if identities[pair[0]].Key == identities[pair[1]].Key {
						t.Fatal("phantom owner erased", pair)
					}
				}
				first := identities["First"]
				if !reflect.DeepEqual(first.OwnerArguments, []string{canonicalTypeIdentity(a.types["int"]), canonicalTypeIdentity(a.types["bool"])}) || !reflect.DeepEqual(first.Arguments, []string{canonicalTypeIdentity(a.types["string"]), canonicalTypeIdentity(a.types["int"])}) {
					t.Fatalf("lost declaration order: %+v", first)
				}
				demands, err := a.InstantiationDemands()
				if err != nil {
					t.Fatal(err)
				}
				if len(demands) != 11 {
					t.Fatalf("got %d demands, want 11: %+v", len(demands), demands)
				}
				repeated, nested := 0, 0
				seen := map[string]bool{}
				for _, demand := range demands {
					seen[demand.Identity.Key] = true
					if demand.Identity.Key == first.Key {
						repeated = len(demand.Causes)
					}
					for _, cause := range demand.Causes {
						if cause.Caller != "" {
							nested++
						}
					}
				}
				if repeated != 2 || nested != 2 {
					t.Fatalf("repeat/nested causes = %d/%d", repeated, nested)
				}
				for name, identity := range identities {
					if !seen[identity.Key] {
						t.Fatal("missing direct demand", name)
					}
				}
				unresolved := a.functions["Plain.Pass"][0]
				unresolved.GenericSubstitution = map[string]Type{"U": {Kind: FunctionType, FunctionParameterTypes: []Type{{Name: "X", Kind: GenericType}}}}
				if _, known := a.InstantiationIdentityOf(unresolved); known {
					t.Fatal("symbolic callback acquired a concrete identity")
				}
				template := a.functions["Plain.Pass"][0]
				a.instantiateGenericFunction(template, map[string]Type{"U": a.types["bool"]})
				cacheSize := len(a.genericFuncInstances)
				after, err := a.InstantiationDemands()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(demands, after) || cacheSize != len(a.genericFuncInstances) {
					t.Fatal("cache/query changed active demand")
				}
				demands[0].Identity.Arguments = append(demands[0].Identity.Arguments, "changed")
				demands[0].Causes[0].Caller = "changed"
				again, err := a.InstantiationDemands()
				if err != nil || !reflect.DeepEqual(after, again) {
					t.Fatal("publication aliases query state", err)
				}
				a.Analyze(&ast.Program{})
				empty, err := a.InstantiationDemands()
				if err != nil || len(empty) != 0 {
					t.Fatal("stale demand", empty, err)
				}
			})
		}
	}
}

// TestMethodInstantiationIdentityRetainsEffects checks that separating phantom
// identities does not sever the specialization's template effects, including
// methods whose only generic parameters belong to their owner.
// Rules: rules/compiler/monomorphization.md — §§6.4,12.4–12.5;
// rules/errors/panic.md — §21.
func TestMethodInstantiationIdentityRetainsEffects(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generics/method_identity_effects_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "method_identity_effects_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 2 {
		t.Fatalf("expected two noPanic failures: %+v", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.NoPanicViolation {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
	}
}
