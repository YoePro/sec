package semantic

import (
	"fmt"
	"strings"
)

// Range operations retain original endpoint types. Validation precedes any
// inclusive-end increment or narrowing to the target-sized unsigned outputs.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§13–19,77–80.
const (
	OpSliceRangeNormalize    OpKind              = "slice.range-normalize"
	OpSliceRangeCheck        OpKind              = "slice.range-check"
	SliceRangeProofOpenBound ArrayIndexProofKind = "source-open-bound"
)

// SliceRangeMetadata describes operands ordered as source length, present
// start, present end. An omitted bound has no operand or invented type.
// Normalize returns (start, length); check returns (start, length, failed,
// RangeError). Check outputs start/length are usable only on failed=false.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§35,78–80.
type SliceRangeMetadata struct {
	StartPresent, EndPresent bool
	EndExclusive             bool
	StartType, EndType       TypeID
	StartSigned, EndSigned   bool
	ProofKind                ArrayIndexProofKind
}

// verifySliceRangeOperation validates original integer endpoints and the
// canonical total checked tuple, without reconstructing Sema's proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§35,78–80.
func verifySliceRangeOperation(module *Module, op Operation, values map[ValueID]Value) error {
	metadata := op.SliceRange
	if metadata == nil {
		return fmt.Errorf("%s has no range metadata", op.Kind)
	}
	count := 1
	if metadata.StartPresent {
		count++
	}
	if metadata.EndPresent {
		count++
	}
	if len(op.Operands) != count || len(op.Successors) != 0 {
		return fmt.Errorf("%s has invalid source/endpoint operands", op.Kind)
	}
	sourceType := values[op.Operands[0]].Type
	source, ok := module.Types.Lookup(sourceType)
	if !ok || source.Kind != TypeUint || !source.TargetSize || source.Name != "uint" {
		return fmt.Errorf("%s source length must be uint", op.Kind)
	}
	next := 1
	for _, endpoint := range []struct {
		present bool
		typ     TypeID
		signed  bool
	}{
		{metadata.StartPresent, metadata.StartType, metadata.StartSigned},
		{metadata.EndPresent, metadata.EndType, metadata.EndSigned},
	} {
		if !endpoint.present {
			if endpoint.typ != 0 || endpoint.signed {
				return fmt.Errorf("%s omitted endpoint has type/signedness", op.Kind)
			}
			continue
		}
		signed, integer := semanticIntegerSigned(module.Types, endpoint.typ)
		if !integer || signed != endpoint.signed || values[op.Operands[next]].Type != endpoint.typ {
			return fmt.Errorf("%s endpoint type/signedness does not match original SSA", op.Kind)
		}
		next++
	}
	expected := 2
	if op.Kind == OpSliceRangeCheck {
		expected = 4
	}
	if len(op.Results) != expected || op.Results[0].Type != sourceType || op.Results[1].Type != sourceType {
		return fmt.Errorf("%s must return normalized uint start and length", op.Kind)
	}
	if op.Kind == OpSliceRangeNormalize {
		if !validArrayProof(metadata.ProofKind) && metadata.ProofKind != SliceRangeProofOpenBound {
			return fmt.Errorf("slice.range-normalize requires range proof provenance")
		}
		if metadata.ProofKind == SliceRangeProofOpenBound && (metadata.StartPresent || metadata.EndPresent) {
			return fmt.Errorf("source-open-bound proof requires both endpoints omitted")
		}
	} else {
		if metadata.ProofKind != "" || !typeHasKind(module.Types, op.Results[2].Type, TypeBool) || !sliceRangeErrorType(module, op.Results[3].Type) {
			return fmt.Errorf("slice.range-check requires failed bool and canonical RangeError without an assumed proof")
		}
	}
	return nil
}

// sliceRangeErrorType requires P11's canonical core enum with the three
// failure classes used by the range precedence; no BoundsError is invented.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§18–19,36,79.
func sliceRangeErrorType(module *Module, id TypeID) bool {
	typ, ok := module.Types.Lookup(id)
	if !ok || typ.Kind != TypeEnum || typ.Identity != "core::RangeError" {
		return false
	}
	for _, definition := range module.Enums {
		if definition.TypeID != id {
			continue
		}
		names := []string{"InvalidRange", "StartAfterEnd", "OutOfBounds"}
		if len(definition.Cases) != len(names) {
			return false
		}
		for index, entry := range definition.Cases {
			if entry.Name != names[index] || entry.Value == nil || !entry.Value.IsInt64() || entry.Value.Int64() != int64(index) {
				return false
			}
		}
		return true
	}
	return false
}

// formatSliceRangeOperation prints every original operand and its range
// metadata deterministically, without physical descriptor vocabulary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§35,78–79;
// rules/compiler/semantic_ir.md — §108.
func formatSliceRangeOperation(out *strings.Builder, op Operation) {
	for _, operand := range op.Operands {
		fmt.Fprintf(out, " %%%d", operand)
	}
	if m := op.SliceRange; m != nil {
		fmt.Fprintf(out, " start-present=%t end-present=%t exclusive=%t start-type=!%d start-signed=%t end-type=!%d end-signed=%t",
			m.StartPresent, m.EndPresent, m.EndExclusive, m.StartType, m.StartSigned, m.EndType, m.EndSigned)
		if m.ProofKind != "" {
			fmt.Fprintf(out, " proof=%s", m.ProofKind)
		}
	}
}

// verifySliceRangeGuards confines normalized values to the successful edge
// and RangeError to the failed edge of their exact check. Rechecking on a
// loop backedge may re-establish the proof; a bypass edge may not.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–80,83–84.
func verifySliceRangeGuards(fn *Function, blocks map[BlockID]*Block, dom map[BlockID]map[BlockID]bool) error {
	graph := map[BlockID][]BlockID{}
	uses := map[ValueID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, id := range operationOperands(op) {
				uses[id]++
			}
			for _, successor := range op.Successors {
				graph[block.ID] = append(graph[block.ID], successor.Block)
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpSliceRangeCheck {
				continue
			}
			if index+1 != len(block.Operations)-1 {
				return fmt.Errorf("slice.range-check must be immediately guarded")
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.Results[2].ID || len(branch.Successors) != 2 || uses[op.Results[2].ID] != 1 {
				return fmt.Errorf("slice.range-check failure must guard exactly one conditional branch")
			}
			failure, success := branch.Successors[0].Block, branch.Successors[1].Block
			if failure == success {
				return fmt.Errorf("slice.range-check failure and success edges must remain distinct")
			}
			for _, checked := range []struct {
				id                 ValueID
				allowed, forbidden BlockID
			}{
				{op.Results[0].ID, success, failure}, {op.Results[1].ID, success, failure}, {op.Results[3].ID, failure, success},
			} {
				for edge, successor := range branch.Successors {
					for _, arg := range successor.Arguments {
						if arg == checked.id && (edge == 0 && checked.allowed != failure || edge == 1 && checked.allowed != success) {
							return fmt.Errorf("slice range result %%%d forwarded on the wrong edge", checked.id)
						}
					}
				}
				for _, userBlock := range fn.Blocks {
					for userIndex, user := range userBlock.Operations {
						if userBlock.ID == block.ID && userIndex == index+1 {
							continue
						}
						for _, operand := range operationOperands(user) {
							if operand == checked.id && (!dom[userBlock.ID][checked.allowed] || reachableAvoiding(graph, checked.forbidden, userBlock.ID, block.ID)) {
								return fmt.Errorf("slice range result %%%d used outside its guarded edge", checked.id)
							}
						}
					}
				}
			}
		}
	}
	return nil
}
