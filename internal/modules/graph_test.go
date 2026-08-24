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

func TestModuleCycleDiagnosticRetainsDeterministicImportLocations(t *testing.T) {
	a := testIdentity("a")
	b := testIdentity("b")
	c := testIdentity("c")
	edges := []ImportEdge{
		{From: c, To: a, SourceFile: "c.sec", Line: 30, Column: 3},
		{From: b, To: c, SourceFile: "b.sec", Line: 20, Column: 2},
		{From: a, To: b, SourceFile: "a.sec", Line: 10, Column: 1},
	}
	graph := NewModuleGraph()
	for _, edge := range edges {
		if err := graph.AddImport(edge); err != nil {
			t.Fatal(err)
		}
	}
	cycles := graph.Cycles()
	if len(cycles) != 1 {
		t.Fatalf("cycles = %v, want one", cycles)
	}
	diagnostic := cycles[0].Diagnostic()
	if got, want := diagnostic.Message, "module import cycle: project:test:a -> project:test:b -> project:test:c -> project:test:a"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if got, want := diagnostic.Primary, edges[2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("primary edge = %#v, want %#v", got, want)
	}
	if got, want := diagnostic.Related, []ImportEdge{edges[1], edges[0]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("related edges = %#v, want %#v", got, want)
	}

	diagnostic.Related[0].SourceFile = "changed.sec"
	if cycles[0].Edges[1].SourceFile != "b.sec" {
		t.Fatal("diagnostic related locations alias mutable graph storage")
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
