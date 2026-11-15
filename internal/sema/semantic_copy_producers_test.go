package sema

import "testing"

// P17-09 / § 136 rows 7 and 13: Sec 0.1 has no leaf type whose implicit copy
// is a language-defined semantic copy (copy_move.md § 3.3(2) excludes named
// duplication methods), so no concrete runtime type classifies as
// CopySemantic and own.semantic-copy has no source producer. Every builtin
// type and every aggregate over them is trivial, move-only, non-copyable or
// conditional; a future semantic-copy leaf must update this test with its
// producer.
// Rules: rules/memory/copy_move.md — §§ 3.2–3.5;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 5–7.
func TestNoConcreteTypeIsSemanticallyCopyable(t *testing.T) {
	for name, typ := range builtinTypes() {
		element := typ
		for _, candidate := range []Type{
			typ,
			{Kind: ArrayType, Element: &element, ArrayLengthDecimal: "2", ArrayLength: 2},
			{Kind: StructType, Name: "Holder", Fields: []StructField{{Name: "value", Type: typ}}},
			{Kind: ResultType, TypeArgs: []Type{typ, builtinType("CollectionError")}},
		} {
			if CopyClassificationOf(candidate) == CopySemantic {
				t.Errorf("%s (%s) classifies as CopySemantic without a semantic-copy producer", name, candidate.Kind)
			}
		}
	}
}
