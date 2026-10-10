package mlir

import (
	"fmt"

	"sec/internal/ast"
)

// emitSwitch lowers ordered clause tests and independent case bodies, evaluating
// a present subject once and routing subjectless tests through lazy alternatives.
// Rules: rules/control-flow/flowcontrol_switch.md §§4–6, 12–19, 39.
func (g *Generator) emitSwitch(stmt *ast.SwitchStatement) error {
	if err := validateMLIRSwitchCases(stmt); err != nil {
		return err
	}

	var subject value
	if stmt.Subject != nil {
		var err error
		subject, err = g.emitExpression(stmt.Subject)
		if err != nil {
			return err
		}
		if subject.typ != "i1" && !isMLIRIntegerType(subject.typ) {
			return fmt.Errorf("emit-mlir switch currently supports bool and integer subjects, got %s", subject.typ)
		}
	}

	clauses := append([]*ast.SwitchCase(nil), stmt.Cases...)
	endLabel := g.nextLabel("switch.end")
	defaultLabel := endLabel
	if stmt.Default != nil {
		defaultLabel = g.nextLabel("switch.default")
	}

	testLabels := make([]string, len(clauses))
	bodyLabels := make([]string, len(clauses))
	for i := range clauses {
		testLabels[i] = g.nextLabel("switch.test")
		bodyLabels[i] = g.nextLabel("switch.case")
	}

	if len(clauses) == 0 {
		g.write("    llvm.br ^%s\n", defaultLabel)
	} else {
		g.write("    llvm.br ^%s\n", testLabels[0])
	}
	g.blockOpen = false

	for i, clause := range clauses {
		falseLabel := defaultLabel
		if i+1 < len(testLabels) {
			falseLabel = testLabels[i+1]
		}

		g.write("  ^%s:\n", testLabels[i])
		g.blockOpen = true
		if stmt.Subject == nil {
			if err := g.emitSubjectlessSwitchCase(clause, bodyLabels[i], falseLabel); err != nil {
				return err
			}
			continue
		}
		condition, err := g.emitSwitchValueCaseCondition(subject, clause)
		if err != nil {
			return err
		}
		g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition.ref, bodyLabels[i], falseLabel)
		g.blockOpen = false
	}

	outerLocals := g.locals
	for i, clause := range clauses {
		g.write("  ^%s:\n", bodyLabels[i])
		g.blockOpen = true
		g.locals = copyMLIRLocals(outerLocals)
		nextBody := ""
		if i+1 < len(bodyLabels) {
			nextBody = bodyLabels[i+1]
		} else if stmt.Default != nil {
			nextBody = defaultLabel
		}
		if err := g.emitSwitchBody(clause.Body, nextBody, endLabel); err != nil {
			g.locals = outerLocals
			return err
		}
	}

	if stmt.Default != nil {
		g.write("  ^%s:\n", defaultLabel)
		g.blockOpen = true
		g.locals = copyMLIRLocals(outerLocals)
		if err := g.emitSwitchBody(stmt.Default.Body, "", endLabel); err != nil {
			g.locals = outerLocals
			return err
		}
	}
	g.locals = outerLocals

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

// validateMLIRSwitchCases rejects case forms whose lowering is still unsupported.
// Rules: rules/control-flow/flowcontrol_switch.md §§7–10, 16, 39.
func validateMLIRSwitchCases(stmt *ast.SwitchStatement) error {
	for i, clause := range stmt.Cases {
		if clause == nil {
			return fmt.Errorf("emit-mlir switch contains nil case")
		}
		for _, item := range clause.Items {
			if _, ok := item.(*ast.SwitchValueCase); !ok {
				return fmt.Errorf("emit-mlir switch currently supports only value cases, got %T", item)
			}
		}
		if i+1 == len(stmt.Cases) && stmt.Default == nil && switchBodyHasFallthrough(clause.Body) {
			return fmt.Errorf("fallthrough is not allowed in the final switch case")
		}
	}
	if stmt.Default != nil && switchBodyHasFallthrough(stmt.Default.Body) {
		return fmt.Errorf("fallthrough is not allowed in default")
	}
	return nil
}

// switchBodyHasFallthrough detects explicit case transfer for the readiness gate.
// Rules: rules/control-flow/flowcontrol_switch.md §§16–17.
func switchBodyHasFallthrough(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if _, ok := stmt.(*ast.FallthroughStatement); ok {
			return true
		}
	}
	return false
}

// emitSwitchValueCaseCondition compares the supported subject-value alternatives.
// Rules: rules/control-flow/flowcontrol_switch.md §§7, 11, 39.
func (g *Generator) emitSwitchValueCaseCondition(subject value, clause *ast.SwitchCase) (value, error) {
	if clause == nil || len(clause.Items) == 0 {
		return g.emitBoolConstant(false), nil
	}

	var combined value
	for i, item := range clause.Items {
		valueCase, ok := item.(*ast.SwitchValueCase)
		if !ok {
			return value{}, fmt.Errorf("emit-mlir switch currently supports only value cases, got %T", item)
		}
		candidate, err := g.emitExpressionForTargetUnsigned(valueCase.Value, subject.typ, subject.unsigned)
		if err != nil {
			return value{}, err
		}
		candidate, err = g.coerceValue(candidate, subject.typ, subject.unsigned)
		if err != nil {
			return value{}, fmt.Errorf("emit-mlir switch case does not match subject type %s", subject.typ)
		}
		equal, err := g.emitSwitchEquality(subject, candidate)
		if err != nil {
			return value{}, err
		}
		if i == 0 {
			combined = equal
			continue
		}
		combined, err = g.emitBooleanOr(combined, equal)
		if err != nil {
			return value{}, err
		}
	}
	return combined, nil
}

// emitSwitchEquality uses ordinary equality for the supported scalar carriers.
// Rules: rules/control-flow/flowcontrol_switch.md §§7, 39.
func (g *Generator) emitSwitchEquality(left value, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir switch equality requires matching types")
	}
	if left.typ != "i1" && !isMLIRIntegerType(left.typ) {
		return value{}, fmt.Errorf("emit-mlir switch equality does not support %s", left.typ)
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.icmp \"eq\" %s, %s : %s\n", tmp, left.ref, right.ref, left.typ)
	return value{typ: "i1", ref: tmp}, nil
}

// emitBooleanOr combines already materialized boolean comparison results.
// Rules: rules/control-flow/flowcontrol_switch.md §11.
func (g *Generator) emitBooleanOr(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-mlir boolean or expects bool operands")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.or %s, %s : i1\n", tmp, left.ref, right.ref)
	return value{typ: "i1", ref: tmp}, nil
}

// emitSwitchBody joins normal completion or branches directly to the next body
// for explicit fallthrough, bypassing the destination clause tests.
// Rules: rules/control-flow/flowcontrol_switch.md §§15–19, 38–39.
func (g *Generator) emitSwitchBody(block *ast.BlockStatement, nextBody, endLabel string) error {
	if block != nil {
		for i, stmt := range block.Statements {
			if !g.blockOpen {
				return nil
			}
			if _, ok := stmt.(*ast.FallthroughStatement); ok {
				if nextBody == "" {
					return fmt.Errorf("fallthrough is not allowed in the final switch case")
				}
				for _, following := range block.Statements[i+1:] {
					if _, comment := following.(*ast.CommentStatement); !comment {
						return fmt.Errorf("fallthrough must be the final statement in a switch case")
					}
				}
				g.write("    llvm.br ^%s\n", nextBody)
				g.blockOpen = false
				return nil
			}
			if err := g.emitStatement(stmt); err != nil {
				return err
			}
		}
	}
	if g.blockOpen {
		g.write("    llvm.br ^%s\n", endLabel)
		g.blockOpen = false
	}
	return nil
}

// emitSubjectlessSwitchCase emits a separate conditional edge for every
// comma alternative. A true edge enters the body immediately; a false edge
// evaluates only the next alternative, or reaches the following clause.
// Rules: rules/control-flow/flowcontrol_switch.md §§5–6, 12–13, 39;
// rules/corrections/applied/correction24-20260823.md — "Normative requirement".
func (g *Generator) emitSubjectlessSwitchCase(clause *ast.SwitchCase, bodyLabel, falseLabel string) error {
	if len(clause.Items) == 0 {
		g.write("    llvm.br ^%s\n", falseLabel)
		g.blockOpen = false
		return nil
	}
	for i, item := range clause.Items {
		candidate, ok := item.(*ast.SwitchValueCase)
		if !ok {
			return fmt.Errorf("emit-mlir subjectless switch requires boolean value cases")
		}
		condition, err := g.emitExpression(candidate.Value)
		if err != nil {
			return err
		}
		if condition.typ != "i1" {
			return fmt.Errorf("emit-mlir subjectless switch case must be bool, got %s", condition.typ)
		}
		nextLabel := falseLabel
		if i+1 < len(clause.Items) {
			nextLabel = g.nextLabel("switch.alternative")
		}
		g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition.ref, bodyLabel, nextLabel)
		g.blockOpen = false
		if i+1 < len(clause.Items) {
			g.write("  ^%s:\n", nextLabel)
			g.blockOpen = true
		}
	}
	return nil
}
