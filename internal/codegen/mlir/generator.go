package mlir

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
)

type Generator struct {
	out            strings.Builder
	activeOut      *strings.Builder
	prologue       strings.Builder
	globals        strings.Builder
	label          int
	temp           int
	returnType     string
	returnUnsigned bool
	targetTriple   string
	blockOpen      bool
	locals         map[string]local
	functions      map[string]*ast.FunctionDeclaration
	structs        map[string]*mlirStruct
	loops          []loopContext
	stringID       int
}

type value struct {
	typ        string
	ref        string
	len        string
	structName string
	unsigned   bool
}

type local struct {
	typ        string
	ptr        string
	lenPtr     string
	ref        string
	len        string
	structName string
	unsigned   bool
	direct     bool
}

type mlirStruct struct {
	name        string
	declaration *ast.StructType
	fields      []mlirStructField
	typ         string
	resolving   bool
}

type mlirStructField struct {
	name       string
	typ        string
	structName string
	unsigned   bool
}

type loopContext struct {
	breakLabel    string
	continueLabel string
}

const (
	mlirDecimalType    = "!llvm.struct<(i64, i32)>"
	mlirDecimal128Type = "!llvm.struct<(i128, i32)>"
)

func GenerateWithTriple(program *ast.Program, triple string) (string, error) {
	g := &Generator{targetTriple: triple}
	return g.Generate(program)
}

func (g *Generator) Generate(program *ast.Program) (string, error) {
	if err := validateEntrypoint(program); err != nil {
		return "", err
	}
	g.functions = map[string]*ast.FunctionDeclaration{}
	g.structs = map[string]*mlirStruct{}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name != nil {
				g.functions[stmt.Name.Value] = stmt
			}
		case *ast.TypeDeclStatement:
			if stmt.Name != nil && stmt.StructType != nil {
				if len(stmt.GenericParameters) > 0 {
					return "", fmt.Errorf("emit-mlir does not support generic struct %s yet", stmt.Name.Value)
				}
				g.structs[stmt.Name.Value] = &mlirStruct{
					name:        stmt.Name.Value,
					declaration: stmt.StructType,
				}
			}
		}
	}
	for name := range g.structs {
		if _, err := g.resolveStruct(name); err != nil {
			return "", err
		}
	}

	g.write("module attributes {llvm.target_triple = %q} {\n", g.targetTriple)
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil {
			continue
		}
		if err := g.emitFunction(fn); err != nil {
			return "", err
		}
	}
	if g.globals.Len() > 0 {
		g.out.WriteString(g.globals.String())
	}
	g.write("}\n")
	return g.out.String(), nil
}

func (g *Generator) emitFunction(fn *ast.FunctionDeclaration) error {
	var body strings.Builder
	var signature strings.Builder
	previousActiveOut := g.activeOut
	g.activeOut = &body
	g.prologue.Reset()
	defer func() {
		g.activeOut = previousActiveOut
	}()

	returnType := g.mlirType(fn.ReturnType)
	if fn.Name.Value == "main" && returnType == "void" {
		returnType = "i32"
	}
	previousReturnType := g.returnType
	previousReturnUnsigned := g.returnUnsigned
	g.returnType = returnType
	g.returnUnsigned = isUnsignedTypeReference(fn.ReturnType)
	defer func() {
		g.returnType = previousReturnType
		g.returnUnsigned = previousReturnUnsigned
	}()
	previousLocals := g.locals
	g.locals = map[string]local{}

	fmt.Fprintf(&signature, "  llvm.func @%s(", fn.Name.Value)
	writtenParams := 0
	for _, param := range fn.Parameters {
		if writtenParams > 0 {
			fmt.Fprintf(&signature, ", ")
		}
		paramType := g.mlirType(param.Type)
		paramUnsigned := isUnsignedTypeReference(param.Type)
		if paramType == "string" {
			fmt.Fprintf(&signature, "%%%s.ptr: !llvm.ptr, %%%s.len: i64", param.Name.Value, param.Name.Value)
			if param.Name != nil {
				g.locals[param.Name.Value] = local{typ: "string", ref: "%" + param.Name.Value + ".ptr", len: "%" + param.Name.Value + ".len", direct: true}
			}
			writtenParams += 2
			continue
		}
		fmt.Fprintf(&signature, "%%%s: %s", param.Name.Value, paramType)
		if param.Name != nil {
			g.locals[param.Name.Value] = local{
				typ:        paramType,
				ref:        "%" + param.Name.Value,
				structName: g.structName(param.Type),
				unsigned:   paramUnsigned,
				direct:     true,
			}
		}
		writtenParams++
	}
	if returnType == "void" {
		signature.WriteString(") {\n")
	} else {
		fmt.Fprintf(&signature, ") -> %s {\n", returnType)
	}
	g.blockOpen = true

	if fn.Body != nil {
		for _, stmt := range fn.Body.Statements {
			if err := g.emitStatement(stmt); err != nil {
				g.locals = previousLocals
				return err
			}
		}
	}

	if g.blockOpen {
		if returnType == "void" {
			g.write("    llvm.return\n")
		} else {
			zero := g.zeroValue(returnType)
			g.write("    llvm.return %s : %s\n", zero.ref, zero.typ)
		}
		g.blockOpen = false
	}
	g.activeOut = previousActiveOut
	g.out.WriteString(signature.String())
	g.out.WriteString(g.prologue.String())
	g.out.WriteString(body.String())
	g.out.WriteString("  }\n")
	g.locals = previousLocals
	return nil
}

func (g *Generator) emitStatement(stmt ast.Statement) error {
	if !g.blockOpen {
		return nil
	}
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
	case *ast.ReturnStatement:
		return g.emitReturn(stmt)
	case *ast.IfStatement:
		return g.emitIf(stmt)
	case *ast.ForStatement:
		return g.emitFor(stmt)
	case *ast.WhileStatement:
		return g.emitWhile(stmt)
	case *ast.SwitchStatement:
		return g.emitSwitch(stmt)
	case *ast.BreakStatement:
		return g.emitBreak()
	case *ast.ContinueStatement:
		return g.emitContinue()
	case *ast.ExpressionStatement:
		if stmt.Expression != nil {
			_, err := g.emitExpression(stmt.Expression)
			return err
		}
		return nil
	default:
		return fmt.Errorf("emit-mlir does not support %T yet", stmt)
	}
}

func (g *Generator) emitLet(stmt *ast.LetStatement) error {
	if stmt.Name == nil {
		return fmt.Errorf("emit-mlir let missing name")
	}
	if stmt.Address != nil || stmt.AddressToken.Lexeme != "" {
		return fmt.Errorf("emit-mlir does not support addressed let declarations yet")
	}

	targetType := ""
	targetStructName := ""
	targetUnsigned := false
	if stmt.Type != nil {
		targetType = g.mlirType(stmt.Type)
		targetStructName = g.structName(stmt.Type)
		targetUnsigned = isUnsignedTypeReference(stmt.Type)
	}
	var initial *value
	if stmt.Value != nil {
		val, err := g.emitExpressionForTargetUnsigned(stmt.Value, targetType, targetUnsigned)
		if err != nil {
			return err
		}
		if targetType == "" {
			targetType = val.typ
			targetStructName = val.structName
			targetUnsigned = val.unsigned
		}
		coerced, err := g.coerceValue(val, targetType, targetUnsigned)
		if err != nil {
			return fmt.Errorf("emit-mlir cannot initialize %s with %s", targetType, val.typ)
		}
		initial = &coerced
	}
	if targetType == "" || targetType == "void" {
		return fmt.Errorf("emit-mlir cannot determine type for local %s", stmt.Name.Value)
	}
	if targetType == "string" {
		if initial == nil {
			return fmt.Errorf("emit-mlir string local %s requires initializer", stmt.Name.Value)
		}
		ptrSlot, lenSlot := g.emitStringAlloca()
		g.write("    llvm.store %s, %s : !llvm.ptr, !llvm.ptr\n", initial.ref, ptrSlot)
		g.write("    llvm.store %s, %s : i64, !llvm.ptr\n", initial.len, lenSlot)
		g.locals[stmt.Name.Value] = local{typ: "string", ptr: ptrSlot, lenPtr: lenSlot}
		return nil
	}

	ptr := g.emitAlloca(targetType)
	g.locals[stmt.Name.Value] = local{
		typ:        targetType,
		ptr:        ptr,
		structName: targetStructName,
		unsigned:   targetUnsigned,
	}
	if initial != nil {
		g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", initial.ref, ptr, targetType)
	}
	return nil
}

func (g *Generator) emitFor(stmt *ast.ForStatement) error {
	if len(stmt.Bindings) == 0 && stmt.Iterable == nil {
		return g.emitInfiniteFor(stmt)
	}

	rangeExpr, ok := stmt.Iterable.(*ast.RangeExpression)
	if !ok {
		return fmt.Errorf("emit-mlir currently supports only range for loops")
	}
	if len(stmt.Bindings) != 1 {
		return fmt.Errorf("emit-mlir currently supports one loop binding")
	}
	return g.emitRangeFor(stmt, rangeExpr)
}

func (g *Generator) emitWhile(stmt *ast.WhileStatement) error {
	if stmt.Condition == nil || stmt.Body == nil {
		return fmt.Errorf("emit-mlir requires complete while statements")
	}

	conditionLabel := g.nextLabel("while.condition")
	bodyLabel := g.nextLabel("while.body")
	endLabel := g.nextLabel("while.end")

	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", conditionLabel)
	g.blockOpen = true
	condition, err := g.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}
	if condition.typ != "i1" {
		return fmt.Errorf("emit-mlir while condition must be bool")
	}
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition.ref, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	previousLocals := g.locals
	g.locals = copyMLIRLocals(previousLocals)
	g.pushLoop(endLabel, conditionLabel)
	for _, child := range stmt.Body.Statements {
		if err := g.emitStatement(child); err != nil {
			g.popLoop()
			g.locals = previousLocals
			return err
		}
	}
	g.popLoop()
	g.locals = previousLocals
	if g.blockOpen {
		g.write("    llvm.br ^%s\n", conditionLabel)
		g.blockOpen = false
	}

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitInfiniteFor(stmt *ast.ForStatement) error {
	if stmt.Body == nil {
		return fmt.Errorf("emit-mlir requires complete for statements")
	}

	bodyLabel := g.nextLabel("for.body")
	endLabel := g.nextLabel("for.end")

	g.write("    llvm.br ^%s\n", bodyLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	previousLocals := g.locals
	g.locals = copyMLIRLocals(previousLocals)
	g.pushLoop(endLabel, bodyLabel)
	for _, child := range stmt.Body.Statements {
		if err := g.emitStatement(child); err != nil {
			g.popLoop()
			g.locals = previousLocals
			return err
		}
	}
	g.popLoop()
	g.locals = previousLocals
	if g.blockOpen {
		g.write("    llvm.br ^%s\n", bodyLabel)
		g.blockOpen = false
	}

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitRangeFor(stmt *ast.ForStatement, rangeExpr *ast.RangeExpression) error {
	if rangeExpr.Start == nil || rangeExpr.End == nil || stmt.Body == nil {
		return fmt.Errorf("emit-mlir range for requires finite range and body")
	}

	start, err := g.emitExpression(rangeExpr.Start)
	if err != nil {
		return err
	}
	end, err := g.emitExpressionForTargetUnsigned(rangeExpr.End, start.typ, start.unsigned)
	if err != nil {
		return err
	}
	end, err = g.coerceValue(end, start.typ, start.unsigned)
	if err != nil {
		return fmt.Errorf("emit-mlir range bounds must have same type")
	}
	if !isMLIRIntegerType(start.typ) && !isMLIRFloatType(start.typ) {
		return fmt.Errorf("emit-mlir range for currently supports integer and float bounds")
	}

	var step value
	explicitStep := stmt.Step != nil
	if stmt.Step != nil {
		step, err = g.emitExpressionForTargetUnsigned(stmt.Step, start.typ, start.unsigned)
		if err != nil {
			return err
		}
		step, err = g.coerceValue(step, start.typ, start.unsigned)
		if err != nil {
			return fmt.Errorf("emit-mlir range step must match range bounds")
		}
	}

	conditionLabel := g.nextLabel("for.condition")
	bodyLabel := g.nextLabel("for.body")
	nextLabel := g.nextLabel("for.next")
	endLabel := g.nextLabel("for.end")

	loopPtr := g.emitAlloca(start.typ)
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", start.ref, loopPtr, start.typ)
	descending, err := g.emitOrderedPredicate(">", start, end)
	if err != nil {
		return err
	}
	if !explicitStep {
		positiveStep, err := g.emitNumericOne(start.typ, false)
		if err != nil {
			return err
		}
		negativeStep, err := g.emitNumericOne(start.typ, true)
		if err != nil {
			return err
		}
		stepRef := g.nextTemp()
		g.write("    %s = llvm.select %s, %s, %s : i1, %s\n", stepRef, descending.ref, negativeStep.ref, positiveStep.ref, start.typ)
		step = value{typ: start.typ, ref: stepRef, unsigned: start.unsigned}
	}

	previousLocals := g.locals
	g.locals = copyMLIRLocals(previousLocals)
	defer func() {
		g.locals = previousLocals
	}()
	binding := stmt.Bindings[0]
	if !binding.Discard {
		g.locals[binding.Name] = local{typ: start.typ, ptr: loopPtr, unsigned: start.unsigned}
	}

	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", conditionLabel)
	g.blockOpen = true
	current, err := g.loadLocal(local{typ: start.typ, ptr: loopPtr, unsigned: start.unsigned})
	if err != nil {
		return err
	}
	ascendingOperator := "<="
	descendingOperator := ">="
	if rangeExpr.Exclusive {
		ascendingOperator = "<"
		descendingOperator = ">"
	}
	ascendingCondition, err := g.emitOrderedPredicate(ascendingOperator, current, end)
	if err != nil {
		return err
	}
	descendingCondition, err := g.emitOrderedPredicate(descendingOperator, current, end)
	if err != nil {
		return err
	}
	condition := g.nextTemp()
	g.write("    %s = llvm.select %s, %s, %s : i1, i1\n", condition, descending.ref, descendingCondition.ref, ascendingCondition.ref)
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	g.pushLoop(endLabel, nextLabel)
	for _, child := range stmt.Body.Statements {
		if err := g.emitStatement(child); err != nil {
			g.popLoop()
			return err
		}
	}
	g.popLoop()
	if g.blockOpen {
		g.write("    llvm.br ^%s\n", nextLabel)
		g.blockOpen = false
	}

	g.write("  ^%s:\n", nextLabel)
	g.blockOpen = true
	loaded, err := g.loadLocal(local{typ: start.typ, ptr: loopPtr, unsigned: start.unsigned})
	if err != nil {
		return err
	}
	incremented, err := g.emitNumericBinary("add", "fadd", loaded, step)
	if err != nil {
		return err
	}
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", incremented.ref, loopPtr, start.typ)
	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitBreak() error {
	if len(g.loops) == 0 {
		return fmt.Errorf("emit-mlir break outside loop")
	}
	ctx := g.loops[len(g.loops)-1]
	g.write("    llvm.br ^%s\n", ctx.breakLabel)
	g.blockOpen = false
	return nil
}

func (g *Generator) emitContinue() error {
	if len(g.loops) == 0 {
		return fmt.Errorf("emit-mlir continue outside loop")
	}
	ctx := g.loops[len(g.loops)-1]
	g.write("    llvm.br ^%s\n", ctx.continueLabel)
	g.blockOpen = false
	return nil
}

func (g *Generator) pushLoop(breakLabel string, continueLabel string) {
	g.loops = append(g.loops, loopContext{breakLabel: breakLabel, continueLabel: continueLabel})
}

func (g *Generator) popLoop() {
	g.loops = g.loops[:len(g.loops)-1]
}

func (g *Generator) emitAssignment(stmt *ast.AssignmentStatement) error {
	ident, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("emit-mlir only supports identifier assignment targets for now")
	}
	slot, ok := g.locals[ident.Value]
	if !ok {
		return fmt.Errorf("emit-mlir unknown local %s", ident.Value)
	}
	if slot.direct {
		return fmt.Errorf("emit-mlir cannot assign to parameter %s", ident.Value)
	}
	if slot.typ == "string" {
		return fmt.Errorf("emit-mlir does not support string assignment yet")
	}
	if slot.ptr == "" {
		return fmt.Errorf("emit-mlir local %s is not assignable", ident.Value)
	}

	val, err := g.emitExpressionForTargetUnsigned(stmt.Value, slot.typ, slot.unsigned)
	if err != nil {
		return err
	}
	val, err = g.coerceValue(val, slot.typ, slot.unsigned)
	if err != nil {
		return fmt.Errorf("emit-mlir cannot assign %s to %s", val.typ, slot.typ)
	}

	if stmt.Operator != "=" {
		current, err := g.loadLocal(slot)
		if err != nil {
			return err
		}
		val, err = g.emitAssignmentOperation(stmt.Operator, current, val)
		if err != nil {
			return err
		}
	}

	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", val.ref, slot.ptr, slot.typ)
	return nil
}

func (g *Generator) loadLocal(slot local) (value, error) {
	if slot.direct {
		return value{typ: slot.typ, ref: slot.ref, len: slot.len, structName: slot.structName, unsigned: slot.unsigned}, nil
	}
	if slot.typ == "string" {
		if slot.ptr == "" || slot.lenPtr == "" {
			return value{}, fmt.Errorf("emit-mlir string local is not loadable")
		}
		ptr := g.nextTemp()
		g.write("    %s = llvm.load %s : !llvm.ptr -> !llvm.ptr\n", ptr, slot.ptr)
		lenValue := g.nextTemp()
		g.write("    %s = llvm.load %s : !llvm.ptr -> i64\n", lenValue, slot.lenPtr)
		return value{typ: "string", ref: ptr, len: lenValue}, nil
	}
	if slot.ptr == "" {
		return value{}, fmt.Errorf("emit-mlir local is not loadable")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.load %s : !llvm.ptr -> %s\n", tmp, slot.ptr, slot.typ)
	return value{typ: slot.typ, ref: tmp, structName: slot.structName, unsigned: slot.unsigned}, nil
}

func (g *Generator) emitAssignmentOperation(operator string, left value, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir assignment operator requires matching operand types")
	}
	switch operator {
	case "+=":
		return g.emitNumericBinary("add", "fadd", left, right)
	case "-=":
		return g.emitNumericBinary("sub", "fsub", left, right)
	case "*=":
		return g.emitNumericBinary("mul", "fmul", left, right)
	case "/=":
		return g.emitIntegerOrFloatBinary(signedIntegerOp("sdiv", "udiv", left.unsigned), "fdiv", left, right)
	case "%=":
		return g.emitIntegerBinary(signedIntegerOp("srem", "urem", left.unsigned), left, right)
	case "&=":
		return g.emitIntegerBinary("and", left, right)
	case "|=":
		return g.emitIntegerBinary("or", left, right)
	case "^=":
		return g.emitIntegerBinary("xor", left, right)
	case "<<=":
		return g.emitIntegerBinary("shl", left, right)
	case ">>=":
		return g.emitIntegerBinary(signedIntegerOp("ashr", "lshr", left.unsigned), left, right)
	default:
		return value{}, fmt.Errorf("emit-mlir does not support assignment operator %q yet", operator)
	}
}

func (g *Generator) emitReturn(stmt *ast.ReturnStatement) error {
	if stmt.Value == nil {
		if g.returnType == "void" {
			g.write("    llvm.return\n")
		} else {
			zero := g.zeroValue(g.returnType)
			g.write("    llvm.return %s : %s\n", zero.ref, zero.typ)
		}
		g.blockOpen = false
		return nil
	}
	val, err := g.emitExpressionForTargetUnsigned(stmt.Value, g.returnType, g.returnUnsigned)
	if err != nil {
		return err
	}
	if val.typ != g.returnType && g.returnType != "void" {
		coerced, err := g.coerceValue(val, g.returnType, g.returnUnsigned)
		if err != nil {
			return fmt.Errorf("emit-mlir cannot return %s from %s function", val.typ, g.returnType)
		}
		val = coerced
	}
	g.write("    llvm.return %s : %s\n", val.ref, val.typ)
	g.blockOpen = false
	return nil
}

func (g *Generator) emitIf(stmt *ast.IfStatement) error {
	if stmt.Condition == nil || stmt.Consequence == nil {
		return fmt.Errorf("emit-mlir requires complete if statements")
	}
	condition, err := g.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}
	if condition.typ != "i1" {
		return fmt.Errorf("emit-mlir if condition must be bool")
	}

	thenLabel := g.nextLabel("if.then")
	endLabel := g.nextLabel("if.end")
	falseLabel := endLabel
	elseLabel := ""
	if stmt.Alternative != nil {
		elseLabel = g.nextLabel("if.else")
		falseLabel = elseLabel
	}
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition.ref, thenLabel, falseLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", thenLabel)
	g.blockOpen = true
	for _, child := range stmt.Consequence.Statements {
		if err := g.emitStatement(child); err != nil {
			return err
		}
	}
	thenFallsThrough := g.blockOpen
	if thenFallsThrough {
		g.write("    llvm.br ^%s\n", endLabel)
		g.blockOpen = false
	}

	elseFallsThrough := stmt.Alternative == nil
	if stmt.Alternative != nil {
		g.write("  ^%s:\n", elseLabel)
		g.blockOpen = true
		for _, child := range stmt.Alternative.Statements {
			if err := g.emitStatement(child); err != nil {
				return err
			}
		}
		elseFallsThrough = g.blockOpen
		if elseFallsThrough {
			g.write("    llvm.br ^%s\n", endLabel)
			g.blockOpen = false
		}
	}

	if !thenFallsThrough && !elseFallsThrough {
		g.blockOpen = false
		return nil
	}

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitSwitch(stmt *ast.SwitchStatement) error {
	if stmt.Subject == nil {
		return fmt.Errorf("emit-mlir does not support subjectless switch yet")
	}
	if err := validateMLIRSwitchCases(stmt); err != nil {
		return err
	}

	subject, err := g.emitExpression(stmt.Subject)
	if err != nil {
		return err
	}
	if subject.typ != "i1" && !isMLIRIntegerType(subject.typ) {
		return fmt.Errorf("emit-mlir switch currently supports bool and integer subjects, got %s", subject.typ)
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
		if err := g.emitSwitchBody(clause.Body, endLabel); err != nil {
			g.locals = outerLocals
			return err
		}
	}

	if stmt.Default != nil {
		g.write("  ^%s:\n", defaultLabel)
		g.blockOpen = true
		g.locals = copyMLIRLocals(outerLocals)
		if err := g.emitSwitchBody(stmt.Default.Body, endLabel); err != nil {
			g.locals = outerLocals
			return err
		}
	}
	g.locals = outerLocals

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func validateMLIRSwitchCases(stmt *ast.SwitchStatement) error {
	for _, clause := range stmt.Cases {
		if clause == nil {
			return fmt.Errorf("emit-mlir switch contains nil case")
		}
		for _, item := range clause.Items {
			if _, ok := item.(*ast.SwitchValueCase); !ok {
				return fmt.Errorf("emit-mlir switch currently supports only value cases, got %T", item)
			}
		}
		if switchBodyHasFallthrough(clause.Body) {
			return fmt.Errorf("emit-mlir does not support switch fallthrough yet")
		}
	}
	if stmt.Default != nil && switchBodyHasFallthrough(stmt.Default.Body) {
		return fmt.Errorf("emit-mlir does not support switch fallthrough yet")
	}
	return nil
}

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

func (g *Generator) emitBooleanOr(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-mlir boolean or expects bool operands")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.or %s, %s : i1\n", tmp, left.ref, right.ref)
	return value{typ: "i1", ref: tmp}, nil
}

func (g *Generator) emitSwitchBody(block *ast.BlockStatement, endLabel string) error {
	if block != nil {
		for _, stmt := range block.Statements {
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

func (g *Generator) emitExpression(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return g.emitIdentifier(expr)
	case *ast.IntegerLiteral:
		if expr.Suffix() == "d" {
			return g.emitDecimalLiteral(expr, mlirDecimalType)
		}
		parsed, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
		if !ok {
			return value{}, fmt.Errorf("emit-mlir could not parse integer %q", expr.Token.Lexeme)
		}
		typ := inferredIntegerLiteralType(parsed, expr.Suffix())
		if expr.Suffix() == "u" {
			typ = inferredUnsignedIntegerLiteralType(parsed)
		}
		return g.emitIntegerConstantUnsigned(parsed.String(), typ, expr.Suffix() == "u"), nil
	case *ast.FloatLiteral:
		if expr.Suffix() != "f" {
			return g.emitDecimalLiteral(expr, mlirDecimalType)
		}
		return g.emitFloatConstant(expr.Token.Lexeme, "f64")
	case *ast.BooleanLiteral:
		return g.emitBoolConstant(expr.Value), nil
	case *ast.StringLiteral:
		return g.emitStringLiteral(expr)
	case *ast.PrefixExpression:
		return g.emitPrefixExpression(expr)
	case *ast.InfixExpression:
		return g.emitInfixExpression(expr)
	case *ast.CallExpression:
		return g.emitCallExpression(expr)
	case *ast.StructLiteral:
		return g.emitStructLiteral(expr)
	case *ast.MemberExpression:
		return g.emitMemberExpression(expr)
	default:
		return value{}, fmt.Errorf("emit-mlir does not support expression %T yet", expr)
	}
}

func (g *Generator) emitExpressionForTarget(expr ast.Expression, targetType string) (value, error) {
	return g.emitExpressionForTargetUnsigned(expr, targetType, false)
}

func (g *Generator) emitExpressionForTargetUnsigned(expr ast.Expression, targetType string, targetUnsigned bool) (value, error) {
	if targetType == "" {
		return g.emitExpression(expr)
	}
	if isMLIRDecimalType(targetType) {
		if _, _, ok := decimalLiteralParts(expr); ok {
			return g.emitDecimalLiteral(expr, targetType)
		}
	}
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		if isMLIRIntegerType(targetType) {
			parsed, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
			if !ok {
				return value{}, fmt.Errorf("emit-mlir could not parse integer %q", expr.Token.Lexeme)
			}
			return g.emitIntegerConstantUnsigned(parsed.String(), targetType, targetUnsigned || expr.Suffix() == "u"), nil
		}
		if isMLIRFloatType(targetType) {
			parsed, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
			if !ok {
				return value{}, fmt.Errorf("emit-mlir could not parse integer %q", expr.Token.Lexeme)
			}
			return g.emitFloatConstant(parsed.String(), targetType)
		}
	case *ast.FloatLiteral:
		if isMLIRFloatType(targetType) {
			return g.emitFloatConstant(expr.Token.Lexeme, targetType)
		}
	}
	return g.emitExpression(expr)
}

func (g *Generator) emitIdentifier(expr *ast.Identifier) (value, error) {
	slot, ok := g.locals[expr.Value]
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown identifier %s", expr.Value)
	}
	if slot.direct {
		return value{typ: slot.typ, ref: slot.ref, len: slot.len, structName: slot.structName, unsigned: slot.unsigned}, nil
	}
	return g.loadLocal(slot)
}

func (g *Generator) emitStructLiteral(expr *ast.StructLiteral) (value, error) {
	if expr.Type == nil {
		return value{}, fmt.Errorf("emit-mlir struct literal is missing its type")
	}
	info, ok := g.structs[expr.Type.Name]
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown struct type %s", expr.Type.Name)
	}
	if len(expr.Type.TypeArgs) > 0 {
		return value{}, fmt.Errorf("emit-mlir does not support generic struct literal %s yet", expr.Type.Name)
	}

	for _, literalField := range expr.Fields {
		if literalField != nil && literalField.Spread {
			return value{}, fmt.Errorf("emit-mlir does not support struct spread yet")
		}
	}

	aggregate := g.nextTemp()
	g.write("    %s = llvm.mlir.undef : %s\n", aggregate, info.typ)
	for _, literalField := range expr.Fields {
		if literalField == nil || literalField.Name == nil || literalField.Value == nil {
			return value{}, fmt.Errorf("emit-mlir struct literal %s contains an incomplete field", info.name)
		}
		index, field, ok := info.field(literalField.Name.Value)
		if !ok {
			return value{}, fmt.Errorf("emit-mlir unknown field %s.%s", info.name, literalField.Name.Value)
		}
		fieldValue, err := g.emitExpressionForTargetUnsigned(literalField.Value, field.typ, field.unsigned)
		if err != nil {
			return value{}, err
		}
		fieldValue, err = g.coerceValue(fieldValue, field.typ, field.unsigned)
		if err != nil {
			return value{}, fmt.Errorf("emit-mlir cannot initialize field %s.%s with %s", info.name, field.name, fieldValue.typ)
		}
		next := g.nextTemp()
		g.write("    %s = llvm.insertvalue %s, %s[%d] : %s\n", next, fieldValue.ref, aggregate, index, info.typ)
		aggregate = next
	}
	return value{typ: info.typ, ref: aggregate, structName: info.name}, nil
}

func (g *Generator) emitMemberExpression(expr *ast.MemberExpression) (value, error) {
	if expr.Object == nil || expr.Property == nil {
		return value{}, fmt.Errorf("emit-mlir member expression is incomplete")
	}
	object, err := g.emitExpression(expr.Object)
	if err != nil {
		return value{}, err
	}
	if object.structName == "" {
		return value{}, fmt.Errorf("emit-mlir member access currently requires a struct value")
	}
	info, ok := g.structs[object.structName]
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown struct type %s", object.structName)
	}
	index, field, ok := info.field(expr.Property.Value)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown field %s.%s", info.name, expr.Property.Value)
	}
	result := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[%d] : %s\n", result, object.ref, index, info.typ)
	return value{
		typ:        field.typ,
		ref:        result,
		structName: field.structName,
		unsigned:   field.unsigned,
	}, nil
}

func (g *Generator) emitPrefixExpression(expr *ast.PrefixExpression) (value, error) {
	if expr.Operator == "-" && isDefaultDecimalLiteral(expr.Right) {
		if _, _, ok := decimalLiteralParts(expr); ok {
			return g.emitDecimalLiteral(expr, mlirDecimalType)
		}
	}
	if expr.Operator == "-" {
		if literal, ok := expr.Right.(*ast.IntegerLiteral); ok {
			parsed, parseOK := ast.ParseIntegerLiteralLexeme(literal.Token.Lexeme)
			if !parseOK {
				return value{}, fmt.Errorf("emit-mlir could not parse integer %q", literal.Token.Lexeme)
			}
			if literal.Suffix() == "u" {
				return value{}, fmt.Errorf("emit-mlir cannot negate unsigned integer literal %q", literal.Token.Lexeme)
			}
			parsed.Neg(parsed)
			typ := inferredIntegerLiteralType(parsed, literal.Suffix())
			return g.emitIntegerConstant(parsed.String(), typ), nil
		}
	}

	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	switch expr.Operator {
	case "-":
		if !isMLIRIntegerType(right.typ) {
			return value{}, fmt.Errorf("emit-mlir unary - expects a signed integer")
		}
		zero := g.emitIntegerConstant("0", right.typ)
		return g.emitIntegerBinary("sub", zero, right)
	case "!":
		if right.typ != "i1" {
			return value{}, fmt.Errorf("emit-mlir unary ! currently expects bool")
		}
		one := g.emitBoolConstant(true)
		tmp := g.nextTemp()
		g.write("    %s = llvm.xor %s, %s : i1\n", tmp, right.ref, one.ref)
		return value{typ: "i1", ref: tmp}, nil
	case "~":
		if !isMLIRIntegerType(right.typ) {
			return value{}, fmt.Errorf("emit-mlir unary ~ expects an integer")
		}
		allOnes := g.emitIntegerConstant("-1", right.typ)
		return g.emitIntegerBinary("xor", right, allOnes)
	default:
		return value{}, fmt.Errorf("emit-mlir does not support prefix operator %q yet", expr.Operator)
	}
}

func isDefaultDecimalLiteral(expr ast.Expression) bool {
	switch literal := expr.(type) {
	case *ast.IntegerLiteral:
		return literal.Suffix() == "d"
	case *ast.FloatLiteral:
		return literal.Suffix() != "f"
	default:
		return false
	}
}

func (g *Generator) emitInfixExpression(expr *ast.InfixExpression) (value, error) {
	if expr.Operator == "&&" || expr.Operator == "||" {
		return g.emitShortCircuitExpression(expr)
	}
	if expr.Operator == "in" {
		return g.emitInExpression(expr)
	}

	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	right, err := g.emitExpressionForTargetUnsigned(expr.Right, left.typ, left.unsigned)
	if err != nil {
		return value{}, err
	}
	if left.typ != right.typ {
		coerced, coerceErr := g.coerceValue(right, left.typ, left.unsigned)
		if coerceErr != nil {
			return value{}, fmt.Errorf("emit-mlir binary operator requires matching operand types")
		}
		right = coerced
	}
	switch expr.Operator {
	case "+":
		return g.emitNumericBinary("add", "fadd", left, right)
	case "-":
		return g.emitNumericBinary("sub", "fsub", left, right)
	case "*":
		return g.emitNumericBinary("mul", "fmul", left, right)
	case "/":
		return g.emitIntegerOrFloatBinary(signedIntegerOp("sdiv", "udiv", left.unsigned), "fdiv", left, right)
	case "%":
		return g.emitIntegerBinary(signedIntegerOp("srem", "urem", left.unsigned), left, right)
	case "&":
		return g.emitIntegerBinary("and", left, right)
	case "|":
		return g.emitIntegerBinary("or", left, right)
	case "^":
		return g.emitIntegerBinary("xor", left, right)
	case "<<":
		return g.emitIntegerBinary("shl", left, right)
	case ">>":
		return g.emitIntegerBinary(signedIntegerOp("ashr", "lshr", left.unsigned), left, right)
	case "==", "!=", "<", "<=", ">", ">=":
		return g.emitIntegerCompare(expr.Operator, left, right)
	default:
		return value{}, fmt.Errorf("emit-mlir does not support operator %q yet", expr.Operator)
	}
}

func (g *Generator) emitInExpression(expr *ast.InfixExpression) (value, error) {
	if expr.Left == nil {
		return value{}, fmt.Errorf("emit-mlir in expression missing left operand")
	}
	rangeExpr, ok := expr.Right.(*ast.RangeExpression)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir in currently requires a range expression")
	}
	if rangeExpr.Start == nil || rangeExpr.End == nil {
		return value{}, fmt.Errorf("emit-mlir in currently requires finite range bounds")
	}

	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	if !isMLIRIntegerType(left.typ) && !isMLIRFloatType(left.typ) {
		return value{}, fmt.Errorf("emit-mlir in currently supports integer and float values")
	}

	start, err := g.emitExpressionForTargetUnsigned(rangeExpr.Start, left.typ, left.unsigned)
	if err != nil {
		return value{}, err
	}
	start, err = g.coerceValue(start, left.typ, left.unsigned)
	if err != nil {
		return value{}, fmt.Errorf("emit-mlir range lower bound must match tested value")
	}
	end, err := g.emitExpressionForTargetUnsigned(rangeExpr.End, left.typ, left.unsigned)
	if err != nil {
		return value{}, err
	}
	end, err = g.coerceValue(end, left.typ, left.unsigned)
	if err != nil {
		return value{}, fmt.Errorf("emit-mlir range upper bound must match tested value")
	}

	lower, err := g.emitOrderedPredicate(">=", left, start)
	if err != nil {
		return value{}, err
	}
	upperOperator := "<="
	if rangeExpr.Exclusive {
		upperOperator = "<"
	}
	upper, err := g.emitOrderedPredicate(upperOperator, left, end)
	if err != nil {
		return value{}, err
	}
	return g.emitBooleanAnd(lower, upper)
}

func (g *Generator) emitShortCircuitExpression(expr *ast.InfixExpression) (value, error) {
	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	if left.typ != "i1" {
		return value{}, fmt.Errorf("emit-mlir %s expects bool operands", expr.Operator)
	}

	resultPtr := g.emitAlloca("i1")

	rightLabel := g.nextLabel("logic.right")
	endLabel := g.nextLabel("logic.end")
	g.write("    llvm.store %s, %s : i1, !llvm.ptr\n", left.ref, resultPtr)
	switch expr.Operator {
	case "&&":
		g.write("    llvm.cond_br %s, ^%s, ^%s\n", left.ref, rightLabel, endLabel)
	case "||":
		g.write("    llvm.cond_br %s, ^%s, ^%s\n", left.ref, endLabel, rightLabel)
	default:
		return value{}, fmt.Errorf("emit-mlir does not support operator %q yet", expr.Operator)
	}
	g.blockOpen = false

	g.write("  ^%s:\n", rightLabel)
	g.blockOpen = true
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	if right.typ != "i1" {
		return value{}, fmt.Errorf("emit-mlir %s expects bool operands", expr.Operator)
	}
	g.write("    llvm.store %s, %s : i1, !llvm.ptr\n", right.ref, resultPtr)
	g.write("    llvm.br ^%s\n", endLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	result, err := g.loadLocal(local{typ: "i1", ptr: resultPtr})
	if err != nil {
		return value{}, err
	}
	return result, nil
}

func (g *Generator) emitCallExpression(expr *ast.CallExpression) (value, error) {
	name := callExpressionName(expr)
	if name == "" {
		return value{}, fmt.Errorf("emit-mlir requires named function calls")
	}
	if isMLIRBuiltinNumericTypeName(name) {
		return g.emitBuiltinNumericConversion(expr, name)
	}
	fn, ok := g.functions[name]
	if !ok {
		return value{}, fmt.Errorf("emit-mlir unknown function %s", name)
	}
	if len(fn.Parameters) != len(expr.Arguments) {
		return value{}, fmt.Errorf("call %s expects %d arguments, got %d", name, len(fn.Parameters), len(expr.Arguments))
	}

	args := []value{}
	for i, argExpr := range expr.Arguments {
		param := fn.Parameters[i]
		targetType := g.mlirType(param.Type)
		targetUnsigned := isUnsignedTypeReference(param.Type)
		arg, err := g.emitExpressionForTargetUnsigned(argExpr, targetType, targetUnsigned)
		if err != nil {
			return value{}, err
		}
		arg, err = g.coerceValue(arg, targetType, targetUnsigned)
		if err != nil {
			return value{}, fmt.Errorf("argument %d to %s cannot use %s as %s", i+1, name, arg.typ, targetType)
		}
		if targetType == "string" {
			args = append(args, value{typ: "!llvm.ptr", ref: arg.ref}, value{typ: "i64", ref: arg.len})
			continue
		}
		args = append(args, arg)
	}

	returnType := g.mlirType(fn.ReturnType)
	returnUnsigned := isUnsignedTypeReference(fn.ReturnType)
	if returnType == "string" {
		return value{}, fmt.Errorf("emit-mlir does not support string return values yet")
	}
	result := ""
	if returnType != "void" {
		result = g.nextTemp()
		g.write("    %s = ", result)
	} else {
		g.write("    ")
	}
	g.write("llvm.call @%s(", name)
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s", arg.ref)
	}
	g.write(") : (")
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s", arg.typ)
	}
	if returnType == "void" {
		g.write(") -> ()\n")
		return value{typ: "void"}, nil
	}
	g.write(") -> %s\n", returnType)
	return value{
		typ:        returnType,
		ref:        result,
		structName: g.structName(fn.ReturnType),
		unsigned:   returnUnsigned,
	}, nil
}

func (g *Generator) emitIntegerBinary(op string, left value, right value) (value, error) {
	if !isMLIRIntegerType(left.typ) || left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir integer operator currently expects int")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.%s %s, %s : %s\n", tmp, op, left.ref, right.ref, left.typ)
	return value{typ: left.typ, ref: tmp, unsigned: left.unsigned}, nil
}

func (g *Generator) emitBooleanAnd(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-mlir boolean and expects bool operands")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.and %s, %s : i1\n", tmp, left.ref, right.ref)
	return value{typ: "i1", ref: tmp}, nil
}

func (g *Generator) emitNumericBinary(intOp string, floatOp string, left value, right value) (value, error) {
	return g.emitIntegerOrFloatBinary(intOp, floatOp, left, right)
}

func (g *Generator) emitIntegerOrFloatBinary(intOp string, floatOp string, left value, right value) (value, error) {
	if isMLIRFloatType(left.typ) {
		tmp := g.nextTemp()
		g.write("    %s = llvm.%s %s, %s : %s\n", tmp, floatOp, left.ref, right.ref, left.typ)
		return value{typ: left.typ, ref: tmp}, nil
	}
	return g.emitIntegerBinary(intOp, left, right)
}

func signedIntegerOp(signed string, unsigned string, useUnsigned bool) string {
	if useUnsigned {
		return unsigned
	}
	return signed
}

func (g *Generator) emitIntegerConstant(literal string, typ string) value {
	return g.emitIntegerConstantUnsigned(literal, typ, false)
}

func (g *Generator) emitIntegerConstantUnsigned(literal string, typ string, unsigned bool) value {
	tmp := g.nextTemp()
	g.write("    %s = llvm.mlir.constant(%s : %s) : %s\n", tmp, literal, typ, typ)
	return value{typ: typ, ref: tmp, unsigned: unsigned}
}

func (g *Generator) emitDecimalLiteral(expr ast.Expression, typ string) (value, error) {
	coefficient, scale, ok := decimalLiteralParts(expr)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir could not parse decimal literal %q", expr.TokenLiteral())
	}
	coefficientType, ok := decimalCoefficientType(typ)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir invalid decimal type %s", typ)
	}
	if !fitsSignedBits(coefficient, uint(integerBitWidth(coefficientType))) {
		return value{}, fmt.Errorf("emit-mlir decimal coefficient %s overflows %s", coefficient.String(), coefficientType)
	}

	number := g.emitIntegerConstant(coefficient.String(), coefficientType)
	scaleValue := g.emitIntegerConstant(strconv.FormatInt(int64(scale), 10), "i32")
	return g.emitDecimalValue(number, scaleValue, typ)
}

func decimalLiteralParts(expr ast.Expression) (*big.Int, int32, bool) {
	negative := false
	switch prefixed := expr.(type) {
	case *ast.PrefixExpression:
		if prefixed.Operator != "-" {
			return nil, 0, false
		}
		negative = true
		expr = prefixed.Right
	}

	lexeme := ""
	switch literal := expr.(type) {
	case *ast.IntegerLiteral:
		lexeme = literal.Token.Lexeme
	case *ast.FloatLiteral:
		lexeme = literal.Token.Lexeme
	default:
		return nil, 0, false
	}

	digits, suffix := ast.SplitNumericLiteralSuffix(lexeme)
	if suffix == "f" || suffix == "i" || suffix == "u" || digits == "" {
		return nil, 0, false
	}
	if strings.HasPrefix(digits, "0x") || strings.HasPrefix(digits, "0X") ||
		strings.HasPrefix(digits, "0b") || strings.HasPrefix(digits, "0B") ||
		strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0O") {
		return nil, 0, false
	}

	parts := strings.Split(digits, ".")
	if len(parts) > 2 {
		return nil, 0, false
	}
	coefficientDigits := parts[0]
	scale := 0
	if len(parts) == 2 {
		coefficientDigits += parts[1]
		scale = len(parts[1])
	}
	if coefficientDigits == "" || int64(scale) > int64(^uint32(0)>>1) {
		return nil, 0, false
	}
	coefficient, ok := new(big.Int).SetString(coefficientDigits, 10)
	if !ok {
		return nil, 0, false
	}
	if negative {
		coefficient.Neg(coefficient)
	}
	return coefficient, int32(scale), true
}

func (g *Generator) emitDecimalValue(coefficient value, scale value, typ string) (value, error) {
	coefficientType, ok := decimalCoefficientType(typ)
	if !ok || coefficient.typ != coefficientType || scale.typ != "i32" {
		return value{}, fmt.Errorf("emit-mlir invalid decimal components")
	}
	undef := g.nextTemp()
	g.write("    %s = llvm.mlir.undef : %s\n", undef, typ)
	withCoefficient := g.nextTemp()
	g.write("    %s = llvm.insertvalue %s, %s[0] : %s\n", withCoefficient, coefficient.ref, undef, typ)
	result := g.nextTemp()
	g.write("    %s = llvm.insertvalue %s, %s[1] : %s\n", result, scale.ref, withCoefficient, typ)
	return value{typ: typ, ref: result}, nil
}

func (g *Generator) emitDecimalComponents(decimal value) (value, value, error) {
	coefficientType, ok := decimalCoefficientType(decimal.typ)
	if !ok {
		return value{}, value{}, fmt.Errorf("emit-mlir expected decimal value, got %s", decimal.typ)
	}
	coefficientRef := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[0] : %s\n", coefficientRef, decimal.ref, decimal.typ)
	scaleRef := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[1] : %s\n", scaleRef, decimal.ref, decimal.typ)
	return value{typ: coefficientType, ref: coefficientRef}, value{typ: "i32", ref: scaleRef}, nil
}

func (g *Generator) emitBuiltinNumericConversion(expr *ast.CallExpression, name string) (value, error) {
	if len(expr.Arguments) != 1 {
		return value{}, fmt.Errorf("conversion to %s expects 1 argument", name)
	}
	targetType := mlirBuiltinNumericType(name)
	targetUnsigned := isUnsignedBuiltinName(name)
	source, err := g.emitExpression(expr.Arguments[0])
	if err != nil {
		return value{}, err
	}

	switch {
	case isMLIRDecimalType(targetType):
		return g.convertToDecimal(source, targetType)
	case isMLIRIntegerType(targetType) && isMLIRDecimalType(source.typ):
		return g.convertDecimalToInteger(source, targetType, targetUnsigned)
	default:
		return g.coerceValue(source, targetType, targetUnsigned)
	}
}

func (g *Generator) convertToDecimal(source value, targetType string) (value, error) {
	targetCoefficientType, ok := decimalCoefficientType(targetType)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir invalid decimal target %s", targetType)
	}
	if isMLIRIntegerType(source.typ) {
		coefficient, err := g.coerceValue(source, targetCoefficientType, false)
		if err != nil {
			return value{}, err
		}
		scale := g.emitIntegerConstant("0", "i32")
		return g.emitDecimalValue(coefficient, scale, targetType)
	}
	if isMLIRDecimalType(source.typ) {
		coefficient, scale, err := g.emitDecimalComponents(source)
		if err != nil {
			return value{}, err
		}
		coefficient, err = g.coerceValue(coefficient, targetCoefficientType, false)
		if err != nil {
			return value{}, err
		}
		return g.emitDecimalValue(coefficient, scale, targetType)
	}
	return value{}, fmt.Errorf("emit-mlir cannot convert %s to %s yet", source.typ, targetType)
}

func (g *Generator) convertDecimalToInteger(source value, targetType string, targetUnsigned bool) (value, error) {
	coefficient, scale, err := g.emitDecimalComponents(source)
	if err != nil {
		return value{}, err
	}
	coefficientPtr := g.emitAlloca(coefficient.typ)
	scalePtr := g.emitAlloca("i32")
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", coefficient.ref, coefficientPtr, coefficient.typ)
	g.write("    llvm.store %s, %s : i32, !llvm.ptr\n", scale.ref, scalePtr)

	conditionLabel := g.nextLabel("decimal.cast.condition")
	bodyLabel := g.nextLabel("decimal.cast.body")
	endLabel := g.nextLabel("decimal.cast.end")
	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", conditionLabel)
	g.blockOpen = true
	currentScale, err := g.loadLocal(local{typ: "i32", ptr: scalePtr})
	if err != nil {
		return value{}, err
	}
	zeroScale := g.emitIntegerConstant("0", "i32")
	hasFractionalDigits, err := g.emitIntegerPredicate("sgt", currentScale, zeroScale)
	if err != nil {
		return value{}, err
	}
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", hasFractionalDigits.ref, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	currentCoefficient, err := g.loadLocal(local{typ: coefficient.typ, ptr: coefficientPtr})
	if err != nil {
		return value{}, err
	}
	ten := g.emitIntegerConstant("10", coefficient.typ)
	scaled, err := g.emitIntegerBinary("sdiv", currentCoefficient, ten)
	if err != nil {
		return value{}, err
	}
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", scaled.ref, coefficientPtr, coefficient.typ)
	oneScale := g.emitIntegerConstant("1", "i32")
	nextScale, err := g.emitIntegerBinary("sub", currentScale, oneScale)
	if err != nil {
		return value{}, err
	}
	g.write("    llvm.store %s, %s : i32, !llvm.ptr\n", nextScale.ref, scalePtr)
	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	integerValue, err := g.loadLocal(local{typ: coefficient.typ, ptr: coefficientPtr})
	if err != nil {
		return value{}, err
	}
	return g.coerceValue(integerValue, targetType, targetUnsigned)
}

func (g *Generator) emitIndexConstant(literal string) value {
	tmp := g.nextTemp()
	g.write("    %s = llvm.mlir.constant(%s : i64) : i64\n", tmp, literal)
	return value{typ: "i64", ref: tmp}
}

func (g *Generator) emitStringAlloca() (ptrSlot string, lenSlot string) {
	return g.emitAlloca("!llvm.ptr"), g.emitAlloca("i64")
}

func (g *Generator) emitAlloca(elementType string) string {
	count := g.nextTemp()
	ptr := g.nextTemp()
	fmt.Fprintf(&g.prologue, "    %s = llvm.mlir.constant(1 : i64) : i64\n", count)
	fmt.Fprintf(&g.prologue, "    %s = llvm.alloca %s x %s : (i64) -> !llvm.ptr\n", ptr, count, elementType)
	return ptr
}

func (g *Generator) emitFloatConstant(lexeme string, typ string) (value, error) {
	parsed, ok := ast.ParseFloatLiteralFloat64(lexeme)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir could not parse float %q", lexeme)
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.mlir.constant(%s : %s) : %s\n", tmp, mlirFloatLiteral(parsed), typ, typ)
	return value{typ: typ, ref: tmp}, nil
}

func (g *Generator) emitBoolConstant(literal bool) value {
	tmp := g.nextTemp()
	text := "false"
	if literal {
		text = "true"
	}
	g.write("    %s = llvm.mlir.constant(%s) : i1\n", tmp, text)
	return value{typ: "i1", ref: tmp}
}

func (g *Generator) emitIntegerCompare(operator string, left value, right value) (value, error) {
	if isMLIRFloatType(left.typ) {
		return g.emitFloatCompare(operator, left, right)
	}
	if !isMLIRIntegerType(left.typ) || left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir comparison currently expects matching integer operands")
	}
	predicate := integerComparePredicate(operator, left.unsigned)
	if predicate == "" {
		return value{}, fmt.Errorf("emit-mlir unsupported integer predicate %q", operator)
	}
	return g.emitIntegerPredicate(predicate, left, right)
}

func (g *Generator) emitOrderedPredicate(operator string, left value, right value) (value, error) {
	if isMLIRFloatType(left.typ) {
		return g.emitFloatCompare(operator, left, right)
	}
	predicate := integerComparePredicate(operator, left.unsigned)
	if predicate == "" {
		return value{}, fmt.Errorf("emit-mlir unsupported ordered predicate %q", operator)
	}
	return g.emitIntegerPredicate(predicate, left, right)
}

func (g *Generator) emitIntegerPredicate(predicate string, left value, right value) (value, error) {
	if !isMLIRIntegerType(left.typ) || left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir comparison currently expects matching integer operands")
	}
	tmp := g.nextTemp()
	g.write("    %s = llvm.icmp %q %s, %s : %s\n", tmp, predicate, left.ref, right.ref, left.typ)
	return value{typ: "i1", ref: tmp}, nil
}

func integerComparePredicate(operator string, unsigned bool) string {
	switch operator {
	case "==":
		return "eq"
	case "!=":
		return "ne"
	case "<":
		if unsigned {
			return "ult"
		}
		return "slt"
	case "<=":
		if unsigned {
			return "ule"
		}
		return "sle"
	case ">":
		if unsigned {
			return "ugt"
		}
		return "sgt"
	case ">=":
		if unsigned {
			return "uge"
		}
		return "sge"
	default:
		return ""
	}
}

func (g *Generator) emitFloatCompare(operator string, left value, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-mlir comparison currently expects matching float operands")
	}
	predicate := map[string]string{
		"==": "oeq",
		"!=": "one",
		"<":  "olt",
		"<=": "ole",
		">":  "ogt",
		">=": "oge",
	}[operator]
	tmp := g.nextTemp()
	g.write("    %s = llvm.fcmp %q %s, %s : %s\n", tmp, predicate, left.ref, right.ref, left.typ)
	return value{typ: "i1", ref: tmp}, nil
}

func (g *Generator) emitNumericOne(typ string, negative bool) (value, error) {
	literal := "1"
	if negative {
		literal = "-1"
	}
	if isMLIRIntegerType(typ) {
		return g.emitIntegerConstant(literal, typ), nil
	}
	if isMLIRFloatType(typ) {
		if negative {
			return g.emitFloatConstant("-1.0", typ)
		}
		return g.emitFloatConstant("1.0", typ)
	}
	return value{}, fmt.Errorf("emit-mlir cannot create numeric step for %s", typ)
}

func (g *Generator) resolveStruct(name string) (*mlirStruct, error) {
	info, ok := g.structs[name]
	if !ok {
		return nil, fmt.Errorf("emit-mlir unknown struct type %s", name)
	}
	if info.typ != "" {
		return info, nil
	}
	if info.resolving {
		return nil, fmt.Errorf("emit-mlir does not support recursively embedded struct %s", name)
	}
	info.resolving = true
	defer func() {
		info.resolving = false
	}()

	fieldTypes := make([]string, 0, len(info.declaration.Fields))
	fields := make([]mlirStructField, 0, len(info.declaration.Fields))
	for _, declarationField := range info.declaration.Fields {
		if declarationField == nil || declarationField.Name == nil || declarationField.Type == nil {
			return nil, fmt.Errorf("emit-mlir struct %s contains an incomplete field", name)
		}
		ref := declarationField.Type
		if ref.Ref || ref.MutableRef || ref.Slice || ref.ElementType != nil || ref.ArrayLengthExpression != nil || ref.ArrayLength != 0 {
			return nil, fmt.Errorf("emit-mlir struct field %s.%s uses an unsupported compound type", name, declarationField.Name.Value)
		}
		if len(ref.TypeArgs) > 0 {
			return nil, fmt.Errorf("emit-mlir struct field %s.%s uses an unsupported generic type", name, declarationField.Name.Value)
		}

		fieldStructName := ""
		if _, nested := g.structs[ref.Name]; nested {
			nestedInfo, err := g.resolveStruct(ref.Name)
			if err != nil {
				return nil, err
			}
			fieldStructName = nestedInfo.name
		}
		fieldType := g.mlirType(ref)
		if fieldType == "string" {
			return nil, fmt.Errorf("emit-mlir does not support string fields in struct %s yet", name)
		}
		if fieldType == "void" {
			return nil, fmt.Errorf("emit-mlir does not support field type %s in struct %s", ref.Name, name)
		}
		fieldTypes = append(fieldTypes, fieldType)
		fields = append(fields, mlirStructField{
			name:       declarationField.Name.Value,
			typ:        fieldType,
			structName: fieldStructName,
			unsigned:   isUnsignedTypeReference(ref),
		})
	}
	info.fields = fields
	info.typ = "!llvm.struct<(" + strings.Join(fieldTypes, ", ") + ")>"
	return info, nil
}

func (info *mlirStruct) field(name string) (int, mlirStructField, bool) {
	for index, field := range info.fields {
		if field.name == name {
			return index, field, true
		}
	}
	return 0, mlirStructField{}, false
}

func (g *Generator) structName(ref *ast.TypeReference) string {
	if ref == nil {
		return ""
	}
	if _, ok := g.structs[ref.Name]; ok {
		return ref.Name
	}
	return ""
}

func (g *Generator) mlirType(ref *ast.TypeReference) string {
	if ref == nil {
		return "void"
	}
	switch ref.Name {
	case "bool":
		return "i1"
	case "void":
		return "void"
	case "int":
		return "i32"
	case "uint":
		return "i64"
	case "int8", "uint8", "byte":
		return "i8"
	case "int16", "uint16":
		return "i16"
	case "int32", "uint32":
		return "i32"
	case "int64", "uint64":
		return "i64"
	case "int128", "uint128":
		return "i128"
	case "int256", "uint256":
		return "i256"
	case "float", "float64":
		return "f64"
	case "float32":
		return "f32"
	case "string":
		return "string"
	case "decimal":
		return mlirDecimalType
	case "decimal128":
		return mlirDecimal128Type
	default:
		if info, ok := g.structs[ref.Name]; ok {
			return info.typ
		}
		return "void"
	}
}

func (g *Generator) emitStringLiteral(expr *ast.StringLiteral) (value, error) {
	name := fmt.Sprintf("__sec_str_%d", g.stringID)
	g.stringID++
	g.globals.WriteString(fmt.Sprintf("  llvm.mlir.global internal constant @%s(\"%s\") : !llvm.array<%d x i8>\n", name, mlirCString(expr.Value), len([]byte(expr.Value))))
	ptr := g.nextTemp()
	g.write("    %s = llvm.mlir.addressof @%s : !llvm.ptr\n", ptr, name)
	lenValue := g.emitIndexConstant(strconv.Itoa(len(expr.Value)))
	return value{typ: "string", ref: ptr, len: lenValue.ref}, nil
}

func (g *Generator) coerceValue(val value, targetType string, targetUnsigned bool) (value, error) {
	if val.typ == targetType {
		val.unsigned = targetUnsigned
		return val, nil
	}
	if isMLIRDecimalType(targetType) {
		return g.convertToDecimal(val, targetType)
	}
	if isMLIRDecimalType(val.typ) {
		if isMLIRIntegerType(targetType) {
			return g.convertDecimalToInteger(val, targetType, targetUnsigned)
		}
		return value{}, fmt.Errorf("cannot convert %s to %s", val.typ, targetType)
	}
	if isMLIRIntegerType(val.typ) && isMLIRIntegerType(targetType) {
		if integerBitWidth(targetType) == integerBitWidth(val.typ) {
			val.typ = targetType
			val.unsigned = targetUnsigned
			return val, nil
		}
		tmp := g.nextTemp()
		op := "sext"
		if integerBitWidth(targetType) < integerBitWidth(val.typ) {
			op = "trunc"
		} else if val.unsigned {
			op = "zext"
		}
		g.write("    %s = llvm.%s %s : %s to %s\n", tmp, op, val.ref, val.typ, targetType)
		return value{typ: targetType, ref: tmp, unsigned: targetUnsigned}, nil
	}
	if isMLIRIntegerType(val.typ) && isMLIRFloatType(targetType) {
		tmp := g.nextTemp()
		op := "sitofp"
		if val.unsigned {
			op = "uitofp"
		}
		g.write("    %s = llvm.%s %s : %s to %s\n", tmp, op, val.ref, val.typ, targetType)
		return value{typ: targetType, ref: tmp}, nil
	}
	return value{}, fmt.Errorf("cannot convert %s to %s", val.typ, targetType)
}

func (g *Generator) zeroValue(typ string) value {
	if isMLIRDecimalType(typ) {
		coefficientType, _ := decimalCoefficientType(typ)
		coefficient := g.emitIntegerConstant("0", coefficientType)
		scale := g.emitIntegerConstant("0", "i32")
		decimal, _ := g.emitDecimalValue(coefficient, scale, typ)
		return decimal
	}
	switch typ {
	case "i1":
		return g.emitBoolConstant(false)
	case "f32", "f64":
		tmp := g.nextTemp()
		g.write("    %s = llvm.mlir.constant(0.000000e+00 : %s) : %s\n", tmp, typ, typ)
		return value{typ: typ, ref: tmp}
	case "i32":
		return g.emitIntegerConstant("0", typ)
	default:
		return g.emitIntegerConstant("0", typ)
	}
}

func isMLIRIntegerType(typ string) bool {
	return strings.HasPrefix(typ, "i") && typ != "i1"
}

func isMLIRFloatType(typ string) bool {
	return typ == "f32" || typ == "f64"
}

func isMLIRDecimalType(typ string) bool {
	return typ == mlirDecimalType || typ == mlirDecimal128Type
}

func decimalCoefficientType(typ string) (string, bool) {
	switch typ {
	case mlirDecimalType:
		return "i64", true
	case mlirDecimal128Type:
		return "i128", true
	default:
		return "", false
	}
}

func isMLIRBuiltinNumericTypeName(name string) bool {
	return mlirBuiltinNumericType(name) != ""
}

func mlirBuiltinNumericType(name string) string {
	switch name {
	case "int":
		return "i32"
	case "uint":
		return "i64"
	case "int8", "uint8", "byte":
		return "i8"
	case "int16", "uint16":
		return "i16"
	case "int32", "uint32":
		return "i32"
	case "int64", "uint64":
		return "i64"
	case "int128", "uint128":
		return "i128"
	case "int256", "uint256":
		return "i256"
	case "float", "float64":
		return "f64"
	case "float32":
		return "f32"
	case "decimal":
		return mlirDecimalType
	case "decimal128":
		return mlirDecimal128Type
	default:
		return ""
	}
}

func isUnsignedBuiltinName(name string) bool {
	return strings.HasPrefix(name, "uint") || name == "byte"
}

func isUnsignedTypeReference(ref *ast.TypeReference) bool {
	if ref == nil {
		return false
	}
	switch ref.Name {
	case "uint", "uint8", "byte", "uint16", "uint32", "uint64", "uint128", "uint256":
		return true
	default:
		return false
	}
}

func integerBitWidth(typ string) int {
	width, _ := strconv.Atoi(strings.TrimPrefix(typ, "i"))
	return width
}

func inferredIntegerLiteralType(value *big.Int, suffix string) string {
	if suffix == "u" {
		return inferredUnsignedIntegerLiteralType(value)
	}
	if fitsSignedBits(value, 32) {
		return "i32"
	}
	if fitsSignedBits(value, 64) {
		return "i64"
	}
	if fitsSignedBits(value, 128) {
		return "i128"
	}
	return "i256"
}

func inferredUnsignedIntegerLiteralType(value *big.Int) string {
	if fitsUnsignedBits(value, 64) {
		return "i64"
	}
	if fitsUnsignedBits(value, 128) {
		return "i128"
	}
	return "i256"
}

func fitsSignedBits(value *big.Int, bits uint) bool {
	min := new(big.Int).Lsh(big.NewInt(1), bits-1)
	min.Neg(min)
	max := new(big.Int).Lsh(big.NewInt(1), bits-1)
	max.Sub(max, big.NewInt(1))
	return value.Cmp(min) >= 0 && value.Cmp(max) <= 0
}

func fitsUnsignedBits(value *big.Int, bits uint) bool {
	if value.Sign() < 0 {
		return false
	}
	max := new(big.Int).Lsh(big.NewInt(1), bits)
	max.Sub(max, big.NewInt(1))
	return value.Cmp(max) <= 0
}

func copyMLIRLocals(in map[string]local) map[string]local {
	out := make(map[string]local, len(in))
	for name, slot := range in {
		out[name] = slot
	}
	return out
}

func mlirCString(input string) string {
	var out strings.Builder
	for _, b := range []byte(input) {
		switch {
		case b >= 0x20 && b <= 0x7e && b != '\\' && b != '"':
			out.WriteByte(b)
		default:
			out.WriteString(fmt.Sprintf("\\%02X", b))
		}
	}
	return out.String()
}

func mlirFloatLiteral(value float64) string {
	return strconv.FormatFloat(value, 'e', 6, 64)
}

func callExpressionName(expr *ast.CallExpression) string {
	if expr == nil {
		return ""
	}
	if expr.Function != nil {
		return expr.Function.Value
	}
	if ident, ok := expr.Callee.(*ast.Identifier); ok {
		return ident.Value
	}
	return ""
}

func (g *Generator) write(format string, args ...any) {
	if g.activeOut != nil {
		fmt.Fprintf(g.activeOut, format, args...)
		return
	}
	fmt.Fprintf(&g.out, format, args...)
}

func (g *Generator) nextTemp() string {
	name := fmt.Sprintf("%%t%d", g.temp)
	g.temp++
	return name
}

func (g *Generator) nextLabel(prefix string) string {
	name := fmt.Sprintf("%s%d", strings.ReplaceAll(prefix, ".", "_"), g.label)
	g.label++
	return name
}

func validateEntrypoint(program *ast.Program) error {
	hasMainModule := false
	for _, stmt := range program.Statements {
		module, ok := stmt.(*ast.ModuleStatement)
		if ok && module.Path == "main" {
			hasMainModule = true
			break
		}
	}
	if !hasMainModule {
		return fmt.Errorf("emit-mlir requires module main")
	}
	mainFn := findMainFunction(program)
	if mainFn == nil {
		return fmt.Errorf("emit-mlir requires fn main() int or fn main() void")
	}
	if len(mainFn.Parameters) != 0 {
		return fmt.Errorf("emit-mlir requires fn main() with no parameters")
	}
	if mainFn.ReturnType == nil || (mainFn.ReturnType.Name != "int" && mainFn.ReturnType.Name != "void") {
		return fmt.Errorf("emit-mlir requires fn main() int or fn main() void")
	}
	return nil
}

func findMainFunction(program *ast.Program) *ast.FunctionDeclaration {
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if ok && fn.Name != nil && fn.Name.Value == "main" {
			return fn
		}
	}
	return nil
}
