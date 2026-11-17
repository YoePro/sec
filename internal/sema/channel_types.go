package sema

import (
	"sec/internal/ast"
)

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

// channelOptionsType is the exact public ChannelOptions configuration type.
// Rules: rules/concurrency/channels.md — § 6(1).
func channelOptionsType() Type {
	uintType := Type{Name: "uint", Kind: UintType}
	capabilities := Type{Name: "set", Kind: StructType, TypeArgs: []Type{builtinChannelCapabilityType()}}
	return Type{Name: "ChannelOptions", Module: "core", Kind: StructType, Fields: []StructField{
		{Name: "Capacity", Type: uintType},
		{Name: "MaxSenders", Type: uintType},
		{Name: "Capabilities", Type: capabilities},
	}}
}

// inferChannelConstructor types `Channel[T](capacity)` and
// `Channel[T](ChannelOptions { ... })`. Construction is fallible only for
// allocation, so both forms yield Result[Channel[T], AllocationError]; there
// is no ChannelCreateError. Invalid compile-time-known options are
// compile-time diagnostics: MaxSenders must be compile-time-known and at
// least 1 (an omitted field defaults to 0 and is rejected the same way).
//
// Rules:
//   - rules/concurrency/channels.md — § 6(2)–(6) options, § 19(1)–(4) fallibility, § 20 construction syntax
func (a *Analyzer) inferChannelConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	invalid := func() (Type, expressionValue, bool) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "Channel expects 1 capacity or ChannelOptions argument, got %d", len(expr.Arguments))
		return invalid()
	}
	messageType, ok := a.resolveType(expr.GenericArguments[0])
	if !ok {
		return invalid()
	}
	argument := expr.Arguments[0]
	argumentType, _ := a.inferExpression(argument)
	switch {
	case argumentType.Kind == InvalidType:
		return invalid()
	case isIntegerType(argumentType):
	case argumentType.Name == "ChannelOptions" && argumentType.Kind == StructType:
		if !a.checkChannelOptions(argument) {
			return invalid()
		}
	default:
		a.addErrorAtToken(expressionToken(argument), "Channel capacity must be an integer or ChannelOptions, got %s", typeDisplayName(argumentType))
		return invalid()
	}
	return compilerKnownResult(channelType(messageType), a.types["AllocationError"]), expressionValue{Display: expr.String()}, true
}

// checkChannelOptions validates the compile-time-known parts of a
// ChannelOptions argument: the options must be written as a literal at the
// construction so MaxSenders and Capabilities are compile-time-known, and
// MaxSenders must be at least 1. Capacity may be runtime-known.
// Rules: rules/concurrency/channels.md — § 6(2)–(4), (6); § 19(4).
func (a *Analyzer) checkChannelOptions(argument ast.Expression) bool {
	literal, ok := argument.(*ast.StructLiteral)
	if !ok {
		a.addErrorAtTokenWithMetadata(expressionToken(argument), "",
			"Write the ChannelOptions literal at the Channel construction so MaxSenders and Capabilities are compile-time-known.",
			"ChannelOptions.MaxSenders and Capabilities must be compile-time-known at channel construction")
		return false
	}
	maxSenders := ast.Expression(nil)
	for _, field := range literal.Fields {
		if field != nil && field.Name != nil && field.Name.Value == "MaxSenders" {
			maxSenders = field.Value
		}
	}
	if maxSenders == nil {
		a.addErrorAtTokenWithMetadata(literal.Token, "",
			"Set MaxSenders to the maximum number of simultaneously live senders; the initial Tx counts as one.",
			"ChannelOptions.MaxSenders must be at least 1; an omitted MaxSenders is 0")
		return false
	}
	value, known := a.integerConstantValue(maxSenders)
	if !known {
		a.addErrorAtTokenWithMetadata(expressionToken(maxSenders), "",
			"Use a compile-time constant for MaxSenders; endpoint identity layout is fixed before lowering.",
			"ChannelOptions.MaxSenders must be compile-time-known at channel construction")
		return false
	}
	if value.Sign() <= 0 {
		a.addErrorAtTokenWithMetadata(expressionToken(maxSenders), "",
			"Set MaxSenders to at least 1; the initial Tx counts toward it.",
			"ChannelOptions.MaxSenders must be at least 1, got %s", value.String())
		return false
	}
	return true
}
