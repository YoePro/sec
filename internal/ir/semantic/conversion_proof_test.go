package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

const conversionProofSource = `module main

enum State int32 {
    Idle = 1
    Running = 2
    Failed = 200
}

enum DeviceMode bit[8] {
    Off = 0
    Running = 1
}

fn Widen(a: int8, b: uint8) int64 {
    return int64(a) + int64(b)
}

fn Same(value: int32) int32 {
    return int32(value)
}

fn Constant() int8 {
    return int8(100)
}

fn Native(value: uint32) int {
    return int(value)
}

fn StateCode(value: State) uint8 {
    return uint8(value)
}

fn ModeByte(value: DeviceMode) uint8 {
    return uint8(value)
}

fn ModeWide(value: DeviceMode) int16 {
    return int16(value)
}
`

// buildWithPlan analyzes and builds source for a selected pointer width.
func buildWithPlan(t *testing.T, source string, width uint16) (*sema.Analyzer, *Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "proof.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		// A conversion that may fail is rejected by Sema unless try protects
		// it (types.md "Explicit conversions"); callers see that as err.
		return analyzer, nil, fmt.Errorf("sema: %v", errs)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"proof.sec"}})
	return analyzer, module, err
}

// MD-104: an exact conversion carries Sema's proof — constant, source-domain,
// closed-enum-domain or open-enum-domain — and a target-sized proof records
// the selected plan (Option A). uint32 -> int is exact on a 64-bit plan.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 124
//   - rules/types/types.md — "Exact integer conversion proof"
func TestExactConversionsCarryPlanProofs(t *testing.T) {
	_, module, err := buildWithPlan(t, conversionProofSource, 64)
	if err != nil {
		t.Fatal(err)
	}
	if module.PointerWidthBits != 64 {
		t.Fatalf("module plan width %d", module.PointerWidthBits)
	}
	text := Format(module)
	for _, want := range []string{
		"numeric.convert-exact %0 proof=source-domain",
		"proof=source-domain plan=64",
		"enum.to-integer %0 proof=closed-enum-domain",
		"enum.to-integer %0 proof=open-enum-domain",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// MD-104: on a 32-bit plan uint32 -> int is not exact (fail closed, never
// truncate: without try Sema rejects it); an open bit[8] enum is not exact into int8, and a closed enum
// with one case beyond the target is not exact.
func TestUnprovenConversionsFailClosed(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		width  uint16
		want   string
	}{
		"uint32 to int on 32-bit plan": {"module main\nfn Native(value: uint32) int {\n    return int(value)\n}\n", 32, "converting uint32 to int may fail with ConversionError.OutOfRange and requires try"},
		"open bit[8] enum to int8":     {"module main\nenum DeviceMode bit[8] {\n    Off = 0\n    Running = 1\n}\nfn Mode(value: DeviceMode) Result[int8, ArithmeticError] {\n    return int8(value)\n}\n", 64, "enum"},
		"closed enum case beyond int8": {"module main\nenum State int32 {\n    Idle = 1\n    Failed = 200\n}\nfn Code(value: State) Result[int8, ArithmeticError] {\n    return int8(value)\n}\n", 64, "enum"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := buildWithPlan(t, test.source, test.width)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want fail-closed %q, got %v", test.want, err)
			}
		})
	}
}

// MD-104: the verifier rejects a convert-exact without proof, with an
// unknown basis, or with a proof made for another plan; a module analyzed for
// 64 bits cannot be reused as a 32-bit module.
func TestConversionProofVerifierRejectsStaleProofs(t *testing.T) {
	_, module, err := buildWithPlan(t, conversionProofSource, 64)
	if err != nil {
		t.Fatal(err)
	}
	var site *Operation
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				op := &block.Operations[index]
				if op.Kind == OpNumericConvertExact && op.ConversionProof != nil && op.ConversionProof.PlanPointerWidthBits == 64 {
					site = op
				}
			}
		}
	}
	if site == nil {
		t.Fatal("no plan-specific convert-exact")
	}
	original := *site.ConversionProof
	for name, proof := range map[string]*ConversionProof{
		"missing":    nil,
		"basis":      {Basis: "bit-pattern"},
		"other plan": {Basis: original.Basis, PlanPointerWidthBits: 32},
	} {
		site.ConversionProof = proof
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "numeric.convert-exact") {
			t.Errorf("%s: verifier accepted %+v: %v", name, proof, err)
		}
	}
	site.ConversionProof = &original
	module.PointerWidthBits = 32
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "64-bit plan") {
		t.Fatalf("64-bit proof accepted in a 32-bit module: %v", err)
	}
}

// MD-104: proofs are recomputed per analysis and plan, never inherited:
// analyzing the same source cold for 64 then 32 bits gives different
// results for the target-sized conversion.
func TestConversionProofIsRecomputedPerPlan(t *testing.T) {
	source := "module main\nfn Native(value: uint32) int {\n    return int(value)\n}\n"
	if _, _, err := buildWithPlan(t, source, 64); err != nil {
		t.Fatalf("64-bit: %v", err)
	}
	if _, _, err := buildWithPlan(t, source, 32); err == nil {
		t.Fatal("32-bit build reused the 64-bit exactness proof")
	}
	if _, _, err := buildWithPlan(t, source, 64); err != nil {
		t.Fatalf("64-bit after 32-bit: %v", err)
	}
}
