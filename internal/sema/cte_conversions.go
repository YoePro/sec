package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema/membership"
)

// cteFactRequest identifies the type fact a deferred required position
// establishes: a range bound of a named type's contract, at its zero-based
// source-order contract index, or the type's explicit default.
type cteFactRequest struct {
	TypeIdentity  string
	TypeName      string
	Fact          string
	ContractIndex int
}

// factRequest finds the pending request establishing a type fact.
func (x *cteExecutor) factRequest(identity, fact string, index int) (ctePendingRequest, bool) {
	for _, request := range x.a.ctePending {
		if request.Fact != nil && request.Fact.TypeIdentity == identity && request.Fact.Fact == fact &&
			(fact == "default" || request.Fact.ContractIndex == index) {
			return request, true
		}
	}
	return ctePendingRequest{}, false
}

// establishFact returns the value of a type fact required while executing,
// establishing it on demand in dependency order (§ 38(2)). A fact that is
// already being established by an enclosing evaluation is a
// type/value dependency cycle (§ 41), reported with the chain of facts.
func (x *cteExecutor) establishFact(request ctePendingRequest, at lexer.Token) (DefaultConstant, bool) {
	fact := request.Fact
	label := "the " + fact.Fact + " of " + fact.TypeName
	for _, active := range x.inProgress {
		if active == label {
			chain := append(append([]string(nil), x.inProgress...), label)
			x.fail(cteDependencyCycle, at, "%s depends on itself: %s", label, strings.Join(chain, " -> "))
			return DefaultConstant{}, false
		}
	}
	if x.results != nil {
		if result, done := x.results[request.Root]; done {
			if result.Failure != nil {
				x.fail(cteNotEvaluable, at, "%s could not be established at compile time", label)
				return DefaultConstant{}, false
			}
			return result.Value, true
		}
	}
	child := newCTEExecutor(x.a, x.program)
	child.results = x.results
	child.inProgress = append(append([]string(nil), x.inProgress...), label)
	previousModule, previousFile := x.a.currentModule, x.a.currentSourceFile
	x.a.currentModule, x.a.currentSourceFile = request.Module, request.File
	// Type the root in its module scope exactly as the request loop does.
	x.a.inferExpression(request.Root)
	result := child.evaluate(request.Root)
	x.a.currentModule, x.a.currentSourceFile = previousModule, previousFile
	if x.results != nil {
		x.results[request.Root] = result
	}
	x.mergeDependencies(result.Dependencies)
	if result.Failure != nil {
		if result.Failure.Kind == cteDependencyCycle {
			failure := *result.Failure
			failure.CallStack = append(append([]cteCallSite(nil), x.calls...), failure.CallStack...)
			x.failure = &failure
			return DefaultConstant{}, false
		}
		x.fail(cteNotEvaluable, at, "%s could not be established at compile time: %s", label, result.Failure.Reason)
		return DefaultConstant{}, false
	}
	return result.Value, true
}

// conversionTarget recognizes an explicit conversion call `T(value)`: a
// call Sema did not resolve to a function whose callee names a type.
func (x *cteExecutor) conversionTarget(expr ast.Expression) (*ast.CallExpression, Type, bool) {
	call, ok := expr.(*ast.CallExpression)
	if !ok || len(call.Arguments) != 1 {
		return nil, Type{}, false
	}
	if _, resolved := x.a.resolvedCalls[call]; resolved {
		return nil, Type{}, false
	}
	if _, named := call.Callee.(*ast.Identifier); !named {
		return nil, Type{}, false
	}
	target, ok := x.a.ResolvedTypeOf(call)
	if !ok || target.Kind == InvalidType {
		return nil, Type{}, false
	}
	return call, target, true
}

// checkedConversion executes a checked conversion with the runtime order of
// checks (contracts.md § 13.1): the target's intrinsic domain first
// (ConversionError.OutOfRange), then each declared contract in source order
// (ContractError.Violation { Kind, DeclarationIndex }, the channel Sema
// types for numeric contract conversions). A contract bound established by semantic CTE is
// read from its fact, establishing it on demand. It returns the converted
// value or the ConversionError value.
// Rules: rules/types/contracts.md — §§ 13, 13.1; rules/compiler/compile_time_evaluation.md — §§ 38, 41.
func (x *cteExecutor) checkedConversion(call *ast.CallExpression, target Type) (DefaultConstant, *DefaultConstant, bool) {
	value, ok := x.eval(call.Arguments[0])
	if !ok {
		return DefaultConstant{}, nil, false
	}
	return x.checkConversion(call, target, value)
}

// checkConversion applies the checks of a checked conversion to an
// evaluated argument value.
func (x *cteExecutor) checkConversion(call *ast.CallExpression, target Type, value DefaultConstant) (DefaultConstant, *DefaultConstant, bool) {
	x.dependOnType(target)
	if value.Integer == nil {
		if hasContracts(target) {
			x.fail(cteImplementationGap, call.Token, "checked conversion of a non-integer value into constrained type %s is not executed by the compile-time evaluator yet", typeDisplayName(target))
			return DefaultConstant{}, nil, false
		}
		converted, ok := x.convertValue(call, target, value)
		return converted, nil, ok
	}
	if low, high, integer := scalarIntegerDomain(target); integer && (value.Integer.Cmp(low) < 0 || value.Integer.Cmp(high) > 0) {
		failure := x.conversionError("OutOfRange", nil)
		return DefaultConstant{}, &failure, true
	}
	identity := canonicalTypeIdentity(target)
	for index, contract := range target.Contracts {
		violated := false
		kind := ""
		switch c := contract.(type) {
		case RangeContract:
			kind = "Range"
			low, high := c.Min, c.Max
			if low == nil {
				if request, pending := x.factRequest(identity, "range lower bound", index); pending {
					bound, ok := x.establishFact(request, call.Token)
					if !ok {
						return DefaultConstant{}, nil, false
					}
					low = bound.Integer
				}
			}
			if high == nil {
				if request, pending := x.factRequest(identity, "range upper bound", index); pending {
					bound, ok := x.establishFact(request, call.Token)
					if !ok {
						return DefaultConstant{}, nil, false
					}
					high = bound.Integer
				}
			}
			violated = low != nil && value.Integer.Cmp(low) < 0 ||
				high != nil && (value.Integer.Cmp(high) > 0 || c.Exclusive && value.Integer.Cmp(high) == 0)
		case MultipleOfContract:
			kind = "MultipleOf"
			violated = c.Value != nil && c.Value.Sign() != 0 && new(big.Int).Rem(new(big.Int).Abs(value.Integer), new(big.Int).Abs(c.Value)).Sign() != 0
		case MarkerContract:
			switch c.Name {
			case "odd":
				kind, violated = "Odd", value.Integer.Bit(0) == 0
			case "even":
				kind, violated = "Even", value.Integer.Bit(0) != 0
			default:
				x.fail(cteImplementationGap, call.Token, "the %s contract of %s is not checked by the compile-time evaluator yet", c.Name, typeDisplayName(target))
				return DefaultConstant{}, nil, false
			}
		default:
			x.fail(cteImplementationGap, call.Token, "this contract of %s is not checked by the compile-time evaluator yet", typeDisplayName(target))
			return DefaultConstant{}, nil, false
		}
		if violated {
			// Sema types a numeric contract conversion's failure as the
			// ContractError channel (try_failure_set.go, pending MD-012
			// runtime integration); string contracts use ConversionError.
			violation := x.contractViolation(kind, index)
			return DefaultConstant{}, &violation, true
		}
	}
	converted := value
	converted.Kind = target.Kind
	return converted, nil, true
}

// convertValue applies an unconstrained scalar conversion.
func (x *cteExecutor) convertValue(call *ast.CallExpression, target Type, value DefaultConstant) (DefaultConstant, bool) {
	if value.Integer != nil && (target.Kind == FloatType || target.Kind == DecimalType) {
		if prepared, outcome := x.a.prepareCompileTimeConstant(value, target); outcome == compileTimeEvaluated {
			return prepared, true
		}
	}
	if value.Kind == target.Kind && value.Integer == nil {
		return value, true
	}
	x.fail(cteImplementationGap, call.Token, "conversion %s is not executed by the compile-time evaluator yet", call.String())
	return DefaultConstant{}, false
}

// conversionError builds a core ConversionError value.
// Rules: rules/types/contracts.md — § 13.1.
func (x *cteExecutor) conversionError(variant string, payload *DefaultConstant) DefaultConstant {
	return variantValue(x.a.types["ConversionError"], variant, payload)
}

// contractViolation builds ContractError.Violation { Kind, DeclarationIndex }.
func (x *cteExecutor) contractViolation(kind string, index int) DefaultConstant {
	kindType := x.a.types["ContractKind"]
	owner := nominalConstantOwner(kindType)
	kindValue := DefaultConstant{Kind: kindType.Kind, Nominal: membership.Value{Owner: owner, Member: kind, Key: "variant:" + kind, Type: typeDisplayName(kindType), TypeName: kindType.Name}, Lexeme: owner + "." + kind, String: kind}
	declarationIndex := DefaultConstant{Kind: UintType, Integer: big.NewInt(int64(index)), Lexeme: big.NewInt(int64(index)).String()}
	payload := DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Names: []string{"Kind", "DeclarationIndex"}, Fields: map[string]DefaultConstant{"Kind": kindValue, "DeclarationIndex": declarationIndex}}}
	return variantValue(x.a.types["ContractError"], "Violation", &payload)
}
