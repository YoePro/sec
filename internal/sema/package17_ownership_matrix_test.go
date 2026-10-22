package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// P17-17 exercises supported source transfers together: custom-free owners,
// consuming call/return, whole/field move and reinit, shared/exclusive refs and
// slices. Every published action must agree with the canonical classification.
// Rules: rules/memory/copy_move.md — §§3–4,8–10,12,14;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§5–7,12–18,27.
func TestPackage17OwnershipMatrix(t *testing.T) {
	a, program := analyzeDeferPlanFixture(t, "package17_ownership_matrix.sec")
	type expected struct {
		site   TransferSiteKind
		action TransferAction
		source string
	}
	want := map[expected]bool{
		{TransferSiteReturn, TransferMove, "value"}:               false,
		{TransferSiteArgument, TransferMove, "second"}:            false,
		{TransferSiteInitialization, TransferMove, "first"}:       false,
		{TransferSiteInitialization, TransferMove, "pair.left"}:   false,
		{TransferSiteInitialization, TransferMove, "pair"}:        false,
		{TransferSiteInitialization, TransferCopyTrivial, "view"}: false,
		{TransferSiteInitialization, TransferMove, "view"}:        false,
	}
	classes := map[string]bool{}
	for _, plan := range a.ResolvedTransferPlans() {
		key := expected{plan.Site, plan.Action, plan.Source.String()}
		if _, ok := want[key]; ok {
			want[key] = true
		}
		classification := CopyClassificationOf(plan.Source.Type)
		if plan.Action == TransferCopyTrivial && classification != CopyTrivial {
			t.Fatal("copy action disagrees with source classification")
		}
		if plan.Source.Root == "view" {
			isSlice := plan.Source.Type.Element != nil && plan.Source.Type.Element.Kind == SliceType
			name := "reference"
			if isSlice {
				name = "slice"
			}
			if plan.Source.Type.ReferenceMutable {
				name = "mutable " + name
			} else {
				name = "shared " + name
			}
			classes[name] = true
			if !TriviallyDestructible(plan.Source.Type) {
				t.Fatal("borrow holder acquired referent destruction")
			}
			if plan.Source.Type.ReferenceMutable && (classification != CopyMoveOnly || plan.Action != TransferMove) {
				t.Fatal("exclusive holder was copied")
			}
		}
		if plan.Source.String() == "pair.left" && !plan.Partial {
			t.Fatal("field move lost its partial path")
		}
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("missing transfer %+v", key)
		}
	}
	for _, name := range []string{"shared reference", "mutable reference", "shared slice", "mutable slice"} {
		if !classes[name] {
			t.Errorf("missing %s coverage", name)
		}
	}
	// A post-reinit whole transfer is accepted only if the field restoration
	// actually restored the complete source. Querying must not change that state.
	reference, _ := analyzeDeferPlanFixture(t, "package17_ownership_matrix.sec")
	before := reference.ResolvedTransferPlans()
	beforeMoves := copyMoved(a.moved)
	beforeBorrows := copyBorrows(a.borrows)
	beforeTypes, beforePlaces := len(a.expressionTypes), len(a.resolvedPlaces)
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, statement := range fn.Body.Statements {
			decl, ok := statement.(*ast.LetStatement)
			if !ok || decl.Value == nil {
				continue
			}
			if _, ok := a.ResolvedTransferPlanOf(decl.Value); !ok {
				continue
			}
			plan, _ := a.ResolvedTransferPlanOf(decl.Value)
			plan.Source.Root = "changed"
			if plan.Source.Type.Element != nil {
				plan.Source.Type.Element.Name = "changed"
			}
		}
	}
	listed := a.ResolvedTransferPlans()
	for index := range listed {
		place := &listed[index].Source
		if place.Type.Element != nil {
			place.Type.Element.Name = "changed"
		}
		if len(place.Type.Fields) != 0 {
			place.Type.Fields[0].Name = "changed"
		}
	}
	if !reflect.DeepEqual(before, a.ResolvedTransferPlans()) || !reflect.DeepEqual(beforeMoves, a.moved) || !reflect.DeepEqual(beforeBorrows, a.borrows) ||
		beforeTypes != len(a.expressionTypes) || beforePlaces != len(a.resolvedPlaces) {
		t.Fatal("transfer query aliases source type facts or changes ownership")
	}
}

// P17-17 requires complete destruction graphs for the supported source types,
// with no dangling child plans, active-payload-only variants and compact arrays.
// References/slices are independently trivial regardless of referent ownership.
// Rules: rules/memory/destruction.md — §§7.2–7.3,15–18;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§19–27.
func TestPackage17DestructionGraphMatrix(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "package17_ownership_matrix.sec")
	types := []Type{a.types["Handle"], a.types["Pair"], a.types["Choice"], a.types["Optional"], a.types["Outcome"]}
	types = append(types, NewFixedArrayType(a.types["Pair"], new(big.Int).Lsh(big.NewInt(1), 80)))
	for _, transfer := range a.ResolvedTransferPlans() {
		if transfer.Source.Root == "view" {
			types = append(types, transfer.Source.Type)
		}
	}
	families := map[string]bool{}
	for _, typ := range types {
		set := a.ResolvedDestructionPlanOf(typ)
		if _, ok := set.Plans[set.Root]; !ok {
			t.Fatal("destruction graph has no root")
		}
		for _, plan := range set.Plans {
			if plan.Kind == DestructionUnsupported {
				t.Fatalf("supported type %s lost its destruction plan: %s", typ.Name, plan.Unsupported)
			}
			var children []DestructionPlanID
			for _, field := range plan.Fields {
				children = append(children, field.Plan)
			}
			if plan.Element != "" {
				children = append(children, plan.Element)
			}
			for _, variant := range plan.Variants {
				for _, payload := range variant.Payload {
					children = append(children, payload.Plan)
				}
			}
			for _, child := range children {
				if _, ok := set.Plans[child]; !ok {
					t.Fatalf("%s has missing child %s", plan.ID, child)
				}
			}
			if plan.Kind == DestructionVariant {
				families[plan.VariantFamily] = true
				if len(plan.Variants) != 1 || len(plan.Variants[0].Payload) != 1 {
					t.Fatal("variant plan destroys an empty/trivial payload or omits its owned payload")
				}
			}
		}
		root := set.Plans[set.Root]
		if typ.Kind == ReferenceType && (root.Kind != DestructionTrivial || len(set.Plans) != 1) {
			t.Fatal("borrow holder graph recursively destroys referent storage")
		}
		if typ.Kind == ArrayType && (root.ElementCount != "1208925819614629174706176" || len(set.Plans) != 3) {
			t.Fatal("fixed-array destruction lost exact length or expanded by element count")
		}
	}
	for _, family := range []string{"union", "option", "result"} {
		if !families[family] {
			t.Errorf("missing %s payload coverage", family)
		}
	}
}

// P17-17 rejects invalid source actions individually, retaining the owning
// custom-free, consuming-call, exclusivity and array-extraction diagnostics.
// Rules: rules/memory/copy_move.md — §§3.5,4.4,8,14;
// rules/memory/destruction.md — §10.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§12,15,22,27.
func TestPackage17OwnershipRejectionMatrix(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/package17_ownership_matrix_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	want := map[string]string{
		"CustomCopy": "move-only", "MissingTransfer": "explicit <-",
		"CustomFieldMove": "custom free", "CustomFieldDiscard": "defines free",
		"ArrayElementMove": "indexed extraction", "UseAfterMove": "no longer available",
		"MutableReferenceCopy": "move-only", "MutableSliceCopy": "move-only",
	}
	if len(errors) != len(want) {
		t.Fatalf("expected one diagnostic per rejected function: %v", errors)
	}
	checked := map[string]bool{}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		message, targeted := want[fn.Name.Value]
		if !targeted {
			continue
		}
		checked[fn.Name.Value] = true
		for _, bodyStatement := range fn.Body.Statements {
			if decl, ok := bodyStatement.(*ast.LetStatement); ok && decl.Name.Value == "rejected" {
				if _, ok := a.ResolvedTransferPlanOf(decl.Value); ok {
					t.Errorf("%s published a rejected transfer", fn.Name.Value)
				}
			}
		}
		found := false
		for _, failure := range errors {
			if failure.Line >= fn.Token.Line && failure.Line <= statementToken(fn.Body.Statements[len(fn.Body.Statements)-1]).Line && strings.Contains(failure.Message, message) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s missing %q diagnostic: %v", fn.Name.Value, message, errors)
		}
	}
	if len(checked) != len(want) {
		t.Fatal("rejection fixture omitted a required case")
	}
}
