package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ArenaCapacitySource distinguishes a runtime byte length/request from missing
// backing evidence. A nil exact count never means zero capacity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§9–20,70–76.
type ArenaCapacitySource string

const (
	ArenaCapacityUnknown ArenaCapacitySource = "unknown-backing"
	ArenaCapacityBuffer  ArenaCapacitySource = "borrowed-byte-length"
	ArenaCapacityRequest ArenaCapacitySource = "constructor-byte-request"
)

// ArenaCapacityFacts retains initial capacity evidence, not remaining capacity
// or a proof that an allocation fits. RequestedBytes is the constructor request;
// BackingBytes is an exact supplied backing extent when already established by
// canonical source facts. Provider rounding/extra capacity is never guessed.
// These counts are bytes, independently of New/Alloc's element counts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§12–20,39,70–76;
// rules/memory/arena.md — §§10,12–13,17,40.
type ArenaCapacityFacts struct {
	Source         ArenaCapacitySource
	SourceToken    lexer.Token
	RequestedBytes *big.Int
	BackingBytes   *big.Int
	GrowthAllowed  bool
	// Reset retains borrowed backing or acquired stable provider segments.
	RetainsBackingOnReset bool
}

// arenaCapacityFacts consumes already-validated constructor operands and P14/P16
// size/range evidence. Later operations inherit initial facts by stable domain;
// owner moves and Reset do not create a different capacity source. Parameters
// with no represented creation remain unknown. No provider or T layout is chosen.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§12–20,39,72–76;
// rules/memory/arena.md — §40; rules/mlir/packages/sec-mlir-dialect_package16.md — §36.
func (a *Analyzer) arenaCapacityFacts(call *ast.CallExpression, plan ResolvedArenaOperationPlan) ArenaCapacityFacts {
	if plan.Operation != ArenaOperationCreate {
		for _, creation := range a.arenaOperationPlans {
			if creation.Operation == ArenaOperationCreate && creation.Domain == plan.Domain {
				return cloneArenaCapacityFacts(creation.Capacity)
			}
		}
		return ArenaCapacityFacts{Source: ArenaCapacityUnknown}
	}
	facts := ArenaCapacityFacts{SourceToken: expressionToken(call.Arguments[0]),
		RetainsBackingOnReset: true, GrowthAllowed: plan.Constructor == "Growable"}
	if plan.Constructor != "FromBuffer" {
		facts.Source = ArenaCapacityRequest
		facts.RequestedBytes = clonePlaceConstantIndex(plan.Count)
		return facts
	}
	facts.Source = ArenaCapacityBuffer
	argument := call.Arguments[0]
	typ := a.expressionTypes[argument]
	if typ.Kind == ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	if length, fixed := FixedArrayLength(typ); fixed {
		facts.BackingBytes = length // FromBuffer has already checked byte storage.
	} else if place, known := a.sliceSourcePlace(argument, a.expressionTypes[argument]); known {
		facts.BackingBytes = staticSliceExtent(place)
	}
	return facts
}

// cloneArenaCapacityFacts detaches exact byte counts for immutable plan queries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§19,76.
func cloneArenaCapacityFacts(facts ArenaCapacityFacts) ArenaCapacityFacts {
	facts.RequestedBytes = clonePlaceConstantIndex(facts.RequestedBytes)
	facts.BackingBytes = clonePlaceConstantIndex(facts.BackingBytes)
	return facts
}
