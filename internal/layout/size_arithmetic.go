package layout

import (
	"fmt"
	"math/big"
)

// SizeArithmetic holds copied, resolved target arithmetic capabilities. Target
// is a diagnostic label, not a CompilationPlan identity or a layout cache key.
// This helper selects no type representation, alignment, ABI or descriptor.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§8–10,44;
// rules/memory/layout.md — §§3,7(4–5),8(5),9,12,24.
type SizeArithmetic struct {
	target     string
	maximum    *big.Int
	alignments map[string]struct{}
}

// SizeArithmeticError retains an exact failure and its caller-supplied field,
// count or concrete-instantiation path. Numeric strings are detached snapshots;
// they cannot expose mutable arithmetic state to a diagnostic consumer.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,44.
type SizeArithmeticError struct {
	Target    string
	Path      string
	Operation string
	Requested string
	Maximum   string
	Reason    string
}

// Error renders exact extents without narrowing to a host-sized integer.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,44.
func (e *SizeArithmeticError) Error() string {
	return fmt.Sprintf("layout %s at %s for target %s: %s (requested %s; maximum %s)",
		e.Operation, e.Path, e.Target, e.Reason, e.Requested, e.Maximum)
}

// NewSizeArithmetic snapshots explicitly resolved capabilities. Missing target
// facts are errors; pointer width and the host never supply a fallback. An empty
// alignment set permits arithmetic but no alignment-dependent calculation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§6,9–10;
// rules/memory/layout.md — §§3,7(4–5),8(5).
func NewSizeArithmetic(target string, maximum *big.Int, supportedAlignments []*big.Int) (*SizeArithmetic, error) {
	if target == "" || maximum == nil || maximum.Sign() < 0 {
		return nil, fmt.Errorf("layout arithmetic requires a target and a resolved nonnegative maximum byte extent")
	}
	a := &SizeArithmetic{target: target, maximum: new(big.Int).Set(maximum), alignments: make(map[string]struct{}, len(supportedAlignments))}
	for _, alignment := range supportedAlignments {
		if alignment == nil || alignment.Sign() <= 0 {
			return nil, fmt.Errorf("layout arithmetic for target %s requires positive supported alignments", target)
		}
		a.alignments[alignment.String()] = struct{}{}
	}
	return a, nil
}

// CheckedAdd calculates an exact byte extent and checks the resolved target
// bound before any backend APInt/index or host-size conversion.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §10;
// rules/memory/layout.md — §7(4–5).
func (a *SizeArithmetic) CheckedAdd(path string, left, right *big.Int) (*big.Int, error) {
	if err := a.operands(path, "add", left, right); err != nil {
		return nil, err
	}
	return a.extent(path, "add", new(big.Int).Add(left, right))
}

// CheckedMul checks the exact product, not the operands' host representability.
// In particular a zero stride times a huge logical count still has zero extent.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,15;
// rules/memory/layout.md — §§7(4–5),12(5),24(4).
func (a *SizeArithmetic) CheckedMul(path string, left, right *big.Int) (*big.Int, error) {
	if err := a.operands(path, "multiply", left, right); err != nil {
		return nil, err
	}
	return a.extent(path, "multiply", new(big.Int).Mul(left, right))
}

// AlignUp uses the package's exact modular-padding formula. Alignment must be
// positive and present in the resolved support set, including non-power-of-two
// alignments when the selected target explicitly supports them.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §10;
// rules/memory/layout.md — §§7(4–5),8(5).
func (a *SizeArithmetic) AlignUp(path string, offset, alignment *big.Int) (*big.Int, error) {
	if err := a.operands(path, "align-up", offset, alignment); err != nil {
		return nil, err
	}
	if _, ok := a.alignments[alignment.String()]; !ok || alignment.Sign() == 0 {
		return nil, a.failure(path, "align-up", alignment.String(), "alignment is not supported by the resolved target")
	}
	remainder := new(big.Int).Mod(offset, alignment)
	padding := new(big.Int).Mod(new(big.Int).Sub(alignment, remainder), alignment)
	return a.extent(path, "align-up", new(big.Int).Add(offset, padding))
}

// StrideOf computes the smallest aligned stride satisfying both the complete
// element size and the resolved placement minimum. The caller supplies that
// target/policy minimum; this helper does not choose zero-sized-element policy
// or authorize reference/address arithmetic for a zero-sized element.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,15;
// rules/memory/layout.md — §§9(1–5),24(2–5).
func (a *SizeArithmetic) StrideOf(path string, size, alignment, minimumStride *big.Int) (*big.Int, error) {
	if err := a.operands(path, "stride", size, alignment, minimumStride); err != nil {
		return nil, err
	}
	minimum := size
	if minimumStride.Cmp(minimum) > 0 {
		minimum = minimumStride
	}
	stride, err := a.AlignUp(path, minimum, alignment)
	if failure, ok := err.(*SizeArithmeticError); ok {
		failure.Operation = "stride"
	}
	return stride, err
}

// ArraySize calculates bytes from a resolved stride and an exact logical count.
// Neither input is narrowed. Zero-length arrays have zero extent; zero-sized
// elements retain their original logical count in their owner's layout facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,15;
// rules/memory/layout.md — §§12(1–5),24(4–5).
func (a *SizeArithmetic) ArraySize(path string, stride, count *big.Int) (*big.Int, error) {
	if err := a.operands(path, "array-size", stride, count); err != nil {
		return nil, err
	}
	// A zero count cannot make an unrepresentable element stride complete.
	if _, err := a.extent(path, "array-stride", stride); err != nil {
		return nil, err
	}
	return a.extent(path, "array-size", new(big.Int).Mul(stride, count))
}

// operands rejects unresolved or negative facts before exact arithmetic.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§8–10,44.
func (a *SizeArithmetic) operands(path, operation string, values ...*big.Int) error {
	if a == nil || a.maximum == nil {
		return &SizeArithmeticError{Path: path, Operation: operation, Requested: "unresolved", Maximum: "unresolved", Reason: "target arithmetic capabilities are unresolved"}
	}
	if path == "" {
		return a.failure(path, operation, "unresolved", "layout field/count/instantiation path is unresolved")
	}
	for _, value := range values {
		if value == nil {
			return a.failure(path, operation, "unresolved", "operand is unresolved")
		}
		if value.Sign() < 0 {
			return a.failure(path, operation, value.String(), "operand must be nonnegative")
		}
	}
	return nil
}

// extent enforces the selected target's maximum and returns an owned result.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,44;
// rules/memory/layout.md — §7(4–5).
func (a *SizeArithmetic) extent(path, operation string, result *big.Int) (*big.Int, error) {
	if result.Cmp(a.maximum) > 0 {
		return nil, a.failure(path, operation, result.String(), "extent exceeds the resolved target bound")
	}
	return result, nil
}

// failure snapshots the exact failing extent and caller's source path.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,44.
func (a *SizeArithmetic) failure(path, operation, requested, reason string) *SizeArithmeticError {
	return &SizeArithmeticError{Target: a.target, Path: path, Operation: operation, Requested: requested, Maximum: a.maximum.String(), Reason: reason}
}
