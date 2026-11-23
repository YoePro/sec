package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// OpStringMaterialize prepares one maximal concatenation using the explicit
// allocation capability. It either produces an ordinary string (panicking on
// materialization failure), or Result[string, StringError] for a protecting try.
// No intermediate concatenation strings or implicit operand conversions exist.
// Rules: rules/compiler/semantic_ir.md — §17(2),(4),(6);
// rules/foundations/operators.md — §§29.3,29.6; MD-039 §§2–5.
const OpStringMaterialize OpKind = "string.materialize"

// StringMaterializationFacts preserves segment interpretation and failure policy
// so consumers need neither source syntax nor repeated overload resolution.
// ErrorType is the canonical StringError selected by Sema in both modes.
// Rules: rules/compiler/semantic_ir.md — §17(6); rules/memory/allocation.md — §17.
type StringMaterializationFacts struct {
	// A fallible-text operand is already the successful string extracted by
	// preceding guarded CFG; materialization never receives or unwraps a Result.
	Segments      []sema.StringConcatSegmentKind
	Failure       sema.StringMaterializationFailureMode
	ErrorType     TypeID
	PanicReasonID uint16
}

// buildRuntimeStringConcat consumes a maximal non-interpolated plan. Operand
// evaluation precedes the single materialization in source order. Fallible text
// segments route their StringError before evaluating later segments; the same
// protecting try handles their error and the final materialization failure.
// Rules: rules/foundations/operators.md — §§29.3,29.6;
// rules/compiler/semantic_ir.md — §17(6); MD-039 §§2–5.
func (fb *functionBuilder) buildRuntimeStringConcat(expr ast.Expression, resultType TypeID) (builtValue, bool, error) {
	plan, known := fb.owner.analyzer.StringConcatPlanOf(expr)
	if !known || !plan.Runtime || plan.Interpolated || fb.owner.maxPackage < arenaLoweringPackage {
		return builtValue{}, false, nil
	}
	if fb.allocationContext == nil || !plan.Allocation.Context.Available {
		return builtValue{}, true, fb.unsupported("runtime string materialization without a resolved allocation capability", expressionToken(expr))
	}
	if plan.Failure != sema.StringMaterializationPanic && plan.Failure != sema.StringMaterializationTry {
		return builtValue{}, true, fmt.Errorf("runtime string materialization lacks its source-selected failure mode")
	}
	errorType, err := fb.owner.internType(plan.Allocation.FailureType)
	if err != nil {
		return builtValue{}, true, err
	}
	facts := &StringMaterializationFacts{Failure: plan.Failure, ErrorType: errorType}
	context := *fb.allocationContext
	operands := []ValueID{context.Value}
	loc := locationFromExpression(expr)
	for _, segment := range plan.Segments {
		var value builtValue
		switch segment.Kind {
		case sema.StringConcatConstantString:
			value = fb.result(Operation{Kind: OpConstString, String: segment.Text, Location: loc}, resultType)
		case sema.StringConcatString, sema.StringConcatChar, sema.StringConcatRune:
			typ, typeErr := fb.owner.internType(segment.ValueType)
			if typeErr != nil {
				return builtValue{}, true, typeErr
			}
			value, err = fb.buildExpr(segment.Expression, typ)
			if err != nil {
				return builtValue{}, true, err
			}
		case sema.StringConcatFallibleText:
			value, err = fb.buildFallibleTextSegment(expr, segment, plan, resultType, errorType)
			if err != nil {
				return builtValue{}, true, err
			}
		default:
			return builtValue{}, true, fb.unsupported("runtime string materialization segment "+string(segment.Kind)+" (formatting)", expressionToken(expr))
		}
		operands = append(operands, value.id)
		facts.Segments = append(facts.Segments, segment.Kind)
	}
	producedType := resultType
	if plan.Failure == sema.StringMaterializationTry {
		producedType = fb.owner.module.Types.Intern(Type{Kind: TypeResult, Name: "Result", Success: resultType, Error: errorType})
	} else {
		facts.PanicReasonID = uint16(diagnostics.PanicReasonStringMaterializationFailure)
	}
	materialized := fb.result(Operation{Kind: OpStringMaterialize, Operands: operands, StringMaterialization: facts, AllocationContext: &context, Location: loc}, producedType)
	if plan.Failure == sema.StringMaterializationPanic {
		return materialized, true, nil
	}
	point, protected := fb.failurePoint(expr)
	if !protected || point.Kind != sema.TryFailureAllocation {
		return builtValue{}, true, fmt.Errorf("fallible string materialization has no protecting failure route")
	}
	value, err := fb.routeCarrierValue(materialized, point, expressionToken(expr))
	return value, true, err
}

// verifyStringMaterialization validates one explicit maximal materialization,
// including exact segment types, canonical error identity and failure policy.
// Context capability origin and dominance are checked by the context verifier.
// Rules: rules/compiler/semantic_ir.md — §17(2),(4),(6);
// rules/foundations/operators.md — §§29.3,29.6; rules/errors/panic.md — §13.
func verifyStringMaterialization(types *TypeTable, op Operation, values map[ValueID]Value) error {
	facts := op.StringMaterialization
	if facts == nil || len(facts.Segments) == 0 || len(op.Operands) != len(facts.Segments)+1 || len(op.Results) != 1 || len(op.Successors) != 0 || op.AllocationContext == nil {
		return fmt.Errorf("invalid string.materialize shape or allocation context")
	}
	if op.AllocationContext.Value != op.Operands[0] || !typeHasKind(types, values[op.Operands[0]].Type, TypeAllocationContext) {
		return fmt.Errorf("string.materialize requires its selected context as the first operand")
	}
	failure, found := types.Lookup(facts.ErrorType)
	if !found || failure.Name != "StringError" || failure.Identity != "core::StringError" {
		return fmt.Errorf("string.materialize requires canonical StringError")
	}
	result, found := types.Lookup(op.Results[0].Type)
	if !found {
		return fmt.Errorf("missing string.materialize result type")
	}
	switch facts.Failure {
	case sema.StringMaterializationPanic:
		if result.Kind != TypeString || facts.PanicReasonID != uint16(diagnostics.PanicReasonStringMaterializationFailure) {
			return fmt.Errorf("ordinary string.materialize requires string and StringMaterializationFailure")
		}
	case sema.StringMaterializationTry:
		if result.Kind != TypeResult || result.Error != facts.ErrorType || !typeHasKind(types, result.Success, TypeString) || facts.PanicReasonID != 0 {
			return fmt.Errorf("fallible string.materialize requires Result[string, StringError] without a panic")
		}
	default:
		return fmt.Errorf("invalid string.materialize failure mode")
	}
	for index, kind := range facts.Segments {
		expected := TypeString
		switch kind {
		case sema.StringConcatConstantString, sema.StringConcatString, sema.StringConcatFallibleText:
		case sema.StringConcatChar:
			expected = TypeChar
		case sema.StringConcatRune:
			expected = TypeRune
		default:
			return fmt.Errorf("unsupported string.materialize segment %q", kind)
		}
		if !typeHasKind(types, values[op.Operands[index+1]].Type, expected) {
			return fmt.Errorf("string.materialize segment %d has incompatible type", index)
		}
	}
	return nil
}

// buildFallibleTextSegment consumes Sema's exact text Result category. It tests
// the carrier once and routes its error through the maximal plan's allocation
// failure point, whose canonical StringError channel covers both operand and
// materialization errors. Only the success block evaluates subsequent segments.
// No new error type, implicit ToString selection or panic conversion is inferred.
// Rules: rules/foundations/operators.md — §29.3(1)–(5);
// rules/errors/errorhandling.md — §§11,13.1;
// rules/corrections/applied/MD-004-MD-039-string-concatenation-try-or-panic-correction-20261031-20261121.md — §3.
func (fb *functionBuilder) buildFallibleTextSegment(root ast.Expression, segment sema.StringConcatSegment, plan sema.StringConcatPlan, stringType, errorType TypeID) (builtValue, error) {
	point, protected := fb.failurePoint(root)
	if plan.Failure != sema.StringMaterializationTry || !protected || point.Kind != sema.TryFailureAllocation {
		return builtValue{}, fmt.Errorf("fallible text operand has no protecting StringError route")
	}
	carrierType, err := fb.owner.internType(segment.ValueType)
	if err != nil {
		return builtValue{}, err
	}
	carrier, known := fb.owner.module.Types.Lookup(carrierType)
	if !known || carrier.Kind != TypeResult || carrier.Success != stringType || carrier.Error != errorType {
		return builtValue{}, fmt.Errorf("fallible text operand must be exactly Result[string, StringError]")
	}
	// Strings and the canonical error family in this subset have no temporary
	// destruction obligation. Keep nontrivial carriers at a named boundary until
	// failure-edge cleanup can represent their obligations.
	if !fb.owner.triviallyDestructible(segment.ValueType) {
		return builtValue{}, fb.unsupported("fallible text operand requiring temporary cleanup", expressionToken(segment.Expression))
	}
	value, err := fb.buildExpr(segment.Expression, carrierType)
	if err != nil {
		return builtValue{}, err
	}
	if value.typ != carrierType {
		return builtValue{}, fmt.Errorf("fallible text operand lost its Result carrier")
	}
	return fb.routeCarrierValue(value, point, expressionToken(segment.Expression))
}
