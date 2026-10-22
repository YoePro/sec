package secmlir

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/codegen/targetplan"
	semantic "sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// package15IntegrationFixture combines Package 15 references with P12 match
// payload borrows, P13 struct fields and P14 fixed arrays, including a
// runtime-checked element whose failure CFG must survive.
const package15IntegrationFixture = `module main

type Pair struct {
    left: int,
    right: int,
}

fn Field(pair: ref Pair) ref int {
    return ref pair.right
}

fn Bump(value: ref mut int) void {
    *value += 1
}

fn At(values: ref mut int[4], index: int) void {
    let element := ref mut values[index]
    *element = 9
}

fn Payload(value: Option[Pair]) int {
    match value {
        Some(ref pair) => {
            return pair.left
        }
        None => {
            return 0
        }
    }
}

fn main() int {
    let mut pair := Pair { left: 1, right: 2 }
    let mut values := [1, 2, 3, 4]
    At(values, 2)
    Bump(pair.left)
    let right := *Field(pair)
    return right + Payload(None) - 2
}
`

// P15-65: every successful Package 15 program travels source -> target-aware
// Sema -> verified Semantic IR -> schema-11 Sec MLIR -> sec-mlir-opt (absolute
// tool path) on 32- and 64-bit CompilationPlans: the three schema-11
// verifiers pass, P6 keeps Place/reference wrappers and metadata while
// resolving referents, P8 lowers the integer computation selectively, and the
// runtime-check failure CFG survives. Dynamic-epoch validity is not
// producible from Package 15 source (Arena allocation does not lower yet); it
// is covered from verified Semantic IR onward by
// TestPackage15EmitsDynamicReferenceGuards.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 59–67, 155
//   - rules/compiler/compiler_pipeline.md — § 59 (MD-086)
func TestPackage15SourceToSchema11Pipeline(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	if !filepath.IsAbs(tool) {
		t.Fatalf("sec-mlir-opt path %q is not absolute", tool)
	}
	sources := map[string]string{"integration": package15IntegrationFixture}
	for name, source := range package15EmitFixtures {
		sources[name] = source
	}
	for name, source := range sources {
		for _, width := range []uint16{32, 64} {
			t.Run(name+"/"+strconv.Itoa(int(width)), func(t *testing.T) {
				plan := testPlan(width)
				resolved, err := targetplan.Plan(plan.LLVMTriple)
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(source, name+".sec"))
				parsed := p.Parse()
				if parsed.HasErrors {
					t.Fatalf("parse: %v", p.Errors())
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(resolved)
				if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
					t.Fatalf("sema: %v", errs)
				}
				module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{name + ".sec"}, MaxPackage: 15})
				if err != nil {
					t.Fatalf("semantic build: %v", err)
				}
				if err := semantic.Verify(module); err != nil {
					t.Fatalf("semantic verify: %v", err)
				}
				output, err := Emit(module, resolved)
				if err != nil {
					t.Fatalf("emit: %v", err)
				}
				text := string(output)
				if !strings.Contains(text, "sec.dialect_version = 11 : i32") || !strings.Contains(text, `sec.target_arch = "`+plan.TargetArch+`"`) {
					t.Fatalf("output does not record schema 11 and the %s plan", plan.TargetArch)
				}
				if name == "integration" && (!strings.Contains(text, `bounds_kind = "runtime-check"`) || !strings.Contains(text, `"sec.fail.bounds"`)) {
					t.Fatalf("the runtime-checked element lost its failure CFG:\n%s", text)
				}
				verifyPackage15MLIR(t, name, output)
			})
		}
	}
}
