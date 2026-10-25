package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Candidate matching must leave source authority intact and publish a new,
// shared pass only after selection, including inferred/explicit generic calls.
// Rules: rules/memory/borrowing.md — §15.2(4), §15.3(2), §15.4.
func TestSharedReferenceCallPlans(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, file := range []string{"../semantic_ir/reference_shared_call_reborrow.sec", "../semantic_ir/slice_shared_call_reborrow.sec", "../semantic_ir/slice_expression_call_reborrow.sec", "slice_shared_call_generics_valid.sec"} {
			t.Run(fmt.Sprintf("%d/%s", width, file), func(t *testing.T) {
				source, err := os.ReadFile("../../testdata/sema/" + file)
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(string(source), file))
				parsed := p.Parse()
				if parsed.HasErrors {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errs := a.Analyze(parsed.Program); len(errs) != 0 {
					t.Fatal(errs)
				}
				passes := 0
				if err := walk.Inspect(parsed.Program, func(node any) error {
					call, ok := node.(*ast.CallExpression)
					if !ok {
						return nil
					}
					target, ok := a.ResolvedCallTarget(call)
					if !ok {
						t.Fatal("call has no resolved target")
					}
					for index, arg := range call.Arguments {
						typ, _ := a.ResolvedTypeOf(arg)
						parameter := target.Function.Parameters[index]
						if !sharedReferenceCallReborrow(parameter.Type, typ) {
							continue
						}
						passes++
						use, ok := a.ResolvedReferenceUseOf(arg)
						if !ok || use.Kind != ReferenceUsePass || !use.Facts.Checkable() || use.Facts.MutableAuthority || use.Facts.Borrow == 0 || use.Facts.Lifetime == 0 {
							t.Fatalf("shared slice pass lost its resolved authority: %+v", use)
						}
						switch expression := arg.(type) {
						case *ast.RefExpression:
							creation, ok := a.ResolvedSliceCreationPlanOf(expression)
							if !ok || creation.BorrowID == use.Facts.Borrow || creation.LifetimeID == use.Facts.Lifetime {
								t.Fatalf("fresh slice creation and pass share identities: %+v", use)
							}
						case *ast.CallExpression:
							returned, ok := a.ResolvedCallReferenceOf(expression)
							if !ok || returned.Borrow == use.Facts.Borrow || returned.Lifetime == use.Facts.Lifetime {
								t.Fatalf("returned slice and pass share identities: %+v", use)
							}
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if passes < 2 {
					t.Fatalf("missing shared reborrow coverage: %d passes", passes)
				}
			})
		}
	}
}

// Reborrow compatibility is call-local; it neither upgrades authority,
// converts element types nor permits overlapping/retained mutable access.
// Rules: rules/memory/borrowing.md — §4(1)–(3), §15.3(2), §15.4(3)–(4);
// rules/types/types.md — Type identity, Assignability.
func TestSharedReferenceCallRestrictions(t *testing.T) {
	for _, test := range []struct {
		file string
		want []string
	}{
		{"reference_shared_call_overlap_invalid.sec", []string{"argument 2 overlaps mutable borrow prepared for argument 1", "argument 2 overlaps shared borrow prepared for argument 1"}},
		{"reference_shared_call_retained_invalid.sec", []string{"while it is shared borrowed"}},
		{"reference_shared_call_types_invalid.sec", []string{"argument 1 to Mutate must be ref mut int", "argument 1 to ReadUnsigned must be ref uint", "ref int"}},
		{"slice_expression_call_invalid.sec", []string{"argument 2 overlaps shared borrow prepared for argument 1", "while it is shared borrowed", "argument 1 to ReadUnsigned must be ref uint[]", "argument 1 to Mutate must be ref mut int[]", "cannot return reference to local variable data"}},
		{"slice_shared_call_overlap_invalid.sec", []string{"argument 2 overlaps mutable borrow prepared for argument 1", "argument 2 overlaps shared borrow prepared for argument 1"}},
		{"slice_shared_call_retained_invalid.sec", []string{"while it is shared borrowed"}},
		{"slice_shared_call_types_invalid.sec", []string{"argument 1 to Mutate must be ref mut int[]", "argument 1 to ReadUnsigned must be ref uint[]", "ref int[]"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			errs := analyzeSourceRaw(t, string(source))
			if len(errs) != len(test.want) {
				t.Fatalf("diagnostics = %+v, want %d errors", errs, len(test.want))
			}
			for index, want := range test.want {
				if !strings.Contains(errs[index].Message, want) {
					t.Fatalf("diagnostic %d = %+v, want %q", index, errs[index], want)
				}
			}
		})
	}
}
