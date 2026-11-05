package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// stringViewMembers exposes exactly the read-only text-view surface. It does
// not inherit string members or the universal type-summary ToString fallback.
// Rules: rules/library/core-library.md — §5.8; rules/compiler/compiler_known_members.md
// — "StringView and string ranges"; MD-126 — §4(8a–10).
func stringViewMembers() []CompilerKnownMember {
	view := builtinType("StringView")
	uintType, boolType := builtinType("uint"), builtinType("bool")
	index := []FunctionParameter{{Name: "index", Type: uintType}}
	members := []CompilerKnownMember{
		{ID: "CKM-STRINGVIEW-LEN", Name: "Len", Kind: CompilerKnownProperty, Result: uintType},
		{ID: "CKM-STRINGVIEW-SIZEOF", Name: "SizeOf", Kind: CompilerKnownProperty, Result: uintType},
		{ID: "CKM-STRINGVIEW-ISEMPTY", Name: "IsEmpty", Kind: CompilerKnownMethod, Result: boolType},
		{ID: "CKM-STRINGVIEW-GETRUNE", Name: "GetRune", Kind: CompilerKnownMethod, Parameters: index, Result: compilerKnownResult(builtinType("rune"), builtinType("IndexError"))},
		{ID: "CKM-STRINGVIEW-RUNEAT", Name: "RuneAt", Kind: CompilerKnownMethod, Parameters: index, Result: builtinType("rune"), Effects: []EffectKind{EffectMayPanicBounds}},
		{ID: "CKM-STRINGVIEW-GETBYTE", Name: "GetByte", Kind: CompilerKnownMethod, Parameters: index, Result: compilerKnownResult(builtinType("byte"), builtinType("IndexError"))},
		{ID: "CKM-STRINGVIEW-BYTEAT", Name: "ByteAt", Kind: CompilerKnownMethod, Parameters: index, Result: builtinType("byte"), Effects: []EffectKind{EffectMayPanicBounds}},
		{ID: "CKM-STRINGVIEW-CHARAT", Name: "CharAt", Kind: CompilerKnownMethod, Parameters: index, Result: builtinType("char"), Effects: []EffectKind{EffectMayPanicBounds}},
		{ID: "CKM-STRINGVIEW-STARTSWITH", Name: "StartsWith", Kind: CompilerKnownMethod, Parameters: []FunctionParameter{{Name: "prefix", Type: view}}, Result: boolType},
		{ID: "CKM-STRINGVIEW-TOSTRING", Name: "ToString", Kind: CompilerKnownMethod, Result: toStringResultType()},
	}
	for i := range members {
		members[i].Category = CompilerKnownOperation
		members[i].Rule = "rules/library/core-library.md §5.8"
		members[i].AllocationBehavior = AllocationFree
		if members[i].Name == "ToString" {
			members[i].AllocationBehavior = AllocationUnknown
		}
	}
	return members
}

// inferStringViewCall validates the exact read-only registry contract and
// retains content materialization as a separate potentially allocating operation.
// Rules: rules/library/core-library.md — §5.8; MD-126 — §4(8a–10).
func (a *Analyzer) inferStringViewCall(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	invalid := Type{Kind: InvalidType}
	if len(expr.Arguments) != len(member.Parameters) {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.InvalidStringViewCall, "Use the read-only StringView member signature shown by hover/signature help.", "StringView.%s expects %d arguments, got %d", member.Name, len(member.Parameters), len(expr.Arguments))
		return invalid, expressionValue{Display: expr.String()}, true
	}
	for i, argument := range expr.Arguments {
		actual, _ := a.inferExpressionWithExpected(argument, member.Parameters[i].Type)
		if !a.canInitialize(member.Parameters[i].Type, actual, argument) {
			a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.InvalidStringViewCall, "Use a view-relative uint index, or a StringView prefix for StartsWith.", "StringView.%s argument must be %s, got %s", member.Name, typeDisplayName(member.Parameters[i].Type), typeDisplayName(actual))
			return invalid, expressionValue{Display: expr.String()}, true
		}
	}
	if !a.summaryPass && a.callGraphPathReachable {
		for _, effect := range member.Effects {
			a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: effect, Source: expr.Token})
		}
		if member.Name == "ToString" {
			a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: expr.Token, UnknownAllocation: true})
		}
	}
	return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
}
