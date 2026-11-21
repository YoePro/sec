package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// TypeAtomic is the compiler-owned `Atomic[T]` object type; Element is T. It
// is only ever reached through a reference operand: an atomic object is
// identified by its storage, never copied as a value.
// Rules: rules/concurrency/atomics.md — §§ 4, 7; rules/compiler/semantic_ir.md — §§ 122, 123A.
const TypeAtomic TypeKind = "atomic"

// Atomic object operations (semantic_ir.md § 123A). Operand 0 is always a
// reference to the Atomic[T] object, which carries its storage identity.
const (
	OpAtomicLoad            OpKind = "atomic.load"
	OpAtomicStore           OpKind = "atomic.store"
	OpAtomicReadModifyWrite OpKind = "atomic.rmw"
	OpAtomicCompareExchange OpKind = "atomic.compare-exchange"
)

// AtomicPathFacts is one normal outcome of an atomic object operation with its
// memory-model classification, copied from Sema's validated plan. Only a
// read-modify-write outcome reads the immediately preceding modification; a
// failed compare-exchange is an ordinary read.
// Rules: rules/concurrency/concurrency_memory_model.md — §§ 16-17, 24, 52; rules/concurrency/atomics.md — §§ 17, 24-25.
type AtomicPathFacts struct {
	Variant                  string `json:"variant,omitempty"`
	Order                    string `json:"order"`
	ReadsStorage             bool   `json:"reads_storage"`
	ModifiesStorage          bool   `json:"modifies_storage"`
	ReadModifyWrite          bool   `json:"read_modify_write"`
	StartsReleaseHead        bool   `json:"starts_release_head"`
	ContinuesReleaseSequence bool   `json:"continues_release_sequence"`
	BreaksReleaseSequence    bool   `json:"breaks_release_sequence"`
}

// AtomicObjectFacts identifies one operation on an Atomic[T] object: the
// source operation name, T, the success and (compare-exchange) failure orders,
// each outcome path and whether Sema resolved the storage to a canonical
// Place. An unresolved storage stays Unknown and may alias any atomic object.
// Rules: rules/compiler/semantic_ir.md — §§ 122, 123A; rules/concurrency/atomics.md — §§ 12-25, 45-46.
type AtomicObjectFacts struct {
	Operation       string            `json:"operation"`
	Element         TypeID            `json:"element"`
	Order           string            `json:"order"`
	FailureOrder    string            `json:"failure_order,omitempty"`
	StorageResolved bool              `json:"storage_resolved"`
	Paths           []AtomicPathFacts `json:"paths"`
}

// atomicReadModifyWriteOperations are the RMW source operations.
var atomicReadModifyWriteOperations = map[string]bool{"Swap": true, "FetchAdd": true, "FetchSub": true, "FetchAnd": true, "FetchOr": true, "FetchXor": true}

// atomicOrderAllowed is the operation/order matrix of atomics.md § 24(1).
func atomicOrderAllowed(category, order string) bool {
	switch category {
	case "Load", "failure":
		return order == "Relaxed" || order == "Acquire" || order == "SeqCst"
	case "Store":
		return order == "Relaxed" || order == "Release" || order == "SeqCst"
	case "rmw":
		return order == "Relaxed" || order == "Acquire" || order == "Release" || order == "AcqRel" || order == "SeqCst"
	}
	return false
}

// buildAtomicObjectCall builds an Atomic[T] object operation from Sema's
// validated memory-operation plan. The receiver must already be a reference
// to the object; borrowing a static or local atomic object for the call is
// not represented yet and fails closed.
// Rules: rules/compiler/semantic_ir.md — §§ 122, 123A; rules/concurrency/atomics.md — §§ 12-25.
func (fb *functionBuilder) buildAtomicObjectCall(call *ast.CallExpression) (builtValue, bool, error) {
	plan, ok := fb.owner.analyzer.ResolvedAtomicMemoryOperationOf(call)
	if !ok {
		return builtValue{}, false, nil
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return builtValue{}, true, fb.unsupported("atomic object operation without a member receiver", call.Token)
	}
	site := member.Property.Token
	if fb.owner.maxPackage < 15 {
		return builtValue{}, true, fb.unsupported("atomic object operations need package-15 references", site)
	}
	receiverType, typed := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	if !typed || receiverType.Kind != sema.ReferenceType || receiverType.Element == nil {
		return builtValue{}, true, fb.unsupported("atomic object operation on a receiver that is not a reference to its Atomic[T] object", site)
	}
	elementType, isAtomic := sema.AtomicElementType(*receiverType.Element)
	if !isAtomic {
		return builtValue{}, true, fb.unsupported("atomic object operation on a non-atomic receiver", site)
	}
	receiverID, err := fb.owner.internType(receiverType)
	if err != nil {
		return builtValue{}, true, err
	}
	element, err := fb.owner.internType(elementType)
	if err != nil {
		return builtValue{}, true, err
	}
	receiver, err := fb.buildExpr(member.Object, receiverID)
	if err != nil {
		return builtValue{}, true, err
	}
	facts := &AtomicObjectFacts{Operation: plan.Operation, Element: element, StorageResolved: plan.StorageResolved}
	for _, path := range plan.Paths {
		facts.Paths = append(facts.Paths, AtomicPathFacts{Variant: path.Variant, Order: path.Order, ReadsStorage: path.ReadsStorage,
			ModifiesStorage: path.ModifiesStorage, ReadModifyWrite: path.ReadModifyWrite, StartsReleaseHead: path.ReleaseSequence.StartsHead,
			ContinuesReleaseSequence: path.ReleaseSequence.ContinuesPredecessorHeads, BreaksReleaseSequence: path.ReleaseSequence.BreaksPredecessorHeads})
	}
	if len(facts.Paths) == 0 {
		return builtValue{}, true, fb.unsupported("atomic object operation without a validated order", site)
	}
	facts.Order = facts.Paths[0].Order
	operation := Operation{Operands: []ValueID{receiver.id}, Atomic: facts, Location: location(site)}
	argument := func(index int) (ValueID, error) {
		value, err := fb.buildExpr(call.Arguments[index], element)
		return value.id, err
	}
	switch {
	case plan.Operation == "Load":
		operation.Kind = OpAtomicLoad
		return fb.result(operation, element), true, nil
	case plan.Operation == "Store":
		operation.Kind = OpAtomicStore
		value, err := argument(0)
		if err != nil {
			return builtValue{}, true, err
		}
		operation.Operands = append(operation.Operands, value)
		fb.emit(operation)
		return builtValue{}, true, nil
	case atomicReadModifyWriteOperations[plan.Operation]:
		operation.Kind = OpAtomicReadModifyWrite
		value, err := argument(0)
		if err != nil {
			return builtValue{}, true, err
		}
		operation.Operands = append(operation.Operands, value)
		return fb.result(operation, element), true, nil
	case plan.Operation == "CompareExchange":
		cas, ok := fb.owner.analyzer.ResolvedCompareExchangeOf(call)
		if !ok || len(facts.Paths) != 2 {
			return builtValue{}, true, fb.unsupported("compare-exchange without its resolved outcome paths", site)
		}
		resultType, err := fb.owner.internType(cas.ResultType)
		if err != nil {
			return builtValue{}, true, err
		}
		operation.Kind = OpAtomicCompareExchange
		facts.FailureOrder = facts.Paths[1].Order
		for index := 0; index < 2; index++ {
			value, err := argument(index)
			if err != nil {
				return builtValue{}, true, err
			}
			operation.Operands = append(operation.Operands, value)
		}
		return fb.result(operation, resultType), true, nil
	}
	return builtValue{}, true, fb.unsupported("atomic object operation "+plan.Operation, site)
}

// atomicReferenceElement returns T when id is a reference to Atomic[T].
// Rules: rules/compiler/semantic_ir.md — § 123A(2).
func atomicReferenceElement(types *TypeTable, id TypeID) (TypeID, bool) {
	reference, ok := types.Lookup(id)
	if !ok || reference.Kind != TypeReference {
		return 0, false
	}
	object, ok := types.Lookup(reference.Element)
	if !ok || object.Kind != TypeAtomic {
		return 0, false
	}
	return object.Element, true
}

// verifyAtomicObjectOperation checks the shape, T, orders and the outcome
// classification of one atomic object operation: a Load only reads, a Store
// only modifies and breaks release sequences, an RMW reads and modifies, and
// a compare-exchange has a successful RMW path and a read-only failure path.
// Rules: rules/compiler/semantic_ir.md — §§ 122, 123A; rules/concurrency/atomics.md — §§ 17, 20, 24-25.
func verifyAtomicObjectOperation(module *Module, op Operation, values map[ValueID]Value) error {
	facts := op.Atomic
	if facts == nil || len(op.Successors) != 0 || len(op.Operands) == 0 {
		return fmt.Errorf("invalid %s: missing atomic facts or receiver", op.Kind)
	}
	element, ok := atomicReferenceElement(module.Types, values[op.Operands[0]].Type)
	if !ok || element != facts.Element {
		return fmt.Errorf("invalid %s: operand 0 is not a reference to Atomic[!%d]", op.Kind, facts.Element)
	}
	for _, operand := range op.Operands[1:] {
		if values[operand].Type != element {
			return fmt.Errorf("invalid %s: value operand is not the atomic element type", op.Kind)
		}
	}
	path := func(index int, reads, modifies, rmw bool) error {
		if index >= len(facts.Paths) {
			return fmt.Errorf("invalid %s: missing outcome path %d", op.Kind, index)
		}
		p := facts.Paths[index]
		if p.ReadsStorage != reads || p.ModifiesStorage != modifies || p.ReadModifyWrite != rmw {
			return fmt.Errorf("invalid %s: outcome %d has the wrong read/modify/RMW classification", op.Kind, index)
		}
		if !modifies && (p.StartsReleaseHead || p.ContinuesReleaseSequence || p.BreaksReleaseSequence) {
			return fmt.Errorf("invalid %s: a read-only outcome cannot affect a release sequence", op.Kind)
		}
		return nil
	}
	shape := func(operands, results, paths int) error {
		if len(op.Operands) != operands || len(op.Results) != results || len(facts.Paths) != paths {
			return fmt.Errorf("invalid %s shape", op.Kind)
		}
		return nil
	}
	switch op.Kind {
	case OpAtomicLoad:
		if err := shape(1, 1, 1); err != nil {
			return err
		}
		if facts.Operation != "Load" || op.Results[0].Type != element || !atomicOrderAllowed("Load", facts.Order) || facts.FailureOrder != "" {
			return fmt.Errorf("invalid atomic.load operation, result or order %q", facts.Order)
		}
		return path(0, true, false, false)
	case OpAtomicStore:
		if err := shape(2, 0, 1); err != nil {
			return err
		}
		if facts.Operation != "Store" || !atomicOrderAllowed("Store", facts.Order) || facts.FailureOrder != "" || !facts.Paths[0].BreaksReleaseSequence {
			return fmt.Errorf("invalid atomic.store operation, order %q or release-sequence effect", facts.Order)
		}
		return path(0, false, true, false)
	case OpAtomicReadModifyWrite:
		if err := shape(2, 1, 1); err != nil {
			return err
		}
		if !atomicReadModifyWriteOperations[facts.Operation] || op.Results[0].Type != element || !atomicOrderAllowed("rmw", facts.Order) || facts.FailureOrder != "" {
			return fmt.Errorf("invalid atomic.rmw operation %q, result or order %q", facts.Operation, facts.Order)
		}
		return path(0, true, true, true)
	case OpAtomicCompareExchange:
		if err := shape(3, 1, 2); err != nil {
			return err
		}
		if facts.Operation != "CompareExchange" || !atomicOrderAllowed("rmw", facts.Order) || !atomicOrderAllowed("failure", facts.FailureOrder) ||
			facts.Paths[0].Order != facts.Order || facts.Paths[1].Order != facts.FailureOrder ||
			facts.Paths[0].Variant != "Exchanged" || facts.Paths[1].Variant != "NotExchanged" {
			return fmt.Errorf("invalid atomic.compare-exchange orders %q/%q or outcome variants", facts.Order, facts.FailureOrder)
		}
		if err := path(0, true, true, true); err != nil {
			return err
		}
		return path(1, true, false, false)
	}
	return fmt.Errorf("unknown atomic object operation %s", op.Kind)
}

// isAtomicObjectOperation names the four § 123A operations.
// Rules: rules/compiler/semantic_ir.md — § 123A(1).
func isAtomicObjectOperation(kind OpKind) bool {
	return kind == OpAtomicLoad || kind == OpAtomicStore || kind == OpAtomicReadModifyWrite || kind == OpAtomicCompareExchange
}
