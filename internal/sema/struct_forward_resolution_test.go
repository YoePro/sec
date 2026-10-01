package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// Acyclic struct field graphs are declaration-order independent. Sema must
// materialize the complete nested field shapes before member analysis rather
// than retaining InvalidType or empty forward placeholders.
//
// Rules:
//   - rules/declarations/struct.md — §20 "Semantic analysis requirements"
//   - rules/memory/layout.md — §13(5) "Nested structs"
func TestForwardStructFieldsResolveInDependencyOrder(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `type Root struct {
    middle: Middle,
}
type Middle struct {
    leaf: Leaf,
}
type Leaf struct {
    value: int,
}
fn Read(root: Root) int {
    return root.middle.leaf.value
}`)
	if len(errors) != 0 {
		t.Fatalf("forward struct errors = %v", errors)
	}

	root := analyzer.Types()["Root"]
	if len(root.Fields) != 1 || root.Fields[0].Type.Name != "Middle" {
		t.Fatalf("Root field shape = %+v", root.Fields)
	}
	middle := root.Fields[0].Type
	if len(middle.Fields) != 1 || middle.Fields[0].Type.Name != "Leaf" {
		t.Fatalf("embedded Middle field shape = %+v", middle.Fields)
	}
	leaf := middle.Fields[0].Type
	if len(leaf.Fields) != 1 || leaf.Fields[0].Type.Kind != IntType {
		t.Fatalf("embedded Leaf field shape = %+v", leaf.Fields)
	}
}

func TestForwardStructResolutionPreservesCycleDiagnostic(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, `type Left struct {
	    right: Right,
}
type Right struct {
	    left: Left,
}`)
	found := false
	for _, diagnostic := range errors {
		if diagnostic.ID == diagnostics.RecursiveStructLayout {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("recursive forward graph errors = %v, want S1055", errors)
	}
}

func TestForwardNamedStructWrapperResolvesBeforeContainingStruct(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `type Root struct {
    wrapped: Wrapper,
}
type Wrapper Leaf
type Leaf struct {
    value: int,
}
fn Read(root: Root) int {
    return root.wrapped.value
}`)
	if len(errors) != 0 {
		t.Fatalf("forward named wrapper errors = %v", errors)
	}
	root := analyzer.Types()["Root"]
	if len(root.Fields) != 1 || root.Fields[0].Type.Name != "Wrapper" || len(root.Fields[0].Type.Fields) != 1 {
		t.Fatalf("Root wrapper field shape = %+v", root.Fields)
	}
}
