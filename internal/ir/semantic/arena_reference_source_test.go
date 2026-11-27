package semantic

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"testing"
)

// P19-18 replaces white-box-only reference integration evidence with source
// Arena allocations and their canonical dynamic-epoch reference-use CFG.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §102;
// rules/memory/reference_model.md — §45.
func TestArenaReferenceSource(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/arena_references/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "armv7"} {
		definition, _ := target.Find(target.Target{OS: "linux", Arch: arch})
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), "arena-reference.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(plan)
		a.SetAllocationProfile("hosted")
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 19})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		for _, fn := range m.Functions {
			allocations, uses, guards := 0, 0, 0
			var domain, epoch uint32
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					op := &block.Operations[i]
					switch op.Kind {
					case OpArenaNew:
						allocations++
						if op.Reference == nil || op.Reference.Validity != ReferenceDynamicEpoch {
							t.Fatal("allocation has no dynamic reference facts")
						}
						if domain != 0 && (domain != op.Reference.InvalidationDomain || epoch != op.Reference.EpochDependency) {
							t.Fatal("allocation advanced the validity epoch")
						}
						domain, epoch = op.Reference.InvalidationDomain, op.Reference.EpochDependency
					case OpReferenceIsValid:
						guards++
					case OpReferenceDerefPlace:
						uses++
						if op.Reference == nil || op.Reference.Validity != ReferenceDynamicEpoch || op.Reference.InvalidationDomain != domain || op.Reference.EpochDependency != epoch || op.ReferenceGuard == 0 {
							t.Fatal("use detached from allocation/validity guard", op)
						}
						saved := op.ReferenceGuard
						op.ReferenceGuard = 0
						if err := Verify(m); err == nil {
							t.Fatal("unguarded Arena reference verified")
						}
						op.ReferenceGuard = saved
						original := op.Reference
						op.Reference = cloneReferenceMetadata(original)
						op.Reference.EpochDependency++
						if err := Verify(m); err == nil {
							t.Fatal("use with inconsistent allocation epoch verified")
						}
						op.Reference = original
					}
				}
			}
			wantAlloc, wantUse := 1, 2
			if fn.Name == "Repeated" {
				wantAlloc, wantUse = 2, 3
			}
			if allocations != wantAlloc || uses != wantUse || guards != wantUse {
				t.Fatalf("%s: allocations=%d uses=%d guards=%d", fn.Name, allocations, uses, guards)
			}
		}
		data, err := Serialize(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Deserialize(data); err != nil {
			t.Fatal(err)
		}
	}
}
