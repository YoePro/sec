package sema

import "sec/internal/ast"

// ResolvedArgumentOperation describes preparation from the source, not a
// second copy or an element move invented by a consuming destination. Whole
// pack transfer is one source transaction even when it supplies many slots.
// Rules: rules/declarations/spread.md §§3,4.2–4.4;
// rules/declarations/functions.md §§18,38A(7); rules/memory/copy_move.md §§3,8.
type ResolvedArgumentOperation string

const (
	ArgumentCopyTrivial       ResolvedArgumentOperation = "copy-trivial"
	ArgumentCopySemantic      ResolvedArgumentOperation = "copy-semantic"
	ArgumentBorrowShared      ResolvedArgumentOperation = "borrow-shared"
	ArgumentBorrowMutable     ResolvedArgumentOperation = "borrow-mutable"
	ArgumentWholePackTransfer ResolvedArgumentOperation = "whole-pack-transfer"
	ArgumentMove              ResolvedArgumentOperation = "move"
	ArgumentValue             ResolvedArgumentOperation = "value"
)

// ResolvedCallArgumentSource is one original argument evaluation. Even an
// empty fixed spread retains a source entry. Expression is compiler-owned AST;
// public queries detach type and slice facts, not AST node identities.
// Types retain nominal identity, collection shape, exact scalar domains and
// callable signatures; aggregate declaration bodies remain separate facts.
// Rules: rules/declarations/spread.md §§3,4.1.
type ResolvedCallArgumentSource struct {
	Expression ast.Expression
	Type       Type
	Spread     bool
	Consuming  bool
}

// ResolvedExpandedCallArgument is a prepared argument slot. Source indexes the
// original source table; ElementIndex is the static projection (-1 for an
// ordinary value or runtime segment). RuntimeSpread denotes a segment whose
// element count is unknown, never a fictitious single argument. ValueType is
// the prepared value's type, distinct from the array/pack source type.
// ParameterIndex indexes the selected signature; every trailing variadic slot
// maps to its final parameter. Parameter retains the destination ownership mode.
// Rules: rules/declarations/spread.md §§3,4.1–4.4;
// rules/declarations/functions.md §§19,33,38A.
type ResolvedExpandedCallArgument struct {
	Source         int
	ElementIndex   int64
	RuntimeSpread  bool
	Prepared       bool
	ValueType      Type
	ParameterIndex int
	Parameter      FunctionParameter
	Operation      ResolvedArgumentOperation
	// Delivery is the transfer/reborrow of the prepared value into the
	// parameter, independent of the operation that prepared it from its source.
	Delivery ResolvedArgumentOperation
}

// ResolvedCallArgumentPlan retains source identity and the expanded order
// after successful overload, ownership and borrow validation. It is semantic
// input for consumers, not a claim that backend spread lowering exists.
// Rules: rules/declarations/spread.md §§3,4.2–4.4.
type ResolvedCallArgumentPlan struct {
	Sources  []ResolvedCallArgumentSource
	Expanded []ResolvedExpandedCallArgument
}

// resolvedSpreadArguments publishes existing inference facts without executing
// or inferring a source again. Unresolved heterogeneous templates use the
// specialization-specific ResolvedTypePackForwardOf projection instead.
// Rules: rules/declarations/spread.md §§3,4; functions.md §38A(4),(7).
func (a *Analyzer) resolvedSpreadArguments(call *ast.CallExpression, function Function) *ResolvedCallArgumentPlan {
	hasSpread := false
	for _, arg := range call.Arguments {
		_, spread := arg.(*ast.SpreadExpression)
		_, _, consuming := consumingPackSpread(arg)
		hasSpread = hasSpread || spread || consuming
	}
	if !hasSpread {
		return nil
	}
	plan := &ResolvedCallArgumentPlan{}
	for sourceIndex, argument := range call.Arguments {
		source := ResolvedCallArgumentSource{Expression: argument}
		spread, spreading := argument.(*ast.SpreadExpression)
		if _, moved, consuming := consumingPackSpread(argument); consuming {
			spread, spreading, source.Consuming = moved, true, true
		}
		if spreading {
			source.Expression, source.Spread = spread.Value, true
		}
		typ, known := a.ResolvedTypeOf(source.Expression)
		if !known {
			return nil
		}
		source.Type = enumConversionTypeSnapshot(typ)
		plan.Sources = append(plan.Sources, source)
		valueType, count, runtime := typ, int64(1), false
		if spreading {
			container := dereferenceType(typ)
			if container.Element == nil {
				return nil
			}
			valueType = *container.Element
			if container.Kind == ArrayType && arrayShapeOf(container) == ArrayShapeFixed {
				var exact bool
				count, exact = legacyArrayLength(container)
				if !exact {
					return nil
				}
			} else {
				runtime = true
			}
		}
		for index := int64(0); index < count; index++ {
			argumentIndex := len(plan.Expanded)
			parameter, known := functionParameterForArgument(function, argumentIndex)
			if !known {
				return nil
			}
			parameterIndex := argumentIndex
			if parameterIndex >= functionFixedParameterCount(function) && parameter.Variadic {
				parameterIndex = len(function.Parameters) - 1
			}
			elementIndex := int64(-1)
			if spreading && !runtime {
				elementIndex = index
			}
			operation := resolvedArgumentOperation(valueType, parameter, source.Consuming)
			if spreading && !runtime {
				operation = resolvedArgumentOperation(valueType, FunctionParameter{}, false)
			}
			if !spreading {
				operation = a.resolvedOrdinaryArgumentOperation(argument, parameter)
			}
			plan.Expanded = append(plan.Expanded, ResolvedExpandedCallArgument{
				Source: sourceIndex, ElementIndex: elementIndex, RuntimeSpread: runtime,
				Prepared: spreading, ValueType: enumConversionTypeSnapshot(valueType),
				ParameterIndex: parameterIndex, Parameter: parameter, Operation: operation,
				Delivery: resolvedArgumentDelivery(parameter),
			})
		}
	}
	return plan
}

// resolvedArgumentOperation projects validated source-to-slot ownership. Fixed
// array expansion copies before destination transfer, even for -> parameters.
// Rules: rules/declarations/spread.md §§4.3–4.4; functions.md §38A(7).
func resolvedArgumentOperation(typ Type, parameter FunctionParameter, consuming bool) ResolvedArgumentOperation {
	if consuming {
		return ArgumentWholePackTransfer
	}
	if mode, borrowed := callParameterBorrowMode(parameter); borrowed {
		if mode {
			return ArgumentBorrowMutable
		}
		return ArgumentBorrowShared
	}
	switch CopyClassificationOf(typ) {
	case CopyTrivial:
		return ArgumentCopyTrivial
	case CopySemantic:
		return ArgumentCopySemantic
	}
	// A non-spread fresh value is transferred without a reusable source.
	// Invalid non-copyable spreads have already been rejected before publication.
	return ArgumentValue
}

// resolvedOrdinaryArgumentOperation reads the transfer plan already validated
// for an ordinary argument beside a spread. A fresh value requires no copy.
// Rules: rules/memory/copy_move.md §§8.1–8.3; functions.md §§17–18.
func (a *Analyzer) resolvedOrdinaryArgumentOperation(argument ast.Expression, parameter FunctionParameter) ResolvedArgumentOperation {
	if _, borrowed := callParameterBorrowMode(parameter); borrowed {
		return resolvedArgumentDelivery(parameter)
	}
	if _, explicit := explicitMoveArgument(argument); explicit {
		return ArgumentMove
	}
	if plan, known := a.resolvedTransferPlans[argument]; known {
		switch plan.Action {
		case TransferCopyTrivial:
			return ArgumentCopyTrivial
		case TransferCopySemantic:
			return ArgumentCopySemantic
		case TransferMove:
			return ArgumentMove
		}
	}
	return ArgumentValue
}

// resolvedArgumentDelivery describes the already validated destination mode
// without converting a copied array projection into a move from that array.
// Rules: rules/declarations/spread.md §4.3; functions.md §§9,18,38A.
func resolvedArgumentDelivery(parameter FunctionParameter) ResolvedArgumentOperation {
	if mutable, borrowed := callParameterBorrowMode(parameter); borrowed {
		if mutable {
			return ArgumentBorrowMutable
		}
		return ArgumentBorrowShared
	}
	return ArgumentMove
}

// resolvedTypePackArgumentPlan projects the selected specialization's existing
// expansion facts into the same source/slot format as fixed and homogeneous
// spreads; template-only calls never publish invented concrete positions.
// Rules: rules/declarations/functions.md §38A(4),(7); spread.md §4.4.
func (a *Analyzer) resolvedTypePackArgumentPlan(forward *typePackForward, resolved ResolvedTypePackForward, outer map[string]Type) *ResolvedCallArgumentPlan {
	plan := &ResolvedCallArgumentPlan{}
	for _, arg := range forward.arguments {
		typ := SubstituteGenericType(arg.typ, outer)
		if arg.spread {
			element := typ
			typ = Type{Kind: VariadicPackType, Element: &element}
		}
		plan.Sources = append(plan.Sources, ResolvedCallArgumentSource{
			Expression: arg.expression, Type: typ,
			Spread: arg.spread, Consuming: arg.spread && forward.consuming,
		})
	}
	pack := outer[forward.pack]
	for index, slot := range resolved.Expanded {
		source := plan.Sources[slot.Source]
		typ := source.Type
		if slot.Position >= 0 {
			typ = pack.TypeArgs[slot.Position]
			if forward.mode != typePackSourceOwned {
				element := typ
				typ = Type{Kind: ReferenceType, Name: referenceTypeName(element, forward.mode == typePackSourceMutable), Element: &element, ReferenceMutable: forward.mode == typePackSourceMutable}
			}
		}
		operation := resolvedArgumentOperation(typ, slot.Parameter, source.Consuming)
		if !source.Spread {
			operation = a.resolvedOrdinaryArgumentOperation(source.Expression, slot.Parameter)
		}
		parameterIndex := index
		if slot.Parameter.Variadic {
			parameterIndex = len(resolved.Function.Parameters) - 1
		}
		plan.Expanded = append(plan.Expanded, ResolvedExpandedCallArgument{
			Source: slot.Source, ElementIndex: int64(slot.Position), Prepared: source.Spread,
			ValueType: typ, ParameterIndex: parameterIndex, Parameter: slot.Parameter,
			Operation: operation, Delivery: resolvedArgumentDelivery(slot.Parameter),
		})
	}
	return cloneResolvedCallArgumentPlan(plan)
}

// cloneResolvedCallArgumentPlan detaches mutable type/parameter facts while
// preserving the compiler-owned source nodes used as stable fact keys.
// Rules: rules/compiler/compiler_analysis.md §55; spread.md §4.2.
func cloneResolvedCallArgumentPlan(plan *ResolvedCallArgumentPlan) *ResolvedCallArgumentPlan {
	if plan == nil {
		return nil
	}
	copy := *plan
	copy.Sources = append([]ResolvedCallArgumentSource(nil), plan.Sources...)
	for i := range copy.Sources {
		copy.Sources[i].Type = enumConversionTypeSnapshot(copy.Sources[i].Type)
	}
	copy.Expanded = append([]ResolvedExpandedCallArgument(nil), plan.Expanded...)
	for i := range copy.Expanded {
		copy.Expanded[i].ValueType = enumConversionTypeSnapshot(copy.Expanded[i].ValueType)
		copy.Expanded[i].Parameter.Type = enumConversionTypeSnapshot(copy.Expanded[i].Parameter.Type)
	}
	return &copy
}
