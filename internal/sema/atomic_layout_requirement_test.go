package sema

import (
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// TestAtomicLayoutRequirement maps Atomic[T] to atomic storage over T rather
// than a fieldless struct, which resolved as a complete zero-sized layout and
// overlapped the next field. A registered target without a verified atomic
// minimum yields an incomplete layout naming the missing fact; with a fact
// the field is placed at the atomic alignment.
// Rules: rules/memory/layout.md — § 47.6; rules/concurrency/atomics.md — §§ 7(9), 37A(7).
func TestAtomicLayoutRequirement(t *testing.T) {
	source := "module main\n\ntype Count uint64\n\ntype Holder struct {\n    flag: bool,\n    count: Atomic[Count],\n}\n\nfn main() int {\n    return 0\n}\n"
	p := parser.New(lexer.NewWithFile(source, "atomic_layout.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	a := NewAnalyzerWithScalarPlan(plan)
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	requirement := LayoutRequirementOf(a.types["Holder"])
	field := requirement.Fields[1].Requirement
	if field.Kind != layout.LayoutAtomic || field.Carrier == nil || field.Carrier.Kind != layout.LayoutNamed || field.Carrier.Carrier.Scalar != layout.ScalarUnsigned {
		t.Fatalf("Atomic[Count] requirement: %+v", field)
	}
	resolver, err := layout.NewResolver(plan, plan.Alignments)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(requirement); got.Completeness != layout.LayoutIncomplete || !strings.Contains(strings.Join(got.Missing, ";"), "atomic storage alignment of int64") {
		t.Fatalf("registered target without atomic facts: %+v", got)
	}
	plan.AtomicAlignments = layout.ScalarAlignments{{Class: layout.IntegerStorage, WidthBits: 64}: 8}
	resolver, err = layout.NewResolver(plan, plan.Alignments)
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.Resolve(requirement)
	if !got.Complete() || got.SizeBytes.Int64() != 16 || got.Fields[1].Offset.Int64() != 8 {
		t.Fatalf("Holder with a synthetic atomic fact: %+v", got)
	}
}
