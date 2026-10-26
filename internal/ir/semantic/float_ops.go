package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
	"sec/internal/sema/constant"
)

// Floating operations of Package 20 admitted early for the binary float
// subset: negation, the five arithmetic kinds and the six comparisons over
// one float type (float, float32, float64). Unary plus is an SSA identity.
// None fails at run time: overflow gives infinity and a zero divisor gives
// an infinity or NaN (package20.md §§ 20–26, 63–65).
const (
	OpFloatNeg     OpKind = "float.neg"
	OpFloatBinary  OpKind = "float.binary"
	OpFloatCompare OpKind = "float.compare"
)

// FloatBinaryKind is the arithmetic kind of float.binary; rem is the
// truncation remainder (package20.md § 23).
type FloatBinaryKind string

const (
	FloatBinaryAdd FloatBinaryKind = "add"
	FloatBinarySub FloatBinaryKind = "sub"
	FloatBinaryMul FloatBinaryKind = "mul"
	FloatBinaryDiv FloatBinaryKind = "div"
	FloatBinaryRem FloatBinaryKind = "rem"
)

func validFloatBinaryKind(kind FloatBinaryKind) bool {
	switch kind {
	case FloatBinaryAdd, FloatBinarySub, FloatBinaryMul, FloatBinaryDiv, FloatBinaryRem:
		return true
	}
	return false
}

// FloatConstant is the format-precise payload of const.float: the binary
// format and its canonical bit pattern, rounded once from the exact source
// value (package20.md §§ 12–19).
type FloatConstant struct {
	Bits      uint64
	Format    uint16 // 32 or 64
	Canonical bool
}

// floatLiteralBits rounds a source float literal directly into binary32 or
// binary64 with round-to-nearest, ties-to-even — never through a wider
// format. Base-prefixed integer forms (`0x10g`) denote the exact integer.
// A finite literal beyond the format's finite range is rejected; a tiny one
// becomes a subnormal or signed zero.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§13–17,82.
func floatLiteralBits(lexeme string, format uint16) (uint64, error) {
	return constant.FloatLiteralBits(lexeme, format)
}

// floatFormat is the binary format of a float type: float32 and float64 by
// name, and plain float by the module's plan width (MD-014).
func (b *builder) floatFormat(typeID TypeID) (uint16, bool) {
	typ, ok := b.module.Types.Lookup(typeID)
	if !ok || typ.Kind != TypeFloat {
		return 0, false
	}
	if typ.BitWidth == 32 || typ.BitWidth == 64 {
		return typ.BitWidth, true
	}
	if width := b.module.PointerWidthBits; width == 32 || width == 64 {
		return width, true
	}
	return 0, false
}

// buildFloatConstant interns a float literal with its canonical bits.
func (fb *functionBuilder) buildFloatConstant(lexeme string, typeID TypeID, loc Location) (builtValue, error) {
	format, ok := fb.owner.floatFormat(typeID)
	if !ok {
		return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: lexeme, Location: loc}, typeID), nil
	}
	bits, err := floatLiteralBits(lexeme, format)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: lexeme, Float: &FloatConstant{Bits: bits, Format: format, Canonical: true}, Location: loc}, typeID), nil
}

func verifyFloatOperation(module *Module, op Operation, values map[ValueID]Value) error {
	isFloat := func(id TypeID) bool { return typeHasKind(module.Types, id, TypeFloat) }
	switch op.Kind {
	case OpFloatNeg:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isFloat(values[op.Operands[0]].Type) || op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid float.neg")
		}
	case OpFloatBinary:
		if !validFloatBinaryKind(op.FloatBinary) || len(op.Operands) != 2 || len(op.Results) != 1 ||
			!isFloat(values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type ||
			op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid float.binary")
		}
	case OpFloatCompare:
		if !validIntegerComparePredicate(op.IntegerCompare) || len(op.Operands) != 2 || len(op.Results) != 1 ||
			!isFloat(values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type ||
			!typeHasKind(module.Types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid float.compare (operand types %d and %d)", values[op.Operands[0]].Type, values[op.Operands[1]].Type)
		}
	}
	return nil
}

// buildShapedFloatSign lowers `-literal` / `+literal` whose untyped operand
// the float context shaped (package20.md § 17: `-0.0g` is positive-zero
// literal shaping followed by floating negation, giving negative zero).
func (fb *functionBuilder) buildShapedFloatSign(expr ast.Expression) (builtValue, bool, error) {
	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok || (prefix.Operator != "-" && prefix.Operator != "+") {
		return builtValue{}, false, nil
	}
	resolved, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok || resolved.Kind != sema.FloatType {
		return builtValue{}, false, nil
	}
	typeID, err := fb.owner.internType(resolved)
	if err != nil {
		return builtValue{}, true, err
	}
	operand, err := fb.buildExpr(prefix.Right, typeID)
	if err != nil {
		return builtValue{}, true, err
	}
	if operand.typ != typeID {
		return builtValue{}, true, fmt.Errorf("shaped float sign operand type mismatch")
	}
	if prefix.Operator == "+" {
		return operand, true, nil
	}
	return fb.result(Operation{Kind: OpFloatNeg, Operands: []ValueID{operand.id}, Location: location(prefix.Token)}, typeID), true, nil
}

// P20 core floating operations (package20.md §§ 28–36, 63, 66–68): total,
// allocation-free and without a failure edge. float.minimum/maximum
// propagate NaN; float.property has no operand.
const (
	OpFloatAbs      OpKind = "float.abs"
	OpFloatMinimum  OpKind = "float.minimum"
	OpFloatMaximum  OpKind = "float.maximum"
	OpFloatClassify OpKind = "float.classify"
	OpFloatRound    OpKind = "float.round"
	OpFloatProperty OpKind = "float.property"
	// OpFloatClampChecked is FloatClampCheckedOp (§ 69): the clamped value
	// and a failed flag for invalid bounds (a NaN bound or minimum > maximum).
	OpFloatClampChecked OpKind = "float.clamp-checked"
)

var (
	floatClassifyKinds = map[string]bool{"finite": true, "infinite": true, "nan": true, "normal": true, "subnormal": true}
	floatRoundKinds    = map[string]bool{"floor": true, "ceiling": true, "truncate": true, "nearest-even": true}
	floatPropertyKinds = map[string]bool{"min": true, "max": true, "epsilon": true, "infinity": true, "negative-infinity": true, "nan": true}
)

// floatCoreMethodOperations maps a compiler-known float method to its
// operation and kind; Clamp is absent (it needs Result/try lowering).
var floatCoreMethodOperations = map[string]struct {
	kind OpKind
	core string
}{
	"CKM-FLOAT-ABS": {OpFloatAbs, ""}, "CKM-FLOAT-MIN": {OpFloatMinimum, ""}, "CKM-FLOAT-MAX": {OpFloatMaximum, ""},
	"CKM-FLOAT-ISFINITE": {OpFloatClassify, "finite"}, "CKM-FLOAT-ISINFINITE": {OpFloatClassify, "infinite"},
	"CKM-FLOAT-ISNAN": {OpFloatClassify, "nan"}, "CKM-FLOAT-ISNORMAL": {OpFloatClassify, "normal"},
	"CKM-FLOAT-ISSUBNORMAL": {OpFloatClassify, "subnormal"},
	"CKM-FLOAT-FLOOR":       {OpFloatRound, "floor"}, "CKM-FLOAT-CEILING": {OpFloatRound, "ceiling"},
	"CKM-FLOAT-TRUNCATE": {OpFloatRound, "truncate"}, "CKM-FLOAT-ROUND": {OpFloatRound, "nearest-even"},
}

// floatPropertyKind maps a compiler-known float associated property ID to
// its float.property kind.
var floatPropertyKind = map[string]string{
	"CKM-FLOAT-MIN": "min", "CKM-FLOAT-MAX": "max", "CKM-FLOAT-EPSILON": "epsilon",
	"CKM-FLOAT-INFINITY": "infinity", "CKM-FLOAT-NEGATIVEINFINITY": "negative-infinity", "CKM-FLOAT-NAN": "nan",
}

// buildNumericTypeProperty lowers a resolved numeric associated property:
// integer Min/Max/Bits are exact constants of the property's type and float
// properties are float.property of the receiver type (core-library.md
// §§ 7.1, 8.1, 9.1).
func (fb *functionBuilder) buildNumericTypeProperty(expr *ast.MemberExpression, member sema.CompilerKnownMember, typeID TypeID) (builtValue, bool, error) {
	loc := location(expr.Property.Token)
	resolved, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok {
		return builtValue{}, false, nil
	}
	switch member.ID {
	case "CKM-NUMERIC-MIN", "CKM-NUMERIC-MAX", "CKM-NUMERIC-BITS":
		resultType, err := fb.owner.internType(resolved)
		if err != nil {
			return builtValue{}, true, err
		}
		owner, typed := fb.owner.analyzer.StaticMemberOwnerOf(expr)
		if !typed {
			return builtValue{}, true, fb.unsupported(member.Name+" of an unresolved integer type", expr.Property.Token)
		}
		value, known := integerTypeProperty(owner, member.ID)
		if !known {
			return builtValue{}, true, fb.unsupported(member.Name+" of a target-sized integer without a CompilationPlan", expr.Property.Token)
		}
		return fb.result(Operation{Kind: OpConstInt, Integer: value, Location: loc}, resultType), true, nil
	}
	kind, isFloat := floatPropertyKind[member.ID]
	if !isFloat || resolved.Kind != sema.FloatType {
		return builtValue{}, false, nil
	}
	resultType, err := fb.owner.internType(resolved)
	if err != nil {
		return builtValue{}, true, err
	}
	return fb.result(Operation{Kind: OpFloatProperty, FloatCore: kind, Location: loc}, resultType), true, nil
}

// integerTypeProperty is the exact Min, Max or Bits of an integer type with a
// resolved value domain.
func integerTypeProperty(typ sema.Type, id string) (*big.Int, bool) {
	if typ.MinInteger == nil || typ.MaxInteger == nil {
		return nil, false
	}
	switch id {
	case "CKM-NUMERIC-MIN":
		return new(big.Int).Set(typ.MinInteger), true
	case "CKM-NUMERIC-MAX":
		return new(big.Int).Set(typ.MaxInteger), true
	}
	bits := typ.MaxInteger.BitLen()
	if typ.MinInteger.Sign() < 0 {
		bits++
	}
	return big.NewInt(int64(bits)), true
}

// buildFloatCoreCall lowers a total float core method call on its receiver
// and same-type arguments, evaluated left to right.
func (fb *functionBuilder) buildFloatCoreCall(call *ast.CallExpression, member *ast.MemberExpression, known sema.CompilerKnownMember) (builtValue, bool, error) {
	if known.ID == "CKM-FLOAT-CLAMP" {
		value, err := fb.buildFloatClamp(call, member)
		return value, true, err
	}
	operation, ok := floatCoreMethodOperations[known.ID]
	if !ok {
		return builtValue{}, false, nil
	}
	receiverType, typed := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	if !typed || receiverType.Kind != sema.FloatType {
		return builtValue{}, true, fb.unsupported("float core method without a resolved float receiver", member.Property.Token)
	}
	floatType, err := fb.owner.internType(receiverType)
	if err != nil {
		return builtValue{}, true, err
	}
	receiver, err := fb.buildExpr(member.Object, floatType)
	if err != nil {
		return builtValue{}, true, err
	}
	operands := []ValueID{receiver.id}
	for _, argument := range call.Arguments {
		value, err := fb.buildExpr(argument, floatType)
		if err != nil {
			return builtValue{}, true, err
		}
		operands = append(operands, value.id)
	}
	resultType := floatType
	if operation.kind == OpFloatClassify {
		boolean, typed := fb.owner.analyzer.ResolvedTypeOf(call)
		if !typed {
			return builtValue{}, true, fb.unsupported("untyped float classification", member.Property.Token)
		}
		if resultType, err = fb.owner.internType(boolean); err != nil {
			return builtValue{}, true, err
		}
	}
	return fb.result(Operation{Kind: operation.kind, Operands: operands, FloatCore: operation.core, Location: location(member.Property.Token)}, resultType), true, nil
}

func verifyFloatCoreOperation(module *Module, op Operation, values map[ValueID]Value) error {
	isFloat := func(id TypeID) bool { return typeHasKind(module.Types, id, TypeFloat) }
	if len(op.Results) != 1 {
		return fmt.Errorf("%s requires one result", op.Kind)
	}
	result := op.Results[0].Type
	switch op.Kind {
	case OpFloatProperty:
		if len(op.Operands) != 0 || !floatPropertyKinds[op.FloatCore] || !isFloat(result) {
			return fmt.Errorf("invalid float.property %q", op.FloatCore)
		}
		return nil
	case OpFloatMinimum, OpFloatMaximum:
		if len(op.Operands) != 2 || op.FloatCore != "" || values[op.Operands[1]].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid %s", op.Kind)
		}
	case OpFloatAbs:
		if len(op.Operands) != 1 || op.FloatCore != "" {
			return fmt.Errorf("invalid float.abs")
		}
	case OpFloatRound:
		if len(op.Operands) != 1 || !floatRoundKinds[op.FloatCore] {
			return fmt.Errorf("invalid float.round %q", op.FloatCore)
		}
	case OpFloatClassify:
		if len(op.Operands) != 1 || !floatClassifyKinds[op.FloatCore] || !isFloat(values[op.Operands[0]].Type) || !typeHasKind(module.Types, result, TypeBool) {
			return fmt.Errorf("invalid float.classify %q", op.FloatCore)
		}
		return nil
	}
	if !isFloat(values[op.Operands[0]].Type) || result != values[op.Operands[0]].Type {
		return fmt.Errorf("%s must produce its operand's float type", op.Kind)
	}
	return nil
}

// buildFloatClamp lowers `x.Clamp(minimum, maximum)` to float.clamp-checked
// and builds its Result[T, RangeError]: Err(RangeError.InvalidRange) when a
// bound is NaN or minimum > maximum, otherwise Ok of the clamped value, which
// is NaN for a NaN input (package20.md §§ 31, 69).
func (fb *functionBuilder) buildFloatClamp(call *ast.CallExpression, member *ast.MemberExpression) (builtValue, error) {
	receiverType, typed := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	resultSemaType, resultTyped := fb.owner.analyzer.ResolvedTypeOf(call)
	if !typed || !resultTyped || receiverType.Kind != sema.FloatType || len(call.Arguments) != 2 || resultSemaType.Kind != sema.ResultType || len(resultSemaType.TypeArgs) != 2 {
		return builtValue{}, fb.unsupported("float Clamp without resolved float and Result types", member.Property.Token)
	}
	floatType, err := fb.owner.internType(receiverType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(resultSemaType)
	if err != nil {
		return builtValue{}, err
	}
	errorType, err := fb.owner.internType(resultSemaType.TypeArgs[1])
	if err != nil {
		return builtValue{}, err
	}
	definition, ok := fb.owner.enumDefinition(errorType)
	if !ok {
		return builtValue{}, fb.unsupported("RangeError enum definition", member.Property.Token)
	}
	caseID, ok := enumCaseIDByName(definition, "InvalidRange")
	if !ok {
		return builtValue{}, fb.unsupported("RangeError.InvalidRange", member.Property.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	operands := []ValueID{}
	for _, expr := range []ast.Expression{member.Object, call.Arguments[0], call.Arguments[1]} {
		value, err := fb.buildExpr(expr, floatType)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, value.id)
	}
	loc := location(member.Property.Token)
	clamped := fb.newValue(floatType, OwnershipImmediate, loc)
	failed := fb.newValue(boolType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpFloatClampChecked, Operands: operands, Results: []Value{clamped, failed}, Location: loc})
	errorBlock, successBlock, merge := fb.newBlock(), fb.newBlock(), fb.newBlock()
	parameter := fb.newValue(resultType, OwnershipImmediate, loc)
	merge.Parameters = []Value{parameter}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: loc})
	fb.current = errorBlock
	invalid := fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, Location: loc}, errorType)
	failure := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{invalid.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{failure.id}}}, Location: loc})
	fb.current = successBlock
	success := fb.result(Operation{Kind: OpResultOk, Operands: []ValueID{clamped.ID}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{success.id}}}, Location: loc})
	fb.current = merge
	return builtValue{id: parameter.ID, typ: resultType}, nil
}

func verifyFloatClampOperation(module *Module, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 3 || len(op.Results) != 2 || !typeHasKind(module.Types, values[op.Operands[0]].Type, TypeFloat) ||
		values[op.Operands[1]].Type != values[op.Operands[0]].Type || values[op.Operands[2]].Type != values[op.Operands[0]].Type ||
		op.Results[0].Type != values[op.Operands[0]].Type || !typeHasKind(module.Types, op.Results[1].Type, TypeBool) {
		return fmt.Errorf("invalid float.clamp-checked")
	}
	return nil
}
