package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMethodGenericDeclarationScopes verifies that an instance/static method
// owns its parameter list while seeing the owner's constrained parameters.
// Sibling methods reuse U independently and neither scope leaks on exit.
// Rules: rules/declarations/generics.md — §§5.6,8–9,30.
func TestMethodGenericDeclarationScopes(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generics/method_scopes_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(data), "method_scopes_valid.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				for _, name := range []string{"Box.Select", "Box.SelectStatic", "Box.Nested", "Box.Sibling", "Converter.Identity", "Converter.IdentityStatic"} {
					group := a.Functions()[name]
					if len(group) != 1 {
						t.Fatalf("%s not registered uniquely: %+v", name, group)
					}
					fn := group[0]
					if len(fn.GenericParameters) != 1 || fn.GenericParameters[0] != "U" || fn.GenericTypePack != "" {
						t.Fatalf("%s: owner parameters merged into method parameters: %+v", name, fn)
					}
					if fn.Static != strings.HasSuffix(name, "Static") {
						t.Fatalf("%s lost static/instance category", name)
					}
				}
				selectFn := a.Functions()["Box.Select"][0]
				if selectFn.Parameters[0].Type.Kind != GenericType || selectFn.Parameters[0].Type.Name != "T" || selectFn.Parameters[1].Type.Name != "U" || selectFn.ReturnType.Name != "U" {
					t.Fatalf("combined signature lost: %+v", selectFn)
				}
				if got := GenericConstraintDisplays("T", selectFn.Parameters[0].Type.GenericConstraints); strings.Join(got, ",") != "Ranked" {
					t.Fatalf("owner constraints lost: %v", got)
				}
				if got := GenericConstraintDisplays("U", selectFn.GenericConstraints); strings.Join(got, ",") != "Named" {
					t.Fatalf("method constraints lost: %v", got)
				}
				nested := a.Functions()["Box.Nested"][0]
				if nested.ReturnType.Name != "Box" || len(nested.ReturnType.TypeArgs) != 1 || nested.ReturnType.TypeArgs[0].Name != "U" {
					t.Fatalf("nested method argument lost: %+v", nested.ReturnType)
				}
				// Definition identity distinguishes each sibling's U and the
				// enclosing impl's T even though all are generic type symbols.
				for _, statement := range program.Statements {
					impl, ok := statement.(*ast.ImplStatement)
					if !ok {
						continue
					}
					for _, member := range impl.Members {
						method, ok := member.(*ast.FunctionDeclaration)
						if !ok || len(method.GenericParameters) == 0 {
							continue
						}
						parameter := method.GenericParameters[0]
						fact, ok := a.GenericParameterFactAt(parameter.Name.Token)
						if !ok || fact.Name != "U" || fact.ImplTarget != "" {
							t.Fatalf("method parameter fact conflates owner: %+v", fact)
						}
						for _, argument := range method.Parameters {
							typ := argument.Type
							if len(typ.TypeArgs) > 0 {
								typ = typ.TypeArgs[0]
							}
							var want lexer.Token
							switch typ.Name {
							case "U":
								want = parameter.Name.Token
							case "T":
								want = impl.Target.TypeArgs[0].Token
							default:
								continue
							}
							definitions := a.DefinitionsAt(typ.Token.File, typ.Token.Line, typ.Token.Column)
							if len(definitions) != 1 || !sameSourceToken(definitions[0], want) {
								t.Fatalf("%s.%s %s resolves to %+v, want %+v", impl.Target.Name, method.Name.Value, typ.Name, definitions, want)
							}
						}
					}
				}
				if len(a.genericTypes) != 0 || len(a.genericTypeDefinitions) != 0 {
					t.Fatal("method/owner generic scope leaked after analysis")
				}
			})
		}
	}
}

// TestMethodGenericScopeFailures rejects owner-name aliasing and parameters
// used outside their method; each conflict retains both declaration locations.
// Rules: rules/declarations/generics.md — §§9,30;
// rules/foundations/names_scopes_visibility.md — §§8,15.
func TestMethodGenericScopeFailures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generics/method_scopes_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "method_scopes_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	shadows, duplicates, leaks := 0, 0, 0
	for _, diagnostic := range errors {
		switch {
		case diagnostic.ID == diagnostics.GenericParameterShadowsDeclaration:
			shadows++
			if !strings.Contains(diagnostic.Message, "enclosing generic parameter T") || diagnostic.PreviousLine != 5 || diagnostic.PreviousColumn == 0 {
				t.Errorf("wrong owner conflict: %+v", diagnostic)
			}
		case strings.Contains(diagnostic.Message, `duplicate generic parameter "U"`):
			duplicates++
		case strings.Contains(diagnostic.Message, "unknown type U"):
			leaks++
		default:
			t.Errorf("unexpected diagnostic: %+v", diagnostic)
		}
	}
	if shadows != 2 || duplicates != 1 || leaks != 2 {
		t.Fatalf("shadow/duplicate/leak counts = %d/%d/%d, want 2/1/2: %v", shadows, duplicates, leaks, errors)
	}
}
