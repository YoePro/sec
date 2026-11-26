package semantic

import (
	"strings"
	"testing"
)

// TestAtomicEvidenceVerifier: semantic_ir.md § 123B(2)-(3). A fence built
// from source carries Sema's open obligation; the verifier rejects a missing
// entry, a proven-unsupported or unknown availability spelling, supported
// evidence without implementation or context, an obligation without its
// fact, evidence that disagrees with the op's operation or order, a carrier
// on a fence and evidence on a non-atomic operation (T43-style fail-closed).
// Rules: rules/compiler/semantic_ir.md — § 123B; rules/concurrency/atomics.md — §§ 37A(11)-(12).
func TestAtomicEvidenceVerifier(t *testing.T) {
	source := "module main\n\nfn Publish() void {\n    atomic.Fence(MemoryOrder.Release)\n}\n\nfn main() int {\n    Publish()\n    return 0\n}\n"
	build := func() (*Module, *Operation) {
		module, err := hostedModule(t, source, 19)
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					if block.Operations[i].Kind == OpAtomicFence {
						return module, &block.Operations[i]
					}
				}
			}
		}
		t.Fatal("no fence")
		return nil, nil
	}
	module, fence := build()
	if len(fence.AtomicEvidence) != 1 || fence.AtomicEvidence[0].Availability != "unknown" || fence.AtomicEvidence[0].Context != "Ordinary" || fence.AtomicEvidence[0].Missing == "" {
		t.Fatalf("fence evidence: %+v", fence.AtomicEvidence)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if text := Format(module); !strings.Contains(text, "capability{Ordinary unknown missing=") {
		t.Fatalf("printed evidence:\n%s", text)
	}
	supported := func(e *AtomicCapabilityEvidence) {
		e.Availability, e.Missing, e.Implementation, e.ImplementationKind = "supported", "", "test-fence", "native-instruction"
	}
	module, fence = build()
	supported(&fence.AtomicEvidence[0])
	if err := Verify(module); err != nil {
		t.Fatalf("supported fence rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Operation){
		"missing evidence":       func(op *Operation) { op.AtomicEvidence = nil },
		"unsupported":            func(op *Operation) { op.AtomicEvidence[0].Availability = "unsupported" },
		"unknown without fact":   func(op *Operation) { op.AtomicEvidence[0].Missing = "" },
		"supported without impl": func(op *Operation) { supported(&op.AtomicEvidence[0]); op.AtomicEvidence[0].Implementation = "" },
		"supported no context":   func(op *Operation) { supported(&op.AtomicEvidence[0]); op.AtomicEvidence[0].Context = "" },
		"noncanonical context":   func(op *Operation) { op.AtomicEvidence[0].Context = "thread::main" },
		"wrong order":            func(op *Operation) { op.AtomicEvidence[0].Order = "SeqCst" },
		"wrong operation":        func(op *Operation) { op.AtomicEvidence[0].Operation = "Load" },
		"fence carrier":          func(op *Operation) { op.AtomicEvidence[0].Carrier = ":uint:uint64" },
		"no provenance":          func(op *Operation) { op.AtomicEvidence[0].Provenance = "" },
	} {
		module, fence := build()
		mutate(fence)
		if err := Verify(module); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	module, fence = build()
	other := Operation{Kind: OpReturn, AtomicEvidence: fence.AtomicEvidence}
	if err := verifyAtomicEvidence(other); err == nil || !strings.Contains(err.Error(), "carries atomic capability evidence") {
		t.Fatalf("evidence on a non-atomic op: %v", err)
	}
	if err := VerifyAtomicEmissionEvidence(module); err == nil || !strings.Contains(err.Error(), "atomic.Fence") {
		t.Fatalf("emission gate: %v", err)
	}
}
