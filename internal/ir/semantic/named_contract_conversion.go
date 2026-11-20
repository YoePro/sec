package semantic

import (
	"math"
	"math/big"
	"strconv"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// buildNamedContractConversion lowers a runtime-checked conversion into a
// constrained named integer type from Sema's plan: the intrinsic conversion
// into the carrier first (its failure is ConversionError.OutOfRange), then
// every declared contract in order, each violation constructing
// ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex })
// for the enclosing try, and finally the nominal value, which exists only on
// the path where every check passed. Without try a violation is the
// ContractFailure panic Sema announces with its advisory.
//
// Rules:
//   - rules/types/contracts.md — §§ 12, 13, 13.1
//   - rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 11–13, 18–19
func (fb *functionBuilder) buildNamedContractConversion(call *ast.CallExpression, plan sema.ResolvedNamedContractConversion) (builtValue, error) {
	if plan.Unavailable != "" {
		return builtValue{}, fb.unsupported("checked conversion into "+plan.TargetType.Name+": "+plan.Unavailable, call.Token)
	}
	point, protected := fb.failurePoint(call)
	carrierType, err := fb.owner.internType(plan.CarrierType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(plan.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	var value builtValue
	switch {
	case plan.Intrinsic == nil:
		if value, err = fb.buildExpr(call.Arguments[0], carrierType); err != nil {
			return builtValue{}, err
		}
	case plan.Intrinsic.Exact && plan.Intrinsic.Proof.Proven():
		sourceType, err := fb.owner.internType(plan.SourceType)
		if err != nil {
			return builtValue{}, err
		}
		source, err := fb.buildExpr(call.Arguments[0], sourceType)
		if err != nil {
			return builtValue{}, err
		}
		value = fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{source.id}, ConversionProof: conversionProofOf(plan.Intrinsic.Proof), Location: loc}, carrierType)
	case protected:
		// The intrinsic target-domain failure precedes every contract.
		if value, err = fb.buildCheckedIntegerConversionOperand(call, *plan.Intrinsic, point, func(source TypeID) (builtValue, error) {
			return fb.buildExpr(call.Arguments[0], source)
		}); err != nil {
			return builtValue{}, err
		}
	default:
		return builtValue{}, fb.unsupported("checked conversion into "+plan.TargetType.Name+" with an intrinsic range check outside try", call.Token)
	}
	for _, predicate := range plan.Predicates {
		if err := fb.buildContractPredicate(call, predicate, value, carrierType, point, protected, loc); err != nil {
			return builtValue{}, err
		}
	}
	return fb.result(Operation{Kind: OpNamedConvert, Operands: []ValueID{value.id}, Location: loc}, targetType), nil
}

// buildContractPredicate emits one declared contract check on the carrier
// value; control continues in a block entered only when the check passed.
// Rules: rules/types/contracts.md — §§ 4–8, 13.1.
func (fb *functionBuilder) buildContractPredicate(call *ast.CallExpression, predicate sema.ResolvedContractPredicate, value builtValue, carrierType TypeID, point sema.TryFailurePoint, protected bool, loc Location) error {
	boolType, err := fb.boolType()
	if err != nil {
		return err
	}
	violation := fb.newBlock()
	constant := func(number *big.Int) builtValue {
		return fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(number), Location: loc}, carrierType)
	}
	compare := func(predicate IntegerComparePredicate, left, right ValueID) builtValue {
		return fb.result(Operation{Kind: OpIntCompare, IntegerCompare: predicate, Operands: []ValueID{left, right}, Location: loc}, boolType)
	}
	// failIf branches to the violation when condition holds.
	failIf := func(condition builtValue) {
		next := fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: violation.ID}, {Block: next.ID}}, Location: loc})
		fb.current = next
	}
	if predicate.FormatBits != 0 {
		if err := fb.buildFloatContractPredicate(predicate, value, carrierType, boolType, failIf, call.Token, loc); err != nil {
			return err
		}
		return fb.finishContractPredicate(call, predicate, violation, point, protected, loc)
	}
	switch predicate.Kind {
	case "Range":
		if predicate.Min != nil {
			failIf(compare(IntegerCompareLT, value.id, constant(predicate.Min).id))
		}
		if predicate.Max != nil {
			above := IntegerCompareGT
			if predicate.Exclusive {
				above = IntegerCompareGE
			}
			failIf(compare(above, value.id, constant(predicate.Max).id))
		}
	case "In":
		passed := fb.newBlock()
		for _, member := range predicate.Values {
			equal := compare(IntegerCompareEQ, value.id, constant(member).id)
			next := fb.newBlock()
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{equal.id}, Successors: []BranchTarget{{Block: passed.ID}, {Block: next.ID}}, Location: loc})
			fb.current = next
		}
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: violation.ID}}, Location: loc})
		fb.current = passed
	case "Odd", "Even":
		low := fb.result(Operation{Kind: OpIntBitwise, IntegerBitwise: IntegerBitwiseAnd, Operands: []ValueID{value.id, constant(big.NewInt(1)).id}, Location: loc}, carrierType)
		odd := compare(IntegerCompareNE, low.id, constant(new(big.Int)).id)
		if predicate.Kind == "Odd" {
			odd = fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{odd.id}, Location: loc}, boolType)
		}
		failIf(odd)
	case "MultipleOf":
		divisor := new(big.Int).Abs(predicate.Divisor)
		if divisor.Cmp(big.NewInt(1)) == 0 {
			break
		}
		// |divisor| > 1 neither divides by zero nor overflows the remainder.
		remainder := Operation{Kind: OpIntBinaryChecked, IntegerBinary: IntegerCheckedRemainder, Operands: []ValueID{value.id, constant(divisor).id}, Operator: "%", Location: loc}
		rest, err := fb.emitCheckedOperator(remainder, carrierType, nil, nil, nil)
		if err != nil {
			return err
		}
		failIf(compare(IntegerCompareNE, rest.id, constant(new(big.Int)).id))
	default:
		return fb.unsupported(predicate.Kind+" contract check", call.Token)
	}
	return fb.finishContractPredicate(call, predicate, violation, point, protected, loc)
}

// finishContractPredicate fills the violation block of one check: the
// ContractFailure panic without try, otherwise the canonical
// ConversionError.Contract payload routed to the enclosing try. Control
// continues in the block where the check passed.
// Rules: rules/types/contracts.md — §§ 12(2), 13.1.
func (fb *functionBuilder) finishContractPredicate(call *ast.CallExpression, predicate sema.ResolvedContractPredicate, violation *Block, point sema.TryFailurePoint, protected bool, loc Location) error {
	passed := fb.current
	fb.current = violation
	if !protected {
		if err := fb.emitPanic(diagnostics.PanicReasonContractFailure, "", false, loc); err != nil {
			return err
		}
		fb.current = passed
		return nil
	}
	failure, err := fb.contractViolationError(point.ErrorType, predicate, call.Token, loc)
	if err != nil {
		return err
	}
	fb.routeFailure(call, failure, loc)
	fb.current = passed
	return nil
}

// contractViolationError constructs
// ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex }).
// Rules: rules/types/contracts.md — § 13.1.
func (fb *functionBuilder) contractViolationError(errorType sema.Type, predicate sema.ResolvedContractPredicate, token lexer.Token, loc Location) (builtValue, error) {
	conversionType, err := fb.owner.internType(errorType)
	if err != nil {
		return builtValue{}, err
	}
	conversion, ok := fb.owner.unionDefinition(conversionType)
	if !ok {
		return builtValue{}, fb.unsupported("ConversionError union definition", token)
	}
	contractIndex := unionVariantIndexByName(conversion, "Contract")
	if contractIndex < 0 || conversion.Variants[contractIndex].Kind != UnionVariantSingle {
		return builtValue{}, fb.unsupported("ConversionError.Contract", token)
	}
	contractType := conversion.Variants[contractIndex].Payload
	contract, ok := fb.owner.unionDefinition(contractType)
	if !ok {
		return builtValue{}, fb.unsupported("ContractError union definition", token)
	}
	violationIndex := unionVariantIndexByName(contract, "Violation")
	if violationIndex < 0 || contract.Variants[violationIndex].Kind != UnionVariantFields {
		return builtValue{}, fb.unsupported("ContractError.Violation", token)
	}
	operands := map[string]ValueID{}
	for _, field := range contract.Variants[violationIndex].PayloadFields {
		switch field.Name {
		case "Kind":
			kinds, ok := fb.owner.enumDefinition(field.Type)
			if !ok {
				return builtValue{}, fb.unsupported("ContractKind enum definition", token)
			}
			caseID, ok := enumCaseIDByName(kinds, predicate.Kind)
			if !ok {
				return builtValue{}, fb.unsupported("ContractKind."+predicate.Kind, token)
			}
			operands[field.Name] = fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, Location: loc}, field.Type).id
		case "DeclarationIndex":
			operands[field.Name] = fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(predicate.DeclarationIndex)), Location: loc}, field.Type).id
		default:
			return builtValue{}, fb.unsupported("ContractError.Violation field "+field.Name, token)
		}
	}
	violation := Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(violationIndex), Location: loc}
	for _, field := range contract.Variants[violationIndex].PayloadFields {
		violation.Operands = append(violation.Operands, operands[field.Name])
		violation.UnionFields = append(violation.UnionFields, field.Name)
		violation.PayloadActions = append(violation.PayloadActions, UnionPayloadCopyTrivial)
	}
	payload := fb.result(violation, contractType)
	return fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(contractIndex), Operands: []ValueID{payload.id}, PayloadActions: []UnionPayloadAction{UnionPayloadCopyTrivial}, Location: loc}, conversionType), nil
}

// unionVariantIndexByName finds a variant by its declared name, or -1.
// Rules: rules/declarations/unions.md — variant identity by declared name.
func unionVariantIndexByName(definition UnionDefinition, name string) int {
	for index, variant := range definition.Variants {
		if variant.Name == name {
			return index
		}
	}
	return -1
}

// buildFloatContractPredicate checks one contract of a float carrier: Range
// with ordered comparisons against Sema's inward-rounded bounds, so NaN
// fails, and Finite through classification.
// Rules: rules/types/contracts.md — §§ 4, 8, 13.1; rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 24–26, 32.
func (fb *functionBuilder) buildFloatContractPredicate(predicate sema.ResolvedContractPredicate, value builtValue, carrierType, boolType TypeID, failIf func(builtValue), token lexer.Token, loc Location) error {
	float := func(bits uint64) builtValue {
		// The canonical bits are authoritative; the lexeme is the shortest
		// exact spelling of the same value, for printing only.
		lexeme := strconv.FormatFloat(math.Float64frombits(bits), 'g', -1, 64)
		if predicate.FormatBits == 32 {
			lexeme = strconv.FormatFloat(float64(math.Float32frombits(uint32(bits))), 'g', -1, 32)
		}
		return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: lexeme, Float: &FloatConstant{Bits: bits, Format: predicate.FormatBits, Canonical: true}, Location: loc}, carrierType)
	}
	fails := func(op Operation) {
		op.FloatSemantics = resolvedFloatSemantics(fb.owner.module.Types, carrierType, op)
		holds := fb.result(op, boolType)
		failIf(fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{holds.id}, Location: loc}, boolType))
	}
	switch predicate.Kind {
	case "Range":
		if predicate.MinBits != nil {
			fails(Operation{Kind: OpFloatCompare, IntegerCompare: IntegerCompareGE, Operands: []ValueID{value.id, float(*predicate.MinBits).id}, Location: loc})
		}
		if predicate.MaxBits != nil {
			upper := IntegerCompareLE
			if predicate.MaxStrict {
				upper = IntegerCompareLT
			}
			fails(Operation{Kind: OpFloatCompare, IntegerCompare: upper, Operands: []ValueID{value.id, float(*predicate.MaxBits).id}, Location: loc})
		}
	case "Finite":
		fails(Operation{Kind: OpFloatClassify, FloatCore: "finite", Operands: []ValueID{value.id}, Location: loc})
	default:
		return fb.unsupported(predicate.Kind+" contract check on a float carrier", token)
	}
	return nil
}
