package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/compiler/compiler_analysis.md — §18(2),(6);
// rules/control-flow/flowcontrol_for.md — §§6–8,12,30–31,37.
func TestIteratorStorageAndBorrowDependencies(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/iterator_dependencies_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "iteration-dependencies.sec"))
	program := parsed.ParseProgram()
	if errors := parsed.Errors(); len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	loops := map[string][]*ast.ForStatement{}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		walkASTValue(reflect.ValueOf(fn.Body), func(node any) {
			if loop, ok := node.(*ast.ForStatement); ok {
				loops[fn.Name.Value] = append(loops[fn.Name.Value], loop)
			}
		})
	}
	get := func(name string, index int) ForIterationDependencies {
		t.Helper()
		fact, ok := analyzer.ForIterationDependenciesOf(loops[name][index])
		if !ok || !fact.BodyValid {
			t.Fatalf("%s dependencies: %#v (%v)", name, fact, ok)
		}
		return fact
	}
	find := func(fact ForIterationDependencies, holder string) IterationBorrowDependency {
		t.Helper()
		for _, borrow := range fact.Borrows {
			if borrow.Holder == holder {
				return borrow
			}
		}
		t.Fatalf("missing %s borrow in %#v", holder, fact)
		return IterationBorrowDependency{}
	}
	reusable := get("Reusable", 0)
	if reusable.Source != ForIteratorReusableStorage || reusable.SourcePlace.Root != "counter" || reusable.TemporaryDestroyedAtExit || len(reusable.Storage) != 1 || reusable.Storage[0].Structural || !reusable.Storage[0].MustRemainLive {
		t.Fatal("reusable lifetime", reusable)
	}
	observation, yielded := find(reusable, "observation"), find(reusable, "yielded")
	if !observation.OverlapsIteratorStorage || yielded.OverlapsIteratorStorage || observation.Phase != "body" {
		t.Fatal("iterator storage and owned yielded value conflated", reusable)
	}
	temporary := get("Temporary", 0)
	if temporary.Source != ForIteratorFreshTemporary || !temporary.TemporaryDestroyedAtExit || temporary.SourcePlace.Root != "" || len(temporary.Storage) != 0 || find(temporary, "yielded").OverlapsIteratorStorage {
		t.Fatal("temporary state or owned value lifetime", temporary)
	}
	shared := get("Shared", 0)
	if len(shared.Storage) != 1 || !shared.Storage[0].Structural || shared.Storage[0].Place.Root != "values" || !find(shared, "before").OverlapsIteratorStorage || find(shared, "before").Phase != "entry" || find(shared, "value").Phase != "iteration-binding" {
		t.Fatal("collection structural/element dependencies", shared)
	}
	local := find(shared, "local")
	if !local.OverlapsIteratorStorage || local.LiveAtLoopExit || local.Place.String() != "values[1]" {
		t.Fatal("body borrow lost exact source projection", local)
	}
	outer, inner := get("Nested", 0), get("Nested", 1)
	if !find(outer, "fromLeft").OverlapsIteratorStorage || find(outer, "fromRight").OverlapsIteratorStorage || !find(inner, "fromRight").OverlapsIteratorStorage || find(inner, "fromLeft").OverlapsIteratorStorage {
		t.Fatal("nested source identities merged", outer, inner)
	}
	for _, name := range []string{"Range", "String"} {
		fact := get(name, 0)
		for _, dependency := range fact.Storage {
			if dependency.Structural {
				t.Fatal("value iteration invented structural dependency", fact)
			}
		}
	}
	carried := get("Carried", 0)
	foundCarried := false
	for _, borrow := range carried.Borrows {
		foundCarried = foundCarried || (borrow.Holder == "kept" && borrow.LoopCarried && borrow.LiveAtLoopExit && borrow.OverlapsIteratorStorage)
	}
	if !foundCarried {
		t.Fatal("loop-carried source borrow lost", carried)
	}
	early := get("Early", 0)
	if !early.TemporaryDestroyedAtExit || len(early.Borrows) != 1 || early.Borrows[0].LoopCarried {
		t.Fatal("early exit lifetime or borrow state", early)
	}
	backed := get("Backed", 0)
	foundBacking := false
	for _, dependency := range backed.Storage {
		foundBacking = foundBacking || (dependency.CarrierPath != "" && dependency.Structural && dependency.MustRemainLive)
	}
	if !foundBacking && !backed.BackingUnknown {
		t.Fatal("iterator backing reference became an empty dependency proof", backed)
	}
	if lambda := get("LambdaOnly", 0); len(lambda.Borrows) != 0 {
		t.Fatal("lambda invocation-local borrow attributed to the enclosing loop", lambda)
	}
	// Shared snapshot storage must never let tooling mutate canonical Places,
	// exact arbitrary-precision projections, or nested type metadata.
	before := get("Shared", 0)
	shared.Storage[0].Place.Root = "mutated"
	shared.SourceType.Element.Name = "mutated"
	shared.Borrows[0].Holder = "mutated"
	for _, borrow := range shared.Borrows {
		for _, projection := range borrow.Place.Projections {
			if projection.ConstantIndex != nil {
				projection.ConstantIndex.SetInt64(99)
			}
		}
	}
	if after := get("Shared", 0); !reflect.DeepEqual(before, after) {
		t.Fatal("mutable dependency snapshot escaped")
	}
	nextParser := parser.New(lexer.NewWithFile(string(data), "iteration-dependencies.sec"))
	if errors := analyzer.Analyze(nextParser.ParseProgram()); len(errors) != 0 {
		t.Fatal(errors)
	}
	if _, ok := analyzer.ForIterationDependenciesOf(loops["Shared"][0]); ok {
		t.Fatal("analyzer reuse retained previous source facts")
	}
	if len(analyzer.activeIterationDependencies) != 0 {
		t.Fatal("loop lifetime boundary leaked")
	}
}

// Rules: rules/compiler/compiler_analysis.md — §18(2);
// rules/control-flow/flowcontrol_for.md — §§8,41.
func TestIteratorDependencyValidationBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/iterator_dependencies_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "invalid-iteration.sec"))
	program := parsed.ParseProgram()
	if errors := parsed.Errors(); len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 2 {
		t.Fatal("expected structural mutation and noniterable errors", errors)
	}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		loop := fn.Body.Statements[0].(*ast.ForStatement)
		fact, ok := analyzer.ForIterationDependenciesOf(loop)
		if fn.Name.Value == "Mutation" {
			if !ok || fact.BodyValid || len(fact.Storage) != 1 || !fact.Storage[0].Structural {
				t.Fatal("invalid body became validation proof or lost source dependency", fact, ok)
			}
		} else if ok {
			t.Fatal("noniterable source acquired positive dependencies", fact)
		}
	}
}
