package sema

import "sec/internal/ast"

// inferRawPointerCall validates compiler-known raw-address operations. Unsafe
// authorizes the operation class but does not manufacture a pointee type for
// RawPtr[void] or waive the operation's concrete element requirements.
//
// Rules:
//   - rules/memory/raw_pointers.md — §3(3)–(4) "RawPtr[void]"
//   - rules/memory/raw_pointers.md — §11 "Raw-pointer read"
//   - rules/memory/raw_pointers.md — §12 "Raw-pointer write"
//   - rules/memory/raw_pointers.md — §13 "Volatile raw-pointer access"
//   - rules/memory/raw_pointers.md — §14 "Pointer arithmetic"
func (a *Analyzer) inferRawPointerCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !a.rawPointerCallMayHaveValueReceiver(member.Object) {
		return Type{}, expressionValue{}, false
	}
	receiverType, _ := a.inferExpression(member.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	receiverType = dereferenceType(receiverType)
	if receiverType.Kind != RawPtrType {
		return Type{}, expressionValue{}, false
	}
	knownMember, known := compilerKnownMember(receiverType, member.Property.Value, false)

	switch member.Property.Value {
	case "Read":
		if !a.authorizeUnsafe(UnsafeRawPointerAccess, "RawPtr.Read", member.Property.Token, expr, rawPointerObligations(receiverType, "Read")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "Read", "RawPtr.Read requires unsafe because it reads through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.rejectProvenNullDereference(member, "Read") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkRawPointerCallArity(expr, "RawPtr.Read", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr[void].Read cannot materialize a value; select a concrete pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return element, expressionValue{Display: expr.String()}, true
	case "Write":
		if !a.authorizeUnsafe(UnsafeRawPointerAccess, "RawPtr.Write", member.Property.Token, expr, rawPointerObligations(receiverType, "Write")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "Write", "RawPtr.Write requires unsafe because it writes through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.rejectProvenNullDereference(member, "Write") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkRawPointerCallArity(expr, "RawPtr.Write", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr[void].Write cannot consume a value; select a concrete pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if !a.canInitialize(element, valueType, expr.Arguments[0]) {
			a.reportInvalidRawPointerOperation(expressionToken(expr.Arguments[0]), rawPointerArgumentHelp, "RawPtr.Write value must be %s, got %s", typeDisplayName(element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return a.types["void"], expressionValue{Display: expr.String()}, true
	case "VolatileRead":
		if !a.authorizeUnsafe(UnsafeRawPointerAccess, "RawPtr.VolatileRead", member.Property.Token, expr, rawPointerObligations(receiverType, "VolatileRead")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "VolatileRead", "RawPtr.VolatileRead requires unsafe because it performs observable physical-storage access")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.rejectProvenNullDereference(member, "VolatileRead") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkRawPointerCallArity(expr, "RawPtr.VolatileRead", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr[void].VolatileRead cannot materialize a value; select a concrete physical pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return element, expressionValue{Display: expr.String()}, true
	case "VolatileWrite":
		if !a.authorizeUnsafe(UnsafeRawPointerAccess, "RawPtr.VolatileWrite", member.Property.Token, expr, rawPointerObligations(receiverType, "VolatileWrite")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "VolatileWrite", "RawPtr.VolatileWrite requires unsafe because it performs observable physical-storage access")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.rejectProvenNullDereference(member, "VolatileWrite") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkRawPointerCallArity(expr, "RawPtr.VolatileWrite", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr[void].VolatileWrite cannot consume a value; select a concrete physical pointee type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if !a.canInitialize(element, valueType, expr.Arguments[0]) {
			a.reportInvalidRawPointerOperation(expressionToken(expr.Arguments[0]), rawPointerArgumentHelp, "RawPtr.VolatileWrite value must be %s, got %s", typeDisplayName(element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if known {
			a.recordCompilerKnownEffects(knownMember, member.Property.Token)
		}
		return a.types["void"], expressionValue{Display: expr.String()}, true
	case "Offset":
		if !a.authorizeUnsafe(UnsafeRawPointerArith, "RawPtr.Offset", member.Property.Token, expr, rawPointerObligations(receiverType, "Offset")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "Offset", "RawPtr.Offset requires unsafe because it performs raw-pointer arithmetic")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.reportInvalidRawPointerOperation(expr.Token, rawPointerArityHelp, "RawPtr.Offset expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(receiverType.TypeArgs) == 1 && receiverType.TypeArgs[0].Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr.Offset requires a typed element pointer, got RawPtr[void]")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.Offset") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "AddBytes":
		if !a.authorizeUnsafe(UnsafeRawPointerArith, "RawPtr.AddBytes", member.Property.Token, expr, rawPointerObligations(receiverType, "AddBytes")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "AddBytes", "RawPtr.AddBytes requires unsafe because it performs raw-pointer arithmetic")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.reportInvalidRawPointerOperation(expr.Token, rawPointerArityHelp, "RawPtr.AddBytes expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !isRawBytePointer(receiverType) {
			a.reportInvalidRawPointerOperation(member.Property.Token, "Use AddBytes only on RawPtr[byte]; use Offset with an int element count for another concrete pointee type.", "RawPtr.AddBytes requires RawPtr[byte], got %s", typeDisplayName(receiverType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.AddBytes") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "Difference":
		if !a.authorizeUnsafe(UnsafeRawPointerArith, "RawPtr.Difference", member.Property.Token, expr, rawPointerObligations(receiverType, "Difference")) {
			a.reportRawPointerUnsafe(member.Property.Token, receiverType, "Difference", "RawPtr.Difference requires unsafe because it performs raw-pointer arithmetic")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.reportInvalidRawPointerOperation(expr.Token, rawPointerArityHelp, "RawPtr.Difference expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(receiverType.TypeArgs) != 1 || receiverType.TypeArgs[0].Kind == VoidType {
			a.reportInvalidRawPointerOperation(member.Property.Token, rawPointerTypedHelp, "RawPtr.Difference requires a typed element pointer, got %s", typeDisplayName(receiverType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		otherType, _ := a.inferExpression(expr.Arguments[0])
		if otherType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !sameConcreteType(receiverType, otherType) {
			a.reportInvalidRawPointerOperation(expressionToken(expr.Arguments[0]), rawPointerArgumentHelp, "RawPtr.Difference argument must be %s, got %s", typeDisplayName(receiverType), typeDisplayName(otherType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["int"], expressionValue{Display: expr.String()}, true
	case "AsRef", "AsRefMut":
		a.reportInvalidRawPointerOperation(member.Property.Token, "Read pointer."+member.Property.Value+" inside an unsafe { } block, without parentheses. As* conversions are properties; a functional getter uses a Get* name.", "RawPtr.%s is a property and cannot be called", member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// rawPointerCallMayHaveValueReceiver leaves type-qualified/static lookup to its owner.
// Rules: rules/compiler/compiler_known_members.md — "RawPtr[T] compiler-known members".
func (a *Analyzer) rawPointerCallMayHaveValueReceiver(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		_, exists := a.symbols.lookup(expr.Value)
		return exists
	case *ast.MemberExpression:
		return a.rawPointerCallMayHaveValueReceiver(expr.Object)
	case *ast.IndexExpression:
		return true
	case *ast.CallExpression, *ast.ConversionExpression, *ast.RefExpression, *ast.AwaitExpression, *ast.TryExpression:
		return true
	default:
		return false
	}
}

// rawPointerArgumentIsInt enforces canonical signed element/byte offsets.
// Rules: rules/memory/raw_pointers.md — §14(4–7).
func (a *Analyzer) rawPointerArgumentIsInt(expr ast.Expression, operation string) bool {
	argType, _ := a.inferExpression(expr)
	if argType.Kind == InvalidType {
		return false
	}
	if argType.Kind != IntType || argType.Name != "int" {
		a.reportInvalidRawPointerOperation(expressionToken(expr), "Supply an int offset with valid target-address arithmetic; do not silently truncate or reinterpret an unsigned count.", "%s argument must be int, got %s", operation, typeDisplayName(argType))
		return false
	}
	return true
}

// isRawBytePointer limits byte arithmetic to RawPtr[byte].
// Rules: rules/memory/raw_pointers.md — §14(6).
func isRawBytePointer(typ Type) bool {
	return typ.Kind == RawPtrType && len(typ.TypeArgs) == 1 && typ.TypeArgs[0].Name == "byte"
}

// rawPointerObligations are a raw-pointer member's caller obligations from
// the compiler-known member registry.
func rawPointerObligations(pointer Type, operation string) []string {
	if member, ok := compilerKnownMember(pointer, operation, false); ok {
		return member.CallerObligations
	}
	return nil
}
