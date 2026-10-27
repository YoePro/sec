package semantic

import (
	"fmt"
	"math/big"
)

// appendReturnOriginPath detaches exact integers and composes consecutive
// source-relative slice intervals without loss of known precision.
// Rules: rules/corrections/applied/MD-115-returned-slice-origin-projections-correction-20261027.md §§4–5.
func appendReturnOriginPath(prefix, suffix []ReturnOriginStep) []ReturnOriginStep {
	result := make([]ReturnOriginStep, 0, len(prefix)+len(suffix))
	for _, step := range append(append([]ReturnOriginStep(nil), prefix...), suffix...) {
		step.Index, step.Start, step.EndExclusive = cloneBigInt(step.Index), cloneBigInt(step.Start), cloneBigInt(step.EndExclusive)
		if len(result) > 0 && step.Kind == ReturnOriginSlice && result[len(result)-1].Kind == ReturnOriginSlice {
			prior := result[len(result)-1]
			if prior.UnknownRange || step.UnknownRange {
				step = ReturnOriginStep{Kind: ReturnOriginSlice, UnknownRange: true}
			} else if step.Start == nil || step.EndExclusive == nil || prior.Start == nil || prior.EndExclusive == nil || step.Start.Sign() < 0 || new(big.Int).Add(prior.Start, step.EndExclusive).Cmp(prior.EndExclusive) > 0 {
				// Preserve a noncanonical chain so verification rejects it;
				// never compose an out-of-parent interval into a valid path.
				result = append(result, step)
				continue
			} else {
				step.Start.Add(step.Start, prior.Start)
				step.EndExclusive.Add(step.EndExclusive, prior.Start)
			}
			result[len(result)-1] = step
		} else {
			result = append(result, step)
		}
	}
	return result
}

// equalOriginInteger compares canonical numeric values, never pointer identity.
// Rules: rules/compiler/semantic_ir.md — §45(12a), MD-115 §5(3).
func equalOriginInteger(a, b *big.Int) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Cmp(b) == 0
}

// returnedSliceRange derives a relative interval only from the represented
// normalized tuple. Dynamic endpoints retain containment as [?]; full-open
// reborrows retain the entire existing path, including any prior narrowing.
// Guards and source/tuple identity are separately verified by P16 contracts.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.34 (MD-115 §§4,7); rules/mlir/packages/sec-mlir-dialect_package16.md §§78–86.
func returnedSliceRange(op Operation, definitions map[ValueID]Operation, functions map[FunctionID]*Function) (ReturnOriginStep, bool, bool) {
	step := ReturnOriginStep{Kind: ReturnOriginSlice, UnknownRange: true}
	if len(op.Operands) < 3 {
		return step, false, false
	}
	rangeOp, found := definitions[op.Operands[1]]
	if !found || (rangeOp.Kind != OpSliceRangeNormalize && rangeOp.Kind != OpSliceRangeCheck) || rangeOp.SliceRange == nil || len(rangeOp.Results) < 2 || rangeOp.Results[0].ID != op.Operands[1] || rangeOp.Results[1].ID != op.Operands[2] {
		return step, false, false
	}
	m := rangeOp.SliceRange
	if !m.StartPresent && !m.EndPresent {
		return step, true, true
	}
	constant := func(id ValueID) *big.Int { return returnedOriginConstant(id, definitions, functions, 0) }
	start := new(big.Int)
	next := 1
	if m.StartPresent {
		start = constant(rangeOp.Operands[next])
		next++
	}
	var end *big.Int
	if m.EndPresent {
		end = constant(rangeOp.Operands[next])
		if end != nil && !m.EndExclusive {
			end.Add(end, big.NewInt(1))
		}
	} else {
		end = constant(rangeOp.Operands[0])
	}
	if start != nil && end != nil && start.Sign() >= 0 && start.Cmp(end) <= 0 {
		step.UnknownRange, step.Start, step.EndExclusive = false, start, end
	}
	return step, false, true
}

// returnedOriginConstant evaluates only represented constant range tuples and
// slice lengths. It never converts arbitrary-precision endpoints to host ints.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.34 (MD-115 §§4(1–3),7(2)).
func returnedOriginConstant(id ValueID, definitions map[ValueID]Operation, functions map[FunctionID]*Function, depth int) *big.Int {
	if depth > 64 {
		return nil
	}
	op := definitions[id]
	constant := func(id ValueID) *big.Int { return returnedOriginConstant(id, definitions, functions, depth+1) }
	switch op.Kind {
	case OpConstInt:
		return cloneBigInt(op.Integer)
	case OpSliceLength:
		if len(op.Operands) != 1 {
			return nil
		}
		return returnedSliceExtent(op.Operands[0], definitions, functions, depth+1)
	case OpSliceRangeNormalize, OpSliceRangeCheck:
		m := op.SliceRange
		if m == nil || len(op.Results) < 2 {
			return nil
		}
		start := new(big.Int)
		next := 1
		if m.StartPresent {
			start = constant(op.Operands[next])
			next++
		}
		if id == op.Results[0].ID {
			return start
		}
		if id != op.Results[1].ID {
			return nil
		}
		var end *big.Int
		if m.EndPresent {
			end = constant(op.Operands[next])
			if end != nil && !m.EndExclusive {
				end.Add(end, big.NewInt(1))
			}
		} else {
			end = constant(op.Operands[0])
		}
		if start != nil && end != nil && start.Sign() >= 0 && start.Cmp(end) <= 0 {
			return new(big.Int).Sub(end, start)
		}
	}
	return nil
}

// verifyReturnOriginRangeType checks typed sequence transitions and authority
// for range-bearing paths; old MD-084 paths retain their predecessor contract.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.34 (MD-115 §3).
func verifyReturnOriginRangeType(module *Module, fn *Function, origin ReturnOrigin, returned Type) error {
	hasRange := false
	for _, step := range origin.Path {
		hasRange = hasRange || step.Kind == ReturnOriginSlice
	}
	if !hasRange {
		return nil
	}
	var current Type
	var currentID TypeID
	if origin.Kind == ReturnOriginStatic {
		for _, storage := range module.Statics {
			if storage.ID == origin.Static {
				currentID = storage.Type
				current, _ = module.Types.Lookup(currentID)
			}
		}
	} else {
		if origin.Parameter < 0 || origin.Parameter >= len(fn.Parameters) {
			return fmt.Errorf("returned range names invalid parameter")
		}
		currentID = fn.Parameters[origin.Parameter].Value.Type
		current, _ = module.Types.Lookup(currentID)
	}
	mutable := current.Kind == TypeMutableSlice || current.Kind == TypeReference && current.Mutable
	if returned.Kind == TypeMutableSlice && !mutable {
		return fmt.Errorf("returned mutable slice requires mutable origin authority")
	}
	if current.Kind == TypeReference {
		currentID = current.Element
		current, _ = module.Types.Lookup(currentID)
	}
	previousRange := false
	for _, step := range origin.Path {
		switch step.Kind {
		case ReturnOriginSlice:
			if previousRange {
				return fmt.Errorf("returned range path is not normalized")
			}
			if current.Kind != TypeArray && current.Kind != TypeSharedSlice && current.Kind != TypeMutableSlice {
				return fmt.Errorf("returned range requires a sequence origin")
			}
			if current.Kind == TypeArray && !step.UnknownRange {
				length, ok := new(big.Int).SetString(current.Length, 10)
				if !ok || step.EndExclusive.Cmp(length) > 0 {
					return fmt.Errorf("returned range exceeds array origin")
				}
			}
			current = Type{Kind: TypeSharedSlice, Element: current.Element}
		case ReturnOriginField:
			found := false
			if definition, ok := structDefinition(module, currentID); ok {
				for _, field := range definition.Fields {
					if field.ID == step.Field {
						currentID = field.Type
						current, _ = module.Types.Lookup(currentID)
						found = true
						break
					}
				}
			}
			if !found {
				return fmt.Errorf("returned range has invalid field path")
			}
		case ReturnOriginElement:
			if current.Kind != TypeArray && current.Kind != TypeSharedSlice && current.Kind != TypeMutableSlice {
				return fmt.Errorf("returned range has invalid element path")
			}
			currentID = current.Element
			current, _ = module.Types.Lookup(currentID)
		default:
			return fmt.Errorf("invalid returned range projection")
		}
		previousRange = step.Kind == ReturnOriginSlice
	}
	if current.Kind == TypeSharedSlice || current.Kind == TypeMutableSlice {
		if (returned.Kind != TypeSharedSlice && returned.Kind != TypeMutableSlice) || returned.Element != current.Element {
			return fmt.Errorf("returned range requires compatible slice result")
		}
	} else if returned.Kind != TypeReference || returned.Element != currentID {
		return fmt.Errorf("returned range requires compatible reference result")
	}
	return nil
}

// returnedSliceExtent uses verified callee range contracts and actual inputs
// to retain exact open-end composition through returned slice expressions.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.34 (MD-115 §§4,7(5)).
func returnedSliceExtent(value ValueID, definitions map[ValueID]Operation, functions map[FunctionID]*Function, depth int) *big.Int {
	if depth > 64 {
		return nil
	}
	op := definitions[value]
	switch op.Kind {
	case OpSliceBorrowShared, OpSliceBorrowMutable, OpSliceReborrowShared, OpSliceReborrowMutable:
		if len(op.Operands) >= 3 {
			return returnedOriginConstant(op.Operands[2], definitions, functions, depth+1)
		}
	case OpSliceCopyShared, OpSliceMoveMutable:
		return returnedSliceExtent(op.Operands[0], definitions, functions, depth+1)
	case OpDirectCall:
		callee := functions[op.Callee]
		if callee == nil || len(callee.ReturnOrigins) == 0 {
			return nil
		}
		var common *big.Int
		for _, origin := range callee.ReturnOrigins {
			var extent *big.Int
			if len(origin.Path) > 0 {
				last := origin.Path[len(origin.Path)-1]
				if last.Kind == ReturnOriginSlice && !last.UnknownRange && last.Start != nil && last.EndExclusive != nil {
					extent = new(big.Int).Sub(last.EndExclusive, last.Start)
				}
			} else if origin.Kind != ReturnOriginStatic && origin.Parameter >= 0 && origin.Parameter < len(op.Operands) {
				extent = returnedSliceExtent(op.Operands[origin.Parameter], definitions, functions, depth+1)
			}
			if extent == nil || common != nil && common.Cmp(extent) != 0 {
				return nil
			}
			common = extent
		}
		return common
	}
	return nil
}
