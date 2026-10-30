package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Shared pack Len reads have a stable uint type even before element-body
// specialization. Empty/nonempty call arity stays in ordered Sema facts; no
// runtime pack representation or executable expansion is introduced here.
// Rules: rules/declarations/functions.md §38A(5)(1–2),(9).
func TestSharedTypePackLength(t *testing.T) {
	for _, name := range []string{"shared_length.sec", "shared_length_write_invalid.sec", "shared_length_return_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		parsed := parser.New(lexer.New(string(data))).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		a := NewAnalyzer()
		errors := a.Analyze(parsed.Program)
		if strings.Contains(name, "invalid") {
			if len(errors) != 1 || errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable {
				t.Fatalf("invalid Len use must receive an ordinary type/read-only error: %v", errors)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		lengthReads, calls := 0, 0
		ast.Visit(parsed.Program, func(node any) bool {
			if member, ok := node.(*ast.MemberExpression); ok && member.Property.Value == "Len" {
				typ, known := a.ResolvedTypeOf(member)
				if !known || typ.Name != "uint" {
					t.Fatalf("Len type: %+v, known=%v", typ, known)
				}
				lengthReads++
			}
			if call, ok := node.(*ast.CallExpression); ok {
				fact, known := a.ResolvedTypePackCallOf(call)
				if !known || len(fact.Types) != len(call.Arguments) || fact.Length != uint64(len(call.Arguments)) {
					t.Fatalf("missing concrete length: %+v, known=%v", fact, known)
				}
				calls++
			}
			return true
		})
		if lengthReads != 1 || calls != 2 {
			t.Fatalf("length reads=%d calls=%d", lengthReads, calls)
		}
	}
}

// Shared, unused packs have ordered concrete call signatures without an erased
// element type; fixed prefix inference and ordinary call borrowing remain intact.
// Rules: rules/declarations/functions.md §38A(2–4),(9); rules/memory/borrowing.md §15.2.
func TestSharedUnusedTypePackCalls(t *testing.T) {
	file := "../../testdata/generics/type_pack_declarations/shared_unused.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(source), file))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
			if errors := a.Analyze(program); len(errors) > 0 {
				t.Fatal(errors)
			}
			seen := 0
			keys := map[genericInstanceKey]string{}
			ast.Visit(program, func(node any) bool {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return true
				}
				facts, ok := a.ResolvedTypePackCallOf(call)
				if !ok {
					return true
				}
				seen++
				resolved, _ := a.ResolvedCallTarget(call)
				key := genericFunctionInstanceKey(a.functions[resolved.Function.Name][0], resolved.Function.GenericSubstitution)
				sequenceKey := canonicalTypeArgumentsKey(facts.Types)
				if previous, exists := keys[key]; exists && previous != sequenceKey {
					t.Fatal("different type sequences share a specialization")
				}
				keys[key] = sequenceKey
				if len(facts.Types) != len(call.Arguments)-facts.FixedCount || len(facts.Parameters) != len(call.Arguments) {
					t.Fatal("wrong concrete arity", facts)
				}
				for i, typ := range facts.Types {
					parameter := facts.Parameters[facts.FixedCount+i]
					if parameter.Type.Kind != ReferenceType || parameter.Type.ReferenceMutable || !sameConcreteType(*parameter.Type.Element, typ) {
						t.Fatalf("argument %d lost shared mode/type: %+v", i, parameter)
					}
					arg := call.Arguments[facts.FixedCount+i]
					if _, planned := a.ResolvedReferencePlanOf(arg); !planned {
						if _, passed := a.ResolvedReferenceUseOf(arg); !passed {
							t.Fatalf("%s argument %d (%s) lacks an ordinary borrow/pass plan", resolved.Function.Name, i, arg.String())
						}
					}
				}
				if len(facts.Types) > 0 {
					old := facts.Types[0].Name
					facts.Types[0].Name = "Changed"
					again, _ := a.ResolvedTypePackCallOf(call)
					if again.Types[0].Name != old {
						t.Fatal("mutable query snapshot")
					}
					sequence := resolved.Function.GenericSubstitution[resolved.Function.GenericTypePack]
					sequence.TypeArgs[0].Name = "Changed"
					resolvedAgain, _ := a.ResolvedCallTarget(call)
					if resolvedAgain.Function.GenericSubstitution[resolved.Function.GenericTypePack].TypeArgs[0].Name != old {
						t.Fatal("mutable call substitution")
					}
				}
				return true
			})
			if seen != 7 {
				t.Fatal("missing selected pack calls", seen)
			}
		}
	}
}

// Shared packs retain ordinary loan/move failures and the explicit unsupported
// expansion boundaries, rather than accepting an erased type or mutating owners.
// Rules: rules/declarations/functions.md §38A(3–8); rules/memory/borrowing.md §15.4.
func TestSharedTypePackChecksAndBoundaries(t *testing.T) {
	for _, name := range []string{"shared_borrow_conflict_invalid.sec", "shared_move_invalid.sec", "shared_runtime_spread_invalid.sec", "shared_use_boundary.sec", "shared_length_capture_boundary.sec", "shared_fixed_spread_boundary.sec", "shared_temporary_escape_invalid.sec", "shared_return_overload_invalid.sec"} {
		source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), name))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if len(errors) == 0 {
			t.Fatalf("accepted %s", name)
		}
		if name == "shared_move_invalid.sec" {
			if len(errors) != 1 || errors[0].ID != diagnostics.MoveIntoBorrowedParameter {
				t.Fatalf("shared pack committed a move: %v", errors)
			}
		}
		if strings.Contains(name, "boundary") {
			if len(errors) != 1 || errors[0].ID != diagnostics.GenericTypePackExpansionUnavailable {
				t.Fatalf("wrong capability boundary: %s %v", name, errors)
			}
		}
	}
}

// An equivalent fixed-arity signature wins over a variadic pack; successful
// pack inference cannot override the existing ordinary overload ranking.
// Rules: rules/declarations/functions.md §38A(4)(5),§37.
func TestSharedTypePackKeepsFixedOverloadPreference(t *testing.T) {
	source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/shared_overloads.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) > 0 {
		t.Fatal(errors)
	}
	ast.Visit(program, func(node any) bool {
		if call, ok := node.(*ast.CallExpression); ok {
			resolved, found := a.ResolvedCallTarget(call)
			if !found || resolved.Function.GenericTypePack != "" {
				t.Fatal("variadic selected over fixed signature")
			}
		}
		return true
	})
}
