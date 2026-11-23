package semantic

import (
	"fmt"
	"strings"
)

// Context selection is explicit and compiler-internal; it never discovers an
// ambient Arena by scanning source scope. Rules:
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§54–60,86,98–99.
const (
	OpAllocationContextFromArena     OpKind         = "alloc-context.from-arena"
	OpAllocationContextTarget        OpKind         = "alloc-context.target"
	OpAllocationContextCompilerLocal OpKind         = "alloc-context.compiler-local"
	ContextExplicitArena                            = "explicit-arena"
	ContextPropagated                               = "propagated-ambient"
	ContextTarget                                   = "target-provided"
	ContextCompilerLocal                            = "compiler-local"
	ArgumentAllocationContext        ArgumentAction = "allocation-context"
)

// AllocationContextFacts identifies the selected SSA capability and its already
// resolved origin plan. Domain is required only for explicitly selected Arena
// authority. OriginPlan owns the target entry/provider or compiler-local
// backing/lifetime/no-escape/failure/profile proof; this record does not infer
// that proof or make an unresolved source site ready for lowering.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§55,59–60,98–99;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§15–19.
type AllocationContextFacts struct {
	Value      ValueID
	Origin     string
	OriginPlan string
	Domain     uint32
}

// isAllocationContextProducer identifies capability-producing operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §86, §§118–120.
func isAllocationContextProducer(kind OpKind) bool {
	return kind == OpAllocationContextFromArena || kind == OpAllocationContextTarget || kind == OpAllocationContextCompilerLocal
}

// verifyAllocationContextSignature keeps one marked hidden input first in
// internal signatures, separate from source parameters and foreign ABIs.
// RequiresAllocationContext is not inferred from MayAllocate.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§56–58,98,108–109.
func verifyAllocationContextSignature(module *Module, fn *Function) error {
	count := 0
	for index, p := range fn.Parameters {
		context := typeHasKind(module.Types, p.Value.Type, TypeAllocationContext)
		if !context && !p.HiddenAllocationContext {
			continue
		}
		if !context || !p.HiddenAllocationContext || index != 0 || p.Value.Ownership != OwnershipCompilerTemporary || p.Reference != nil {
			return fmt.Errorf("allocation context must be the first, marked compiler-hidden capability input")
		}
		count++
	}
	if (count == 1) != fn.RequiresAllocationContext || count > 1 {
		return fmt.Errorf("RequiresAllocationContext must agree with exactly one hidden context input")
	}
	if count != 0 && (fn.Extern || fn.ABI != "") {
		return fmt.Errorf("ordinary foreign ABI cannot carry a hidden allocation context")
	}
	return nil
}

// verifyAllocationContexts checks producers, selected SSA origins and internal
// call forwarding. Context capabilities are usable only by marked calls and
// allocation operations; they cannot become ordinary arguments or CFG data.
// Source summaries and origin proofs are produced by P19-13/20, not here.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§55–60,98–99;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§15–19.
func verifyAllocationContexts(module *Module, fn *Function, functions map[FunctionID]*Function) error {
	values := map[ValueID]Value{}
	origins := map[ValueID]AllocationContextFacts{}
	for _, p := range fn.Parameters {
		values[p.Value.ID] = p.Value
		if p.HiddenAllocationContext {
			origins[p.Value.ID] = AllocationContextFacts{Value: p.Value.ID, Origin: ContextPropagated}
		}
	}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			values[parameter.ID] = parameter
			if typeHasKind(module.Types, parameter.Type, TypeAllocationContext) {
				return fmt.Errorf("allocation context is not an ordinary CFG block argument")
			}
		}
		for _, op := range block.Operations {
			for _, result := range op.Results {
				values[result.ID] = result
				if typeHasKind(module.Types, result.Type, TypeAllocationContext) && !isAllocationContextProducer(op.Kind) {
					return fmt.Errorf("%s cannot manufacture an allocation context", op.Kind)
				}
			}
		}
	}
	// Resolve producer shapes after collecting all values: block list order is
	// not a dominance order. The ordinary SSA verifier checks every use.
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if isAllocationContextProducer(op.Kind) {
				if err := verifyAllocationContextProducer(module, op, values); err != nil {
					return err
				}
				origins[op.Results[0].ID] = *op.AllocationContext
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			producer := isAllocationContextProducer(op.Kind)
			callNeedsContext := false
			if op.Kind == OpDirectCall || op.Kind == OpForeignCall {
				callee := functions[op.Callee]
				if callee != nil {
					callNeedsContext = callee.RequiresAllocationContext
				}
				if callNeedsContext {
					if op.Kind != OpDirectCall || len(op.Operands) == 0 || len(op.ArgumentActions) == 0 || op.ArgumentActions[0] != ArgumentAllocationContext || op.AllocationContext == nil || op.AllocationContext.Value != op.Operands[0] {
						return fmt.Errorf("call to %s must explicitly forward its hidden allocation context", op.Callee)
					}
				}
			}
			allocating := op.Kind == OpArenaCreateOwnedFixed || op.Kind == OpArenaCreateGrowable || op.Kind == OpStringMaterialize
			if (callNeedsContext || allocating) && op.AllocationContext == nil {
				return fmt.Errorf("%s requires a selected allocation context", op.Kind)
			}
			if op.AllocationContext != nil && !producer {
				if !callNeedsContext && !allocating {
					return fmt.Errorf("%s cannot receive an unsolicited allocation context", op.Kind)
				}
				selection := *op.AllocationContext
				origin, ok := origins[selection.Value]
				if !ok || selection != origin {
					return fmt.Errorf("%s context selection disagrees with its capability origin", op.Kind)
				}
			}
			for index, operand := range op.Operands {
				if !typeHasKind(module.Types, values[operand].Type, TypeAllocationContext) {
					continue
				}
				if (!callNeedsContext && !allocating) || index != 0 || op.AllocationContext == nil || op.AllocationContext.Value != operand {
					return fmt.Errorf("allocation context escapes through %s", op.Kind)
				}
			}
			for _, target := range op.Successors {
				for _, argument := range target.Arguments {
					if typeHasKind(module.Types, values[argument].Type, TypeAllocationContext) {
						return fmt.Errorf("allocation context escapes through a branch argument")
					}
				}
			}
		}
	}
	return nil
}

// verifyAllocationContextProducer checks explicit origin/plan and authority
// shapes. The resolved origin plan retains all required provider/profile or
// compiler-local proofs; no allocator or escape promotion is invented.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§59–60,118–120.
func verifyAllocationContextProducer(module *Module, op Operation, values map[ValueID]Value) error {
	facts := op.AllocationContext
	if facts == nil || facts.OriginPlan == "" || len(op.Results) != 1 || facts.Value != op.Results[0].ID || !typeHasKind(module.Types, op.Results[0].Type, TypeAllocationContext) || op.Results[0].Ownership != OwnershipCompilerTemporary || len(op.Successors) != 0 {
		return fmt.Errorf("%s requires one internal context result and its resolved origin plan", op.Kind)
	}
	want := ContextTarget
	if op.Kind == OpAllocationContextCompilerLocal {
		want = ContextCompilerLocal
	}
	if op.Kind == OpAllocationContextFromArena {
		want = ContextExplicitArena
		if len(op.Operands) != 1 || facts.Domain == 0 {
			return fmt.Errorf("alloc-context.from-arena requires explicitly selected Arena authority/domain")
		}
		place, ok := placeType(module.Types, values[op.Operands[0]].Type)
		if !ok || !place.Mutable || !typeHasKind(module.Types, place.Element, TypeArena) {
			return fmt.Errorf("alloc-context.from-arena requires a writable Arena Place")
		}
	} else if len(op.Operands) != 0 || facts.Domain != 0 {
		return fmt.Errorf("%s does not discover lexical Arena operands/domains", op.Kind)
	}
	if facts.Origin != want {
		return fmt.Errorf("%s has incorrect selected context origin", op.Kind)
	}
	return nil
}

// formatAllocationContext records explicit SSA capability selection and origin.
// Rules: rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §27.
func formatAllocationContext(out *strings.Builder, facts *AllocationContextFacts) {
	if facts == nil {
		return
	}
	fmt.Fprintf(out, " context=%%%d origin=%s origin-plan=%q", facts.Value, facts.Origin, facts.OriginPlan)
	if facts.Domain != 0 {
		fmt.Fprintf(out, " arena-domain=#%d", facts.Domain)
	}
}
