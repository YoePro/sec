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

// Concrete-owner pack methods retain ordinary receiver authority and reject
// absent/immutable/overlapping receivers before registering successful calls.
// Rules: rules/declarations/functions.md §§22–24,38A(3–4);
// rules/memory/borrowing.md §§7(5),15.2–15.4.
func TestTypePackMethodCalls(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"method_calls.sec", "method_reference_return.sec", "method_receiver_immutable_invalid.sec", "method_receiver_overlap_invalid.sec", "method_missing_receiver_invalid.sec", "method_receiver_escape_invalid.sec", "method_receiver_move_invalid.sec"} {
			data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.New(string(data))).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			errors := a.Analyze(parsed.Program)
			if name != "method_calls.sec" && name != "method_reference_return.sec" {
				if len(errors) != 1 {
					t.Fatal(name, errors)
				}
				if strings.Contains(name, "boundary") {
					if errors[0].ID != diagnostics.GenericTypePackExpansionUnavailable {
						t.Fatal(errors)
					}
				} else if errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable {
					t.Fatal(name, errors)
				}
				continue
			}
			if len(errors) != 0 {
				t.Fatal(name, errors)
			}
			calls := 0
			ast.Visit(parsed.Program, func(node any) bool {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return true
				}
				fact, known := a.ResolvedTypePackCallOf(call)
				if !known {
					return true
				}
				calls++
				target, _ := a.ResolvedCallTarget(call)
				if target.Kind != ResolvedStaticMethodCall {
					t.Fatal("lost method dispatch kind", target)
				}
				if target.Function.ImplTarget != "Counter" || fact.Length != uint64(len(call.Arguments)) {
					t.Fatal(target, fact)
				}
				return true
			})
			want := 7
			if name == "method_reference_return.sec" {
				want = 1
			}
			if calls != want {
				t.Fatal("missing method pack calls", calls)
			}
		}
	}
}

// A generic owner's arguments compose with the method's own parameters: order,
// arity and owner all select the instance, and the result sees the owner's type.
// Rules: rules/declarations/functions.md §§22,38A(2),(9); rules/compiler/monomorphization.md §§8.2,9.3.
func TestTypePackMethodsOnGenericOwners(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/method_generic_owner.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.New(string(data))).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	owners := map[string]bool{}
	ast.Visit(parsed.Program, func(node any) bool {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return true
		}
		if target, known := a.ResolvedCallTarget(call); known {
			if owner, generic := target.Function.GenericSubstitution["V"]; generic && target.Function.GenericTypePack != "" {
				owners[owner.Name] = true
			}
		}
		return true
	})
	if !owners["int"] || !owners["bool"] {
		t.Fatal("owner arguments not kept in the instance", owners)
	}
	for _, item := range []struct{ source, want string }{
		{"module main\ntype Box[V] struct { Value: V }\nimpl Box[V] {\n    fn Get[...T](values: ...T) V { return self.Value }\n}\nfn U() bool {\n    let b := Box[int] { Value: 1 }\n    return b.Get(true)\n}\n", "must return bool"},
		{"module main\ntype Box[V] struct { Value: V }\nimpl Box[V] {\n    fn Put[...T](value: V, values: ...T) void {}\n}\nfn U() void {\n    let b := Box[int] { Value: 1 }\n    b.Put(true)\n}\n", "argument 1 to Box.Put must be int, got bool"},
		{"module main\ntype Box[V] struct { Value: V }\nimpl Box[V] {\n    fn Put(value: V) void {}\n}\nfn U() void {\n    let b := Box[int] { Value: 1 }\n    b.Put(true)\n}\n", "argument 1 to Box.Put must be int, got bool"},
	} {
		messages := ""
		for _, err := range analyzeSourceRaw(t, item.source) {
			messages += err.Message + "\n"
		}
		if !strings.Contains(messages, item.want) {
			t.Errorf("%s: want %q, got %s", item.source, item.want, messages)
		}
	}
}
