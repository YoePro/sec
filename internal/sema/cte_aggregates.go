package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// CompileTimeAggregate is a struct value during semantic CTE
// (compile_time_evaluation.md §§ 11, 20, 32): its struct type and field values
// in declaration order. Values are immutable once shared; a field write
// clones the aggregate first, which keeps Sec value semantics for copies.
type CompileTimeAggregate struct {
	Type   Type
	Names  []string
	Fields map[string]DefaultConstant
	// Variant and Payload hold an Option, Result or union value: the active
	// variant and its payload (nil for a payload-free variant such as None).
	Variant string
	Payload *DefaultConstant
	// Owner is the logical identity of a value of a custom-free type
	// (cte_ownership.go); copies of the same value share it.
	Owner int
}

func (aggregate *CompileTimeAggregate) clone() *CompileTimeAggregate {
	copy := &CompileTimeAggregate{Type: aggregate.Type, Names: append([]string(nil), aggregate.Names...), Fields: make(map[string]DefaultConstant, len(aggregate.Fields)), Variant: aggregate.Variant, Payload: aggregate.Payload, Owner: aggregate.Owner}
	for name, value := range aggregate.Fields {
		copy.Fields[name] = value
	}
	return copy
}

// structLiteral evaluates the entries of a struct literal in source order and
// assembles the final fields from Sema's construction plan, taking omitted
// fields from their resolved defaults.
func (x *cteExecutor) structLiteral(literal *ast.StructLiteral) (DefaultConstant, bool) {
	plan, ok := x.a.ResolvedStructLiteralPlanOf(literal)
	if !ok || !plan.FullyInitialized {
		x.fail(cteImplementationGap, literal.Token, "struct literal %s has no complete construction plan the compile-time evaluator can execute", literal.String())
		return DefaultConstant{}, false
	}
	entries := make([]DefaultConstant, len(plan.Entries))
	for index, entry := range plan.Entries {
		if entry.Kind != StructEntryExplicit {
			x.fail(cteImplementationGap, literal.Token, "struct spread in %s is not executed by the compile-time evaluator yet", literal.String())
			return DefaultConstant{}, false
		}
		value, ok := x.eval(entry.Expression)
		if !ok {
			return DefaultConstant{}, false
		}
		entries[index] = value
	}
	aggregate := &CompileTimeAggregate{Type: plan.StructType, Fields: map[string]DefaultConstant{}}
	for _, field := range plan.FinalFields {
		var value DefaultConstant
		switch field.SourceKind {
		case StructFieldSourceExplicit:
			value = entries[field.SourceEntryIndex]
		case StructFieldSourceDefault:
			value, ok = x.defaultValue(field.Default, field.FieldType, literal.Token)
			if !ok {
				return DefaultConstant{}, false
			}
		default:
			x.fail(cteImplementationGap, literal.Token, "field %s of %s is not executed by the compile-time evaluator yet", field.FieldName, literal.String())
			return DefaultConstant{}, false
		}
		value, ok = x.inDomain(value, field.FieldType, literal.Token, "field value out of range")
		if !ok {
			return DefaultConstant{}, false
		}
		aggregate.Names = append(aggregate.Names, field.FieldName)
		aggregate.Fields[field.FieldName] = value
	}
	value := DefaultConstant{Kind: StructType, Aggregate: aggregate}
	if !x.chargeAggregate(value, literal.Token) {
		return DefaultConstant{}, false
	}
	return value, true
}

// defaultValue converts a resolved default into an evaluator value.
func (x *cteExecutor) defaultValue(resolution DefaultResolution, typ Type, at lexer.Token) (DefaultConstant, bool) {
	switch resolution.Kind {
	case StructDefault:
		aggregate := &CompileTimeAggregate{Type: typ, Fields: map[string]DefaultConstant{}}
		for _, field := range resolution.Fields {
			fieldType := Type{}
			for _, declared := range typ.Fields {
				if declared.Name == field.Name {
					fieldType = declared.Type
				}
			}
			value, ok := x.defaultValue(field.Value, fieldType, at)
			if !ok {
				return DefaultConstant{}, false
			}
			aggregate.Names = append(aggregate.Names, field.Name)
			aggregate.Fields[field.Name] = value
		}
		return DefaultConstant{Kind: StructType, Aggregate: aggregate}, true
	case PrimitiveDefault, NamedDefault, RangeDefault, MembershipDefault, ExplicitTypeDefault, EnumDefault:
		value := resolution.Value
		if value.Kind == "" && value.Integer == nil && value.Exact == nil && value.Lexeme == "" && value.Nominal.Owner == "" {
			// A default established by semantic CTE is read from its fact,
			// establishing it on demand (§§ 38, 41).
			if request, pending := x.factRequest(canonicalTypeIdentity(typ), "default", 0); pending {
				return x.establishFact(request, at)
			}
			x.fail(cteImplementationGap, at, "the default of %s is not executed by the compile-time evaluator yet", typeDisplayName(typ))
			return DefaultConstant{}, false
		}
		return value, true
	}
	x.fail(cteImplementationGap, at, "the %s default of %s is not executed by the compile-time evaluator yet", resolution.Kind, typeDisplayName(typ))
	return DefaultConstant{}, false
}

// fieldOf reads a stored field of an evaluated struct value.
func (x *cteExecutor) fieldOf(object DefaultConstant, name string, at lexer.Token) (DefaultConstant, bool) {
	if object.Aggregate == nil {
		x.fail(cteImplementationGap, at, "field %s of a non-struct value is not executed by the compile-time evaluator yet", name)
		return DefaultConstant{}, false
	}
	value, ok := object.Aggregate.Fields[name]
	if !ok {
		x.fail(cteImplementationGap, at, "field %s is not part of the evaluated %s value", name, typeDisplayName(object.Aggregate.Type))
		return DefaultConstant{}, false
	}
	return value, true
}

// withField returns a copy of a struct value with one field path replaced.
func (x *cteExecutor) withField(object DefaultConstant, path []string, value DefaultConstant, at lexer.Token) (DefaultConstant, bool) {
	if len(path) == 0 {
		return value, true
	}
	if object.Aggregate == nil {
		x.fail(cteImplementationGap, at, "assignment to field %s of a non-struct value is not executed by the compile-time evaluator yet", path[0])
		return DefaultConstant{}, false
	}
	current, ok := object.Aggregate.Fields[path[0]]
	if !ok {
		x.fail(cteImplementationGap, at, "field %s is not part of the evaluated %s value", path[0], typeDisplayName(object.Aggregate.Type))
		return DefaultConstant{}, false
	}
	updated, ok := x.withField(current, path[1:], value, at)
	if !ok {
		return DefaultConstant{}, false
	}
	aggregate := object.Aggregate.clone()
	aggregate.Fields[path[0]] = updated
	object.Aggregate = aggregate
	if !x.chargeAggregate(object, at) {
		return DefaultConstant{}, false
	}
	return object, true
}

// memberPath splits `root.a.b` into its root expression and field names.
func memberPath(expr ast.Expression) (ast.Expression, []string) {
	path := []string{}
	for {
		member, ok := expr.(*ast.MemberExpression)
		if !ok || member.Property == nil {
			break
		}
		path = append([]string{member.Property.Value}, path...)
		expr = member.Object
	}
	return expr, path
}

// receiverPlace navigates an implicit receiver member Place Sema resolved
// for a bare identifier inside a method body.
func (x *cteExecutor) receiverPlace(identifier *ast.Identifier) (DefaultConstant, bool, bool) {
	receiver := x.frame().receiver
	if receiver == nil {
		return DefaultConstant{}, false, false
	}
	if identifier.Value == "self" {
		return *receiver, true, true
	}
	place, ok := x.a.ResolvedPlaceOf(identifier)
	if !ok || place.RootKind != PlaceRootReceiver {
		return DefaultConstant{}, false, false
	}
	value := *receiver
	for _, projection := range place.Projections {
		if projection.Kind != PlaceField {
			x.fail(cteImplementationGap, identifier.Token, "receiver member %s is not executed by the compile-time evaluator yet", identifier.Value)
			return DefaultConstant{}, false, true
		}
		var ok bool
		value, ok = x.fieldOf(value, projection.Name, identifier.Token)
		if !ok {
			return DefaultConstant{}, false, true
		}
	}
	if len(place.Projections) == 0 && place.Root != "self" {
		value, ok = x.fieldOf(value, identifier.Value, identifier.Token)
		if !ok {
			return DefaultConstant{}, false, true
		}
	}
	return value, true, true
}
