package secmlir

import (
	"errors"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
)

const atomicEvidenceSource = `module main

fn Read(value: ref Atomic[uint64]) uint64 {
    return value.Load()
}

fn Publish() void {
    atomic.Fence(MemoryOrder.Release)
}

fn main() int {
    Publish()
    return 0
}
`

// fenceOnlySource has a single reachable atomic operation.
const fenceOnlySource = `module main

fn Publish() void {
    atomic.Fence(MemoryOrder.Release)
}

fn main() int {
    Publish()
    return 0
}
`

func buildAtomicEvidence(t *testing.T, source string, plan layout.ResolvedScalarPlan) (*semantic.Module, *sema.Analyzer) {
	t.Helper()
	parsed := parser.New(lexer.NewWithFile(source, "atomic.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"atomic.sec"}, MaxPackage: 19})
	if err != nil {
		t.Fatal(err)
	}
	return module, a
}

func atomicEvidenceOps(module *semantic.Module) []semantic.Operation {
	var ops []semantic.Operation
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if len(op.AtomicEvidence) != 0 {
					ops = append(ops, op)
				}
			}
		}
	}
	return ops
}

// TestAtomicEmissionGate: Semantic IR carries Sema's capability result on
// every atomic operation (semantic_ir.md § 123B). On a registered target
// without atomic facts every request is an open obligation and emission
// fails closed with the missing-evidence error, not as a source error or a
// proven unsupported capability (T52). With synthetic supported facts the
// evidence of the fence is retained and emission stops at the missing lowering as
// an implementation limitation, never a weakened or volatile substitute
// (T48, T55, T58, T59).
// Rules: rules/concurrency/atomics.md — §§ 37A(12)-(13), 47(9)-(10); rules/compiler/semantic_ir.md — § 123B.
func TestAtomicEmissionGate(t *testing.T) {
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	module, _ := buildAtomicEvidence(t, atomicEvidenceSource, plan)
	ops := atomicEvidenceOps(module)
	if len(ops) != 2 {
		t.Fatalf("atomic operations with evidence: %d", len(ops))
	}
	missing := map[string]string{}
	for _, op := range ops {
		for _, e := range op.AtomicEvidence {
			if e.Availability != "unknown" || e.Missing == "" {
				t.Fatalf("registered target must leave an open obligation: %+v", e)
			}
			missing[e.Operation] = e.Missing
		}
	}
	// Read is reached by no root; the fence has no plan fact.
	if missing["Load"] != "unresolved execution-context reachability" || !strings.Contains(missing["Fence"], "no capability fact") {
		t.Fatalf("obligations: %v", missing)
	}
	_, err = EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
	var gate *AtomicCapabilityEvidenceError
	if !errors.As(err, &gate) || !strings.Contains(err.Error(), "cannot be verified for an unresolved execution context") || gate.DiagnosticID() != "S1265" {
		t.Fatalf("open obligation must stop emission: %v", err)
	}
	// An incomplete atomic layout is its own category (S1266).
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for i := range block.Operations {
				for j := range block.Operations[i].AtomicEvidence {
					block.Operations[i].AtomicEvidence[j].Missing = "incomplete atomic layout: atomic storage alignment of int64"
				}
			}
		}
	}
	if _, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15}); !errors.As(err, &gate) || gate.DiagnosticID() != "S1266" || !strings.Contains(gate.Help(), "not a proven unsupported operation") {
		t.Fatalf("incomplete layout classification: %v", err)
	}

	withAlignments := plan
	withAlignments.AtomicAlignments = layout.ScalarAlignments{{Class: layout.IntegerStorage, WidthBits: 64}: 8}
	_, analyzer := buildAtomicEvidence(t, fenceOnlySource, withAlignments)
	var facts []layout.AtomicCapability
	for _, result := range analyzer.AtomicCapabilities() {
		facts = append(facts, layout.AtomicCapability{Key: result.Requirement.Key, Availability: layout.AtomicSupported, Provenance: "test-only",
			Implementation: layout.AtomicImplementation{Kind: layout.AtomicNativeInstruction, Identity: "test-" + result.Requirement.Key.Operation, Blocking: layout.AtomicPropertyNo}})
	}
	supportedPlan, err := withAlignments.WithAtomicCapabilities(facts)
	if err != nil {
		t.Fatal(err)
	}
	module, _ = buildAtomicEvidence(t, fenceOnlySource, supportedPlan)
	ops = atomicEvidenceOps(module)
	if len(ops) != 1 {
		t.Fatalf("supported operations: %d", len(ops))
	}
	for _, op := range ops {
		if len(op.AtomicEvidence) != 1 || op.AtomicEvidence[0].Availability != "supported" || !strings.HasPrefix(op.AtomicEvidence[0].Implementation, "test-") {
			t.Fatalf("supported evidence: %+v", op.AtomicEvidence)
		}
	}
	if text := semantic.Format(module); !strings.Contains(text, "capability{Ordinary supported impl=test-Fence}") {
		t.Fatalf("printed evidence:\n%s", text)
	}
	_, err = EmitWithOptions(module, supportedPlan, EmitOptions{DialectVersion: 15})
	var limitation *UnsupportedLoweringError
	if !errors.As(err, &limitation) || errors.As(err, &gate) {
		t.Fatalf("supported atomics stop at the missing lowering: %v", err)
	}
}
