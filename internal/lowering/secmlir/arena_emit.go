package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// arenaMnemonics maps the verified P19 Semantic IR operations to schema 15.
var arenaMnemonics = map[semantic.OpKind]string{
	semantic.OpArenaCreateBorrowed:            "sec.arena.create_borrowed",
	semantic.OpArenaCreateDefault:             "sec.arena.create_default",
	semantic.OpArenaCreateOwnedFixed:          "sec.arena.create_owned_fixed",
	semantic.OpArenaCreateGrowable:            "sec.arena.create_growable",
	semantic.OpArenaNew:                       "sec.arena.new",
	semantic.OpArenaAlloc:                     "sec.arena.alloc",
	semantic.OpArenaReset:                     "sec.arena.reset",
	semantic.OpArenaRelease:                   "sec.arena.release",
	semantic.OpArenaDestroy:                   "sec.arena.destroy",
	semantic.OpAllocationContextFromArena:     "sec.alloc_context.from_arena",
	semantic.OpAllocationContextTarget:        "sec.alloc_context.target",
	semantic.OpAllocationContextCompilerLocal: "sec.alloc_context.compiler_local",
}

// emitArena maps verified P19 Arena and allocation-context operations to
// schema 15: state versions, epochs, backing/growth policy and plan names
// become typed properties, the Arena domain and storage classification reuse
// the P18 backing properties, owned creation takes its selected context as an
// explicit operand, and New/Alloc annotate their candidate with the
// dynamic-epoch sec.reference facts. No allocator, layout or ABI is chosen.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 105-123.
func (e *emitter) emitArena(function *semantic.Function, op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if e.schemaVersion < 15 {
		return &UnsupportedLoweringError{Feature: "Arena and allocation-context operations require schema 15", Function: function.ID}
	}
	mnemonic := arenaMnemonics[op.Kind]
	operands := make([]string, 0, len(op.Operands)+1)
	for _, operand := range op.Operands {
		operands = append(operands, valueName(operand))
	}
	var properties []string
	if facts := op.AllocationContext; facts != nil && (op.Kind == semantic.OpAllocationContextFromArena || op.Kind == semantic.OpAllocationContextTarget || op.Kind == semantic.OpAllocationContextCompilerLocal) {
		properties = append(properties, "origin_plan = "+mlirString(facts.OriginPlan))
		if op.Kind == semantic.OpAllocationContextFromArena {
			properties = append(properties, fmt.Sprintf("arena_domain = %d : i32", facts.Domain))
		}
	} else {
		f := op.Arena
		if f == nil {
			return fmt.Errorf("%s in %s has no Arena facts", op.Kind, function.ID)
		}
		if op.Kind == semantic.OpArenaCreateOwnedFixed || op.Kind == semantic.OpArenaCreateGrowable {
			if op.AllocationContext == nil {
				return fmt.Errorf("%s in %s has no selected allocation context", op.Kind, function.ID)
			}
			operands = append(operands, valueName(op.AllocationContext.Value))
		}
		identity := func(name string, value uint32) {
			if value != 0 {
				properties = append(properties, fmt.Sprintf("%s = %d : i32", name, value))
			}
		}
		text := func(name, value string) {
			if value != "" {
				properties = append(properties, name+" = "+mlirString(value))
			}
		}
		identity("state_before", f.StateBefore)
		properties = append(properties, fmt.Sprintf("state_after = %d : i32", f.StateAfter))
		identity("epoch_before", f.EpochBefore)
		identity("epoch_after", f.EpochAfter)
		text("backing_kind", f.BackingKind)
		text("growth_kind", f.GrowthKind)
		text("provider_plan", f.ProviderPlan)
		text("capacity_plan", f.CapacityPlan)
		text("layout_plan", f.LayoutPlan)
		text("default_plan", f.DefaultPlan)
		text("dependency_proof", f.DependencyProof)
		identity("retained_borrow", f.RetainedBorrow)
		if f.Parent != nil {
			identity("parent_storage", f.Parent.Storage)
			identity("parent_domain", f.Parent.Reference.InvalidationDomain)
			identity("parent_epoch", f.Parent.Reference.EpochDependency)
		}
		if f.FailureAtomic {
			properties = append(properties, "failure_atomic")
		}
		if op.Kind == semantic.OpArenaReset {
			// MD-140 (v15 § 11): the verified epoch-advance facts.
			e := f.EpochAdvance
			if e.Form == "" {
				return fmt.Errorf("arena.reset in %s has no epoch-advance plan", function.ID)
			}
			text("epoch_advance", e.Form)
			properties = append(properties, fmt.Sprintf("epoch_width = %d : i32", e.LogicalEpochBits))
			text("exhaustion_reason", e.PanicReason)
			text("epoch_proof", e.Proof)
		}
		if op.Kind == semantic.OpArenaDestroy {
			if op.Ownership == nil {
				return fmt.Errorf("arena.destroy in %s has no destruction plan", function.ID)
			}
			text("destruction_plan", op.Ownership.DestructionPlan)
			text("cause", op.Ownership.Cause)
		}
		properties = append(properties, backingProperties(op.Backing)...)
	}
	if len(op.Results) != 0 {
		if err := e.emitResults(op); err != nil {
			return err
		}
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) ", mnemonic, strings.Join(operands, ", "))
	if len(properties) != 0 {
		fmt.Fprintf(&e.out, "<{%s}> ", strings.Join(properties, ", "))
	}
	if op.Reference != nil && (op.Kind == semantic.OpArenaNew || op.Kind == semantic.OpArenaAlloc) {
		if err := validateReferenceFacts(op.Reference, function.ID); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "{%s} ", referenceDictionary(op.Reference))
	}
	return e.finishArenaOperation(op, values, operands)
}

// finishArenaOperation prints the operand and result types, including the
// explicit context operand of owned Arena creation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 111-112.
func (e *emitter) finishArenaOperation(op semantic.Operation, values map[semantic.ValueID]semantic.Value, operands []string) error {
	typed := op
	if (op.Kind == semantic.OpArenaCreateOwnedFixed || op.Kind == semantic.OpArenaCreateGrowable) && op.AllocationContext != nil {
		typed.Operands = append(append([]semantic.ValueID(nil), op.Operands...), op.AllocationContext.Value)
	}
	if len(typed.Operands) != len(operands) {
		return fmt.Errorf("%s operand list disagrees with its emitted operands", op.Kind)
	}
	return e.finishTypedOperation(typed, values)
}
