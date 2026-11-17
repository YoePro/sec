package sema

import "sec/internal/lexer"

// checkRawPointerVolatilePointee rejects missing value representations,
// unproven generic parameters and aggregate layouts before publishing a volatile
// operation. Ordinary interface constraints and aggregate layout do not prove
// physical eligibility. Passing this frontend guard still requires a separate
// CompilationPlan-sensitive physical plan before lowering.
// Rules: rules/platform/volatile.md — §§33–34,37–38;
// rules/memory/raw_pointers.md — §3(4), §13(2,8).
func (a *Analyzer) checkRawPointerVolatilePointee(element Type, operation string, token lexer.Token) bool {
	if element.Kind == InvalidType || element.Kind == VoidType {
		verb := "materialize"
		if operation == "VolatileWrite" {
			verb = "consume"
		}
		a.reportInvalidRawPointerOperation(token, rawPointerTypedHelp,
			"RawPtr[void].%s cannot %s a value; select a concrete physical pointee type", operation, verb)
		return false
	}
	if element.Kind == GenericType {
		a.reportInvalidRawPointerOperation(token,
			"Use a concrete physical pointee representation whose volatile eligibility can be proven for the active CompilationPlan. Ordinary interface constraints and unsafe authorization do not establish volatile eligibility.",
			"RawPtr.%s requires proven volatile eligibility for generic pointee %s", operation, typeDisplayName(element))
		return false
	}
	switch element.Kind {
	case StructType, ArrayType, UnionType, ResultType:
		// No canonical verified aggregate access contract is supplied by the
		// current frontend. Ordinary layout or concrete generic substitution
		// must not substitute for that missing proof (§37).
		a.reportInvalidRawPointerOperation(token,
			"Aggregate volatile access needs an explicit physical layout and access contract; ordinary Sec layout, field count and SizeOf do not prove eligibility. No verified aggregate access contract is available for this operation.",
			"RawPtr.%s requires an explicit physical layout and access contract for aggregate %s", operation, typeDisplayName(element))
		return false
	}
	return true
}
