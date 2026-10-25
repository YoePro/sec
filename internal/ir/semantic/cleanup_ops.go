package semantic

import (
	"fmt"
	"strings"
)

// Semantic IR cleanup operations (P17-33; package17 §§ 34–47; dialect v13
// §§ 14–22). Cleanup is a semantic registration stack, not a runtime: an
// owned non-trivial value is tracked after successful initialization and
// canceled when its ownership moves out; an executed defer registers one
// instance whose body is an explicit region over capture Places; leaving a
// scope runs its eligible automatic cleanup, and a normal function exit runs
// every remaining entry in reverse registration order. Panic endpoints run
// no cleanup (§ 46).
const (
	OpCleanupTrackOwned    OpKind = "cleanup.track-owned"
	OpCleanupCancel        OpKind = "cleanup.cancel"
	OpCleanupDeferRegister OpKind = "cleanup.defer-register"
	OpCleanupDeferYield    OpKind = "cleanup.defer-yield"
	OpCleanupRunScope      OpKind = "cleanup.run-scope"
	OpCleanupRunFunction   OpKind = "cleanup.run-function"
)

var (
	// cleanupScopeExits are the exit kinds of cleanup.run-scope and
	// cleanupFunctionExits those of cleanup.run-function (dialect v13 § 22).
	cleanupScopeExits    = []string{"fallthrough", "break", "continue", "match-branch-exit", "switch-branch-exit"}
	cleanupFunctionExits = []string{"fallthrough", "return", "error-propagation"}
	// cleanupCancelReasons: the owned value moved out, its ownership ended
	// in a replacement, or it was transferred out by a return/argument
	// (package17 §§ 38, 40, 55).
	cleanupCancelReasons = []string{"moved", "replaced", "transferred"}
)

// CleanupFacts are the compile-time identities of a cleanup.* operation.
type CleanupFacts struct {
	// Action identifies a registration (track-owned, defer-register) or the
	// registration a cancel ends.
	Action uint32
	// Scope is the registering lexical scope; Ordinal the registration's
	// position in source registration order.
	Scope   uint32
	Ordinal uint32
	// Defer identifies a defer statement.
	Defer uint32
	// DestructionPlan is the plan a tracked owned value is destroyed with.
	DestructionPlan string
	// Reason explains a cancel.
	Reason string
	// Scopes are the exited scopes of run-scope; Exit the exit kind.
	Scopes []uint32
	Exit   string
}

// Region is the body of a cleanup.defer-register: its entry block's
// parameters correspond exactly to the capture operands, and every path ends
// with cleanup.defer-yield.
type Region struct {
	Entry  BlockID
	Blocks []*Block
}

// verifyCleanupOperation checks one cleanup.* operation's local contract.
func verifyCleanupOperation(module *Module, op Operation, values map[ValueID]Value) error {
	facts := op.Cleanup
	if facts == nil {
		return fmt.Errorf("%s has no cleanup facts", op.Kind)
	}
	if len(op.Results) != 0 || op.Kind != OpCleanupDeferYield && len(op.Successors) != 0 {
		return fmt.Errorf("%s has no results or successors", op.Kind)
	}
	if op.Region != nil && op.Kind != OpCleanupDeferRegister {
		return fmt.Errorf("%s has no region", op.Kind)
	}
	switch op.Kind {
	case OpCleanupTrackOwned:
		if len(op.Operands) != 1 || facts.Action == 0 || facts.Scope == 0 || facts.Ordinal == 0 {
			return fmt.Errorf("cleanup.track-owned requires one value/Place and action, scope and ordinal identities")
		}
		tracked := values[op.Operands[0]].Type
		if place, ok := placeType(module.Types, tracked); ok {
			tracked = place.Element
		}
		if isBorrowValueType(module, tracked) {
			return fmt.Errorf("cleanup.track-owned of a reference/slice: a borrow holder is not owned")
		}
		plan, err := destructionPlan(module, facts.DestructionPlan, tracked)
		if err != nil {
			return fmt.Errorf("cleanup.track-owned: %w", err)
		}
		if plan.Kind == "trivial" {
			return fmt.Errorf("cleanup.track-owned of a trivially destructible value registers nothing")
		}
	case OpCleanupCancel:
		if len(op.Operands) != 0 || facts.Action == 0 || !oneOf(facts.Reason, cleanupCancelReasons) {
			return fmt.Errorf("cleanup.cancel requires an action identity and a reason (moved, replaced, transferred)")
		}
	case OpCleanupDeferRegister:
		if facts.Action == 0 || facts.Scope == 0 || facts.Ordinal == 0 || facts.Defer == 0 || facts.DestructionPlan != "" {
			return fmt.Errorf("cleanup.defer-register requires action, scope, ordinal and defer identities and no destruction plan")
		}
		if op.Region == nil {
			return fmt.Errorf("cleanup.defer-register has no body region")
		}
		for _, operand := range op.Operands {
			if _, ok := placeType(module.Types, values[operand].Type); !ok {
				return fmt.Errorf("cleanup.defer-register captures %%%d, which is not a Place", operand)
			}
		}
	case OpCleanupDeferYield:
		if len(op.Operands) != 0 || len(op.Successors) != 0 {
			return fmt.Errorf("cleanup.defer-yield has no operands or successors")
		}
	case OpCleanupRunScope:
		if len(op.Operands) != 0 || len(facts.Scopes) == 0 || !oneOf(facts.Exit, cleanupScopeExits) {
			return fmt.Errorf("cleanup.run-scope requires exited scopes and a scope exit kind")
		}
	case OpCleanupRunFunction:
		if len(op.Operands) != 0 || len(facts.Scopes) != 0 || !oneOf(facts.Exit, cleanupFunctionExits) {
			return fmt.Errorf("cleanup.run-function requires a function exit kind and no scopes")
		}
	}
	return nil
}

// verifyCleanupFunction checks the cleanup structure of a function body:
// unique registration actions and ordinals, cancels of registered actions,
// cleanup.run-function only on a block that returns, defer-yield only in a
// defer region, and every defer region as an isolated body whose entry
// parameters match the captures and whose paths all yield.
func verifyCleanupFunction(module *Module, fn *Function, functions map[FunctionID]*Function) error {
	registered, ordinals := map[uint32]bool{}, map[uint32]bool{}
	var cancels []uint32
	var walk func(blocks []*Block, inRegion bool) error
	walk = func(blocks []*Block, inRegion bool) error {
		for _, block := range blocks {
			for index, op := range block.Operations {
				switch op.Kind {
				case OpCleanupTrackOwned, OpCleanupDeferRegister:
					if registered[op.Cleanup.Action] || ordinals[op.Cleanup.Ordinal] {
						return fmt.Errorf("%s reuses cleanup action #%d or ordinal %d", op.Kind, op.Cleanup.Action, op.Cleanup.Ordinal)
					}
					registered[op.Cleanup.Action], ordinals[op.Cleanup.Ordinal] = true, true
					if op.Kind == OpCleanupDeferRegister {
						if err := verifyDeferRegion(module, fn, op, functions); err != nil {
							return err
						}
						if err := walk(op.Region.Blocks, true); err != nil {
							return err
						}
					}
				case OpCleanupCancel:
					cancels = append(cancels, op.Cleanup.Action)
				case OpCleanupRunFunction:
					last := block.Operations[len(block.Operations)-1]
					if inRegion || last.Kind != OpReturn || index == len(block.Operations)-1 {
						return fmt.Errorf("cleanup.run-function must precede the return of its block")
					}
				case OpCleanupDeferYield:
					if !inRegion {
						return fmt.Errorf("cleanup.defer-yield outside a defer region")
					}
				case OpReturn:
					if inRegion {
						return fmt.Errorf("a defer region cannot return from its function")
					}
				}
			}
		}
		return nil
	}
	if err := walk(fn.Blocks, false); err != nil {
		return err
	}
	for _, action := range cancels {
		if !registered[action] {
			return fmt.Errorf("cleanup.cancel of unregistered action #%d", action)
		}
	}
	return nil
}

// verifyDeferRegion verifies the body as an isolated function: its entry
// parameters are the capture Places, so it cannot use outer SSA values, and
// it ends every path with cleanup.defer-yield.
func verifyDeferRegion(module *Module, fn *Function, op Operation, functions map[FunctionID]*Function) error {
	region := op.Region
	var entry *Block
	for _, block := range region.Blocks {
		if block != nil && block.ID == region.Entry {
			entry = block
		}
	}
	if entry == nil || len(entry.Parameters) != len(op.Operands) {
		return fmt.Errorf("defer region entry must take exactly the %d capture Place(s)", len(op.Operands))
	}
	outer := map[ValueID]TypeID{}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			outer[parameter.ID] = parameter.Type
		}
		for _, operation := range block.Operations {
			for _, result := range operation.Results {
				outer[result.ID] = result.Type
			}
		}
	}
	for _, parameter := range fn.Parameters {
		outer[parameter.Value.ID] = parameter.Value.Type
	}
	for index, parameter := range entry.Parameters {
		if parameter.Type != outer[op.Operands[index]] {
			return fmt.Errorf("defer region parameter %%%d does not match capture %%%d", parameter.ID, op.Operands[index])
		}
	}
	for _, block := range region.Blocks {
		if len(block.Operations) == 0 {
			return fmt.Errorf("defer region block ^%d has no terminator", block.ID)
		}
		last := block.Operations[len(block.Operations)-1]
		if last.Kind != OpCleanupDeferYield && last.Kind != OpBranch && last.Kind != OpCondBranch {
			return fmt.Errorf("defer region block ^%d must end with cleanup.defer-yield or a branch within the region, not %s", block.ID, last.Kind)
		}
	}
	// The isolated body: entry parameters become the body's inputs.
	body := &Function{ID: fn.ID + "#defer", Name: fn.Name, ReturnType: fn.ReturnType, Entry: region.Entry, Location: fn.Location, Storages: fn.Storages}
	for _, block := range region.Blocks {
		copied := *block
		if block.ID == region.Entry {
			for _, parameter := range block.Parameters {
				body.Parameters = append(body.Parameters, Parameter{Name: fmt.Sprintf("capture%d", parameter.ID), Value: parameter})
			}
			copied.Parameters = nil
		}
		body.Blocks = append(body.Blocks, &copied)
	}
	if err := verifyFunction(module, body, functions); err != nil {
		return fmt.Errorf("defer region of %s: %w", fn.ID, err)
	}
	return nil
}

func formatCleanupOperation(out *strings.Builder, op Operation, indent string) {
	for index, operand := range op.Operands {
		if index > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, " %%%d", operand)
	}
	facts := op.Cleanup
	if facts == nil {
		return
	}
	if facts.Action != 0 {
		fmt.Fprintf(out, " action=#%d", facts.Action)
	}
	if facts.Scope != 0 {
		fmt.Fprintf(out, " scope=#%d", facts.Scope)
	}
	if facts.Ordinal != 0 {
		fmt.Fprintf(out, " ordinal=%d", facts.Ordinal)
	}
	if facts.Defer != 0 {
		fmt.Fprintf(out, " defer=#%d", facts.Defer)
	}
	if facts.DestructionPlan != "" {
		fmt.Fprintf(out, " plan=%q", facts.DestructionPlan)
	}
	if facts.Reason != "" {
		fmt.Fprintf(out, " reason=%s", facts.Reason)
	}
	if len(facts.Scopes) != 0 {
		scopes := make([]string, len(facts.Scopes))
		for index, scope := range facts.Scopes {
			scopes[index] = fmt.Sprintf("#%d", scope)
		}
		fmt.Fprintf(out, " scopes=[%s]", strings.Join(scopes, ", "))
	}
	if facts.Exit != "" {
		fmt.Fprintf(out, " exit=%s", facts.Exit)
	}
	if op.Region == nil {
		return
	}
	out.WriteString(" {\n")
	for _, block := range op.Region.Blocks {
		fmt.Fprintf(out, "%s  ^%d", indent, block.ID)
		if len(block.Parameters) > 0 {
			out.WriteByte('(')
			for index, parameter := range block.Parameters {
				if index > 0 {
					out.WriteString(", ")
				}
				fmt.Fprintf(out, "%%%d: !%d [%s]", parameter.ID, parameter.Type, parameter.Ownership)
			}
			out.WriteByte(')')
		}
		out.WriteString(":\n")
		for _, inner := range block.Operations {
			out.WriteString(indent + "    ")
			formatOperation(out, inner)
			out.WriteByte('\n')
		}
	}
	out.WriteString(indent + "}")
}
