package mlir

import "fmt"

// emitFloatNegate flips the sign bit, preserving signed zero and NaN behavior.
// Rules: rules/foundations/operators.md — "Floating arithmetic";
// rules/mlir/packages/sec-mlir-dialect_package20.md — §20 "Unary plus and negation".
func (g *Generator) emitFloatNegate(operand value) (value, error) {
	if !isMLIRFloatType(operand.typ) {
		return value{}, fmt.Errorf("emit-mlir floating negation expects a float")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.fneg %s : %s\n", tmp, operand.ref, operand.typ)
	return value{typ: operand.typ, ref: tmp}, nil
}

// emitFloatCompare preserves unordered NaN inequality and ordered equality and
// ordering, without fast-math flags. Both operands have already been evaluated.
//
// Rules:
//   - rules/foundations/operators.md — "Floating equality", "Floating ordering"
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§24–27
func (g *Generator) emitFloatCompare(operator string, left value, right value) (value, error) {
	if left.typ != right.typ || !isMLIRFloatType(left.typ) {
		return value{}, fmt.Errorf("emit-mlir comparison currently expects matching float operands")
	}
	predicate := map[string]string{
		"==": "oeq",
		"!=": "une",
		"<":  "olt",
		"<=": "ole",
		">":  "ogt",
		">=": "oge",
	}[operator]
	if predicate == "" {
		return value{}, fmt.Errorf("emit-mlir does not support float comparison %q", operator)
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.fcmp %q %s, %s : %s\n", tmp, predicate, left.ref, right.ref, left.typ)
	return value{typ: "i1", ref: tmp}, nil
}

// emitRemainder selects truncation-based floating remainder or the existing
// signedness-correct integer remainder. The operands are evaluated by the caller.
// Rules: rules/foundations/operators.md — "Floating remainder", "Integer remainder".
func (g *Generator) emitRemainder(left, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir remainder expects matching operand types")
	}
	return g.emitIntegerOrFloatBinary(signedIntegerOp("srem", "urem", left.unsigned), "frem", left, right)
}
