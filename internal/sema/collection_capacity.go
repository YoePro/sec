package sema

import "sec/internal/ast"

// Collection capacity reservation (MD-058). Dynamic list[T], map[K, V] and
// set[T] provide the instance method Reserve(additional: uint) and the
// associated factory WithCapacity(capacity: uint), both returning
// CollectionError on failure; bounded forms only observe Capacity.
//
// Rules:
//   - rules/collections/collections.md — §§ 13.12, 14.8, 15.5 "Capacity reservation"
//   - rules/compiler/compiler_known_members.md — "Collection capacity members"
//   - rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md — §§ 2-5

// collectionTypeReceiver resolves the type-form receiver of an associated
// collection call. The parser keeps `list[T].Member` as an indexed owner
// with OwnerGenericArguments, and `map[K, V].Member` / `set[T].Member` as the
// bare collection name with the type arguments on the call.
func (a *Analyzer) collectionTypeReceiver(call *ast.CallExpression, member *ast.MemberExpression) (Type, bool) {
	var name string
	var arguments []*ast.TypeReference
	switch owner := member.Object.(type) {
	case *ast.IndexExpression:
		identifier, ok := owner.Left.(*ast.Identifier)
		if !ok {
			return Type{}, false
		}
		name, arguments = identifier.Value, member.OwnerGenericArguments
		if len(arguments) == 0 {
			// `list[T].Member(...)` with one simple type argument parses as
			// an indexed receiver; read the index as the type argument.
			argument, ok := typeReferenceFromIndex(owner.Index)
			if !ok {
				return Type{}, false
			}
			arguments = []*ast.TypeReference{argument}
		}
	case *ast.Identifier:
		if len(call.GenericArguments) == 0 {
			return Type{}, false
		}
		name, arguments = owner.Value, call.GenericArguments
	default:
		return Type{}, false
	}
	if name != "list" && name != "map" && name != "set" {
		return Type{}, false
	}
	if _, isValue := a.symbols[name]; isValue {
		return Type{}, false
	}
	typ, ok := a.resolveType(&ast.TypeReference{Token: member.Token, Name: name, TypeArgs: arguments})
	if !ok || typ.Kind == InvalidType {
		return Type{}, false
	}
	return typ, true
}

// typeReferenceFromIndex reads a type name, possibly with its own indexed
// type argument (`list[list[int]]`), from an index expression.
func typeReferenceFromIndex(expr ast.Expression) (*ast.TypeReference, bool) {
	switch node := expr.(type) {
	case *ast.Identifier:
		return &ast.TypeReference{Token: node.Token, Name: node.Value}, true
	case *ast.IndexExpression:
		owner, ok := node.Left.(*ast.Identifier)
		if !ok {
			return nil, false
		}
		argument, ok := typeReferenceFromIndex(node.Index)
		if !ok {
			return nil, false
		}
		return &ast.TypeReference{Token: owner.Token, Name: owner.Value, TypeArgs: []*ast.TypeReference{argument}}, true
	}
	return nil, false
}

// inferCollectionWithCapacity resolves Type.WithCapacity(capacity) on a
// collection type: only the dynamic families provide the factory, and its
// result is Result[Type, CollectionError].
func (a *Analyzer) inferCollectionWithCapacity(call *ast.CallExpression, member *ast.MemberExpression, typ Type) (Type, expressionValue, bool) {
	display := expressionValue{Display: call.String()}
	known, exists := compilerKnownMember(typ, member.Property.Value, true)
	if !exists || known.Kind != CompilerKnownAssociatedFunction {
		if member.Property.Value == "WithCapacity" || member.Property.Value == "Reserve" {
			a.addErrorAtToken(member.Property.Token, "%s has no %s; only dynamic list[T], map[K, V] and set[T] reserve capacity", typeDisplayName(typ), member.Property.Value)
		} else {
			a.addErrorAtToken(member.Property.Token, "unknown associated member %s on %s", member.Property.Value, typeDisplayName(typ))
		}
		return Type{Kind: InvalidType}, display, true
	}
	a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)] = known
	// The call's type arguments, if any, belong to the receiver type.
	if len(call.Arguments) != 1 {
		a.addErrorAtToken(call.Token, "%s.WithCapacity expects 1 argument, got %d", typeDisplayName(typ), len(call.Arguments))
		return Type{Kind: InvalidType}, display, true
	}
	if !a.compilerKnownUintArgument(call.Arguments[0], "WithCapacity capacity") {
		return Type{Kind: InvalidType}, display, true
	}
	return a.refinedCompilerKnownType(known.Result), display, true
}

// compilerKnownUintArgument checks an element/entry count argument of type
// uint (an untyped integer literal shapes to uint).
func (a *Analyzer) compilerKnownUintArgument(expr ast.Expression, operation string) bool {
	uintType := builtinType("uint")
	typ, _ := a.inferExpressionWithExpected(expr, uintType)
	if typ.Kind == InvalidType {
		return false
	}
	if !a.canInitialize(uintType, typ, expr) {
		a.addErrorAtToken(expressionToken(expr), "%s must be uint, got %s", operation, typeDisplayName(typ))
		return false
	}
	return true
}
