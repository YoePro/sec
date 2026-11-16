package sema

// ArenaFailureCondition is a semantic allocation failure category, independent
// of provider-specific error codes, machine width and physical Arena layout.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–33.
type ArenaFailureCondition string

const (
	ArenaFailureExhausted        ArenaFailureCondition = "provider-or-capacity-exhausted"
	ArenaFailureUnsupported      ArenaFailureCondition = "operation-unsupported"
	ArenaFailureInvalidExtent    ArenaFailureCondition = "invalid-count-size-or-extent"
	ArenaFailureInvalidAlignment ArenaFailureCondition = "alignment-unsatisfied"
)

// ArenaFailureMapping maps one canonical condition to an existing core error
// variant; it introduces no Arena-specific error type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–33.
type ArenaFailureMapping struct {
	Condition ArenaFailureCondition
	Variant   string
}

// ArenaFailureState distinguishes failed owner creation from failed mutation of
// an existing owner. Neither publishes partial objects or changes an epoch,
// cursor, existing allocation or backing segment. The state is the failure
// outcome of this operation, not a restriction on later source error handlers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §35.
type ArenaFailureState string

const (
	ArenaFailureNoDomain       ArenaFailureState = "no-domain-established"
	ArenaFailureUnchangedOwner ArenaFailureState = "owner-and-allocations-unchanged"
)

// ArenaFailurePlan is the resolved error/result/publication contract consumed
// by later builders. ResultType is the unchanged source Result, including when
// count/capacity is zero or later capacity/provider proofs remove a branch.
// Mappings and StateOnFailure are semantic contracts, not completed provider
// or initialization code; those producers remain separate P19 work.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–35,72–73.
type ArenaFailurePlan struct {
	ResultType     Type
	Mappings       []ArenaFailureMapping
	StateOnFailure ArenaFailureState
}

// ErrorVariant returns only a recorded mapping. An unknown condition does not
// silently fall back to OutOfMemory or manufacture a provider-specific error.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–33.
func (plan ArenaFailurePlan) ErrorVariant(condition ArenaFailureCondition) (string, bool) {
	for _, mapping := range plan.Mappings {
		if mapping.Condition == condition {
			return mapping.Variant, true
		}
	}
	return "", false
}

// arenaFailurePlan records the normative mapping for fallible Arena creation
// and allocation. Infallible FromBuffer/Reset/Release have no error plan. It
// does not narrow Result by constant count, size, capacity or target evidence.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–35.
func arenaFailurePlan(operation ArenaOperationKind, fallible bool, result Type) *ArenaFailurePlan {
	if !fallible {
		return nil
	}
	state := ArenaFailureUnchangedOwner
	if operation == ArenaOperationCreate {
		state = ArenaFailureNoDomain
	}
	return &ArenaFailurePlan{ResultType: semanticSnapshotType(result), StateOnFailure: state, Mappings: []ArenaFailureMapping{
		{ArenaFailureExhausted, "OutOfMemory"},
		{ArenaFailureUnsupported, "Unsupported"},
		{ArenaFailureInvalidExtent, "InvalidSize"},
		{ArenaFailureInvalidAlignment, "InvalidAlignment"},
	}}
}

// cloneArenaFailurePlan detaches all mutable result/error type and mapping
// facts; reading a public operation plan cannot rewrite another query.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §76;
// rules/compiler/compiler_analysis.md — immutable analysis results.
func cloneArenaFailurePlan(plan *ArenaFailurePlan) *ArenaFailurePlan {
	if plan == nil {
		return nil
	}
	copy := *plan
	copy.ResultType = semanticSnapshotType(plan.ResultType)
	copy.Mappings = append([]ArenaFailureMapping(nil), plan.Mappings...)
	return &copy
}
