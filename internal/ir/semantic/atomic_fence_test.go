package semantic

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// atomic.Fence lowers to one atomic.fence operation with Sema's established
// order and no operands, results or storage; the verifier rejects a Relaxed
// or missing order and any operand (atomics.md §§ 26, 40; semantic_ir.md § 122).
func TestAtomicFenceLowersToSemanticIR(t *testing.T) {
	source := "module main\n\nfn Publish() void {\n    atomic.Fence(MemoryOrder.Release)\n    atomic.Fence(MemoryOrder.SeqCst)\n}\n"
	p := parser.New(lexer.NewWithFile(source, "fence.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"fence.sec"}})
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if !strings.Contains(text, "atomic.fence order=Release") || !strings.Contains(text, "atomic.fence order=SeqCst") {
		t.Fatalf("fences not lowered:\n%s", text)
	}
	var fence *Operation
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if block.Operations[index].Kind == OpAtomicFence && fence == nil {
					fence = &block.Operations[index]
				}
			}
		}
	}
	if fence == nil || len(fence.Operands) != 0 || len(fence.Results) != 0 || fence.AtomicOrder != "Release" {
		t.Fatalf("fence operation: %+v", fence)
	}
	for _, bad := range []Operation{
		{Kind: OpAtomicFence, AtomicOrder: "Relaxed"},
		{Kind: OpAtomicFence},
		{Kind: OpAtomicFence, AtomicOrder: "SeqCst", Operands: []ValueID{1}},
	} {
		if verifyAtomicFence(bad) == nil {
			t.Errorf("verifier accepted %+v", bad)
		}
	}
	if err := verifyAtomicFence(Operation{Kind: OpAtomicFence, AtomicOrder: "AcqRel"}); err != nil {
		t.Fatal(err)
	}
}
