package sema

import (
	"fmt"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
)

// Both terminating handlers and naked range propagation keep the successful
// slice's ultimate storage, range and validity facts when it is reborrowed.
// A recovery value or absent plan cannot prove that sole continuing origin;
// preserving the origin never permits a local slice to escape its function.
// Rules: rules/errors/errorhandling.md — §§15–16,21.1,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,59–62,136–137.
func TestSliceTryContinuingOrigins(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, a, errs := readSliceMatrixInput(t, "slice_try_origin_paths.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			for _, statement := range program.Statements {
				decl, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				name := decl.Name.Value
				tried := decl.Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
				parent, ok := a.ResolvedSliceCreationPlanOf(tried.Expression.(*ast.RefExpression))
				if !ok {
					t.Fatal("missing fallible parent plan")
				}
				child, ok := a.ResolvedSliceCreationPlanOf(decl.Body.Statements[1].(*ast.LetStatement).Value.(*ast.RefExpression))
				if !ok {
					t.Fatal("missing child plan")
				}
				p, c := parent.ReferenceFacts.Facts, child.ReferenceFacts.Facts
				if len(p.OriginPlaces) != 1 || len(c.OriginPlaces) != 1 || p.OriginPlaces[0].RootID != c.OriginPlaces[0].RootID || p.OriginPlaces[0].StorageIdentity != c.OriginPlaces[0].StorageIdentity || p.Provenance != c.Provenance || p.ProofProvenance != c.ProofProvenance || p.Borrow == c.Borrow {
					t.Fatalf("%s lost the protected source provenance: parent=%+v child=%+v", name, p, c)
				}
			}
			if err := walk.Inspect(program, func(node any) error {
				if tried, ok := node.(*ast.TryExpression); ok {
					if _, ok := a.continuingSliceTrySource(tried); !ok {
						t.Fatal("resolved success-only try lost its source")
					}
					fresh := *tried
					if _, ok := a.continuingSliceTrySource(&fresh); ok {
						t.Fatal("unresolved try supplied an origin proof")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			program, a, errs = readSliceMatrixInput(t, "../semantic_ir/slice_try_recovery.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			if err := walk.Inspect(program, func(node any) error {
				if tried, ok := node.(*ast.TryExpression); ok {
					if _, ok := a.continuingSliceTrySource(tried); ok {
						t.Fatal("recovery handler incorrectly claimed the protected origin")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			_, _, errs = readSliceMatrixInput(t, "slice_try_local_escape_invalid.sec", width)
			if len(errs) == 0 {
				t.Fatal("local-origin slice escaped through try and reborrow")
			}
		})
	}
}
