package semantic

import "fmt"

// verifySliceValueFacts checks optional detached provenance against the actual
// construction and its existing P15 identity, without deriving new range proof.
// Source producers populate it; older white-box operation graphs may omit it.
// Rules: rules/compiler/semantic_ir.md — §126.2–4,7–8,26;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§25–30,44,91.
func verifySliceValueFacts(types *TypeTable, op Operation) error {
	facts := op.Slice
	if facts == nil {
		return nil
	}
	construction, reborrow, mutable := sliceConstructionKind(op.Kind)
	if !construction || len(op.Results) != 1 || len(op.Operands) != 3 {
		return fmt.Errorf("slice value facts require a construction operation")
	}
	typ, _ := types.Lookup(op.Results[0].Type)
	if facts.Element != typ.Element || facts.Mutable != mutable || facts.RuntimeLength != op.Operands[2] || !sameSliceValidityFacts(&facts.Reference, op.Reference) {
		return fmt.Errorf("slice value facts disagree with construction identity, element, authority or length")
	}
	if !reborrow && (len(facts.OriginPlaces) != 1 || facts.OriginPlaces[0] != op.Operands[0]) || reborrow && len(facts.OriginPlaces) != 0 {
		return fmt.Errorf("slice value facts must retain the construction's source Place or slice operand")
	}
	if r := facts.StaticRange; r != nil {
		if r.Start == nil || r.EndExclusive == nil || r.Start.Sign() < 0 || r.Start.Cmp(r.EndExclusive) > 0 || facts.DynamicRange != 0 {
			return fmt.Errorf("slice value facts have invalid exact/dynamic range provenance")
		}
	}
	return nil
}
