package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// activeLoop is the innermost-first stack entry that `break` and `continue`
// resolve against. The exit block is created on first use so a loop without
// any exit path gets none.
type activeLoop struct {
	id        LoopID
	condition *Block
	exit      *Block
}

// buildWhile lowers a while statement to explicit loop CFG: the code before
// the loop enters the condition block, the condition selects body or exit,
// the body's normal end and every `continue` return to the condition, and
// every `break` leaves through the exit. The condition is evaluated exactly
// once per attempted iteration. Sema's immutable while-flow decision selects
// the constant forms: a proven-true condition is not re-evaluated and the
// loop continues only through a reachable `break`; a proven-false body is
// never entered, so only its zero-iteration exit is built. Loop-carried state
// stays in mutable local storage, which each iteration loads afresh.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 66 "Loop CFG", § 67(4) "`while`"
//   - rules/control-flow/flowcontrol_while.md — § 4 "Condition evaluation", §§ 13–15, §§ 19–20, § 29 "CFG requirements"
func (fb *functionBuilder) buildWhile(stmt *ast.WhileStatement) error {
	flow, ok := fb.owner.analyzer.ResolvedWhileFlowOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved while flow at %s", formatLocation(location(stmt.Token)))
	}
	loc := location(stmt.Token)
	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{
		ID: loop.id, Kind: LoopWhile, ConditionKnown: flow.ConditionKnown, ConditionValue: flow.ConditionValue,
		ContinuesAfter: flow.ContinuesAfterLoop, Location: loc,
	}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1

	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})
	fb.current = loop.condition

	if flow.ConditionKnown && !flow.ConditionValue {
		// § 19: the body is statically checked by Sema but never executes.
		exit := fb.loopExit(loop)
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})
	} else {
		var condition builtValue
		if !flow.ConditionKnown {
			var err error
			condition, err = fb.buildExpr(stmt.Condition, 0)
			if err != nil {
				return err
			}
			conditionType, _ := fb.owner.module.Types.Lookup(condition.typ)
			if conditionType.Kind != TypeBool {
				return fb.unsupported("non-boolean while condition", stmt.Token)
			}
		}
		body := fb.newBlock()
		record.BodyBlock = body.ID
		if flow.ConditionKnown {
			// A proven-true condition is side-effect free (Sema's constant
			// subset), so it selects the body unconditionally.
			fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: body.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})
		} else {
			exit := fb.loopExit(loop)
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})
		}
		fb.current = body
		fb.loops = append(fb.loops, loop)
		err := fb.buildStatements(stmt.Body.Statements)
		fb.loops = fb.loops[:len(fb.loops)-1]
		if err != nil {
			return err
		}
		if fb.current != nil {
			fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
		}
	}

	fb.current = nil
	if loop.exit != nil {
		record.ExitBlock = loop.exit.ID
		fb.current = loop.exit
		if !flow.ContinuesAfterLoop {
			// Every structural exit was a `break` that Sema proved
			// unreachable (§ 20); the exit block keeps the CFG well formed.
			fb.emit(Operation{Kind: OpUnreachable, Synthesized: true, Reason: "loop-exit-excluded-by-sema", LoopID: loop.id, Location: loc})
			fb.current = nil
		}
	} else if flow.ContinuesAfterLoop {
		return fmt.Errorf("while loop at %s continues after the loop but has no exit", formatLocation(loc))
	}
	fb.fn.Loops[recordIndex] = record
	return nil
}

// buildLoopControl transfers to the innermost loop: `break` to its exit and
// `continue` to its next condition evaluation. Switch and match do not create
// loop-control targets.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§ 13–15, § 17 "Interaction with `switch`"
//   - rules/compiler/semantic_ir.md — § 66(1)
func (fb *functionBuilder) buildLoopControl(edge LoopEdge, token lexer.Token) error {
	if len(fb.loops) == 0 {
		return fb.unsupported(string(edge)+" outside a represented loop", token)
	}
	loop := fb.loops[len(fb.loops)-1]
	target := loop.condition
	if edge == LoopEdgeBreak {
		target = fb.loopExit(loop)
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: target.ID}}, LoopID: loop.id, LoopEdge: edge, Location: location(token)})
	fb.current = nil
	return nil
}

func (fb *functionBuilder) loopExit(loop *activeLoop) *Block {
	if loop.exit == nil {
		loop.exit = fb.newBlock()
	}
	return loop.exit
}

// verifyLoopRecords checks every loop record against the edges that carry
// its identity: one enter edge and one condition edge, back and continue
// edges to the condition block, break edges to the exit, and no foreign edge
// into the condition, body, or exit block. A loop whose condition is not
// proven true must continue after the loop; a non-continuing loop either has
// no exit or an exit that only states Sema's exclusion.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 66(1) "Loop CFG", § 67(4)
//   - rules/control-flow/flowcontrol_while.md — §§ 19–20, § 29
func verifyLoopRecords(fn *Function, blocks map[BlockID]*Block) error {
	records := map[LoopID]LoopRecord{}
	for index, loop := range fn.Loops {
		if loop.ID != LoopID(index+1) || loop.Kind != LoopWhile {
			return fmt.Errorf("invalid loop record @%d", loop.ID)
		}
		if blocks[loop.ConditionBlock] == nil {
			return fmt.Errorf("loop @%d has no condition block", loop.ID)
		}
		neverEntered := loop.ConditionKnown && !loop.ConditionValue
		if neverEntered != (loop.BodyBlock == 0) || (loop.BodyBlock != 0 && blocks[loop.BodyBlock] == nil) {
			return fmt.Errorf("loop @%d has an invalid body block", loop.ID)
		}
		if !(loop.ConditionKnown && loop.ConditionValue) && !loop.ContinuesAfter {
			return fmt.Errorf("loop @%d can exit through its condition but is marked non-continuing", loop.ID)
		}
		if loop.ExitBlock != 0 && blocks[loop.ExitBlock] == nil {
			return fmt.Errorf("loop @%d has a missing exit block", loop.ID)
		}
		if loop.ContinuesAfter && loop.ExitBlock == 0 {
			return fmt.Errorf("continuing loop @%d has no exit block", loop.ID)
		}
		if !loop.ContinuesAfter && loop.ExitBlock != 0 {
			exit := blocks[loop.ExitBlock]
			if len(exit.Operations) != 1 || exit.Operations[0].Kind != OpUnreachable || !exit.Operations[0].Synthesized || exit.Operations[0].LoopID != loop.ID {
				return fmt.Errorf("non-continuing loop @%d has a reachable exit", loop.ID)
			}
		}
		records[loop.ID] = loop
	}
	enters := map[LoopID]int{}
	conditions := map[LoopID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.LoopID == 0 {
				if op.LoopEdge != "" {
					return fmt.Errorf("loop edge %s in ^%d has no loop", op.LoopEdge, block.ID)
				}
				for _, successor := range op.Successors {
					for _, loop := range fn.Loops {
						if successor.Block == loop.ConditionBlock || successor.Block == loop.BodyBlock || successor.Block == loop.ExitBlock {
							return fmt.Errorf("^%d enters loop @%d outside its loop edges", block.ID, loop.ID)
						}
					}
				}
				continue
			}
			loop, ok := records[op.LoopID]
			if !ok {
				return fmt.Errorf("operation in ^%d refers to unknown loop @%d", block.ID, op.LoopID)
			}
			if err := verifyLoopEdge(loop, op, fn.Loops); err != nil {
				return fmt.Errorf("loop @%d in ^%d: %w", loop.ID, block.ID, err)
			}
			switch op.LoopEdge {
			case LoopEdgeEnter:
				enters[loop.ID]++
			case LoopEdgeCondition:
				conditions[loop.ID]++
			}
		}
	}
	for _, loop := range fn.Loops {
		if enters[loop.ID] != 1 || conditions[loop.ID] != 1 {
			return fmt.Errorf("loop @%d needs exactly one enter and one condition edge", loop.ID)
		}
	}
	return nil
}

func verifyLoopEdge(loop LoopRecord, op Operation, loops []LoopRecord) error {
	targets := func(want ...BlockID) error {
		if op.Kind != OpBranch && op.Kind != OpCondBranch {
			return fmt.Errorf("%s edge is %s", op.LoopEdge, op.Kind)
		}
		if len(op.Successors) != len(want) {
			return fmt.Errorf("%s edge has %d successors", op.LoopEdge, len(op.Successors))
		}
		for index, successor := range op.Successors {
			if successor.Block != want[index] || len(successor.Arguments) != 0 {
				return fmt.Errorf("%s edge targets ^%d", op.LoopEdge, successor.Block)
			}
		}
		return nil
	}
	switch op.LoopEdge {
	case "":
		if op.Kind != OpUnreachable || !op.Synthesized || op.Reason != "loop-exit-excluded-by-sema" {
			return fmt.Errorf("%s carries a loop without an edge", op.Kind)
		}
		return nil
	case LoopEdgeEnter, LoopEdgeBack, LoopEdgeContinue:
		if op.Kind != OpBranch {
			return fmt.Errorf("%s edge is %s", op.LoopEdge, op.Kind)
		}
		return targets(loop.ConditionBlock)
	case LoopEdgeBreak:
		if op.Kind != OpBranch {
			return fmt.Errorf("break edge is %s", op.Kind)
		}
		return targets(loop.ExitBlock)
	case LoopEdgeCondition:
		switch {
		case loop.ConditionKnown && loop.ConditionValue:
			if op.Kind != OpBranch {
				return fmt.Errorf("constant-true condition edge is %s", op.Kind)
			}
			return targets(loop.BodyBlock)
		case loop.ConditionKnown:
			if op.Kind != OpBranch {
				return fmt.Errorf("constant-false condition edge is %s", op.Kind)
			}
			return targets(loop.ExitBlock)
		default:
			if op.Kind != OpCondBranch {
				return fmt.Errorf("runtime condition edge is %s", op.Kind)
			}
			return targets(loop.BodyBlock, loop.ExitBlock)
		}
	default:
		return fmt.Errorf("unknown loop edge %q", op.LoopEdge)
	}
}
