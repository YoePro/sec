package modules

import (
	"reflect"
	"testing"
)

func testIdentity(path string) ModuleIdentity {
	return ModuleIdentity{ImportRootIdentity: "project:test", CanonicalImportPath: path}
}

func TestModuleGraphTopologicalOrderPlacesDependenciesFirst(t *testing.T) {
	graph := NewModuleGraph()
	main := testIdentity("main")
	orders := testIdentity("domain/orders")
	storage := testIdentity("internal/storage")
	if err := graph.AddImport(ImportEdge{From: main, To: orders}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddImport(ImportEdge{From: main, To: storage}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddImport(ImportEdge{From: orders, To: storage}); err != nil {
		t.Fatal(err)
	}
	order, cycles := graph.TopologicalOrder()
	if len(cycles) != 0 {
		t.Fatalf("unexpected cycles: %v", cycles)
	}
	want := []ModuleIdentity{storage, orders, main}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestModuleGraphReportsDeterministicRepresentativeCycle(t *testing.T) {
	a := testIdentity("a")
	b := testIdentity("b")
	c := testIdentity("c")
	graph := NewModuleGraph()
	for _, edge := range []ImportEdge{{From: c, To: a}, {From: a, To: b}, {From: b, To: c}} {
		if err := graph.AddImport(edge); err != nil {
			t.Fatal(err)
		}
	}
	order, cycles := graph.TopologicalOrder()
	if order != nil || len(cycles) != 1 {
		t.Fatalf("order=%v cycles=%v", order, cycles)
	}
	if got, want := cycles[0].String(), "project:test:a -> project:test:b -> project:test:c -> project:test:a"; got != want {
		t.Fatalf("cycle = %q, want %q", got, want)
	}
}

func TestModuleGraphRejectsSelfAndDuplicateImports(t *testing.T) {
	a := testIdentity("a")
	b := testIdentity("b")
	graph := NewModuleGraph()
	if err := graph.AddImport(ImportEdge{From: a, To: a}); err == nil {
		t.Fatal("self import unexpectedly succeeded")
	}
	if err := graph.AddImport(ImportEdge{From: a, To: b}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddImport(ImportEdge{From: a, To: b}); err == nil {
		t.Fatal("duplicate import unexpectedly succeeded")
	}
}
