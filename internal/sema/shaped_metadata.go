package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// isShapedMetadataLiteralName reports the shaped metadata types with a
// compiler-known positional literal (TensorLayout is an enum without one).
func isShapedMetadataLiteralName(name string) bool {
	switch name {
	case "Shape", "Strides", "Axes", "AxisList":
		return true
	}
	return false
}

// shapedMetadataComponentType is the exact component type of a shaped
// metadata value: signed element strides for Strides, uint otherwise.
func (a *Analyzer) shapedMetadataComponentType(name string) Type {
	if name == "Strides" {
		return a.types["int"]
	}
	return a.types["uint"]
}

// shapedMetadataCardinality is the compile-time Rank/Count of a resolved
// shaped metadata type.
func shapedMetadataCardinality(typ Type) (int64, bool) {
	if !isShapedMetadataLiteralName(typ.Name) && typ.Name != "TensorLayout" {
		return 0, false
	}
	if len(typ.ConstArgs) != 1 || typ.ConstArgs[0] < 0 {
		return 0, false
	}
	return typ.ConstArgs[0], true
}

// inferShapedMetadataLiteral types `Shape[Rank] { e0, ... }`,
// `Strides[Rank] { ... }`, `Axes[Rank] { ... }` and `AxisList[Count] { ... }`:
// exactly Rank/Count positional components of the exact component type. An
// Axes literal must be a statically proven permutation of 0..<Rank and an
// AxisList literal statically duplicate-free; runtime values use
// `try new Axes[Rank](axes)`. Shape and Strides literals are infallible, also
// with runtime components. It reports handled=false for every other literal.
//
// Rules:
//   - rules/collections/shaped-types.md — §§ 4.1, 6.0, 15.3 (MD-016 part 1)
func (a *Analyzer) inferShapedMetadataLiteral(expr *ast.StructLiteral) (Type, bool) {
	if expr == nil || expr.Type == nil || !isShapedMetadataLiteralName(expr.Type.Name) {
		return Type{}, false
	}
	invalid := Type{Kind: InvalidType}
	typ, ok := a.resolveType(expr.Type)
	if !ok {
		return invalid, true
	}
	cardinality, ok := shapedMetadataCardinality(typ)
	if !ok {
		a.shapedLiteralError(expr.Token, "%s literal requires a compile-time cardinality, for example %s[3] { ... }", expr.Type.Name, expr.Type.Name)
		return invalid, true
	}
	display := typeDisplayName(typ)
	for _, field := range expr.Fields {
		if !field.Positional {
			a.shapedLiteralError(field.Token, "%s literal lists its components in order, as %s { c0, c1, ... }; it has no named fields", display, display)
			return invalid, true
		}
	}
	if int64(len(expr.Fields)) != cardinality {
		a.shapedLiteralError(expr.Token, "%s literal requires exactly %d components, got %d", display, cardinality, len(expr.Fields))
		return invalid, true
	}
	component := a.shapedMetadataComponentType(typ.Name)
	valid := true
	constants := make([]*big.Int, len(expr.Fields))
	for index, field := range expr.Fields {
		valueType, _ := a.inferExpressionWithExpected(field.Value, component)
		if valueType.Kind == InvalidType {
			valid = false
			continue
		}
		if !a.canInitialize(component, valueType, field.Value) {
			a.shapedLiteralError(field.Token, "component %d of %s must be %s, got %s", index, display, typeDisplayName(component), typeDisplayName(valueType))
			valid = false
			continue
		}
		if value, constant := a.integerConstantValue(field.Value); constant {
			constants[index] = value
		}
	}
	if !valid {
		return invalid, true
	}
	if typ.Name == "Axes" || typ.Name == "AxisList" {
		if !a.checkStaticAxisInvariant(expr, typ, display, cardinality, constants) {
			return invalid, true
		}
	}
	a.expressionTypes[expr] = typ
	return typ, true
}

// checkStaticAxisInvariant proves an Axes literal a permutation of 0..<Rank
// and an AxisList literal duplicate-free, naming the offending axis.
func (a *Analyzer) checkStaticAxisInvariant(expr *ast.StructLiteral, typ Type, display string, cardinality int64, constants []*big.Int) bool {
	seen := map[string]int{}
	for index, value := range constants {
		token := expr.Fields[index].Token
		if value == nil {
			a.shapedLiteralError(token, "%s literal needs compile-time constant axes; use try new %s(axes) for runtime values", display, display)
			return false
		}
		if typ.Name == "Axes" && value.Cmp(big.NewInt(cardinality)) >= 0 {
			a.shapedLiteralError(token, "%s literal: axis %s is out of range 0..<%d", display, value.String(), cardinality)
			return false
		}
		if previous, duplicate := seen[value.String()]; duplicate {
			a.shapedLiteralError(token, "%s literal: axis %s is duplicated (components %d and %d)", display, value.String(), previous, index)
			return false
		}
		seen[value.String()] = index
	}
	return true
}

func (a *Analyzer) shapedLiteralError(token lexer.Token, format string, args ...any) {
	a.addErrorAtTokenWithMetadata(token, diagnostics.ShapedMetadataLiteralInvalid,
		"Shaped metadata literals list exactly Rank (or Count) components of the exact component type in order (shaped-types.md §§ 4.1, 6.0, 15.3).",
		"%s", fmt.Sprintf(format, args...))
}

// shapedMetadataProperty resolves the only public member of a shaped metadata
// value: the get-only cardinality property, `.Rank` for Shape, Strides and
// Axes and `.Count` for AxisList, typed uint. TensorLayout has none.
//
// Rules:
//   - rules/collections/shaped-types.md — §§ 4.1, 6.0, 15.3 (MD-016 part 1)
//   - rules/compiler/compiler_known_members.md — "Shaped metadata families"
func (a *Analyzer) shapedMetadataProperty(objectType Type, name string) (Type, bool) {
	objectType = dereferenceType(objectType)
	if !isShapedMetadataLiteralName(objectType.Name) {
		return Type{}, false
	}
	if _, ok := shapedMetadataCardinality(objectType); !ok {
		return Type{}, false
	}
	want := "Rank"
	if objectType.Name == "AxisList" {
		want = "Count"
	}
	if name != want {
		return Type{}, false
	}
	return a.types["uint"], true
}

// shapedMetadataIndex resolves the compiler-known read-only indexed
// expression of a shaped metadata value: a uint index in 0..<Rank (Count)
// yielding uint, or int for Strides. A constant out-of-range index is a
// compile-time error; a runtime index keeps the ordinary bounds contract.
func (a *Analyzer) shapedMetadataIndex(expr *ast.IndexExpression, leftType, indexType Type) (Type, bool) {
	if !isShapedMetadataLiteralName(leftType.Name) {
		return Type{}, false
	}
	cardinality, ok := shapedMetadataCardinality(leftType)
	if !ok {
		return Type{}, false
	}
	invalid := Type{Kind: InvalidType}
	if indexType.Kind != UintType || indexType.Named {
		if _, literal := expr.Index.(*ast.IntegerLiteral); !literal {
			a.addErrorAtToken(expressionToken(expr.Index), "%s index must be uint, got %s", typeDisplayName(leftType), typeDisplayName(indexType))
			return invalid, true
		}
	}
	if value, constant := a.integerConstantValue(expr.Index); constant && (value.Sign() < 0 || value.Cmp(big.NewInt(cardinality)) >= 0) {
		a.addErrorAtToken(expressionToken(expr.Index), "index %s is out of bounds for %s with %d components", value.String(), typeDisplayName(leftType), cardinality)
		return invalid, true
	}
	return a.shapedMetadataComponentType(leftType.Name), true
}

// rejectShapedMetadataWrite reports an indexed write to a shaped metadata
// value: its components are read-only, so `shape[0] = 5` never bypasses the
// value's construction or invariant (shaped-types.md §§ 4.1, 15.3; MD-016).
func (a *Analyzer) rejectShapedMetadataWrite(index *ast.IndexExpression) bool {
	leftType, ok := a.ResolvedTypeOf(index.Left)
	if !ok {
		leftType, _ = a.inferExpression(index.Left)
	}
	leftType = dereferenceType(leftType)
	if !isShapedMetadataLiteralName(leftType.Name) {
		return false
	}
	a.addErrorAtTokenWithMetadata(expressionToken(index.Left), diagnostics.ShapedMetadataLiteralInvalid,
		"Construct a new value, for example `"+leftType.Name+"[N] { ... }` or `new "+leftType.Name+"[N](components)`.",
		"components of %s are read-only", typeDisplayName(leftType))
	return true
}

// inferShapedMetadataNew resolves the compiler-known initializers of the
// shaped metadata types: `new Shape[Rank](extents: uint[Rank])` and
// `new Strides[Rank](strides: int[Rank])` are infallible; `new Axes[Rank]
// (axes: uint[Rank])` and `new AxisList[Count](axes: uint[Count])` validate
// their invariant at runtime with the ShapeError construction error channel,
// so they require try or local handling exactly like a source fallible init
// (impl.md § 15).
//
// Rules:
//   - rules/collections/shaped-types.md — §§ 4.1, 6.0, 15.3 (MD-016 part 1)
func (a *Analyzer) inferShapedMetadataNew(expr *ast.NewExpression, target Type, handled bool) (Type, bool) {
	if !isShapedMetadataLiteralName(target.Name) {
		return Type{}, false
	}
	invalid := Type{Kind: InvalidType}
	cardinality, ok := shapedMetadataCardinality(target)
	if !ok {
		return invalid, true
	}
	display := typeDisplayName(target)
	parameter := NewFixedArrayType(a.shapedMetadataComponentType(target.Name), big.NewInt(cardinality))
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "new %s takes exactly one argument of type %s, got %d", display, typeDisplayName(parameter), len(expr.Arguments))
		return invalid, true
	}
	argument := expr.Arguments[0]
	argumentType, _ := a.inferExpressionWithExpected(argument, parameter)
	if argumentType.Kind == InvalidType {
		return invalid, true
	}
	if !a.canInitialize(parameter, argumentType, argument) {
		a.addErrorAtToken(expressionToken(argument), "argument 1 to new %s must be %s, got %s", display, typeDisplayName(parameter), typeDisplayName(argumentType))
		return invalid, true
	}
	construction := ResolvedConstruction{Target: target}
	if target.Name == "Shape" || target.Name == "Strides" {
		a.resolvedConstructions[expr] = construction
		return target, true
	}
	shapeError, known := a.types["ShapeError"]
	if !known {
		a.addErrorAtToken(expr.Token, "new %s needs the core ShapeError declaration", display)
		return invalid, true
	}
	construction.ErrorType = &shapeError
	a.resolvedConstructions[expr] = construction
	if !handled {
		a.addErrorAtToken(expr.Token, "new %s validates its axes and may fail with ShapeError; use try or handle the error locally", display)
		return target, true
	}
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{target, shapeError}}, true
}

// inferTensorLayoutVariant resolves `TensorLayout[Rank].Variant`, parsed as a
// member of the indexed owner `TensorLayout[Rank]`: Rank is a compile-time
// non-negative integer that becomes part of the enum's identity, and the
// variant is one of the three Sec 0.1 layouts.
//
// Rules:
//   - rules/collections/shaped-types.md — § 7.1 (MD-016 part 1)
func (a *Analyzer) inferTensorLayoutVariant(expr *ast.MemberExpression) (Type, bool) {
	owner, ok := expr.Object.(*ast.IndexExpression)
	if !ok || expr.Property == nil {
		return Type{}, false
	}
	name, ok := owner.Left.(*ast.Identifier)
	if !ok || name.Value != "TensorLayout" {
		return Type{}, false
	}
	if _, shadowed := a.symbols[name.Value]; shadowed {
		return Type{}, false
	}
	invalid := Type{Kind: InvalidType}
	rank, constant := a.integerConstantValue(owner.Index)
	if !constant || rank.Sign() < 0 || !rank.IsInt64() {
		a.addErrorAtToken(expressionToken(owner.Index), "TensorLayout requires a compile-time non-negative rank")
		return invalid, true
	}
	layout := a.types["TensorLayout"]
	layout.ConstArgs = []int64{rank.Int64()}
	for _, variant := range layout.EnumValues {
		if variant == expr.Property.Value {
			a.expressionTypes[expr] = layout
			return layout, true
		}
	}
	a.addErrorAtToken(expr.Property.Token, "TensorLayout has no variant %s; the Sec 0.1 variants are DenseRowMajor, DenseColumnMajor and Strided", expr.Property.Value)
	return invalid, true
}
