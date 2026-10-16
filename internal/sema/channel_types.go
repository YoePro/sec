package sema

import "sec/internal/ast"

// inferChannelMessage preserves contextual message typing while admitting an
// explicit move at the message argument boundary. No ownership is committed
// before execution-bound dependencies and the remaining arguments are checked.
// Rules: rules/concurrency/channels.md — §§1(1),53(1–3);
// rules/memory/copy_move.md — §8.2; rules/memory/transferability.md — §15.
func (a *Analyzer) inferChannelMessage(value ast.Expression, expected Type) Type {
	if _, explicit := explicitMoveArgument(value); explicit {
		typ, _ := a.inferCallArgumentExpression(value)
		return typ
	}
	typ, _ := a.inferExpressionWithExpected(value, expected)
	return typ
}

// builtinChannelCapabilityType predeclares the closed channel capability enum.
// The loader-proven core declaration refines this same reserved identity; the
// values describe language features, not proof of runtime or ISR capability.
// Rules: rules/concurrency/channels.md — §§4–5,45;
// rules/corrections/applied/channels-v2-cross-rulebook-correction-20260915.md — §1.
func builtinChannelCapabilityType() Type {
	values := []string{"Revocation", "Expiration", "ISRSafeSend", "Statistics"}
	return Type{Name: "ChannelCapability", Module: "core", Named: true, Declared: true,
		Kind: EnumType, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: "Revocation"}
}
