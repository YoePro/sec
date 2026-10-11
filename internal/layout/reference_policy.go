package layout

import "fmt"

// DefaultLogicalEpochBits is the canonical logical validity-epoch width on
// every target, independent of pointer width.
//
// Rules:
//   - rules/memory/reference_model.md — § 33 "Default logical epoch width", § 34
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — § 24
const DefaultLogicalEpochBits uint16 = 64

// ResolvedReferencePolicy is the reference subset of a CompilationPlan. The
// zero value is the canonical default: a 64-bit logical epoch on 32- and
// 64-bit targets alike. A shorter width is selected only at compile time and
// only together with the proof or exhaustion strategy that keeps stale
// references safe; runtime code never chooses the width, and the width is
// never part of a source reference type.
type ResolvedReferencePolicy struct {
	LogicalEpochBits uint16
	// ShortWidthProof names the compile-time proof/retirement/exhaustion
	// strategy that permits LogicalEpochBits below the default.
	ShortWidthProof string
}

// EpochBits returns the effective logical epoch width.
func (p ResolvedReferencePolicy) EpochBits() uint16 {
	if p.LogicalEpochBits == 0 {
		return DefaultLogicalEpochBits
	}
	return p.LogicalEpochBits
}

// Validate rejects a shorter-than-default width without a compile-time proof
// and widths that are not whole machine-independent byte multiples.
func (p ResolvedReferencePolicy) Validate() error {
	bits := p.EpochBits()
	switch bits {
	case 8, 16, 32, 64:
	default:
		return fmt.Errorf("logical epoch width must be 8, 16, 32 or 64 bits, got %d", bits)
	}
	if bits < DefaultLogicalEpochBits && p.ShortWidthProof == "" {
		return fmt.Errorf("a %d-bit logical epoch requires a compile-time proof or exhaustion strategy", bits)
	}
	return nil
}
