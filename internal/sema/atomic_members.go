package sema

import "strings"

// compilerKnownAtomicMembers supplies one canonical spelling catalog to Sema
// lookup, completion, hover and synthetic navigation. Signatures describe both
// ordinary object-operation overloads; they do not promise target capability.
// Rules: rules/concurrency/atomics.md — §§7,12–13,27–30,43–45.
func compilerKnownAtomicMembers(typ Type) []CompilerKnownMember {
	if !isAtomicType(typ) || !atomicElementTypeSupported(typ.TypeArgs[0]) {
		return nil
	}
	element := typ.TypeArgs[0]
	t := typeDisplayName(element)
	result := Type{Name: "CompareExchangeResult", Kind: StructType, TypeArgs: []Type{element}}
	var members []CompilerKnownMember
	for _, operation := range []struct {
		name, arguments, result, documentation string
		typ                                    Type
	}{
		{"Load", "", t, "Reads the atomic value.", element},
		{"Store", "value: " + t, "void", "Stores a value in the atomic object.", builtinType("void")},
		{"Swap", "value: " + t, t, "Replaces the atomic value and returns the previous value.", element},
		{"CompareExchange", "expected: " + t + ", desired: " + t, "CompareExchangeResult[" + t + "]", "Performs strong compare-exchange; a mismatch is normal control flow.", result},
		{"FetchAdd", "value: " + t, t, "Adds atomically and returns the previous value.", element},
		{"FetchSub", "value: " + t, t, "Subtracts atomically and returns the previous value.", element},
		{"FetchAnd", "value: " + t, t, "Applies bitwise AND atomically and returns the previous value.", element},
		{"FetchOr", "value: " + t, t, "Applies bitwise OR atomically and returns the previous value.", element},
		{"FetchXor", "value: " + t, t, "Applies bitwise XOR atomically and returns the previous value.", element},
	} {
		if strings.HasPrefix(operation.name, "Fetch") && !atomicFetchOperationSupported(operation.name, element) {
			continue
		}
		orderedArguments := operation.arguments
		if orderedArguments != "" {
			orderedArguments += ", "
		}
		if operation.name == "CompareExchange" {
			orderedArguments += "successOrder: MemoryOrder, failureOrder: MemoryOrder"
		} else {
			orderedArguments += "order: MemoryOrder"
		}
		members = append(members, CompilerKnownMember{
			ID: "CKM-ATOMIC-" + strings.ToUpper(operation.name), Name: operation.name,
			Kind: CompilerKnownMethod, Result: operation.typ,
			Signature:     "fn " + operation.name + "(" + operation.arguments + ") " + operation.result + "\nfn " + operation.name + "(" + orderedArguments + ") " + operation.result,
			Documentation: operation.documentation + " Omitted orders default to MemoryOrder.SeqCst.",
			Rule:          "rules/concurrency/atomics.md — §§12–25", Receiver: "shared " + typeDisplayName(typ),
			TargetRestriction: "requires CompilationPlan support for the concrete type, operation, orders and execution context",
		})
	}
	return members
}
