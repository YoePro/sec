package allocation_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	"sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAllocationSelectionBoundary proves that every analyzed output route
// retains exact semantic domain/context facts or rejects without an artifact.
// Creating an Arena domain does not resolve its backing allocator.
// Rules: rules/memory/allocation.md — §§25(2),(5),26(1)–(3),30(6).
func TestAllocationSelectionBoundary(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		for _, name := range []string{"domain", "backing", "context", "growth"} {
			t.Run(triple+"/"+name, func(t *testing.T) {
				program, a := selectionFixture(t, name, triple)
				baseline := a.AllocationContextFacts()
				if len(baseline) != 1 {
					t.Fatal(baseline)
				}
				switch name {
				case "domain":
					if baseline[0].SelectedDomain == "" {
						t.Fatal(baseline)
					}
				case "backing":
					if baseline[0].CreatedDomain == "" || baseline[0].SelectedDomain != "" || baseline[0].ContextKnown {
						t.Fatal(baseline)
					}
				case "context":
					if !baseline[0].ContextKnown || baseline[0].Context.Profile != "hosted" {
						t.Fatal(baseline)
					}
				case "growth":
					if baseline[0].ContextKnown || baseline[0].SelectedDomain != "" {
						t.Fatal(baseline)
					}
				}
				check := func(err error) {
					t.Helper()
					var failure *sema.AllocationLoweringError
					if !errors.As(err, &failure) || !reflect.DeepEqual(failure.Fact, baseline[0]) {
						t.Fatal("selection lost at backend boundary", err)
					}
					capability := failure.ImplementationCapability()
					if capability.Line != baseline[0].Source.Line || capability.File != baseline[0].Source.File || !strings.Contains(failure.Error(), sema.AllocationContextDescription(baseline[0])) {
						t.Fatal(failure)
					}
				}
				check(a.ValidateAllocationLoweringReadiness(program, "main"))
				ll, err := llvm.GenerateAnalyzed(program, a, triple)
				if ll != "" {
					t.Fatal("partial LLVM", ll)
				}
				check(err)
				legacy, err := mlir.GenerateAnalyzed(program, a, triple)
				if legacy != "" {
					t.Fatal("partial legacy MLIR", legacy)
				}
				check(err)
				module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 15})
				if module != nil {
					t.Fatal("partial Semantic IR", module)
				}
				check(err)
				if !reflect.DeepEqual(baseline, a.AllocationContextFacts()) {
					t.Fatal("backend mutated selection facts")
				}
			})
		}
	}
}

// selectionFixture loads source into the target-aware canonical analyzer.
// Rules: rules/memory/allocation.md — §§24(4),25(5).
func selectionFixture(t *testing.T, name, triple string) (*ast.Program, *sema.Analyzer) {
	t.Helper()
	file := "../../../testdata/sema/allocation_lowering_" + name + "_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if name == "modules" {
		dependency := "../../../testdata/sema/allocation_lowering_dependency_valid.sec"
		text, err := os.ReadFile(dependency)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(text), dependency))
		dep := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(dep.Statements, program.Statements...)
	}
	plan, err := targetplan.Plan(triple)
	if err != nil {
		t.Fatal(err)
	}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return program, a
}

// TestAllocationSelectionForeignCall keeps explicit source calls distinct from
// compiler allocator selection. An unknown foreign effect is not permission to
// invent an allocator, nor a prohibition on emitting the declared source call.
// Rules: rules/memory/allocation.md — §§19,24(3),26(1)–(2).
func TestAllocationSelectionForeignCall(t *testing.T) {
	const triple = "x86_64-pc-linux-gnu"
	program, a := selectionFixture(t, "foreign", triple)
	if err := a.ValidateAllocationLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
	if out, err := llvm.GenerateAnalyzed(program, a, triple); err != nil || !strings.Contains(out, "@Native") {
		t.Fatal(out, err)
	}
	if out, err := mlir.GenerateAnalyzed(program, a, triple); err != nil || !strings.Contains(out, "@Native") {
		t.Fatal(out, err)
	}
	if module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"}); err != nil || module == nil {
		t.Fatal(module, err)
	}
}

// TestAllocationSelectionScope excludes declarations outside the requested
// output and rejects unrelated Sema snapshots instead of treating missing facts
// as allocation freedom. Original-statement output subsets retain their facts.
// Rules: rules/memory/allocation.md — §§24(6),25(5),26(1)–(3);
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestAllocationSelectionScope(t *testing.T) {
	const triple = "x86_64-pc-linux-gnu"
	program, a := selectionFixture(t, "modules", triple)
	if err := a.ValidateAllocationLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
	if err := a.ValidateAllocationLoweringReadiness(program, "helper"); err == nil {
		t.Fatal("allocating module accepted")
	}
	if module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"}); err != nil || module == nil {
		t.Fatal(module, err)
	}
	subset := *program
	subset.Statements = append([]ast.Statement(nil), program.Statements[len(program.Statements)-2:]...)
	if err := a.ValidateAllocationLoweringReadiness(&subset, ""); err != nil {
		t.Fatal("non-output allocation leaked into subset", err)
	}
	if out, err := llvm.GenerateAnalyzed(&subset, a, triple); err != nil || out == "" {
		t.Fatal(out, err)
	}
	if out, err := mlir.GenerateAnalyzed(&subset, a, triple); err != nil || out == "" {
		t.Fatal(out, err)
	}
	other, _ := selectionFixture(t, "modules", triple)
	if err := a.ValidateAllocationLoweringReadiness(other, "main"); err == nil {
		t.Fatal("stale analyzer accepted")
	}
	if out, err := llvm.GenerateAnalyzed(other, a, triple); err == nil || out != "" {
		t.Fatal("stale LLVM output", out, err)
	}
	if out, err := mlir.GenerateAnalyzed(other, a, triple); err == nil || out != "" {
		t.Fatal("stale MLIR output", out, err)
	}
	var absent *sema.Analyzer
	if err := absent.ValidateAllocationLoweringReadiness(program, "main"); err == nil {
		t.Fatal("missing facts accepted")
	}
}

// TestAllocationSelectionFolded keeps compile-time text materialization free of
// runtime allocation selection, including disabled and unresolved profiles.
// Rules: rules/memory/allocation.md — §§4,17,22(4),25(1).
func TestAllocationSelectionFolded(t *testing.T) {
	for _, profile := range []string{"hosted", "noalloc", "freestanding"} {
		program, a := selectionFixture(t, "folded", "x86_64-pc-linux-gnu")
		a.SetAllocationProfile(profile)
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(profile, errs)
		}
		if facts := a.AllocationContextFacts(); len(facts) != 0 {
			t.Fatal(profile, facts)
		}
		if err := a.ValidateAllocationLoweringReadiness(program, "main"); err != nil {
			t.Fatal(profile, err)
		}
		if module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"}); err != nil || module == nil {
			t.Fatal(profile, module, err)
		}
	}
}
