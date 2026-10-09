package nominal_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	legacy "sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/lowering/secmlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

// analyze uses a checked-in nominal inventory with exact target scalar facts.
// Rules: rules/types/types.md — nominal identity; memory/layout.md — scalar plan.
func analyze(t *testing.T, typ, triple string) (*ast.Program, *sema.Analyzer) {
	t.Helper()
	data, err := os.ReadFile("../../../../testdata/sema/nominal_core/types.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	return analyzeText(t, strings.ReplaceAll(string(data), "TARGET", typ), triple)
}

// analyzeText establishes source validity before auditing any representation.
// Rules: rules/compiler/compiler_pipeline.md — Sema before lowering.
func analyzeText(t *testing.T, source, triple string) (*ast.Program, *sema.Analyzer) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "nominal.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	plan, err := targetplan.Plan(triple)
	if err != nil {
		t.Fatal(err)
	}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatalf("%s: %v", source, errs)
	}
	return program, a
}

// TestCoreRepresentationAudit distinguishes implemented representation from
// explicit unsupported boundaries on both plans, preserving no-partial output.
// Rules: rules/compiler/semantic_ir.md — source type identity and §20 Units;
// rules/mlir/packages/sec-mlir-dialect_package11.md — Result/Option boundaries;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §6 named arrays.
func TestCoreRepresentationAudit(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		for _, typ := range []string{"Child", "ID[int]", "ID[string]", "Wrapped[int32]", "Box[int]", "Box[ID[int]]", "Result[int,Failure]", "Option[int]", "Choice[int]", "Failure", "int[2]"} {
			t.Run(triple+"/"+typ, func(t *testing.T) {
				program, a := analyze(t, typ, triple)
				module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"})
				if err != nil {
					t.Fatal(err)
				}
				for _, fn := range module.Functions {
					if fn.Name != "Identities" && fn.Name != "Nested" {
						continue
					}
					left, right := fn.Parameters[0].Value.Type, fn.Parameters[1].Value.Type
					l, _ := module.Types.Lookup(left)
					r, _ := module.Types.Lookup(right)
					if left == right || l.Identity == r.Identity || l.Identity == "" || r.Identity == "" {
						t.Fatalf("generic source identities merged: %#v %#v", l, r)
					}
					if len(l.TypeArgs) != 1 || len(r.TypeArgs) != 1 || l.TypeArgs[0] == r.TypeArgs[0] {
						t.Fatalf("concrete arguments lost: %#v %#v", l, r)
					}
				}
				if err := semantic.Verify(module); err != nil {
					t.Fatal(err)
				}
				plan, _ := targetplan.Plan(triple)
				if out, err := secmlir.Emit(module, plan); err != nil || len(out) == 0 {
					t.Fatalf("Sec MLIR: %s %v", out, err)
				}
			})
		}
		for _, typ := range []string{"Outcome", "Samples", "Task[int]", "Thread[int]", "Reader", "Registers", "any", "Instant", "date", "int<Meter>", "decimal<Meter>"} {
			t.Run(triple+"/unsupported/"+typ, func(t *testing.T) {
				program, a := analyze(t, typ, triple)
				module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"})
				var unsupported *semantic.UnsupportedFeatureError
				if module != nil || !errors.As(err, &unsupported) || unsupported.Location.Line == 0 {
					t.Fatalf("erased or unclassified %s: %v %v", typ, module, err)
				}
			})
		}
	}
}

// TestLegacyRepresentationAudit pins the actual legacy signature boundaries;
// source analysis still establishes identity before permitted carrier erasure.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/types/types.md — Type identity, explicit lowering support.
func TestLegacyRepresentationAudit(t *testing.T) {
	for _, typ := range []string{"Base", "Child", "Task[int]", "Thread[int]", "Reader", "Registers", "Instant", "any", "Option[int]", "Result[int,Failure]"} {
		declarations := ""
		switch typ {
		case "Base", "Child":
			declarations = "type Base int32\ntype Child Base"
		case "Reader":
			declarations = "interface Reader { fn Read() int }"
		case "Registers":
			declarations = "type Registers register[8] { Value: bit[8] }"
		case "Result[int,Failure]":
			declarations = "enum Failure error { Failed }"
		}
		data, err := os.ReadFile("../../../../testdata/sema/nominal_core/legacy.sec.in")
		if err != nil {
			t.Fatal(err)
		}
		text := strings.NewReplacer("DECLARATIONS", declarations, "TARGET", typ).Replace(string(data))
		program, a := analyzeText(t, text, "x86_64-pc-linux-gnu")
		for _, generate := range []struct {
			name string
			fn   func(*ast.Program, *sema.Analyzer, string) (string, error)
		}{{"LLVM", llvm.GenerateAnalyzed}, {"legacy MLIR", legacy.GenerateAnalyzed}} {
			out, err := generate.fn(program, a, "x86_64-pc-linux-gnu")
			wantSuccess := typ == "Base" || typ == "Child"
			if (err == nil) != wantSuccess {
				t.Fatalf("unexpected boundary %s %s: %v", generate.name, typ, err)
			}
			if !wantSuccess {
				var unsupported *semantic.UnsupportedFeatureError
				if !errors.As(err, &unsupported) || unsupported.Location.Line == 0 {
					t.Fatalf("unclassified unsupported signature %s %s: %v", generate.name, typ, err)
				}
			}
			if err != nil && out != "" {
				t.Fatalf("partial output for %s %s", generate.name, typ)
			}
			if err == nil && out == "" {
				t.Fatalf("empty success for %s %s", generate.name, typ)
			}
		}
	}
}

// TestNamedArgumentReferencesAreVerified rejects forged generic references
// before an emitter can drop them from a physical carrier representation.
// Rules: rules/compiler/semantic_ir.md — type reference verification;
// rules/types/types.md — Generic and parameterized types.
func TestNamedArgumentReferencesAreVerified(t *testing.T) {
	table := semantic.NewTypeTable()
	base := table.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", TargetSize: true})
	table.Intern(semantic.Type{Kind: semantic.TypeNamed, Name: "ID", Identity: "main::ID<int>", Base: base, TypeArgs: []semantic.TypeID{0}})
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: table}
	if err := semantic.Verify(module); err == nil || !strings.Contains(err.Error(), "invalid type argument") {
		t.Fatalf("invalid generic reference accepted: %v", err)
	}
}

// TestQuantityBoundaryAppliesToRawAndAnalyzedAPIs ensures a valid quantity
// cannot silently turn into an ordinary integer through alternate entry points.
// Rules: rules/compiler/semantic_ir.md — §20; semantic-ir-units correction — Erasure boundary.
func TestQuantityBoundaryAppliesToRawAndAnalyzedAPIs(t *testing.T) {
	program, a := analyze(t, "int<Meter>", "x86_64-pc-linux-gnu")
	for _, generate := range []func() (string, error){
		func() (string, error) { return llvm.Generate(program) },
		func() (string, error) { return llvm.GenerateAnalyzed(program, a, "x86_64-pc-linux-gnu") },
		func() (string, error) { return legacy.GenerateWithTriple(program, "x86_64-pc-linux-gnu") },
		func() (string, error) { return legacy.GenerateAnalyzed(program, a, "x86_64-pc-linux-gnu") },
	} {
		out, err := generate()
		var unsupported *semantic.UnsupportedFeatureError
		if out != "" || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "unit quantity") || unsupported.Location.Line == 0 {
			t.Fatalf("quantity boundary: %q %v", out, err)
		}
	}
}
