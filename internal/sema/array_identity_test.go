package sema

import (
	"math/big"
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestArrayOwnerIdentityTraits verifies owner traits independently of element
// copyability and through containing aggregates, keeping fixed arrays and safe
// borrowed views distinct in equality and initialization.
// Rules: rules/collections/collections.md — §§5,6.1,10–12;
// rules/corrections/applied/correction26-20260823.md — "Required correction", §§1–5.
func TestArrayOwnerIdentityTraits(t *testing.T) {
	integer := Type{Name: "int", Kind: IntType}
	ordinary := Type{Name: "Record", Module: "main", Kind: StructType, Named: true, Fields: []StructField{{Name: "value", Type: integer}}}
	noncopy := ordinary
	noncopy.Name, noncopy.ExplicitlyNonCopyable = "Resource", true
	for _, element := range []Type{integer, ordinary, noncopy} {
		owner := NewDynamicArrayType(element)
		named := owner
		named.Name, named.Named, named.Module = "Owner", true, "main"
		aggregate := Type{Name: "Container", Module: "main", Kind: StructType, Named: true, Fields: []StructField{{Name: "values", Type: owner}}}
		for _, typ := range []Type{owner, named, NewFixedArrayType(owner, big.NewInt(2)), aggregate} {
			if CopyClassificationOf(typ) != CopyMoveOnly || TriviallyDestructible(typ) || EqualityComparable(typ) {
				t.Fatal("owner traits lost", typ)
			}
		}
		fixed := NewFixedArrayType(element, big.NewInt(2))
		if canInitialize(owner, fixed, nil) || canInitialize(fixed, owner, nil) || canCompareEquality(owner, owner) || sameComparableArrayType(owner, owner) {
			t.Fatal("owner inherited fixed semantics", element)
		}
		if !canInitialize(owner, owner, nil) || CopyClassificationOf(fixed) != CopyClassificationOf(element) || TriviallyDestructible(fixed) != TriviallyDestructible(element) {
			t.Fatal("fixed/owner derivation changed", element)
		}
	}
	fixed := NewFixedArrayType(integer, big.NewInt(2))
	left := fixed
	left.Name, left.Named, left.Module = "Row", true, "first"
	right := left
	right.Module = "second"
	for _, other := range []Type{fixed, right} {
		if canCompareEquality(left, other) || canCompareEquality(NewFixedArrayType(left, big.NewInt(2)), NewFixedArrayType(other, big.NewInt(2))) {
			t.Fatal("equality erased nominal identity", left, other)
		}
	}
	if !canCompareEquality(left, left) || !canCompareEquality(fixed, fixed) {
		t.Fatal("same fixed type lost equality")
	}
}

// TestArrayOwnerIdentitySource checks nominal array equality, owner transfer,
// forbidden family initialization and explicit safe views through real source
// at every analysis depth, retaining the distinction after analyzer reuse.
// Rules: rules/collections/collections.md — §§6.1,10–12;
// rules/types/types.md — "Named types".
func TestArrayOwnerIdentitySource(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, test := range []struct {
			file   string
			errors int
		}{{"array_identity_valid.sec", 0}, {"array_identity_invalid.sec", 5}, {"array_nominal_equality_invalid.sec", 3}} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			for attempt := 0; attempt < 2; attempt++ {
				if errs := a.Analyze(program); len(errs) != test.errors {
					t.Fatal(depth, test.file, errs)
				}
				if test.errors != 0 {
					break
				}
				for _, statement := range program.Statements {
					fn, ok := statement.(*ast.FunctionDeclaration)
					if !ok || (fn.Name.Value != "Move" && fn.Name.Value != "AnonymousMove") {
						continue
					}
					source := fn.Body.Statements[0].(*ast.LetStatement)
					typ, known := a.ResolvedTypeOf(source.Value)
					if !known || typ.Kind != ArrayType || arrayShapeOf(typ) != ArrayShapeDynamic || CopyClassificationOf(typ) != CopyMoveOnly {
						t.Fatal("move lost owner identity", typ, known)
					}
				}
			}
		}
	}
}
