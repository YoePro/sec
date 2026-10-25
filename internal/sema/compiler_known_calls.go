package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// inferValidatedCompilerKnownMemberCall resolves compiler-owned receiver and
// argument contracts before publishing a result.
// Rules: rules/compiler/compiler_known_members.md — "Lookup order";
// rules/collections/collections.md — §§6.7,13.10,13.12,14.5,14.8,15.5;
// rules/corrections/md109-core-string-materialization-correction.md — §§2.1–2.2,9.3.
func (a *Analyzer) inferValidatedCompilerKnownMemberCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	memberExpr, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || memberExpr == nil || memberExpr.Property == nil {
		return Type{}, expressionValue{}, false
	}
	if _, unit := memberExpr.Object.(*ast.UnitReferenceExpression); unit {
		// `<U>.Name(...)` calls a unit function, never a compiler-known member.
		return Type{}, expressionValue{}, false
	}

	if typ, isCollection := a.collectionTypeReceiver(expr, memberExpr); isCollection {
		return a.inferCollectionWithCapacity(expr, memberExpr, typ)
	}
	if path, static := typePathFromExpression(memberExpr.Object); static {
		typ, exists := a.types[a.resolveTypeName(path)]
		if exists {
			member, memberExists := compilerKnownMember(typ, memberExpr.Property.Value, true)
			if !memberExists || (member.Kind != CompilerKnownMethod && member.Kind != CompilerKnownAssociatedFunction) {
				return Type{}, expressionValue{}, false
			}
			a.compilerKnownMemberFacts[sourceTokenLocation(memberExpr.Property.Token)] = member
			switch member.Name {
			case "FromByteArray", "FromRuneArray":
				return a.inferCompilerKnownStringConstructor(expr, member)
			case "FromBuffer", "WithCapacity", "Growable":
				return a.inferArenaConstructorCall(expr, member)
			}
			return Type{}, expressionValue{}, false
		}
		if root := compilerKnownReceiverRoot(memberExpr.Object); root != nil {
			if _, isValue := a.symbols[root.Value]; !isValue {
				return Type{}, expressionValue{}, false
			}
		}
	}

	receiverType, _ := a.inferExpression(memberExpr.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	lookupType := dereferenceType(receiverType)
	if lookupType.Kind == StringType && (memberExpr.Property.Value == "FromByteArray" || memberExpr.Property.Value == "FromRuneArray") {
		a.addErrorAtTokenWithMetadata(memberExpr.Property.Token, diagnostics.UnknownCallTarget,
			"Call string."+memberExpr.Property.Value+"(data) using the type name; the input is borrowed.",
			"string.%s is static and cannot be called through an instance", memberExpr.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if lookupType.Name == "map" && len(lookupType.TypeArgs) == 2 && memberExpr.Property.Value == "Set" {
		a.addErrorAtTokenWithMetadata(memberExpr.Property.Token, diagnostics.UnknownCallTarget,
			"Use indexed assignment: try "+memberExpr.Object.String()+"[key] = value (or <- for an explicit move). := is only for declarations and initialization.",
			"map has no Set method; use indexed assignment")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	member, exists := compilerKnownMember(receiverType, memberExpr.Property.Value, false)
	if !exists || member.Kind != CompilerKnownMethod {
		return Type{}, expressionValue{}, false
	}
	// Interface requirements use ordinary overload/conformance validation.
	// Publish identity only after successful resolution in recordResolvedCall.
	// Rules: rules/declarations/interfaces.md — §§6,9.1.
	for _, requirement := range lookupType.InterfaceMethods {
		if requirement.CompilerKnownID == member.ID {
			return Type{}, expressionValue{}, false
		}
	}
	// A trusted core impl on a compiler-known primitive participates in the
	// same overload set as methods on declared nominal types. In particular,
	// byte.ToString(ByteStringFormat) must be considered before the universal
	// zero-argument/numeric-string fallback even though byte is not Named.
	//
	// Rules:
	//   - rules/compiler/compiler_known_members.md — "Lookup order"
	//   - rules/compiler/compiler_known_members.md — "User-defined ToString()"
	if len(a.functions[lookupType.Name+"."+member.Name]) > 0 && (lookupType.Named || member.Name == "ToString") {
		if member.Name != "ToString" {
			return Type{}, expressionValue{}, false
		}
		if _, replaced := a.exactUserToStringReplacement(lookupType); replaced {
			return Type{}, expressionValue{}, false
		}
		if len(expr.Arguments) > 0 {
			return Type{}, expressionValue{}, false
		}
	}
	a.compilerKnownMemberFacts[sourceTokenLocation(memberExpr.Property.Token)] = member
	if member.StructuralMutation && a.structurallyMutatesIteratedCollection(memberExpr.Object, member.Name, memberExpr.Property.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if lookupType.Kind == RawPtrType || lookupType.Name == "Arena" {
		return Type{}, expressionValue{}, false
	}
	switch member.Name {
	case "Ok", "Err":
		return a.inferConsumingResultProjection(expr, memberExpr, receiverType, lookupType, member)
	case "Borrow", "BorrowMut", "Replace":
		if lookupType.Name == "ThreadLocal" {
			return a.inferThreadLocalCall(expr, lookupType, member)
		}
		return Type{}, expressionValue{}, false
	case "ToString":
		maxArguments := 0
		if isNumericType(lookupType) {
			maxArguments = 1
		}
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+".ToString", 0, maxArguments) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 1 {
			formatType, _ := a.inferExpressionWithExpected(expr.Arguments[0], a.types["string"])
			if formatType.Kind != StringType {
				a.addErrorAtToken(expressionToken(expr.Arguments[0]), "ToString format must be string, got %s", typeDisplayName(formatType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "ToByteArray", "ToCharArray", "ToRuneArray", "Clear", "Reverse", "Sort", "RequestCancel", "Observe", "Start":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Append", "Fill":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, hasElement := compilerKnownCollectionElement(lookupType)
		if !hasElement {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if segment, isRange := expr.Arguments[0].(*ast.RangeExpression); isRange {
			// rules/collections/collections.md § 6.7: Append on an owning
			// dynamic array accepts a range segment and appends its values
			// in order, all or nothing.
			if member.ID != "CKM-DYNAMIC-ARRAY-APPEND" {
				a.addErrorAtTokenWithMetadata(segment.Token, diagnostics.ArrayRangeSegmentInvalid,
					"Only Append on an owning dynamic array T[] accepts a range; append the values one at a time.",
					"%s.%s does not accept a range", typeDisplayName(lookupType), member.Name)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			entry, ok := a.resolveRangeSegment(segment, elementType)
			if !ok {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			a.resolvedRangeAppends[expr] = entry
			return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], elementType)
		if !a.canInitialize(elementType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s value must be %s, got %s", member.Name, typeDisplayName(elementType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if member.Name == "Fill" && a.rejectNonCopyableFillElement(elementType, memberExpr.Property.Token, "Fill") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Reserve":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+".Reserve", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.compilerKnownUintArgument(expr.Arguments[0], "Reserve additional") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "RemoveAt":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+".RemoveAt", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[0], "RemoveAt") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Insert":
		if !a.checkCompilerKnownCallArity(expr, "list.Insert", 2, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[0], "Insert") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := compilerKnownCollectionElement(lookupType)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[1], elementType)
		if !a.canInitialize(elementType, valueType, expr.Arguments[1]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Insert value must be %s, got %s", typeDisplayName(elementType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Remove", "Contains", "IndexOf", "Add":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		argumentType, ok := compilerKnownCollectionElement(lookupType)
		if lookupType.Name == "map" && len(lookupType.TypeArgs) == 2 {
			argumentType, ok = lookupType.TypeArgs[0], true
		}
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], argumentType)
		if !a.canInitialize(argumentType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s argument must be %s, got %s", member.Name, typeDisplayName(argumentType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "ContainsKey":
		if !a.checkCompilerKnownCallArity(expr, "map.ContainsKey", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(lookupType.TypeArgs) != 2 {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		keyType := lookupType.TypeArgs[0]
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], keyType)
		if !a.canInitialize(keyType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "ContainsKey argument must be %s, got %s", typeDisplayName(keyType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "SortBy":
		if !a.checkCompilerKnownCallArity(expr, "list.SortBy", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		compareType, _ := a.inferExpression(expr.Arguments[0])
		if compareType.Kind != FunctionType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "SortBy requires a comparison function, got %s", typeDisplayName(compareType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	case "Union", "Intersection", "Difference", "SymmetricDifference":
		if !a.checkCompilerKnownCallArity(expr, "set."+member.Name, 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		otherType, _ := a.inferExpressionWithExpected(expr.Arguments[0], lookupType)
		if !sameConcreteType(lookupType, otherType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s argument must be %s, got %s", member.Name, typeDisplayName(lookupType), typeDisplayName(otherType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := compilerKnownCollectionElement(lookupType)
		classification := CopyClassificationOf(elementType)
		if !ok || classification != CopyTrivial && classification != CopySemantic {
			a.addErrorAtToken(memberExpr.Property.Token, "%s requires a copyable set element type", member.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// rejectCompilerKnownPropertyWrite rejects assignment to a compiler-known
// property such as Len, Ptr, SizeOf, Capacity or decimal Scale: these are
// read-only facts of the value (get-only in their canonical declarations),
// never storage. An ordinary struct field of the same name keeps its own
// rules.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Legacy lowercase member spellings", `Len`, `Ptr`
//   - rules/library/core-library.md — § 10.1 (MD-082)
func (a *Analyzer) rejectCompilerKnownPropertyWrite(objectType Type, member *ast.MemberExpression) bool {
	receiver := dereferenceType(objectType)
	if receiver.Kind == InvalidType || member.Property == nil {
		return false
	}
	if _, field := lookupStructField(receiver, member.Property.Value); field {
		return false
	}
	known, ok := compilerKnownMember(receiver, member.Property.Value, false)
	if !ok || known.Kind != CompilerKnownProperty {
		return false
	}
	a.addErrorAtToken(member.Property.Token, "compiler-known property %s of %s is read-only", known.Name, typeDisplayName(receiver))
	return true
}
