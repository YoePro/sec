package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

var ownershipOperationNames = map[semantic.OpKind]string{
	semantic.OpOwnCopy: "sec.own.copy", semantic.OpOwnSemanticCopy: "sec.own.semantic_copy", semantic.OpOwnMove: "sec.own.move",
	semantic.OpOwnCopyFromPlace: "sec.own.copy_from_place", semantic.OpOwnMoveFromPlace: "sec.own.move_from_place",
	semantic.OpOwnInitializePlace: "sec.own.initialize_place", semantic.OpOwnReplacePlace: "sec.own.replace_place",
	semantic.OpOwnDiscard: "sec.own.discard", semantic.OpOwnDestroyValue: "sec.own.destroy_value", semantic.OpOwnDestroyPlace: "sec.own.destroy_place",
	semantic.OpCleanupTrackOwned: "sec.cleanup.track_owned", semantic.OpCleanupCancel: "sec.cleanup.cancel",
	semantic.OpCleanupDeferRegister: "sec.cleanup.defer_register", semantic.OpCleanupDeferYield: "sec.cleanup.defer_yield",
	semantic.OpCleanupRunScope: "sec.cleanup.run_scope", semantic.OpCleanupRunFunction: "sec.cleanup.run_function",
}

// emitOwnership maps verified own.* and cleanup.* operations to schema 13. It emits
// Sema's resolved plans and identities and selects no destructor ABI, cleanup stack
// or runtime registry.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — §§4–23,36;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §86.
func (e *emitter) emitOwnership(function *semantic.Function, op semantic.Operation, values map[semantic.ValueID]semantic.Value, storages map[semantic.StorageID]semantic.Storage) error {
	if e.schemaVersion < 13 {
		return &UnsupportedLoweringError{Feature: string(op.Kind) + " requires schema 13", Function: function.ID}
	}
	if len(op.Results) != 0 {
		if err := e.emitResults(op); err != nil {
			return err
		}
	}
	operands := make([]string, len(op.Operands))
	types := make([]string, len(op.Operands))
	for index, operand := range op.Operands {
		operands[index] = valueName(operand)
		text, err := e.typeText(values[operand].Type)
		if err != nil {
			return err
		}
		types[index] = text
	}
	fmt.Fprintf(&e.out, "%s(%s)", mlirString(ownershipOperationNames[op.Kind]), strings.Join(operands, ", "))
	if properties := ownershipProperties(op); len(properties) != 0 {
		fmt.Fprintf(&e.out, " <{%s}>", strings.Join(properties, ", "))
	}
	if op.Kind == semantic.OpCleanupDeferRegister {
		if err := e.emitDeferRegion(function, op, values, storages); err != nil {
			return err
		}
	}
	fmt.Fprintf(&e.out, " : (%s) -> ", strings.Join(types, ", "))
	if len(op.Results) == 0 {
		e.out.WriteString("()")
	} else {
		result, err := e.typeText(op.Results[0].Type)
		if err != nil {
			return err
		}
		e.out.WriteString(result)
	}
	e.emitLocation(op.Location)
	e.out.WriteByte('\n')
	return nil
}

func ownershipProperties(op semantic.Operation) []string {
	var properties []string
	text := func(name, value string) {
		if value != "" {
			properties = append(properties, name+" = "+mlirString(value))
		}
	}
	number := func(name string, value uint32) {
		properties = append(properties, fmt.Sprintf("%s = %d : i32", name, value))
	}
	if facts := op.Ownership; facts != nil {
		text("copy_mode", facts.CopyMode)
		text("semantic_copy_plan", facts.SemanticCopyPlan)
		text("destruction_plan", facts.DestructionPlan)
		text("cause", facts.Cause)
		text("discard_origin", facts.DiscardOrigin)
		if facts.IfOwned {
			properties = append(properties, "if_owned")
		}
		if op.Kind == semantic.OpOwnReplacePlace {
			number("old_cleanup_action_id", facts.OldCleanup)
			number("new_cleanup_action_id", facts.NewCleanup)
		}
		if len(facts.Skipped) != 0 {
			paths := make([]string, len(facts.Skipped))
			for index, path := range facts.Skipped {
				paths[index] = mlirString(path.String())
			}
			properties = append(properties, "skipped = ["+strings.Join(paths, ", ")+"]")
		}
	}
	if facts := op.Cleanup; facts != nil {
		switch op.Kind {
		case semantic.OpCleanupTrackOwned, semantic.OpCleanupDeferRegister:
			number("cleanup_action_id", facts.Action)
			number("cleanup_scope_id", facts.Scope)
			number("cleanup_registration_ordinal", facts.Ordinal)
			if op.Kind == semantic.OpCleanupDeferRegister {
				number("defer_id", facts.Defer)
			} else {
				text("destruction_plan", facts.DestructionPlan)
				if len(facts.UnlessActive) != 0 {
					actions := make([]string, len(facts.UnlessActive))
					for index, action := range facts.UnlessActive {
						actions[index] = fmt.Sprint(action)
					}
					properties = append(properties, "unless_active = array<i32: "+strings.Join(actions, ", ")+">")
				}
			}
		case semantic.OpCleanupCancel:
			number("cleanup_action_id", facts.Action)
			text("reason", facts.Reason)
			if facts.IfActive {
				properties = append(properties, "if_active")
			}
		case semantic.OpCleanupRunScope:
			scopes := make([]string, len(facts.Scopes))
			for index, scope := range facts.Scopes {
				scopes[index] = fmt.Sprint(scope)
			}
			properties = append(properties, "scope_ids = array<i32: "+strings.Join(scopes, ", ")+">")
			text("exit_kind", facts.Exit)
		case semantic.OpCleanupRunFunction:
			text("exit_kind", facts.Exit)
		}
	}
	return properties
}

// emitDeferRegion prints the isolated defer body in generic region syntax; its entry
// arguments are the capture Places (dialect v13 § 18).
func (e *emitter) emitDeferRegion(function *semantic.Function, op semantic.Operation, values map[semantic.ValueID]semantic.Value, storages map[semantic.StorageID]semantic.Storage) error {
	if op.Region == nil {
		return &UnsupportedLoweringError{Feature: "cleanup.defer-register without a region", Function: function.ID}
	}
	e.out.WriteString(" ({\n")
	for _, block := range op.Region.Blocks {
		fmt.Fprintf(&e.out, "  ^bb%d", block.ID)
		if len(block.Parameters) != 0 {
			parameters := make([]string, len(block.Parameters))
			for index, parameter := range block.Parameters {
				text, err := e.typeText(parameter.Type)
				if err != nil {
					return err
				}
				parameters[index] = valueName(parameter.ID) + ": " + text
			}
			e.out.WriteString("(" + strings.Join(parameters, ", ") + ")")
		}
		e.out.WriteString(":\n")
		for _, inner := range block.Operations {
			if err := e.emitOperation(function, inner, values, storages); err != nil {
				return err
			}
		}
	}
	e.out.WriteString("    })")
	return nil
}

// emitDestructionPlanTable publishes the closed type-level plan graph that
// --sec-verify-destruction-plans checks; it holds no physical field offsets
// (dialect v13 § 23).
func (e *emitter) emitDestructionPlanTable() error {
	if len(e.module.DestructionPlans) == 0 {
		return nil
	}
	plans := make([]string, 0, len(e.module.DestructionPlans))
	for _, plan := range e.module.DestructionPlans {
		typeText, err := e.typeText(plan.Type)
		if err != nil {
			return err
		}
		parts := []string{"id = " + mlirString(plan.ID), "type = " + typeText, "kind = " + mlirString(plan.Kind)}
		if plan.CustomFree {
			parts = append(parts, "custom_free = true", "free_type = "+mlirString(plan.FreeType))
			if symbol, known := e.functions[plan.FreeFunction]; known {
				parts = append(parts, "free_function = @"+symbol)
			}
		}
		if len(plan.Fields) != 0 {
			fields := make([]string, len(plan.Fields))
			for index, field := range plan.Fields {
				fields[index] = fmt.Sprintf("{field = %d : i32, plan = %s}", field.Field, mlirString(field.Plan))
			}
			parts = append(parts, "fields = ["+strings.Join(fields, ", ")+"]")
		}
		if plan.Element != "" {
			parts = append(parts, "element = "+mlirString(plan.Element), "element_count = "+mlirString(plan.ElementCount))
		}
		if len(plan.Variants) != 0 {
			variants := make([]string, len(plan.Variants))
			for index, variant := range plan.Variants {
				variants[index] = fmt.Sprintf("{variant = %d : i32, plan = %s}", variant.Variant, mlirString(variant.Plan))
			}
			parts = append(parts, "variants = ["+strings.Join(variants, ", ")+"]")
		}
		if plan.Resource != "" {
			parts = append(parts, "resource = "+mlirString(plan.Resource))
		}
		plans = append(plans, "{"+strings.Join(parts, ", ")+"}")
	}
	fmt.Fprintf(&e.out, "sec.destruction_plans = [%s], ", strings.Join(plans, ", "))
	return nil
}
