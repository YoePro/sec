package llvm

import (
	"fmt"

	"sec/internal/ast"
)

// emitSwitch evaluates the subject once and emits ordered case tests and bodies.
// Rules: rules/control-flow/flowcontrol_switch.md §§4–6, 14–16, 39.
func (g *Generator) emitSwitch(stmt *ast.SwitchStatement) error {
	if stmt.Subject == nil {
		return g.emitSubjectlessSwitch(stmt)
	}

	subject, err := g.emitExpression(stmt.Subject)
	if err != nil {
		return err
	}

	clauses := append([]*ast.SwitchCase{}, stmt.Cases...)
	bodyLabels := make([]string, len(clauses))
	testLabels := make([]string, len(clauses)+1)
	for i := range clauses {
		bodyLabels[i] = g.nextLabel("switch.case")
		testLabels[i] = g.nextLabel("switch.test")
	}
	endLabel := g.nextLabel("switch.end")
	defaultLabel := endLabel
	if stmt.Default != nil {
		defaultLabel = g.nextLabel("switch.default")
	}
	testLabels[len(clauses)] = defaultLabel

	if len(clauses) == 0 {
		g.write("  br label %%%s\n\n", defaultLabel)
	} else {
		g.write("  br label %%%s\n\n", testLabels[0])
	}
	g.blockOpen = false

	for i, clause := range clauses {
		g.write("%s:\n", testLabels[i])
		g.blockOpen = true
		condition, err := g.emitSwitchCaseCondition(subject, clause)
		if err != nil {
			return err
		}
		g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, bodyLabels[i], testLabels[i+1])
		g.blockOpen = false
	}

	for i, clause := range clauses {
		g.write("%s:\n", bodyLabels[i])
		g.blockOpen = true
		nextBody := endLabel
		if i+1 < len(bodyLabels) {
			nextBody = bodyLabels[i+1]
		} else if stmt.Default != nil {
			nextBody = defaultLabel
		}
		if err := g.emitSwitchCaseBody(clause.Body, nextBody, endLabel); err != nil {
			return err
		}
	}

	if stmt.Default != nil {
		g.write("%s:\n", defaultLabel)
		g.blockOpen = true
		if err := g.emitSwitchCaseBody(stmt.Default.Body, endLabel, endLabel); err != nil {
			return err
		}
	}

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

// emitSubjectlessSwitch emits reached-only boolean tests and case continuations.
// Rules: rules/control-flow/flowcontrol_switch.md §§12–15, 18–19, 39.
func (g *Generator) emitSubjectlessSwitch(stmt *ast.SwitchStatement) error {
	clauses := append([]*ast.SwitchCase{}, stmt.Cases...)
	bodyLabels := make([]string, len(clauses))
	testLabels := make([]string, len(clauses)+1)
	for i := range clauses {
		bodyLabels[i] = g.nextLabel("switch.case")
		testLabels[i] = g.nextLabel("switch.test")
	}
	endLabel := g.nextLabel("switch.end")
	defaultLabel := endLabel
	if stmt.Default != nil {
		defaultLabel = g.nextLabel("switch.default")
	}
	testLabels[len(clauses)] = defaultLabel

	if len(clauses) == 0 {
		g.write("  br label %%%s\n\n", defaultLabel)
	} else {
		g.write("  br label %%%s\n\n", testLabels[0])
	}
	g.blockOpen = false

	for i, clause := range clauses {
		g.write("%s:\n", testLabels[i])
		g.blockOpen = true
		if err := g.emitSubjectlessSwitchCase(clause, bodyLabels[i], testLabels[i+1]); err != nil {
			return err
		}
	}

	for i, clause := range clauses {
		g.write("%s:\n", bodyLabels[i])
		g.blockOpen = true
		nextBody := endLabel
		if i+1 < len(bodyLabels) {
			nextBody = bodyLabels[i+1]
		} else if stmt.Default != nil {
			nextBody = defaultLabel
		}
		if err := g.emitSwitchCaseBody(clause.Body, nextBody, endLabel); err != nil {
			return err
		}
	}

	if stmt.Default != nil {
		g.write("%s:\n", defaultLabel)
		g.blockOpen = true
		if err := g.emitSwitchCaseBody(stmt.Default.Body, endLabel, endLabel); err != nil {
			return err
		}
	}

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

// emitSwitchCaseCondition combines supported comparisons for a subject case.
// Rules: rules/control-flow/flowcontrol_switch.md §§7–11, 39.
func (g *Generator) emitSwitchCaseCondition(subject value, clause *ast.SwitchCase) (value, error) {
	var combined value
	for i, item := range clause.Items {
		condition, err := g.emitSwitchCaseItemCondition(subject, item)
		if err != nil {
			return value{}, err
		}
		if i == 0 {
			combined = condition
			continue
		}
		combined, err = g.emitBoolOr(combined, condition)
		if err != nil {
			return value{}, err
		}
	}
	if len(clause.Items) == 0 {
		return value{typ: "i1", ref: "false"}, nil
	}
	return combined, nil
}

// emitSubjectlessSwitchCase lowers each comma alternative to its own edge,
// so a matching alternative enters the case without evaluating later operands.
// Rules: rules/control-flow/flowcontrol_switch.md §§5–6, 12–13, 39;
// rules/corrections/applied/correction24-20260823.md — "Normative requirement".
func (g *Generator) emitSubjectlessSwitchCase(clause *ast.SwitchCase, bodyLabel, falseLabel string) error {
	if len(clause.Items) == 0 {
		g.write("  br label %%%s\n\n", falseLabel)
		g.blockOpen = false
		return nil
	}
	for i, item := range clause.Items {
		candidate, ok := item.(*ast.SwitchValueCase)
		if !ok {
			return fmt.Errorf("subjectless switch only supports condition cases")
		}
		condition, err := g.emitExpression(candidate.Value)
		if err != nil {
			return err
		}
		if condition.typ != "i1" {
			return fmt.Errorf("subjectless switch condition must be bool")
		}
		nextLabel := falseLabel
		if i+1 < len(clause.Items) {
			nextLabel = g.nextLabel("switch.alternative")
		}
		g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, bodyLabel, nextLabel)
		g.blockOpen = false
		if i+1 < len(clause.Items) {
			g.write("%s:\n", nextLabel)
			g.blockOpen = true
		}
	}
	return nil
}

// emitSwitchCaseItemCondition emits scalar value, relational and range tests.
// Rules: rules/control-flow/flowcontrol_switch.md §§7–10, 39.
func (g *Generator) emitSwitchCaseItemCondition(subject value, item ast.SwitchCaseItem) (value, error) {
	switch item := item.(type) {
	case *ast.SwitchValueCase:
		candidate, err := g.emitExpression(item.Value)
		if err != nil {
			return value{}, err
		}
		return g.emitCompare("eq", subject, candidate), nil
	case *ast.SwitchRelationalCase:
		candidate, err := g.emitExpression(item.Value)
		if err != nil {
			return value{}, err
		}
		predicate := map[string]string{"<": "slt", "<=": "sle", ">": "sgt", ">=": "sge"}[item.Operator]
		if predicate == "" {
			return value{}, fmt.Errorf("unsupported switch relational operator %q", item.Operator)
		}
		return g.emitCompare(predicate, subject, candidate), nil
	case *ast.SwitchRangeCase:
		return g.emitSwitchRangeCondition(subject, item.Range)
	default:
		return value{}, fmt.Errorf("unsupported switch case item %T", item)
	}
}

// emitSwitchRangeCondition checks inclusive/exclusive range membership.
// Rules: rules/control-flow/flowcontrol_switch.md §§8–9, 39.
func (g *Generator) emitSwitchRangeCondition(subject value, rangeExpr *ast.RangeExpression) (value, error) {
	if rangeExpr == nil {
		return value{}, fmt.Errorf("nil switch range")
	}

	if rangeExpr.Start != nil && rangeExpr.End != nil {
		start, err := g.emitExpression(rangeExpr.Start)
		if err != nil {
			return value{}, err
		}
		end, err := g.emitExpression(rangeExpr.End)
		if err != nil {
			return value{}, err
		}
		if start.typ != end.typ || subject.typ != start.typ {
			return value{}, fmt.Errorf("switch range bounds must match subject type")
		}
		return g.emitBoundedSwitchRangeCondition(subject, start, end, rangeExpr.Exclusive)
	}

	var combined value
	hasCondition := false
	if rangeExpr.Start != nil {
		start, err := g.emitExpression(rangeExpr.Start)
		if err != nil {
			return value{}, err
		}
		combined = g.emitCompare("sge", subject, start)
		hasCondition = true
	}
	if rangeExpr.End != nil {
		end, err := g.emitExpression(rangeExpr.End)
		if err != nil {
			return value{}, err
		}
		predicate := "sle"
		if rangeExpr.Exclusive {
			predicate = "slt"
		}
		endCondition := g.emitCompare(predicate, subject, end)
		if !hasCondition {
			combined = endCondition
			hasCondition = true
		} else {
			var err error
			combined, err = g.emitBoolAnd(combined, endCondition)
			if err != nil {
				return value{}, err
			}
		}
	}
	if !hasCondition {
		return value{typ: "i1", ref: "true"}, nil
	}
	return combined, nil
}

func (g *Generator) emitBoundedSwitchRangeCondition(subject value, start value, end value, exclusive bool) (value, error) {
	descending := g.emitCompare("sgt", start, end)

	ascendingLower := g.emitCompare("sge", subject, start)
	ascendingUpperPredicate := "sle"
	if exclusive {
		ascendingUpperPredicate = "slt"
	}
	ascendingUpper := g.emitCompare(ascendingUpperPredicate, subject, end)
	ascendingCondition, err := g.emitBoolAnd(ascendingLower, ascendingUpper)
	if err != nil {
		return value{}, err
	}

	descendingLowerPredicate := "sge"
	if exclusive {
		descendingLowerPredicate = "sgt"
	}
	descendingLower := g.emitCompare(descendingLowerPredicate, subject, end)
	descendingUpper := g.emitCompare("sle", subject, start)
	descendingCondition, err := g.emitBoolAnd(descendingLower, descendingUpper)
	if err != nil {
		return value{}, err
	}

	result := g.nextTemp()
	g.write("  %s = select i1 %s, i1 %s, i1 %s\n", result, descending.ref, descendingCondition.ref, ascendingCondition.ref)
	return value{typ: "i1", ref: result}, nil
}

// emitSwitchCaseBody joins normal completion or transfers explicit fallthrough.
// Rules: rules/control-flow/flowcontrol_switch.md §§15–18, 39.
func (g *Generator) emitSwitchCaseBody(block *ast.BlockStatement, fallthroughLabel string, endLabel string) error {
	if block == nil {
		g.write("  br label %%%s\n\n", endLabel)
		g.blockOpen = false
		return nil
	}

	for i, stmt := range block.Statements {
		if !g.blockOpen {
			return nil
		}
		if _, ok := stmt.(*ast.FallthroughStatement); ok {
			if i != len(block.Statements)-1 {
				return fmt.Errorf("fallthrough must be the final switch case statement")
			}
			g.write("  br label %%%s\n\n", fallthroughLabel)
			g.blockOpen = false
			return nil
		}
		if err := g.emitStatement(stmt); err != nil {
			return err
		}
	}
	if g.blockOpen {
		g.write("  br label %%%s\n\n", endLabel)
		g.blockOpen = false
	}
	return nil
}
