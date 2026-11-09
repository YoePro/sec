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

// channelEndpointFields describes ordinary stored owning endpoint fields in
// the canonical order, so member lookup, Place projection and partial moves
// share one source identity instead of a channel-specific ownership path.
// Rules: rules/concurrency/channels.md §§15(1–5),21(1–6),57(3).
func channelEndpointFields(message Type) []StructField {
	return []StructField{{Name: "Tx", Type: senderType(message)}, {Name: "Rx", Type: receiverType(message)}}
}

// channelType returns the same concrete field shape used by the generic Channel
// template; constructor inference cannot drop its stored-field identities.
// Rules: rules/concurrency/channels.md §15(1–5),§21.
func channelType(message Type) Type {
	return Type{Name: "Channel", Kind: StructType, Intrinsic: true, TypeArgs: []Type{message}, Fields: channelEndpointFields(message)}
}

// rejectLegacyChannelEndpoint explains non-canonical lowercase endpoint names;
// canonical Tx/Rx are resolved by ordinary stored-field lookup.
// Rules: rules/concurrency/channels.md §§3(3),15(3),62–63.
func (a *Analyzer) rejectLegacyChannelEndpoint(expr *ast.MemberExpression, owner Type) bool {
	if !isChannelType(owner) {
		return false
	}
	canonical := ""
	switch expr.Property.Value {
	case "tx":
		canonical = "Tx"
	case "rx":
		canonical = "Rx"
	}
	if canonical == "" {
		return false
	}
	a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
		"Use the canonical stored field `"+canonical+"`; extract its owning endpoint with an explicit move, for example `let endpoint :<- channel."+canonical+"`.",
		"unknown member %s on %s; use %s", expr.Property.Value, typeDisplayName(owner), canonical)
	return true
}
