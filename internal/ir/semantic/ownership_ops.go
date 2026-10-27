package semantic

import (
	"fmt"
	"strings"
)

// Semantic IR ownership operations (P17-26 … P17-29; package17 §§ 48–58;
// dialect v13 §§ 3–13). They make copy, move, place initialization and
// replacement, discard and destruction explicit, each with its resolved
// copy mode or type-level destruction plan. Borrowing stays the P15/P16
// operations; cleanup registration is separate (P17-33). The verifier checks
// each operation's contract and, inside a block, that a consumed value is
// not used again; the path-sensitive ownership verifier is P17-52.
const (
	OpOwnCopy            OpKind = "own.copy"
	OpOwnSemanticCopy    OpKind = "own.semantic-copy"
	OpOwnMove            OpKind = "own.move"
	OpOwnCopyFromPlace   OpKind = "own.copy-from-place"
	OpOwnMoveFromPlace   OpKind = "own.move-from-place"
	OpOwnInitializePlace OpKind = "own.initialize-place"
	OpOwnReplacePlace    OpKind = "own.replace-place"
	OpOwnDiscard         OpKind = "own.discard"
	OpOwnDestroyValue    OpKind = "own.destroy-value"
	OpOwnDestroyPlace    OpKind = "own.destroy-place"
)

// Copy modes of own.copy-from-place (dialect v13 § 7).
const (
	OwnCopyTrivial            = "trivial"
	OwnCopySemanticInfallible = "semantic-infallible"
)

// Destruction causes (package17 § 57) and discard origins (§ 56).
var (
	ownDestroyCauses  = []string{"scope-exit", "function-exit", "temporary-end", "cleanup", "compiler-generated"}
	ownDiscardOrigins = []string{"explicit", "implicit-call-result", "compiler-temporary"}
)

// OwnershipFacts are the resolved facts an own.* operation carries.
type OwnershipFacts struct {
	// CopyMode is the own.copy-from-place mode.
	CopyMode string
	// SemanticCopyPlan names the resolved infallible semantic-copy plan of
	// own.semantic-copy and semantic copy-from-place.
	SemanticCopyPlan string
	// DestructionPlan is the type-level plan a destroy, a non-trivial
	// discard or the old value of a replacement uses.
	DestructionPlan string
	// Cause is the reason of a destroy; DiscardOrigin of a discard.
	Cause         string
	DiscardOrigin string
	// OldCleanup and NewCleanup are the cleanup action identities a
	// replacement terminates and installs (dialect v13 § 10).
	OldCleanup, NewCleanup uint32
	// Skipped lists the subobjects of an own.destroy-place destination that
	// were moved out and are not destroyed (package17 §§ 12, 14, 23, 59).
	Skipped []OwnedPath
}

// DestructionPlanRecord is one module-level type-level destruction plan
// (package17 §§ 20–26): its deterministic identity, the type it destroys and
// its kind (trivial, struct, fixed-array, active-variant,
// compiler-known-resource), with the derived steps the kind needs. The
// records form a closed graph checked by verifyDestructionPlanGraph
// (ownership_state.go); no physical layout is encoded.
type DestructionPlanRecord struct {
	ID   string
	Type TypeID
	Kind string
	// CustomFree runs the struct's `free` before its remaining fields;
	// FreeFunction is that built lifecycle function and FreeType the source
	// type it belongs to.
	CustomFree   bool
	FreeFunction FunctionID
	FreeType     string
	// Fields destroy remaining owned stored fields, in execution order
	// (reverse declaration order).
	Fields []DestructionFieldRecord
	// Element and ElementCount destroy a fixed array compactly: one element
	// plan and the exact count, in reverse index order.
	Element      string
	ElementCount string
	// Variants destroy only the active, initialized, unmoved payload.
	Variants []DestructionVariantRecord
	// Resource names a compiler-known resource's release contract.
	Resource string
}

// DestructionFieldRecord destroys one owned stored field.
type DestructionFieldRecord struct {
	Field StructFieldID
	Plan  string
}

// DestructionVariantRecord destroys the payload of one variant when active.
type DestructionVariantRecord struct {
	Variant UnionVariantIndex
	Plan    string
}

func isOwnershipOperation(kind OpKind) bool {
	switch kind {
	case OpOwnCopy, OpOwnSemanticCopy, OpOwnMove, OpOwnCopyFromPlace, OpOwnMoveFromPlace,
		OpOwnInitializePlace, OpOwnReplacePlace, OpOwnDiscard, OpOwnDestroyValue, OpOwnDestroyPlace:
		return true
	}
	return false
}

// consumesOwnership reports whether the operation consumes operand index.
func consumesOwnership(op Operation, index int) bool {
	switch op.Kind {
	case OpOwnMove, OpOwnDiscard, OpOwnDestroyValue:
		return index == 0
	case OpOwnInitializePlace, OpOwnReplacePlace:
		return index == 1
	}
	return false
}

// ownedCopyClass classifies a Semantic IR type for copying: "trivial",
// "semantic" or "move-only".
func ownedCopyClass(module *Module, id TypeID) string {
	typ, ok := module.Types.Lookup(id)
	if !ok {
		return "move-only"
	}
	switch typ.Kind {
	case TypeNamed:
		return ownedCopyClass(module, typ.Base)
	case TypeMutableSlice:
		return "move-only"
	case TypeReference:
		if typ.Mutable {
			return "move-only"
		}
		return "trivial"
	case TypeArray:
		return ownedCopyClass(module, typ.Element)
	case TypeStruct:
		if definition, found := structDefinition(module, id); found {
			return definition.CopyClassification
		}
		return "move-only"
	case TypeUnion:
		for _, definition := range module.Unions {
			if definition.TypeID == id {
				return definition.CopyClassification
			}
		}
		return "move-only"
	case TypeResult:
		success, failure := ownedCopyClass(module, typ.Success), ownedCopyClass(module, typ.Error)
		if success == "move-only" || failure == "move-only" {
			return "move-only"
		}
		if success == "semantic" || failure == "semantic" {
			return "semantic"
		}
		return "trivial"
	}
	// Scalars and strings are copy-trivial, as Sema classifies them.
	return "trivial"
}

// destructionPlan finds the module plan id and checks it destroys typ.
func destructionPlan(module *Module, id string, typ TypeID) (DestructionPlanRecord, error) {
	if id == "" {
		return DestructionPlanRecord{}, fmt.Errorf("missing destruction plan")
	}
	for _, plan := range module.DestructionPlans {
		if plan.ID == id {
			if plan.Type != typ {
				return DestructionPlanRecord{}, fmt.Errorf("destruction plan %q destroys !%d, not !%d", id, plan.Type, typ)
			}
			return plan, nil
		}
	}
	return DestructionPlanRecord{}, fmt.Errorf("unknown destruction plan %q", id)
}

func isBorrowValueType(module *Module, id TypeID) bool {
	return typeHasKind(module.Types, id, TypeReference, TypeSharedSlice, TypeMutableSlice)
}

func oneOf(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// verifyDestructionPlans checks the module table: unique identities naming
// known types.
func verifyDestructionPlans(module *Module) error {
	seen := map[string]bool{}
	for _, plan := range module.DestructionPlans {
		if plan.ID == "" || plan.Kind == "" || seen[plan.ID] {
			return fmt.Errorf("invalid or duplicate destruction plan %q", plan.ID)
		}
		if _, ok := module.Types.Lookup(plan.Type); !ok {
			return fmt.Errorf("destruction plan %q names unknown type !%d", plan.ID, plan.Type)
		}
		if plan.FreeFunction != "" && !customFreeFunctionOf(module, plan) {
			return fmt.Errorf("destruction plan %q names %q, which is not a custom free over ref mut of its type", plan.ID, plan.FreeFunction)
		}
		seen[plan.ID] = true
	}
	return verifyDestructionPlanGraph(module)
}

// verifyOwnershipOperation checks one own.* operation's local contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§48–58,
// especially §54 (initialization has no live old value to destroy) and §55
// (replacement owns old destruction and a new ownership incarnation).
func verifyOwnershipOperation(module *Module, op Operation, values map[ValueID]Value) error {
	facts := op.Ownership
	if facts == nil {
		return fmt.Errorf("%s has no ownership facts", op.Kind)
	}
	place := func(id ValueID) (Type, error) {
		typ, ok := module.Types.Lookup(values[id].Type)
		if !ok || typ.Kind != TypePlace {
			return Type{}, fmt.Errorf("%s requires a Place operand", op.Kind)
		}
		return typ, nil
	}
	shape := func(operands, results int) error {
		if len(op.Operands) != operands || len(op.Results) != results || len(op.Successors) != 0 {
			return fmt.Errorf("%s requires %d operand(s) and %d result(s)", op.Kind, operands, results)
		}
		return nil
	}
	switch op.Kind {
	case OpOwnCopy, OpOwnSemanticCopy, OpOwnMove:
		if err := shape(1, 1); err != nil {
			return err
		}
		source := values[op.Operands[0]].Type
		if op.Results[0].Type != source {
			return fmt.Errorf("%s must keep its value type", op.Kind)
		}
		class := ownedCopyClass(module, source)
		switch op.Kind {
		case OpOwnCopy:
			if class != "trivial" {
				return fmt.Errorf("own.copy requires a copy-trivial type, got %s", class)
			}
		case OpOwnSemanticCopy:
			if class != "semantic" || facts.SemanticCopyPlan == "" {
				return fmt.Errorf("own.semantic-copy requires a resolved infallible semantic-copy plan")
			}
		}
	case OpOwnCopyFromPlace, OpOwnMoveFromPlace:
		if err := shape(1, 1); err != nil {
			return err
		}
		source, err := place(op.Operands[0])
		if err != nil {
			return err
		}
		if op.Results[0].Type != source.Element {
			return fmt.Errorf("%s result must be the Place element type", op.Kind)
		}
		if op.Kind == OpOwnMoveFromPlace {
			if !source.Mutable {
				return fmt.Errorf("own.move-from-place requires a writable Place")
			}
			break
		}
		class := ownedCopyClass(module, source.Element)
		switch facts.CopyMode {
		case OwnCopyTrivial:
			if class != "trivial" {
				return fmt.Errorf("trivial own.copy-from-place of a %s type", class)
			}
		case OwnCopySemanticInfallible:
			if class != "semantic" || facts.SemanticCopyPlan == "" {
				return fmt.Errorf("semantic own.copy-from-place requires a resolved infallible semantic-copy plan")
			}
		default:
			return fmt.Errorf("own.copy-from-place has unknown copy mode %q", facts.CopyMode)
		}
	case OpOwnInitializePlace, OpOwnReplacePlace:
		if err := shape(2, 0); err != nil {
			return err
		}
		destination, err := place(op.Operands[0])
		if err != nil {
			return err
		}
		if !destination.Mutable || values[op.Operands[1]].Type != destination.Element {
			return fmt.Errorf("%s requires a writable Place of the value's type", op.Kind)
		}
		if op.Kind == OpOwnInitializePlace && (facts.DestructionPlan != "" || facts.OldCleanup != 0) {
			return fmt.Errorf("own.initialize-place must not destroy an old value or cancel an old cleanup action")
		}
		if op.Kind == OpOwnReplacePlace {
			plan, err := destructionPlan(module, facts.DestructionPlan, destination.Element)
			if err != nil {
				return fmt.Errorf("own.replace-place old value: %w", err)
			}
			if plan.Kind != "trivial" && (facts.OldCleanup == 0 || facts.NewCleanup == 0 || facts.OldCleanup == facts.NewCleanup) {
				return fmt.Errorf("own.replace-place requires distinct old and new cleanup actions")
			}
		}
	case OpOwnDiscard:
		if err := shape(1, 0); err != nil {
			return err
		}
		if !oneOf(facts.DiscardOrigin, ownDiscardOrigins) {
			return fmt.Errorf("own.discard has unknown origin %q", facts.DiscardOrigin)
		}
		source := values[op.Operands[0]].Type
		if isBorrowValueType(module, source) {
			// Discarding a reference/slice never destroys its referent
			// (dialect v13 § 31).
			if facts.DestructionPlan != "" {
				return fmt.Errorf("own.discard of a reference/slice must not destroy its referent")
			}
			break
		}
		if facts.DestructionPlan != "" {
			if _, err := destructionPlan(module, facts.DestructionPlan, source); err != nil {
				return fmt.Errorf("own.discard: %w", err)
			}
		} else if ownedCopyClass(module, source) != "trivial" {
			return fmt.Errorf("own.discard of a non-trivial value requires its destruction plan")
		}
	case OpOwnDestroyValue, OpOwnDestroyPlace:
		if err := shape(1, 0); err != nil {
			return err
		}
		if !oneOf(facts.Cause, ownDestroyCauses) {
			return fmt.Errorf("%s has unknown cause %q", op.Kind, facts.Cause)
		}
		destroyed := values[op.Operands[0]].Type
		if op.Kind == OpOwnDestroyPlace {
			destination, err := place(op.Operands[0])
			if err != nil {
				return err
			}
			if !destination.Mutable {
				return fmt.Errorf("own.destroy-place requires a writable Place")
			}
			destroyed = destination.Element
		} else if isBorrowValueType(module, destroyed) {
			return fmt.Errorf("own.destroy-value of a reference/slice: borrow holders end with ref.end-borrow")
		}
		if _, err := destructionPlan(module, facts.DestructionPlan, destroyed); err != nil {
			return fmt.Errorf("%s: %w", op.Kind, err)
		}
	}
	return nil
}

// verifyOwnershipFlow rejects a use of an SSA value after an own.*
// operation consumed it in the same block, and a second consumption.
func verifyOwnershipFlow(module *Module, fn *Function) error {
	for _, block := range fn.Blocks {
		consumed := map[ValueID]OpKind{}
		for _, op := range block.Operations {
			for index, operand := range op.Operands {
				if by, gone := consumed[operand]; gone {
					return fmt.Errorf("%%%d is used by %s after %s consumed it", operand, op.Kind, by)
				}
				arrayTransfer := op.Kind == OpArrayConstruct && index < len(op.ArraySegmentKinds) && op.ArraySegmentKinds[index] == ArraySegmentElement &&
					(op.ArrayActions[index] == ArrayActionMove || op.ArrayActions[index] == ArrayActionConstructDirect && ArrayConstructionNeedsOwnership(module, op))
				if consumesOwnership(op, index) || arrayTransfer {
					consumed[operand] = op.Kind
				}
			}
			for _, target := range op.Successors {
				for _, argument := range target.Arguments {
					if by, gone := consumed[argument]; gone {
						return fmt.Errorf("%%%d is passed to ^%d after %s consumed it", argument, target.Block, by)
					}
				}
			}
		}
	}
	return nil
}

func formatOwnershipOperation(out *strings.Builder, op Operation) {
	for index, operand := range op.Operands {
		if index > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, " %%%d", operand)
	}
	facts := op.Ownership
	if facts == nil {
		return
	}
	if facts.CopyMode != "" {
		fmt.Fprintf(out, " mode=%s", facts.CopyMode)
	}
	if facts.SemanticCopyPlan != "" {
		fmt.Fprintf(out, " copy-plan=%q", facts.SemanticCopyPlan)
	}
	if facts.DestructionPlan != "" {
		fmt.Fprintf(out, " plan=%q", facts.DestructionPlan)
	}
	if facts.Cause != "" {
		fmt.Fprintf(out, " cause=%s", facts.Cause)
	}
	if facts.DiscardOrigin != "" {
		fmt.Fprintf(out, " origin=%s", facts.DiscardOrigin)
	}
	if facts.OldCleanup != 0 || facts.NewCleanup != 0 {
		fmt.Fprintf(out, " cleanup=#%d->#%d", facts.OldCleanup, facts.NewCleanup)
	}
	if len(facts.Skipped) != 0 {
		paths := make([]string, len(facts.Skipped))
		for index, path := range facts.Skipped {
			paths[index] = path.String()
		}
		fmt.Fprintf(out, " skip=[%s]", strings.Join(paths, ", "))
	}
}

func formatDestructionPlans(out *strings.Builder, plans []DestructionPlanRecord) {
	if len(plans) == 0 {
		return
	}
	out.WriteString("\n  destruction-plans {\n")
	for _, plan := range plans {
		fmt.Fprintf(out, "    %q !%d %s", plan.ID, plan.Type, plan.Kind)
		if plan.CustomFree {
			out.WriteString(" custom-free")
			if plan.FreeFunction != "" {
				fmt.Fprintf(out, "=%q", plan.FreeFunction)
			}
		}
		for _, field := range plan.Fields {
			fmt.Fprintf(out, " field#%d=%q", field.Field, field.Plan)
		}
		if plan.Element != "" {
			fmt.Fprintf(out, " element=%q count=%s", plan.Element, plan.ElementCount)
		}
		for _, variant := range plan.Variants {
			fmt.Fprintf(out, " variant#%d=%q", variant.Variant, variant.Plan)
		}
		if plan.Resource != "" {
			fmt.Fprintf(out, " resource=%q", plan.Resource)
		}
		out.WriteByte('\n')
	}
	out.WriteString("  }\n")
}

// customFreeFunctionOf reports whether a plan's free function exists and
// takes exactly a mutable reference to the destroyed type and returns void.
func customFreeFunctionOf(module *Module, plan DestructionPlanRecord) bool {
	for _, fn := range module.Functions {
		if fn.ID != plan.FreeFunction || len(fn.Parameters) != 1 {
			continue
		}
		receiver, ok := module.Types.Lookup(fn.Parameters[0].Value.Type)
		return ok && receiver.Kind == TypeReference && receiver.Mutable && receiver.Element == plan.Type && typeHasKind(module.Types, fn.ReturnType, TypeVoid)
	}
	return false
}
