package llvm

import (
	"fmt"

	"sec/internal/ast"
)

func (g *Generator) emitBlock(block *ast.BlockStatement) error {
	if block == nil {
		return nil
	}

	for _, stmt := range block.Statements {
		if !g.blockOpen {
			return nil
		}
		if err := g.emitStatement(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) emitStatement(stmt ast.Statement) error {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		return g.emitLet(stmt)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			if err := g.emitLet(let); err != nil {
				return err
			}
		}
		return nil
	case *ast.AssignmentStatement:
		return g.emitAssignment(stmt)
	case *ast.TryAssignmentStatement:
		return g.emitTryAssignment(stmt)
	case *ast.ReturnStatement:
		return g.emitReturn(stmt)
	case *ast.IfStatement:
		return g.emitIf(stmt)
	case *ast.ForStatement:
		return g.emitFor(stmt)
	case *ast.WhileStatement:
		return g.emitWhile(stmt)
	case *ast.BreakStatement:
		return g.emitBreak()
	case *ast.ContinueStatement:
		return g.emitContinue()
	case *ast.SwitchStatement:
		return g.emitSwitch(stmt)
	case *ast.MatchStatement:
		if stmt.Match == nil {
			return nil
		}
		return g.emitMatchStatement(stmt.Match)
	case *ast.UnsafeStatement:
		return g.emitUnsafe(stmt)
	case *ast.AsmStatement:
		return g.emitAsmStatement(stmt)
	case *ast.DeferStatement:
		return g.emitDefer(stmt)
	case *ast.DiscardStatement:
		return nil
	case *ast.ExpressionStatement:
		_, err := g.emitExpressionStatement(stmt.Expression)
		return err
	default:
		return fmt.Errorf("emit-llvm does not support %T yet", stmt)
	}
}

func (g *Generator) emitTryAssignment(stmt *ast.TryAssignmentStatement) error {
	// TODO: Lower fallible property assignment through the generated setter and
	// branch on Result once the Result ABI is implemented.
	return nil
}

func (g *Generator) emitDefer(stmt *ast.DeferStatement) error {
	// TODO: Queue defer blocks and emit them on every function exit. For the
	// first LLVM pass we only need the syntax to be recognized and compilable.
	return g.emitBlock(stmt.Body)
}

func (g *Generator) emitFor(stmt *ast.ForStatement) error {
	if len(stmt.Bindings) == 0 && stmt.Iterable == nil {
		return g.emitInfiniteFor(stmt)
	}

	rangeExpr, ok := stmt.Iterable.(*ast.RangeExpression)
	if !ok {
		return fmt.Errorf("emit-llvm currently supports only range for loops")
	}
	if len(stmt.Bindings) != 1 {
		return fmt.Errorf("emit-llvm currently supports one loop binding")
	}
	return g.emitRangeFor(stmt, rangeExpr)
}

func (g *Generator) emitWhile(stmt *ast.WhileStatement) error {
	if stmt.Condition == nil || stmt.Body == nil {
		return fmt.Errorf("emit-llvm requires complete while statements")
	}

	conditionLabel := g.nextLabel("while.condition")
	bodyLabel := g.nextLabel("while.body")
	endLabel := g.nextLabel("while.end")

	g.write("  br label %%%s\n\n", conditionLabel)
	g.blockOpen = false

	g.write("%s:\n", conditionLabel)
	g.blockOpen = true
	condition, err := g.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}
	if condition.typ != "i1" {
		return fmt.Errorf("emit-llvm while condition must be bool")
	}
	g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("%s:\n", bodyLabel)
	g.blockOpen = true
	g.pushLoop(endLabel, conditionLabel)
	if err := g.emitBlock(stmt.Body); err != nil {
		g.popLoop()
		return err
	}
	g.popLoop()
	if g.blockOpen {
		g.write("  br label %%%s\n\n", conditionLabel)
		g.blockOpen = false
	}

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitInfiniteFor(stmt *ast.ForStatement) error {
	bodyLabel := g.nextLabel("for.body")
	endLabel := g.nextLabel("for.end")

	g.write("  br label %%%s\n\n", bodyLabel)
	g.blockOpen = false

	g.write("%s:\n", bodyLabel)
	g.blockOpen = true
	g.pushLoop(endLabel, bodyLabel)
	if err := g.emitBlock(stmt.Body); err != nil {
		g.popLoop()
		return err
	}
	g.popLoop()
	if g.blockOpen {
		g.write("  br label %%%s\n\n", bodyLabel)
		g.blockOpen = false
	}

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitRangeFor(stmt *ast.ForStatement, rangeExpr *ast.RangeExpression) error {
	if rangeExpr.Start == nil || rangeExpr.End == nil {
		return fmt.Errorf("emit-llvm range for requires finite range")
	}

	start, err := g.emitExpression(rangeExpr.Start)
	if err != nil {
		return err
	}
	end, err := g.emitExpression(rangeExpr.End)
	if err != nil {
		return err
	}
	if start.typ != end.typ {
		return fmt.Errorf("emit-llvm range bounds must have same type")
	}
	if start.typ != "i32" && start.typ != "i64" {
		return fmt.Errorf("emit-llvm range for currently supports integer bounds")
	}
	step := value{typ: start.typ, ref: "1"}
	if stmt.Step != nil {
		step, err = g.emitExpression(stmt.Step)
		if err != nil {
			return err
		}
		if step.typ != start.typ {
			return fmt.Errorf("emit-llvm range step must match range bounds")
		}
	}

	conditionLabel := g.nextLabel("for.condition")
	bodyLabel := g.nextLabel("for.body")
	nextLabel := g.nextLabel("for.next")
	endLabel := g.nextLabel("for.end")

	binding := stmt.Bindings[0]
	previousLocals := g.locals
	g.locals = copyCodegenLocals(previousLocals)
	defer func() {
		g.locals = previousLocals
	}()

	loopPtr := g.nextTemp()
	descending := g.nextTemp()
	g.write("  %s = alloca %s\n", loopPtr, start.typ)
	g.write("  store %s %s, ptr %s\n", start.typ, start.ref, loopPtr)
	g.write("  %s = icmp sgt %s %s, %s\n", descending, start.typ, start.ref, end.ref)
	if !binding.Discard {
		g.locals[binding.Name] = local{typ: start.typ, ptr: loopPtr}
	}

	g.write("  br label %%%s\n\n", conditionLabel)
	g.blockOpen = false

	g.write("%s:\n", conditionLabel)
	g.blockOpen = true
	current := value{typ: start.typ, ref: g.nextTemp()}
	g.write("  %s = load %s, ptr %s\n", current.ref, current.typ, loopPtr)
	ascendingPredicate := "sle"
	descendingPredicate := "sge"
	if rangeExpr.Exclusive {
		ascendingPredicate = "slt"
		descendingPredicate = "sgt"
	}
	ascendingCondition := g.emitCompare(ascendingPredicate, current, end)
	descendingCondition := g.emitCompare(descendingPredicate, current, end)
	conditionRef := g.nextTemp()
	g.write("  %s = select i1 %s, i1 %s, i1 %s\n", conditionRef, descending, descendingCondition.ref, ascendingCondition.ref)
	g.write("  br i1 %s, label %%%s, label %%%s\n\n", conditionRef, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("%s:\n", bodyLabel)
	g.blockOpen = true
	g.pushLoop(endLabel, nextLabel)
	if err := g.emitBlock(stmt.Body); err != nil {
		g.popLoop()
		return err
	}
	g.popLoop()
	if g.blockOpen {
		g.write("  br label %%%s\n\n", nextLabel)
		g.blockOpen = false
	}

	g.write("%s:\n", nextLabel)
	g.blockOpen = true
	loaded := g.nextTemp()
	nextAscending := g.nextTemp()
	nextDescending := g.nextTemp()
	incremented := g.nextTemp()
	g.write("  %s = load %s, ptr %s\n", loaded, start.typ, loopPtr)
	g.write("  %s = add %s %s, %s\n", nextAscending, start.typ, loaded, step.ref)
	g.write("  %s = sub %s %s, %s\n", nextDescending, start.typ, loaded, step.ref)
	g.write("  %s = select i1 %s, %s %s, %s %s\n", incremented, descending, start.typ, nextDescending, start.typ, nextAscending)
	g.write("  store %s %s, ptr %s\n", start.typ, incremented, loopPtr)
	g.write("  br label %%%s\n\n", conditionLabel)
	g.blockOpen = false

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitBreak() error {
	if len(g.loops) == 0 {
		return fmt.Errorf("emit-llvm break outside loop")
	}
	ctx := g.loops[len(g.loops)-1]
	g.write("  br label %%%s\n\n", ctx.breakLabel)
	g.blockOpen = false
	return nil
}

func (g *Generator) emitContinue() error {
	if len(g.loops) == 0 {
		return fmt.Errorf("emit-llvm continue outside loop")
	}
	ctx := g.loops[len(g.loops)-1]
	g.write("  br label %%%s\n\n", ctx.continueLabel)
	g.blockOpen = false
	return nil
}

func (g *Generator) pushLoop(breakLabel string, continueLabel string) {
	g.loops = append(g.loops, loopContext{breakLabel: breakLabel, continueLabel: continueLabel})
}

func (g *Generator) popLoop() {
	g.loops = g.loops[:len(g.loops)-1]
}

func copyCodegenLocals(in map[string]local) map[string]local {
	out := make(map[string]local, len(in))
	for name, slot := range in {
		out[name] = slot
	}
	return out
}

func (g *Generator) emitUnsafe(stmt *ast.UnsafeStatement) error {
	if stmt.Body == nil {
		return nil
	}
	if len(stmt.Body.Statements) == 1 {
		if asmStmt, ok := stmt.Body.Statements[0].(*ast.AsmStatement); ok {
			out, err := g.emitAsm(asmStmt)
			if err != nil {
				return err
			}
			if g.returnType != "" && g.returnType != "void" && out.typ != "" {
				if out.typ != g.returnType {
					return fmt.Errorf("emit-llvm asm output %s does not match function return %s", out.typ, g.returnType)
				}
				g.write("  ret %s %s\n", out.typ, out.ref)
				g.blockOpen = false
			}
			return nil
		}
	}
	return g.emitBlock(stmt.Body)
}

func (g *Generator) emitAsm(stmt *ast.AsmStatement) (value, error) {
	if stmt.Block == nil {
		if stmt.Template == nil {
			return value{}, fmt.Errorf("asm statement requires string template")
		}
		g.write("  call void asm sideeffect %q, \"\"()\n", stmt.Template.Value)
		return value{typ: "void"}, nil
	}

	return g.emitAsmBlock(stmt.Block)
}

func (g *Generator) emitAsmStatement(stmt *ast.AsmStatement) error {
	out, err := g.emitAsm(stmt)
	if err != nil {
		return err
	}
	if stmt.Block == nil || len(stmt.Block.Outputs) == 0 || stmt.Block.Outputs[0].Name == "" {
		return nil
	}
	name := stmt.Block.Outputs[0].Name
	if g.returnType != "" && g.returnType != "void" {
		out, err = g.coerceValue(out, g.returnType)
		if err != nil {
			return err
		}
	}
	g.locals[name] = local{typ: out.typ, ref: out.ref, direct: true}
	return nil
}

func (g *Generator) emitAsmBlock(block *ast.AsmBlock) (value, error) {
	if block.Template == nil {
		return value{}, fmt.Errorf("asm block requires string template")
	}
	if len(block.Outputs) != 1 {
		return value{}, fmt.Errorf("emit-llvm currently supports exactly one asm output")
	}

	constraints := "={" + block.Outputs[0].Register + "}"
	args := make([]value, 0, len(block.Inputs))
	for _, input := range block.Inputs {
		constraints += ",{" + input.Register + "}"
		arg, err := g.emitExpression(input.Value)
		if err != nil {
			return value{}, err
		}
		arg, err = g.coerceAsmInput(arg)
		if err != nil {
			return value{}, err
		}
		args = append(args, arg)
	}
	for _, clobber := range asmClobbers(block) {
		constraints += ",~{" + clobber + "}"
	}

	result := g.nextTemp()
	g.write("  %s = call i64 asm sideeffect %q, %q(", result, block.Template.Value, constraints)
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s %s", arg.typ, arg.ref)
	}
	g.write(")\n")
	return value{typ: "i64", ref: result}, nil
}

func asmClobbers(block *ast.AsmBlock) []string {
	if len(block.Clobbers) > 0 {
		return block.Clobbers
	}
	return []string{"rcx", "r11"}
}

func (g *Generator) coerceAsmInput(arg value) (value, error) {
	return g.coerceValue(arg, "i64")
}

func (g *Generator) coerceValue(arg value, targetType string) (value, error) {
	if arg.typ == targetType {
		return arg, nil
	}
	if targetType == llvmDecimalType {
		switch arg.typ {
		case "i32":
			temp := g.nextTemp()
			g.write("  %s = sext i32 %s to i64\n", temp, arg.ref)
			return g.emitDecimalFromI64(temp), nil
		case "i64":
			return g.emitDecimalFromI64(arg.ref), nil
		}
	}
	switch arg.typ {
	case "i32":
		if targetType != "i64" {
			break
		}
		temp := g.nextTemp()
		g.write("  %s = sext i32 %s to i64\n", temp, arg.ref)
		return value{typ: "i64", ref: temp}, nil
	case "i64":
		if targetType != "i32" {
			break
		}
		temp := g.nextTemp()
		g.write("  %s = trunc i64 %s to i32\n", temp, arg.ref)
		return value{typ: "i32", ref: temp}, nil
	case "ptr":
		if targetType != "i64" {
			break
		}
		temp := g.nextTemp()
		g.write("  %s = ptrtoint ptr %s to i64\n", temp, arg.ref)
		return value{typ: "i64", ref: temp}, nil
	}
	return value{}, fmt.Errorf("emit-llvm cannot convert %s to %s", arg.typ, targetType)
}

func (g *Generator) emitDecimalFromI64(numberRef string) value {
	g.needsDecimal = true
	tmp := g.nextTemp()
	g.write("  %s = insertvalue %s undef, i64 %s, 0\n", tmp, llvmDecimalType, numberRef)
	result := g.nextTemp()
	g.write("  %s = insertvalue %s %s, i8 0, 1\n", result, llvmDecimalType, tmp)
	return value{typ: llvmDecimalType, ref: result}
}

func (g *Generator) emitLet(stmt *ast.LetStatement) error {
	if stmt.Name == nil {
		return fmt.Errorf("emit-llvm let missing name")
	}

	var typ string
	var initial *value
	if stmt.Type != nil {
		typ = g.llvmType(stmt.Type)
	}
	if stmt.Value != nil {
		var val value
		var err error
		if typ == llvmDecimalType {
			val, err = g.emitDecimalLiteral(stmt.Value, true)
			if err != nil {
				val, err = g.emitExpression(stmt.Value)
			}
		} else {
			val, err = g.emitExpression(stmt.Value)
		}
		if err != nil {
			return err
		}
		initial = &val
		if typ == "" {
			typ = val.typ
		}
	}
	if typ == "" || typ == "void" {
		return fmt.Errorf("emit-llvm cannot determine type for local %s", stmt.Name.Value)
	}
	if typ == "string" {
		if initial == nil {
			return fmt.Errorf("emit-llvm string local %s requires initializer", stmt.Name.Value)
		}
		g.locals[stmt.Name.Value] = local{typ: "string", ref: initial.ref, lenRef: initial.lenRef, direct: true}
		return nil
	}

	ptr := g.nextTemp()
	g.write("  %s = alloca %s\n", ptr, typ)
	var fnType *ast.TypeReference
	if stmt.Type != nil && (stmt.Type.Name == "fn" || stmt.Type.FunctionReturnType != nil) {
		fnType = stmt.Type
	} else if initial != nil && initial.fnType != nil {
		fnType = initial.fnType
	}
	g.locals[stmt.Name.Value] = local{typ: typ, ptr: ptr, fnType: fnType}

	if initial != nil {
		if initial.typ != typ {
			return fmt.Errorf("emit-llvm cannot initialize %s with %s", typ, initial.typ)
		}
		g.write("  store %s %s, ptr %s\n", typ, initial.ref, ptr)
	}
	return nil
}

func (g *Generator) emitAssignment(stmt *ast.AssignmentStatement) error {
	ident, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("emit-llvm only supports identifier assignment targets for now")
	}
	slot, ok := g.locals[ident.Value]
	if !ok {
		return fmt.Errorf("emit-llvm unknown local %s", ident.Value)
	}
	if slot.direct {
		return fmt.Errorf("emit-llvm cannot assign to parameter %s", ident.Value)
	}
	val, err := g.emitExpression(stmt.Value)
	if err != nil {
		return err
	}
	if val.typ != slot.typ {
		return fmt.Errorf("emit-llvm cannot assign %s to %s", val.typ, slot.typ)
	}
	if stmt.Operator != "=" {
		current := g.nextTemp()
		g.write("  %s = load %s, ptr %s\n", current, slot.typ, slot.ptr)
		op := map[string]string{
			"+=": "add",
			"-=": "sub",
			"*=": "mul",
			"/=": "sdiv",
		}[stmt.Operator]
		if op == "" {
			return fmt.Errorf("emit-llvm does not support assignment operator %q yet", stmt.Operator)
		}
		combined, err := g.emitIntegerBinary(op, value{typ: slot.typ, ref: current}, val)
		if err != nil {
			return err
		}
		val = combined
	}
	g.write("  store %s %s, ptr %s\n", slot.typ, val.ref, slot.ptr)
	return nil
}

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
		condition, err := g.emitSubjectlessSwitchCaseCondition(clause)
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

func (g *Generator) emitSubjectlessSwitchCaseCondition(clause *ast.SwitchCase) (value, error) {
	var combined value
	for i, item := range clause.Items {
		valueCase, ok := item.(*ast.SwitchValueCase)
		if !ok {
			return value{}, fmt.Errorf("subjectless switch only supports condition cases")
		}
		condition, err := g.emitExpression(valueCase.Value)
		if err != nil {
			return value{}, err
		}
		if condition.typ != "i1" {
			return value{}, fmt.Errorf("subjectless switch condition must be bool")
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

func (g *Generator) emitMatchStatement(expr *ast.MatchExpression) error {
	subject, err := g.emitExpression(expr.Subject)
	if err != nil {
		return err
	}

	bodyLabels, testLabels, endLabel := g.matchLabels(expr)
	g.write("  br label %%%s\n\n", testLabels[0])
	g.blockOpen = false

	for i, arm := range expr.Arms {
		g.write("%s:\n", testLabels[i])
		g.blockOpen = true
		condition, err := g.emitMatchArmCondition(subject, arm)
		if err != nil {
			return err
		}
		g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, bodyLabels[i], testLabels[i+1])
		g.blockOpen = false
	}

	g.write("%s:\n", testLabels[len(expr.Arms)])
	g.blockOpen = true
	g.write("  unreachable\n\n")
	g.blockOpen = false

	for i, arm := range expr.Arms {
		g.write("%s:\n", bodyLabels[i])
		g.blockOpen = true
		if err := g.emitMatchArmBody(arm, subject, "", endLabel); err != nil {
			return err
		}
	}

	g.write("%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitMatchExpression(expr *ast.MatchExpression) (value, error) {
	subject, err := g.emitExpression(expr.Subject)
	if err != nil {
		return value{}, err
	}
	if len(expr.Arms) == 0 {
		return value{}, fmt.Errorf("emit-llvm match requires at least one arm")
	}

	resultType, err := g.matchExpressionResultType(expr)
	if err != nil {
		return value{}, err
	}
	resultPtr := g.nextTemp()
	g.write("  %s = alloca %s\n", resultPtr, resultType)

	bodyLabels, testLabels, endLabel := g.matchLabels(expr)
	g.write("  br label %%%s\n\n", testLabels[0])
	g.blockOpen = false

	for i, arm := range expr.Arms {
		g.write("%s:\n", testLabels[i])
		g.blockOpen = true
		condition, err := g.emitMatchArmCondition(subject, arm)
		if err != nil {
			return value{}, err
		}
		g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, bodyLabels[i], testLabels[i+1])
		g.blockOpen = false
	}

	g.write("%s:\n", testLabels[len(expr.Arms)])
	g.blockOpen = true
	g.write("  unreachable\n\n")
	g.blockOpen = false

	for i, arm := range expr.Arms {
		g.write("%s:\n", bodyLabels[i])
		g.blockOpen = true
		if err := g.emitMatchArmBody(arm, subject, resultPtr, endLabel); err != nil {
			return value{}, err
		}
	}

	result := g.nextTemp()
	g.write("%s:\n", endLabel)
	g.blockOpen = true
	g.write("  %s = load %s, ptr %s\n", result, resultType, resultPtr)
	return value{typ: resultType, ref: result}, nil
}

func (g *Generator) matchLabels(expr *ast.MatchExpression) ([]string, []string, string) {
	bodyLabels := make([]string, len(expr.Arms))
	testLabels := make([]string, len(expr.Arms)+1)
	for i := range expr.Arms {
		bodyLabels[i] = g.nextLabel("match.arm")
		testLabels[i] = g.nextLabel("match.test")
	}
	testLabels[len(expr.Arms)] = g.nextLabel("match.unreachable")
	return bodyLabels, testLabels, g.nextLabel("match.end")
}

func (g *Generator) emitMatchArmCondition(subject value, arm *ast.MatchArm) (value, error) {
	if arm.Guard != nil {
		return value{}, fmt.Errorf("emit-llvm match guards are not supported yet")
	}

	switch pattern := arm.Pattern.(type) {
	case *ast.Identifier:
		return value{typ: "i1", ref: "true"}, nil
	case *ast.MemberExpression, *ast.IntegerLiteral, *ast.BooleanLiteral:
		candidate, err := g.emitExpression(pattern)
		if err != nil {
			return value{}, err
		}
		return g.emitCompare("eq", subject, candidate), nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support match pattern %T yet", arm.Pattern)
	}
}

func (g *Generator) emitMatchArmBody(arm *ast.MatchArm, subject value, resultPtr string, endLabel string) error {
	previousLocals := g.locals
	g.locals = copyCodegenLocals(previousLocals)
	defer func() {
		g.locals = previousLocals
	}()

	if ident, ok := arm.Pattern.(*ast.Identifier); ok && ident.Value != "_" {
		g.locals[ident.Value] = local{typ: subject.typ, ref: subject.ref, direct: true}
	}

	switch {
	case arm.ReturnBody != nil:
		return g.emitReturn(arm.ReturnBody)
	case arm.BlockBody != nil:
		if err := g.emitBlock(arm.BlockBody); err != nil {
			return err
		}
	case arm.Body != nil:
		result, err := g.emitExpression(arm.Body)
		if err != nil {
			return err
		}
		if resultPtr != "" {
			g.write("  store %s %s, ptr %s\n", result.typ, result.ref, resultPtr)
		}
	default:
		return fmt.Errorf("emit-llvm match arm missing body")
	}
	if g.blockOpen {
		g.write("  br label %%%s\n\n", endLabel)
		g.blockOpen = false
	}
	return nil
}

func (g *Generator) matchExpressionResultType(expr *ast.MatchExpression) (string, error) {
	for _, arm := range expr.Arms {
		if arm.Body == nil {
			continue
		}
		body, err := g.emitExpressionTypeOnly(arm.Body)
		if err != nil {
			return "", err
		}
		return body, nil
	}
	return "", fmt.Errorf("emit-llvm match expression must produce a value")
}

func (g *Generator) emitExpressionTypeOnly(expr ast.Expression) (string, error) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return "i32", nil
	case *ast.BooleanLiteral:
		return "i1", nil
	case *ast.StringLiteral:
		return "string", nil
	case *ast.MemberExpression:
		if typeName, ok := expressionPath(expr.Object); ok {
			if enum, exists := g.enums[typeName]; exists {
				return enum.typ, nil
			}
		}
	case *ast.InfixExpression:
		if isCodegenComparison(expr.Operator) || expr.Operator == "&&" || expr.Operator == "||" {
			return "i1", nil
		}
	}
	return "", fmt.Errorf("emit-llvm cannot infer match expression result type for %T yet", expr)
}

func isCodegenComparison(operator string) bool {
	switch operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func (g *Generator) emitExpressionStatement(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		return g.emitCallExpression(expr)
	case *ast.RuntimeCallExpression:
		return g.emitRuntimeCallExpression(expr)
	default:
		return g.emitExpression(expr)
	}
}

func (g *Generator) emitCallExpression(expr *ast.CallExpression) (value, error) {
	name := callExpressionName(expr)
	if _, ok := g.functions[name]; ok {
		return g.emitFunctionCallExpression(expr)
	}
	if g.isCodegenConversion(name) {
		return g.emitBuiltinConversionCall(expr, name)
	}

	switch name {
	case "fmt.Println":
		if len(expr.Arguments) != 1 {
			return value{}, fmt.Errorf("fmt.Println expects 1 argument")
		}
		arg, err := g.emitExpression(expr.Arguments[0])
		if err != nil {
			return value{}, err
		}
		if arg.typ == "string" {
			arg = value{typ: "ptr", ref: arg.ref}
		}
		if arg.typ != "ptr" {
			return value{}, fmt.Errorf("fmt.Println currently expects string")
		}
		g.needsPuts = true
		g.write("  call i32 @puts(ptr %s)\n", arg.ref)
		return value{typ: "void"}, nil
	default:
		return g.emitFunctionCallExpression(expr)
	}
}

func isCodegenBuiltinConversion(name string) bool {
	switch name {
	case "int", "int64", "uint", "uint64", "byte", "bool", "decimal":
		return true
	default:
		return false
	}
}

func (g *Generator) isCodegenConversion(name string) bool {
	if isCodegenBuiltinConversion(name) {
		return true
	}
	if _, ok := g.typeAliases[name]; ok {
		return true
	}
	if _, ok := g.enums[name]; ok {
		return true
	}
	return false
}

func (g *Generator) emitBuiltinConversionCall(expr *ast.CallExpression, name string) (value, error) {
	if len(expr.Arguments) != 1 {
		return value{}, fmt.Errorf("conversion to %s expects 1 argument, got %d", name, len(expr.Arguments))
	}
	arg, err := g.emitExpression(expr.Arguments[0])
	if err != nil {
		return value{}, err
	}
	return g.coerceValue(arg, g.llvmType(&ast.TypeReference{Name: name}))
}

func (g *Generator) emitFunctionCallExpression(expr *ast.CallExpression) (value, error) {
	name := callExpressionName(expr)
	fn, ok := g.functions[name]
	if !ok {
		if local, exists := g.locals[name]; exists && local.fnType != nil {
			return g.emitFunctionValueCallExpression(expr, local)
		}
		return value{}, fmt.Errorf("emit-llvm does not support call %s yet", name)
	}
	if len(fn.Parameters) != len(expr.Arguments) {
		return value{}, fmt.Errorf("call %s expects %d arguments, got %d", name, len(fn.Parameters), len(expr.Arguments))
	}

	args := []value{}
	for i, argExpr := range expr.Arguments {
		arg, err := g.emitExpression(argExpr)
		if err != nil {
			return value{}, err
		}
		param := fn.Parameters[i]
		if param.Type != nil && param.Type.Name == "string" {
			if arg.typ != "string" {
				return value{}, fmt.Errorf("argument %d to %s must be string", i+1, name)
			}
			args = append(args, value{typ: "ptr", ref: arg.ref}, value{typ: "i64", ref: arg.lenRef})
			continue
		}
		targetType := g.llvmParameterType(param)
		arg, err = g.coerceValue(arg, targetType)
		if err != nil {
			return value{}, err
		}
		args = append(args, arg)
	}

	returnType := g.llvmType(fn.ReturnType)
	var result string
	if returnType != "void" {
		result = g.nextTemp()
		g.write("  %s = ", result)
	} else {
		g.write("  ")
	}
	g.write("call %s @%s(", returnType, name)
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s %s", arg.typ, arg.ref)
	}
	g.write(")\n")
	if returnType == "void" {
		return value{typ: "void"}, nil
	}
	return value{typ: returnType, ref: result}, nil
}

func (g *Generator) emitFunctionValueCallExpression(expr *ast.CallExpression, callee local) (value, error) {
	if callee.fnType == nil || callee.fnType.FunctionReturnType == nil {
		return value{}, fmt.Errorf("emit-llvm function value missing type")
	}
	if len(callee.fnType.FunctionParameterTypes) != len(expr.Arguments) {
		return value{}, fmt.Errorf("function value expects %d arguments, got %d", len(callee.fnType.FunctionParameterTypes), len(expr.Arguments))
	}

	var fnRef string
	if callee.direct {
		fnRef = callee.ref
	} else {
		fnRef = g.nextTemp()
		g.write("  %s = load ptr, ptr %s\n", fnRef, callee.ptr)
	}

	args := []value{}
	for i, argExpr := range expr.Arguments {
		arg, err := g.emitExpression(argExpr)
		if err != nil {
			return value{}, err
		}
		paramType := callee.fnType.FunctionParameterTypes[i]
		if paramType != nil && paramType.Name == "string" {
			if arg.typ != "string" {
				return value{}, fmt.Errorf("argument %d must be string", i+1)
			}
			args = append(args, value{typ: "ptr", ref: arg.ref}, value{typ: "i64", ref: arg.lenRef})
			continue
		}
		targetType := g.llvmType(paramType)
		arg, err = g.coerceValue(arg, targetType)
		if err != nil {
			return value{}, err
		}
		args = append(args, arg)
	}

	returnType := g.llvmType(callee.fnType.FunctionReturnType)
	var result string
	if returnType != "void" {
		result = g.nextTemp()
		g.write("  %s = ", result)
	} else {
		g.write("  ")
	}
	g.write("call %s %s(", returnType, fnRef)
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s %s", arg.typ, arg.ref)
	}
	g.write(")\n")
	if returnType == "void" {
		return value{typ: "void"}, nil
	}
	return value{typ: returnType, ref: result}, nil
}

func (g *Generator) emitRuntimeCallExpression(expr *ast.RuntimeCallExpression) (value, error) {
	switch expr.Name {
	case "runtime.PrintlnString":
		if len(expr.Arguments) != 1 {
			return value{}, fmt.Errorf("@runtime.PrintlnString expects 1 argument")
		}
		arg, err := g.emitExpression(expr.Arguments[0])
		if err != nil {
			return value{}, err
		}
		if arg.typ != "string" {
			return value{}, fmt.Errorf("@runtime.PrintlnString currently expects string")
		}
		g.needsPuts = true
		g.write("  call i32 @puts(ptr %s)\n", arg.ref)
		return value{typ: "void"}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support @%s yet", expr.Name)
	}
}

func (g *Generator) emitReturn(stmt *ast.ReturnStatement) error {
	if stmt.Value == nil {
		if g.returnType == "" || g.returnType == "void" {
			g.write("  ret void\n")
		} else {
			g.write("  ret %s %s\n", g.returnType, llvmZeroValue(g.returnType))
		}
		g.blockOpen = false
		return nil
	}

	value, err := g.emitExpression(stmt.Value)
	if err != nil {
		if g.returnType == llvmDecimalType {
			if decimal, ok := decimalLiteralValue(stmt.Value, true); ok {
				value = g.emitDecimalValue(decimal)
			} else {
				return err
			}
		} else {
			return err
		}
	}
	if value.typ != g.returnType && g.returnType != "" {
		coerced, err := g.coerceValue(value, g.returnType)
		if err != nil {
			return err
		}
		value = coerced
	}
	g.write("  ret %s %s\n", value.typ, value.ref)
	g.blockOpen = false
	return nil
}

func (g *Generator) emitIf(stmt *ast.IfStatement) error {
	if stmt.Condition == nil || stmt.Consequence == nil {
		return fmt.Errorf("emit-llvm requires complete if statements")
	}

	condition, err := g.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}
	if condition.typ != "i1" {
		return fmt.Errorf("emit-llvm if condition must be bool")
	}

	thenLabel := g.nextLabel("if.then")
	endLabel := g.nextLabel("if.end")

	falseLabel := endLabel
	elseLabel := ""
	if stmt.Alternative != nil {
		elseLabel = g.nextLabel("if.else")
		falseLabel = elseLabel
	}
	g.write("  br i1 %s, label %%%s, label %%%s\n\n", condition.ref, thenLabel, falseLabel)
	g.blockOpen = false

	g.write("%s:\n", thenLabel)
	g.blockOpen = true
	if err := g.emitBlock(stmt.Consequence); err != nil {
		return err
	}
	if g.blockOpen {
		g.write("  br label %%%s\n\n", endLabel)
		g.blockOpen = false
	}

	if stmt.Alternative != nil {
		g.write("\n%s:\n", elseLabel)
		g.blockOpen = true
		if err := g.emitBlock(stmt.Alternative); err != nil {
			return err
		}
		if g.blockOpen {
			g.write("  br label %%%s\n\n", endLabel)
			g.blockOpen = false
		}
	}

	g.write("\n%s:\n", endLabel)
	g.blockOpen = true
	return nil
}
