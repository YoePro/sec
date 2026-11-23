package layout

import "strings"

// FloatCapabilities is the CompilationPlan's binary floating-point contract
// (P20 § 83): whether the target preserves every Sec float semantic, by
// hardware or by a preserving helper (a soft-float runtime). The zero value
// is Unknown — never host-derived support.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 6, 9, 83.
type FloatCapabilities struct {
	Known bool
	// Each field is a required P20 semantic the plan preserves.
	Binary32, Binary64, NearestEven, Subnormals, IEEEComparison, Remainder, SignedZero bool
}

// IEEEPreservingFloats is the contract of an LLVM-lowered target without
// fast-math: LLVM keeps IEEE-754 binary32/binary64 semantics (nearest-even,
// subnormals, ordered/unordered comparison, frem, signed zero) in hardware
// or through its soft-float runtime.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 75-83.
func IEEEPreservingFloats() FloatCapabilities {
	return FloatCapabilities{Known: true, Binary32: true, Binary64: true, NearestEven: true, Subnormals: true, IEEEComparison: true, Remainder: true, SignedZero: true}
}

// Missing names the required semantics a known plan lacks, in canonical
// order; an unknown plan reports none (it is validated when resolved).
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — § 83.
func (f FloatCapabilities) Missing(width int) []string {
	if !f.Known {
		return nil
	}
	var missing []string
	add := func(present bool, name string) {
		if !present {
			missing = append(missing, name)
		}
	}
	if width == 32 {
		add(f.Binary32, "binary32")
	} else {
		add(f.Binary64, "binary64")
	}
	add(f.NearestEven, "round-to-nearest-even")
	add(f.Subnormals, "subnormal preservation")
	add(f.IEEEComparison, "IEEE comparison")
	add(f.Remainder, "IEEE remainder")
	add(f.SignedZero, "signed zero")
	return missing
}

// MissingText joins Missing for a diagnostic.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — § 83.
func (f FloatCapabilities) MissingText(width int) string {
	return strings.Join(f.Missing(width), ", ")
}
