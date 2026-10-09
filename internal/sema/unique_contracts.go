package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema/collectionshape"
)

// uniqueElementType resolves the direct equality domain without applying the
// collection's own uniqueness requirement to its nested elements. Map entry
// semantics are not guessed from its key type; unsupported families retain
// their existing runtime/lowering boundary.
// Rules: rules/types/contracts.md — Applicability, String and collection contracts;
// rules/collections/collections.md — §10 Equality.
func (a *Analyzer) uniqueElementType(typ Type) (Type, bool) {
	typ = a.namedRepresentation(typ)
	if typ.Kind == ArrayType || typ.Kind == SliceType {
		if typ.Element != nil {
			return *typ.Element, true
		}
		return Type{}, false
	}
	switch typ.Name {
	case "list", "set", "vector", "matrix", "tensor", "tensor_view":
		if len(typ.TypeArgs) > 0 {
			return typ.TypeArgs[0], true
		}
	}
	return Type{}, false
}

// validateUniqueEquality rejects a non-comparable concrete element domain;
// generic templates defer the same check to their instantiated type reference.
// Rules: rules/types/contracts.md — Applicability, String and collection contracts;
// rules/declarations/generics.md — Substitution.
func (a *Analyzer) validateUniqueEquality(typ Type, token lexer.Token) bool {
	if element, known := a.uniqueElementType(typ); known && element.Kind != GenericType && !EqualityComparable(element) {
		a.addErrorAtTokenWithMetadata(token, diagnostics.InapplicableContract, "use a collection whose direct element type supports equality", "unique contract on %s requires equality-comparable direct elements; %s does not support equality", typeDisplayName(typ), typeDisplayName(element))
		return false
	}
	return true
}

type uniqueConstant struct {
	known  bool
	scalar *DefaultConstant
	fields []uniqueConstant
}

// duplicateArrayLiteralConstant uses the canonical contextual constant evaluator
// and resolved aggregate construction facts to prove direct-element equality.
// Unknown runtime values, calls and spreads are opaque, never equal by spelling.
// Rules: rules/types/contracts.md — String and collection contracts, Ordered membership;
// rules/compiler/compile_time_evaluation.md — semantic constants.
func (a *Analyzer) duplicateArrayLiteralConstant(typ Type, literal *ast.ArrayLiteral) (int, int, bool) {
	if literal == nil || typ.Element == nil {
		return 0, 0, false
	}
	values := make([]uniqueConstant, len(literal.Elements))
	for i, element := range literal.Elements {
		values[i] = a.uniqueConstantOf(element, *typ.Element)
	}
	return collectionshape.FirstDuplicate(values, uniqueConstantsEqual)
}

// uniqueConstantsEqual recursively compares the value of one direct aggregate
// element. This is ordinary aggregate equality, not recursive uniqueness: two
// internally repeated arrays can be distinct direct elements.
// Rules: rules/collections/collections.md — §10 Equality;
// rules/declarations/struct.md — equality; rules/types/contracts.md — unique.
func uniqueConstantsEqual(left, right uniqueConstant) bool {
	if !left.known || !right.known {
		return false
	}
	if left.scalar != nil || right.scalar != nil {
		return left.scalar != nil && right.scalar != nil && defaultConstantsEqual(*left.scalar, *right.scalar)
	}
	if len(left.fields) != len(right.fields) {
		return false
	}
	for i := range left.fields {
		if !uniqueConstantsEqual(left.fields[i], right.fields[i]) {
			return false
		}
	}
	return true
}

// uniqueConstantOf builds only already-established semantic value proofs. Arrays
// retain ordered elements; structs use final field origins, including defaults,
// without executing getters or manufacturing a proof for an unknown operand.
// Rules: rules/types/contracts.md — unique; rules/collections/collections.md — §10;
// rules/declarations/struct.md — omitted fields and equality.
func (a *Analyzer) uniqueConstantOf(expr ast.Expression, typ Type) uniqueConstant {
	switch expr := expr.(type) {
	case *ast.ArrayLiteral:
		if typ.Kind != ArrayType || typ.Element == nil {
			return uniqueConstant{}
		}
		value := uniqueConstant{known: true}
		for _, element := range expr.Elements {
			child := a.uniqueConstantOf(element, *typ.Element)
			if !child.known {
				return uniqueConstant{}
			}
			value.fields = append(value.fields, child)
		}
		return value
	case *ast.StructLiteral:
		plan, ok := a.resolvedStructLiteralPlans[expr]
		if !ok || !plan.FullyInitialized {
			return uniqueConstant{}
		}
		value := uniqueConstant{known: true}
		entries := map[int]ast.Expression{}
		for _, entry := range plan.Entries {
			entries[entry.SourceIndex] = entry.Expression
		}
		for _, field := range plan.FinalFields {
			var child uniqueConstant
			switch field.SourceKind {
			case StructFieldSourceDefault:
				child = uniqueDefaultConstant(field.Default)
			case StructFieldSourceExplicit:
				child = a.uniqueConstantOf(entries[field.SourceEntryIndex], field.FieldType)
			case StructFieldSourceSpread:
				spread := a.uniqueConstantOf(entries[field.SourceEntryIndex], plan.StructType)
				if spread.known && int(field.SpreadFieldID) < len(spread.fields) {
					child = spread.fields[field.SpreadFieldID]
				}
			}
			if !child.known {
				return uniqueConstant{}
			}
			value.fields = append(value.fields, child)
		}
		return value
	case *ast.SpreadExpression:
		return uniqueConstant{}
	}
	value, outcome := a.semanticCompileTimeConstantVisiting(expr, map[string]bool{}, typ)
	if outcome != compileTimeEvaluated {
		return uniqueConstant{}
	}
	return uniqueConstant{known: true, scalar: &value}
}

// uniqueDefaultConstant consumes canonical resolved scalar/struct defaults;
// other aggregate construction remains opaque until its shared CTE support exists.
// Rules: rules/types/default_values.md — Struct defaults; rules/types/contracts.md — unique.
func uniqueDefaultConstant(resolution DefaultResolution) uniqueConstant {
	switch resolution.Kind {
	case PrimitiveDefault, NamedDefault, RangeDefault, MembershipDefault, ExplicitTypeDefault, EnumDefault:
		value := resolution.Value
		return uniqueConstant{known: true, scalar: &value}
	case StructDefault:
		value := uniqueConstant{known: true}
		for _, field := range resolution.Fields {
			child := uniqueDefaultConstant(field.Value)
			if !child.known {
				return uniqueConstant{}
			}
			value.fields = append(value.fields, child)
		}
		return value
	}
	return uniqueConstant{}
}
