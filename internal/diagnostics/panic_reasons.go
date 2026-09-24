package diagnostics

// PanicReasonID is the compiler-side representation of the source-visible
// nominal PanicID. Zero is reserved for an unresolved or unavailable reason.
type PanicReasonID uint32

const (
	PanicReasonArithmeticOverflow         PanicReasonID = 1
	PanicReasonDivisionByZero             PanicReasonID = 2
	PanicReasonInvalidShift               PanicReasonID = 3
	PanicReasonBoundsFailure              PanicReasonID = 4
	PanicReasonContractFailure            PanicReasonID = 5
	PanicReasonAssertionFailure           PanicReasonID = 6
	PanicReasonCheckedUnreachableReached  PanicReasonID = 7
	PanicReasonInvalidReferenceGeneration PanicReasonID = 8
	PanicReasonExplicitPanic              PanicReasonID = 9
	PanicReasonForeignAbort               PanicReasonID = 10
)

// PanicReasonDefinition is one stable panic registry entry. Name is a
// compiler/tooling identity and is independent of localized diagnostic text.
type PanicReasonDefinition struct {
	ID   PanicReasonID
	Name string
}

var panicReasonRegistry = [...]PanicReasonDefinition{
	{ID: PanicReasonArithmeticOverflow, Name: "ArithmeticOverflow"},
	{ID: PanicReasonDivisionByZero, Name: "DivisionByZero"},
	{ID: PanicReasonInvalidShift, Name: "InvalidShift"},
	{ID: PanicReasonBoundsFailure, Name: "BoundsFailure"},
	{ID: PanicReasonContractFailure, Name: "ContractFailure"},
	{ID: PanicReasonAssertionFailure, Name: "AssertionFailed"},
	{ID: PanicReasonCheckedUnreachableReached, Name: "UnreachableReached"},
	{ID: PanicReasonInvalidReferenceGeneration, Name: "InvalidReferenceGeneration"},
	{ID: PanicReasonExplicitPanic, Name: "ExplicitPanic"},
	{ID: PanicReasonForeignAbort, Name: "ForeignAbort"},
}

// PanicReasonByID resolves a stable numeric PanicID without allocation.
//
// Rules:
//   - rules/errors/panic.md — §13(1)–(2) "Panic information and reason IDs"
//   - rules/errors/panic.md — §14 "Allocation-free panic path"
func PanicReasonByID(id PanicReasonID) (PanicReasonDefinition, bool) {
	for _, definition := range panicReasonRegistry {
		if definition.ID == id {
			return definition, true
		}
	}
	return PanicReasonDefinition{}, false
}

// PanicReasonByName resolves the canonical symbolic identity without dynamic
// registry construction or backend-specific translation.
//
// Rules:
//   - rules/errors/panic.md — §13(1)–(2) "Panic information and reason IDs"
func PanicReasonByName(name string) (PanicReasonDefinition, bool) {
	for _, definition := range panicReasonRegistry {
		if definition.Name == name {
			return definition, true
		}
	}
	return PanicReasonDefinition{}, false
}

// PanicReasonDefinitions returns a snapshot for verification and tooling.
func PanicReasonDefinitions() []PanicReasonDefinition {
	return append([]PanicReasonDefinition(nil), panicReasonRegistry[:]...)
}
