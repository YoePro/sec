package semantic

import (
	"fmt"
	"strings"
)

// FloatSemantics is a value-only contract attached to primitive float operations.
// SourceType retains the nominal scalar identity even when float and float64
// have the same representation. FormatBits zero explicitly means an unresolved
// target-sized float; it never adopts the build host's width.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,20–27,63–69;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md §2.
type FloatSemantics struct {
	SourceType          TypeID
	FormatBits          uint16
	Rounding            string
	FastMath            string
	Contraction         string
	Subnormals          string
	ComparisonPredicate string
	RemainderKind       string
}

// floatSemanticOperation identifies operations with the P20 strict contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,63–69.
func floatSemanticOperation(kind OpKind) bool {
	switch kind {
	case OpConstFloat, OpFloatNeg, OpFloatBinary, OpFloatCompare, OpFloatAbs, OpFloatMinimum, OpFloatMaximum, OpFloatClassify, OpFloatRound, OpFloatProperty, OpFloatClampChecked:
		return true
	}
	return false
}

// resolvedFloatSemantics materializes the fixed P20 operation contract from an
// already resolved scalar identity. Round's method kind remains separate from
// the ordinary arithmetic rounding environment; Floor never changes that
// environment. This helper does not infer source types or choose a host format.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§20–27,34,64–69.
func resolvedFloatSemantics(types *TypeTable, source TypeID, op Operation) FloatSemantics {
	typ, _ := types.Lookup(source)
	facts := FloatSemantics{SourceType: source, FormatBits: typ.BitWidth, Rounding: "nearest-even", FastMath: "none", Contraction: "off", Subnormals: "preserve"}
	if op.Kind == OpFloatCompare {
		if op.IntegerCompare == IntegerCompareNE {
			facts.ComparisonPredicate = "une"
		} else {
			facts.ComparisonPredicate = "o" + string(op.IntegerCompare)
		}
	}
	if op.Kind == OpFloatBinary && op.FloatBinary == FloatBinaryRem {
		facts.RemainderKind = "truncation"
	}
	return facts
}

// verifyFloatSemantics checks published facts against the actual SSA operand
// and source type, without repairing absent or contradictory facts. Existing
// operation verifiers separately check arity, type agreement and valid kinds.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,20–27,63–69,81.
func verifyFloatSemantics(module *Module, op Operation, values map[ValueID]Value) error {
	if !floatSemanticOperation(op.Kind) {
		return nil
	}
	source := TypeID(0)
	if op.Kind == OpConstFloat || op.Kind == OpFloatProperty {
		if len(op.Results) > 0 {
			source = op.Results[0].Type
		}
	} else if len(op.Operands) > 0 {
		source = values[op.Operands[0]].Type
	}
	typ, ok := module.Types.Lookup(source)
	if !ok || typ.Kind != TypeFloat || (typ.BitWidth != 32 && typ.BitWidth != 64 && !(typ.BitWidth == 0 && typ.TargetSize)) {
		return fmt.Errorf("%s has invalid float scalar provenance", op.Kind)
	}
	expected := resolvedFloatSemantics(module.Types, source, op)
	if op.FloatSemantics != expected {
		return fmt.Errorf("%s float semantic facts disagree with its strict policy or scalar provenance: got %+v, want %+v", op.Kind, op.FloatSemantics, expected)
	}
	if op.Kind == OpConstFloat && op.Float != nil && op.Float.Format != typ.BitWidth {
		return fmt.Errorf("const.float payload format disagrees with scalar provenance")
	}
	return nil
}

// formatFloatSemantics makes every strict policy and scalar origin inspectable
// in the Semantic IR dump, including the unordered != predicate and remainder.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,64–69.
func formatFloatSemantics(out *strings.Builder, facts FloatSemantics) {
	if facts.SourceType == 0 {
		return
	}
	fmt.Fprintf(out, " float-policy={source=#%d format=binary%d rounding=%s fast-math=%s contraction=%s subnormals=%s", facts.SourceType, facts.FormatBits, facts.Rounding, facts.FastMath, facts.Contraction, facts.Subnormals)
	if facts.ComparisonPredicate != "" {
		fmt.Fprintf(out, " predicate=%s", facts.ComparisonPredicate)
	}
	if facts.RemainderKind != "" {
		fmt.Fprintf(out, " remainder=%s", facts.RemainderKind)
	}
	out.WriteString("}")
}
