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
	functions      map[string][]*ast.FunctionDeclaration
	functionNames  map[*ast.FunctionDeclaration]string
	reachable      map[*ast.FunctionDeclaration]bool
	constants      map[string]mlirConstant
	namedTypes     map[string]mlirNamedType
	structs        map[string]*mlirStruct
	enums          map[string]*mlirEnum
	loops          []loopContext
	defers         []*deferEntry
	deferByStmt    map[*ast.DeferStatement]*deferEntry
	stringID       int
}

type value struct {
	typ        string
	ref        string
	len        string
	structName string
	enumName   string
	unsigned   bool
}

type local struct {
	typ        string
	ptr        string
	lenPtr     string
	ref        string
	len        string
	structName string
	enumName   string
	unsigned   bool
	direct     bool
}

type mlirEnum struct {
	name     string
	typ      string
	unsigned bool
	values   map[string]string
}

type mlirConstant struct {
	literal  string
	typ      string
	unsigned bool
	boolean  *bool
	text     *string
}

type mlirNamedType struct {
	name     string
	typ      string
	unsigned bool
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
	storageTyp string
	structName string
	enumName   string
	unsigned   bool
	string     bool
}

type loopContext struct {
	breakLabel    string
	continueLabel string
}

type deferEntry struct {
	stmt      *ast.DeferStatement
	activePtr string
	locals    map[string]local
	seen      bool
}

const (
	mlirDecimalType    = "!llvm.struct<(i64, i32)>"
	mlirDecimal128Type = "!llvm.struct<(i128, i32)>"
	mlirStringType     = "!llvm.struct<(!llvm.ptr, i64)>"
)

func GenerateWithTriple(program *ast.Program, triple string) (string, error) {
	g := &Generator{targetTriple: triple}
	return g.Generate(program)
}

func (g *Generator) Generate(program *ast.Program) (string, error) {
	if err := validateEntrypoint(program); err != nil {
		return "", err
	}
	g.functions = map[string][]*ast.FunctionDeclaration{}
	g.functionNames = map[*ast.FunctionDeclaration]string{}
	g.reachable = map[*ast.FunctionDeclaration]bool{}
	g.constants = map[string]mlirConstant{}
	g.namedTypes = map[string]mlirNamedType{}
	g.structs = map[string]*mlirStruct{}
	g.enums = map[string]*mlirEnum{}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name != nil {
				g.functions[stmt.Name.Value] = append(g.functions[stmt.Name.Value], stmt)
			}
		case *ast.LetStatement:
			if stmt.Name != nil && !stmt.Mutable {
				if constant, ok := g.resolveTopLevelConstant(stmt); ok {
					g.constants[stmt.Name.Value] = constant
				}
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
			} else if stmt.Name != nil && stmt.BaseType != nil {
				baseType := g.mlirType(stmt.BaseType)
				if baseType != "void" {
					g.namedTypes[stmt.Name.Value] = mlirNamedType{
						name:     stmt.Name.Value,
						typ:      baseType,
						unsigned: g.typeUnsigned(stmt.BaseType),
					}
				}
			}
		case *ast.EnumDeclaration:
			if err := g.registerEnum(stmt, ""); err != nil {
				return "", err
			}
		case *ast.ImplStatement:
			owner := ""
			if stmt.Target != nil {
				owner = stmt.Target.Name
			}
			for _, member := range stmt.Members {
				enumDecl, ok := member.(*ast.EnumDeclaration)
				if !ok {
					continue
				}
				if err := g.registerEnum(enumDecl, owner); err != nil {
					return "", err
				}
			}
		}
	}
	for name, overloads := range g.functions {
		arities := map[int]bool{}
		for _, fn := range overloads {
			symbol := name
			if fn.Extern && fn.LinkName != "" {
				symbol = fn.LinkName
			} else if len(overloads) > 1 {
				if arities[len(fn.Parameters)] {
					return "", fmt.Errorf("emit-mlir does not yet support overloads of %s with the same arity", name)
				}
				arities[len(fn.Parameters)] = true
				symbol = fmt.Sprintf("%s__sec_arity_%d", name, len(fn.Parameters))
			}
			g.functionNames[fn] = symbol
		}
	}
	for name := range g.structs {
		if _, err := g.resolveStruct(name); err != nil {
			return "", err
		}
	}

	g.write("module attributes {llvm.target_triple = %q} {\n", g.targetTriple)
	mainFn := findMainFunction(program)
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if ok && fn.Name != nil && (mainFn == nil || mainFn.Token.File == "" || fn.Token.File == mainFn.Token.File) {
			g.reachable[fn] = true
		}
	}
	emitted := map[*ast.FunctionDeclaration]bool{}
	for {
		progress := false
		for _, stmt := range program.Statements {
			fn, ok := stmt.(*ast.FunctionDeclaration)
			if !ok || fn.Name == nil || !g.reachable[fn] || emitted[fn] {
				continue
			}
			if err := g.emitFunction(fn); err != nil {
				return "", err
			}
			emitted[fn] = true
			progress = true
		}
		if !progress {
			break
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
	g.returnUnsigned = g.typeUnsigned(fn.ReturnType)
	defer func() {
		g.returnType = previousReturnType
		g.returnUnsigned = previousReturnUnsigned
	}()
	previousLocals := g.locals
	g.locals = map[string]local{}
	previousDefers := g.defers
	previousDeferByStmt := g.deferByStmt
	g.defers = collectFunctionDefers(fn.Body)
	g.deferByStmt = map[*ast.DeferStatement]*deferEntry{}
	for _, entry := range g.defers {
		g.deferByStmt[entry.stmt] = entry
	}
	defer func() {
		g.defers = previousDefers
		g.deferByStmt = previousDeferByStmt
	}()

	declaration := fn.Extern && fn.Body == nil
	fmt.Fprintf(&signature, "  llvm.func @%s(", mlirSymbolName(g.functionName(fn)))
	writtenParams := 0
	for _, param := range fn.Parameters {
		if writtenParams > 0 {
			fmt.Fprintf(&signature, ", ")
		}
		paramType := g.mlirParameterType(param)
		paramUnsigned := g.typeUnsigned(param.Type)
		if paramType == "string" {
			if declaration {
				signature.WriteString("!llvm.ptr, i64")
			} else {
				fmt.Fprintf(&signature, "%%%s.ptr: !llvm.ptr, %%%s.len: i64", param.Name.Value, param.Name.Value)
			}
			if param.Name != nil {
				g.locals[param.Name.Value] = local{typ: "string", ref: "%" + param.Name.Value + ".ptr", len: "%" + param.Name.Value + ".len", direct: true}
			}
			writtenParams += 2
			continue
		}
		if declaration {
			fmt.Fprintf(&signature, "%s", paramType)
		} else {
			fmt.Fprintf(&signature, "%%%s: %s", param.Name.Value, paramType)
		}
		if param.Name != nil {
			g.locals[param.Name.Value] = local{
				typ:        paramType,
				ref:        "%" + param.Name.Value,
				structName: g.structName(param.Type),
				enumName:   g.enumName(param.Type),
				unsigned:   paramUnsigned,
				direct:     true,
			}
		}
		writtenParams++
	}
	signatureReturnType := returnType
	if returnType == "string" {
		signatureReturnType = mlirStringType
	}
	if returnType == "void" {
		signature.WriteString(")")
	} else {
		fmt.Fprintf(&signature, ") -> %s", signatureReturnType)
	}
	if declaration {
		signature.WriteString("\n")
		g.activeOut = previousActiveOut
		g.out.WriteString(signature.String())
		g.locals = previousLocals
		return nil
	}
	signature.WriteString(" {\n")
	g.blockOpen = true
	g.initializeDeferFlags()

	if fn.Body != nil {
		for _, stmt := range fn.Body.Statements {
			if err := g.emitStatement(stmt); err != nil {
				g.locals = previousLocals
				return err
			}
		}
	}

	if g.blockOpen {
		if err := g.emitActiveDefers(); err != nil {
			g.locals = previousLocals
			return err
		}
		if returnType == "void" {
			g.write("    llvm.return\n")
		} else if returnType == "string" {
			undef := g.nextTemp()
			g.write("    %s = llvm.mlir.undef : %s\n", undef, mlirStringType)
			g.write("    llvm.return %s : %s\n", undef, mlirStringType)
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
	case *ast.MatchStatement:
		return g.emitMatchStatement(stmt)
	case *ast.UnsafeStatement:
		return g.emitUnsafe(stmt)
	case *ast.AsmStatement:
		return g.emitAsmStatement(stmt)
	case *ast.DeferStatement:
		return g.emitDefer(stmt)
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

func (g *Generator) emitUnsafe(stmt *ast.UnsafeStatement) error {
	if stmt == nil || stmt.Body == nil {
		return nil
	}
	for _, child := range stmt.Body.Statements {
		if !g.blockOpen {
			return nil
		}
		if err := g.emitStatement(child); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) emitAsmStatement(stmt *ast.AsmStatement) error {
	if stmt == nil {
		return fmt.Errorf("emit-mlir asm statement is missing")
	}
	if stmt.Block == nil {
		if stmt.Template == nil {
			return fmt.Errorf("asm statement requires string template")
		}
		g.write("    llvm.inline_asm has_side_effects %q, %q : () -> ()\n", stmt.Template.Value, "")
		return nil
	}
	if stmt.Block.Template == nil {
		return fmt.Errorf("asm block requires string template")
	}
	if len(stmt.Block.Outputs) != 1 {
		return fmt.Errorf("emit-mlir currently supports exactly one asm output")
	}

	constraints := "={" + stmt.Block.Outputs[0].Register + "}"
	args := make([]value, 0, len(stmt.Block.Inputs))
	for _, input := range stmt.Block.Inputs {
		constraints += ",{" + input.Register + "}"
		arg, err := g.emitExpression(input.Value)
		if err != nil {
			return err
		}
		arg, err = g.coerceValue(arg, "i64", arg.unsigned)
		if err != nil {
			return fmt.Errorf("emit-mlir asm input %s cannot be passed in a machine register", input.Register)
		}
		args = append(args, arg)
	}
	clobbers := stmt.Block.Clobbers
	if len(clobbers) == 0 {
		clobbers = []string{"rcx", "r11"}
	}
	for _, clobber := range clobbers {
		constraints += ",~{" + clobber + "}"
	}

	result := g.nextTemp()
	g.write("    %s = llvm.inline_asm has_side_effects %q, %q", result, stmt.Block.Template.Value, constraints)
	for i, arg := range args {
		if i == 0 {
			g.write(" %s", arg.ref)
		} else {
			g.write(", %s", arg.ref)
		}
	}
	g.write(" : (")
	for i, arg := range args {
		if i > 0 {
			g.write(", ")
		}
		g.write("%s", arg.typ)
	}
	g.write(") -> i64\n")

	output := value{typ: "i64", ref: result}
	var err error
	if g.returnType != "" && g.returnType != "void" {
		output, err = g.coerceValue(output, g.returnType, g.returnUnsigned)
		if err != nil {
			return err
		}
	}
	if name := stmt.Block.Outputs[0].Name; name != "" {
		g.locals[name] = local{
			typ:      output.typ,
			ref:      output.ref,
			unsigned: output.unsigned,
			direct:   true,
		}
	}
	return nil
}

func (g *Generator) emitDefer(stmt *ast.DeferStatement) error {
	if len(g.loops) > 0 {
		return fmt.Errorf("emit-mlir does not support defer inside loops yet")
	}
	entry, ok := g.deferByStmt[stmt]
	if !ok {
		return fmt.Errorf("emit-mlir internal error: unregistered defer statement")
	}
	if entry.activePtr == "" {
		entry.activePtr = g.emitAlloca("i1")
	}
	if deferBodyIsBareReturnMLIR(stmt.Body) {
		return nil
	}
	entry.seen = true
	entry.locals = copyMLIRLocals(g.locals)
	active := g.emitBoolConstant(true)
	g.write("    llvm.store %s, %s : i1, !llvm.ptr\n", active.ref, entry.activePtr)
	return nil
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
	targetEnumName := ""
	targetUnsigned := false
	if stmt.Type != nil {
		targetType = g.mlirType(stmt.Type)
		targetStructName = g.structName(stmt.Type)
		targetEnumName = g.enumName(stmt.Type)
		targetUnsigned = g.typeUnsigned(stmt.Type)
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
			targetEnumName = val.enumName
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
		enumName:   targetEnumName,
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

	if rangeExpr, ok := stmt.Iterable.(*ast.RangeExpression); ok {
		if len(stmt.Bindings) != 1 {
			return fmt.Errorf("emit-mlir range for currently supports one loop binding")
		}
		return g.emitRangeFor(stmt, rangeExpr)
	}

	iterable, err := g.emitExpression(stmt.Iterable)
	if err != nil {
		return err
	}
	if _, _, ok := parseMLIRArrayType(iterable.typ); ok {
		return g.emitArrayFor(stmt, iterable)
	}
	return fmt.Errorf("emit-mlir cannot iterate over %s yet", iterable.typ)
}

func (g *Generator) emitArrayFor(stmt *ast.ForStatement, array value) error {
	if stmt == nil || stmt.Body == nil {
		return fmt.Errorf("emit-mlir requires complete array for statements")
	}
	if len(stmt.Bindings) < 1 || len(stmt.Bindings) > 2 {
		return fmt.Errorf("emit-mlir array iteration requires one or two loop bindings")
	}
	length, elementType, ok := parseMLIRArrayType(array.typ)
	if !ok {
		return fmt.Errorf("emit-mlir cannot iterate over non-array type %s", array.typ)
	}

	arrayPtr := g.emitAlloca(array.typ)
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", array.ref, arrayPtr, array.typ)
	indexPtr := g.emitAlloca("i64")
	zero := g.emitIntegerConstantUnsigned("0", "i64", true)
	g.write("    llvm.store %s, %s : i64, !llvm.ptr\n", zero.ref, indexPtr)

	conditionLabel := g.nextLabel("for.array.condition")
	bodyLabel := g.nextLabel("for.array.body")
	nextLabel := g.nextLabel("for.array.next")
	endLabel := g.nextLabel("for.array.end")

	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", conditionLabel)
	g.blockOpen = true
	index, err := g.loadLocal(local{typ: "i64", ptr: indexPtr, unsigned: true})
	if err != nil {
		return err
	}
	lengthValue := g.emitIntegerConstantUnsigned(strconv.FormatInt(length, 10), "i64", true)
	condition := g.nextTemp()
	g.write("    %s = llvm.icmp \"ult\" %s, %s : i64\n", condition, index.ref, lengthValue.ref)
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition, bodyLabel, endLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	elementPtr := g.nextTemp()
	g.write("    %s = llvm.getelementptr %s[0, %s] : (!llvm.ptr, i64) -> !llvm.ptr, %s\n", elementPtr, arrayPtr, index.ref, array.typ)
	elementRef := g.nextTemp()
	g.write("    %s = llvm.load %s : !llvm.ptr -> %s\n", elementRef, elementPtr, elementType)
	element := value{
		typ:        elementType,
		ref:        elementRef,
		structName: g.structNameForMLIRType(elementType),
		unsigned:   array.unsigned,
	}

	previousLocals := g.locals
	g.locals = copyMLIRLocals(previousLocals)
	if len(stmt.Bindings) == 1 {
		if binding := stmt.Bindings[0]; !binding.Discard {
			g.locals[binding.Name] = local{
				typ:        element.typ,
				ref:        element.ref,
				structName: element.structName,
				unsigned:   element.unsigned,
				direct:     true,
			}
		}
	} else {
		if binding := stmt.Bindings[0]; !binding.Discard {
			indexValue, err := g.coerceIntegerStorageValue(index, "i32", false)
			if err != nil {
				g.locals = previousLocals
				return err
			}
			g.locals[binding.Name] = local{typ: "i32", ref: indexValue.ref, direct: true}
		}
		if binding := stmt.Bindings[1]; !binding.Discard {
			g.locals[binding.Name] = local{
				typ:        element.typ,
				ref:        element.ref,
				structName: element.structName,
				unsigned:   element.unsigned,
				direct:     true,
			}
		}
	}

	g.pushLoop(endLabel, nextLabel)
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
		g.write("    llvm.br ^%s\n", nextLabel)
		g.blockOpen = false
	}

	g.write("  ^%s:\n", nextLabel)
	g.blockOpen = true
	current, err := g.loadLocal(local{typ: "i64", ptr: indexPtr, unsigned: true})
	if err != nil {
		return err
	}
	one := g.emitIntegerConstantUnsigned("1", "i64", true)
	next, err := g.emitIntegerBinary("add", current, one)
	if err != nil {
		return err
	}
	g.write("    llvm.store %s, %s : i64, !llvm.ptr\n", next.ref, indexPtr)
	g.write("    llvm.br ^%s\n", conditionLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
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
	if index, ok := stmt.Target.(*ast.IndexExpression); ok {
		return g.emitIndexAssignment(stmt, index)
	}
	if member, ok := stmt.Target.(*ast.MemberExpression); ok {
		return g.emitMemberAssignment(stmt, member)
	}
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
		if stmt.Operator != "=" {
			return fmt.Errorf("emit-mlir does not support compound string assignment")
		}
		val, err := g.emitExpression(stmt.Value)
		if err != nil {
			return err
		}
		val, err = g.coerceValue(val, "string", false)
		if err != nil {
			return fmt.Errorf("emit-mlir cannot assign %s to string", val.typ)
		}
		g.write("    llvm.store %s, %s : !llvm.ptr, !llvm.ptr\n", val.ref, slot.ptr)
		g.write("    llvm.store %s, %s : i64, !llvm.ptr\n", val.len, slot.lenPtr)
		return nil
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

func (g *Generator) emitIndexAssignment(stmt *ast.AssignmentStatement, target *ast.IndexExpression) error {
	root, indexes, ok := mlirIndexPath(target)
	if !ok || len(indexes) == 0 {
		return fmt.Errorf("emit-mlir indexed assignment requires a stored array local")
	}
	slot, ok := g.locals[root]
	if !ok {
		return fmt.Errorf("emit-mlir unknown local %s", root)
	}
	if slot.direct {
		return fmt.Errorf("emit-mlir cannot assign through array parameter %s", root)
	}
	if slot.ptr == "" {
		return fmt.Errorf("emit-mlir local %s is not assignable", root)
	}

	elementPtr := slot.ptr
	elementType := slot.typ
	for _, indexExpr := range indexes {
		length, nextType, ok := parseMLIRArrayType(elementType)
		if !ok {
			return fmt.Errorf("emit-mlir cannot index non-array type %s", elementType)
		}
		index, err := g.emitArrayIndex(indexExpr, length)
		if err != nil {
			return err
		}
		nextPtr := g.nextTemp()
		g.write("    %s = llvm.getelementptr %s[0, %s] : (!llvm.ptr, i64) -> !llvm.ptr, %s\n", nextPtr, elementPtr, index.ref, elementType)
		elementPtr = nextPtr
		elementType = nextType
	}

	assigned, err := g.emitExpressionForTargetUnsigned(stmt.Value, elementType, slot.unsigned)
	if err != nil {
		return err
	}
	assigned, err = g.coerceValue(assigned, elementType, slot.unsigned)
	if err != nil {
		return fmt.Errorf("emit-mlir cannot assign %s to array element %s", assigned.typ, elementType)
	}
	if stmt.Operator != "=" {
		currentRef := g.nextTemp()
		g.write("    %s = llvm.load %s : !llvm.ptr -> %s\n", currentRef, elementPtr, elementType)
		assigned, err = g.emitAssignmentOperation(
			stmt.Operator,
			value{typ: elementType, ref: currentRef, unsigned: slot.unsigned},
			assigned,
		)
		if err != nil {
			return err
		}
	}
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", assigned.ref, elementPtr, elementType)
	return nil
}

func mlirIndexPath(expr *ast.IndexExpression) (string, []ast.Expression, bool) {
	if expr == nil || expr.Index == nil {
		return "", nil, false
	}
	indexes := []ast.Expression{expr.Index}
	left := expr.Left
	for {
		switch current := left.(type) {
		case *ast.Identifier:
			return current.Value, indexes, true
		case *ast.IndexExpression:
			if current.Index == nil {
				return "", nil, false
			}
			indexes = append([]ast.Expression{current.Index}, indexes...)
			left = current.Left
		default:
			return "", nil, false
		}
	}
}

type mlirMemberAssignmentStep struct {
	info  *mlirStruct
	index int
	field mlirStructField
}

func (g *Generator) emitMemberAssignment(stmt *ast.AssignmentStatement, member *ast.MemberExpression) error {
	rootName, fields, ok := mlirMemberPath(member)
	if !ok || len(fields) == 0 {
		return fmt.Errorf("emit-mlir requires a local struct field assignment target")
	}
	slot, ok := g.locals[rootName]
	if !ok {
		return fmt.Errorf("emit-mlir unknown local %s", rootName)
	}
	if slot.direct {
		return fmt.Errorf("emit-mlir cannot assign to field through parameter %s", rootName)
	}
	if slot.ptr == "" || slot.structName == "" {
		return fmt.Errorf("emit-mlir field assignment root %s is not a stored struct", rootName)
	}

	structName := slot.structName
	steps := make([]mlirMemberAssignmentStep, 0, len(fields))
	for fieldIndex, fieldName := range fields {
		info, exists := g.structs[structName]
		if !exists {
			return fmt.Errorf("emit-mlir unknown struct type %s", structName)
		}
		index, field, exists := info.field(fieldName)
		if !exists {
			return fmt.Errorf("emit-mlir unknown field %s.%s", structName, fieldName)
		}
		steps = append(steps, mlirMemberAssignmentStep{info: info, index: index, field: field})
		if fieldIndex+1 < len(fields) {
			if field.structName == "" {
				return fmt.Errorf("emit-mlir cannot select field %s through non-struct field %s.%s", fields[fieldIndex+1], structName, fieldName)
			}
			structName = field.structName
		}
	}

	root, err := g.loadLocal(slot)
	if err != nil {
		return err
	}
	parents := []value{root}
	current := root
	for _, step := range steps[:len(steps)-1] {
		childRef := g.nextTemp()
		g.write("    %s = llvm.extractvalue %s[%d] : %s\n", childRef, current.ref, step.index, step.info.typ)
		current = value{
			typ:        step.field.typ,
			ref:        childRef,
			structName: step.field.structName,
			enumName:   step.field.enumName,
			unsigned:   step.field.unsigned,
		}
		parents = append(parents, current)
	}

	leaf := steps[len(steps)-1]
	if leaf.field.string && stmt.Operator != "=" {
		return fmt.Errorf("emit-mlir does not support compound assignment to string field %s.%s", leaf.info.name, leaf.field.name)
	}
	assigned, err := g.emitExpressionForTargetUnsigned(stmt.Value, leaf.field.typ, leaf.field.unsigned)
	if err != nil {
		return err
	}
	assigned, err = g.coerceValue(assigned, leaf.field.typ, leaf.field.unsigned)
	if err != nil {
		return fmt.Errorf("emit-mlir cannot assign %s to field %s.%s", assigned.typ, leaf.info.name, leaf.field.name)
	}
	if stmt.Operator != "=" {
		currentRef := g.nextTemp()
		g.write("    %s = llvm.extractvalue %s[%d] : %s\n", currentRef, current.ref, leaf.index, leaf.info.typ)
		currentValue := value{
			typ:        leaf.field.typ,
			ref:        currentRef,
			structName: leaf.field.structName,
			enumName:   leaf.field.enumName,
			unsigned:   leaf.field.unsigned,
		}
		assigned, err = g.emitAssignmentOperation(stmt.Operator, currentValue, assigned)
		if err != nil {
			return err
		}
	}

	storedAssigned := assigned
	if leaf.field.string {
		storedAssigned, err = g.packString(assigned)
		if err != nil {
			return err
		}
	}
	updatedRef := g.nextTemp()
	g.write("    %s = llvm.insertvalue %s, %s[%d] : %s\n", updatedRef, storedAssigned.ref, current.ref, leaf.index, leaf.info.typ)
	updated := value{typ: leaf.info.typ, ref: updatedRef, structName: leaf.info.name}
	for index := len(steps) - 2; index >= 0; index-- {
		step := steps[index]
		parent := parents[index]
		nextRef := g.nextTemp()
		g.write("    %s = llvm.insertvalue %s, %s[%d] : %s\n", nextRef, updated.ref, parent.ref, step.index, step.info.typ)
		updated = value{typ: step.info.typ, ref: nextRef, structName: step.info.name}
	}
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", updated.ref, slot.ptr, slot.typ)
	return nil
}

func mlirMemberPath(member *ast.MemberExpression) (string, []string, bool) {
	if member == nil || member.Property == nil {
		return "", nil, false
	}
	fields := []string{member.Property.Value}
	object := member.Object
	for {
		switch expr := object.(type) {
		case *ast.Identifier:
			return expr.Value, fields, true
		case *ast.MemberExpression:
			if expr.Property == nil {
				return "", nil, false
			}
			fields = append([]string{expr.Property.Value}, fields...)
			object = expr.Object
		default:
			return "", nil, false
		}
	}
}

func (g *Generator) loadLocal(slot local) (value, error) {
	if slot.direct {
		return value{typ: slot.typ, ref: slot.ref, len: slot.len, structName: slot.structName, enumName: slot.enumName, unsigned: slot.unsigned}, nil
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
	return value{typ: slot.typ, ref: tmp, structName: slot.structName, enumName: slot.enumName, unsigned: slot.unsigned}, nil
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
		if err := g.emitActiveDefers(); err != nil {
			return err
		}
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
	if err := g.emitActiveDefers(); err != nil {
		return err
	}
	if g.returnType == "string" {
		descriptor, err := g.packString(val)
		if err != nil {
			return err
		}
		g.write("    llvm.return %s : %s\n", descriptor.ref, mlirStringType)
		g.blockOpen = false
		return nil
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

func (g *Generator) emitMatchStatement(stmt *ast.MatchStatement) error {
	if stmt == nil || stmt.Match == nil {
		return nil
	}
	return g.emitMatch(stmt.Match, "", false, false)
}

func (g *Generator) emitMatchExpression(expr *ast.MatchExpression, targetType string, targetUnsigned bool) (value, error) {
	if targetType == "" {
		return value{}, fmt.Errorf("emit-mlir match expression requires an expected result type")
	}
	resultPtr := g.emitAlloca(targetType)
	if err := g.emitMatch(expr, targetType, targetUnsigned, true, resultPtr); err != nil {
		return value{}, err
	}
	return g.loadLocal(local{typ: targetType, ptr: resultPtr, unsigned: targetUnsigned})
}

func (g *Generator) emitMatch(expr *ast.MatchExpression, targetType string, targetUnsigned bool, valueContext bool, resultPtr ...string) error {
	if expr.Subject == nil {
		return fmt.Errorf("emit-mlir match requires a subject")
	}
	if len(expr.Arms) == 0 {
		return fmt.Errorf("emit-mlir match requires at least one arm")
	}
	subject, err := g.emitExpression(expr.Subject)
	if err != nil {
		return err
	}
	if subject.typ != "i1" && !isMLIRIntegerType(subject.typ) {
		return fmt.Errorf("emit-mlir match currently supports bool, integer and enum subjects, got %s", subject.typ)
	}
	for _, arm := range expr.Arms {
		if arm == nil {
			return fmt.Errorf("emit-mlir match contains nil arm")
		}
		if arm.Guard != nil {
			return fmt.Errorf("emit-mlir does not support match guards yet")
		}
		if !valueContext && arm.Body != nil {
			return fmt.Errorf("emit-mlir match statement arms must use block or return bodies")
		}
		if valueContext && arm.Body == nil {
			return fmt.Errorf("emit-mlir match expression arms must produce values")
		}
	}

	endLabel := g.nextLabel("match.end")
	testLabels := make([]string, len(expr.Arms))
	bodyLabels := make([]string, len(expr.Arms))
	for i := range expr.Arms {
		testLabels[i] = g.nextLabel("match.test")
		bodyLabels[i] = g.nextLabel("match.arm")
	}

	g.write("    llvm.br ^%s\n", testLabels[0])
	g.blockOpen = false
	for i, arm := range expr.Arms {
		falseLabel := endLabel
		if i+1 < len(testLabels) {
			falseLabel = testLabels[i+1]
		}
		g.write("  ^%s:\n", testLabels[i])
		g.blockOpen = true
		condition, err := g.emitMatchPatternCondition(subject, arm.Pattern)
		if err != nil {
			return err
		}
		g.write("    llvm.cond_br %s, ^%s, ^%s\n", condition.ref, bodyLabels[i], falseLabel)
		g.blockOpen = false
	}

	outerLocals := g.locals
	for i, arm := range expr.Arms {
		g.write("  ^%s:\n", bodyLabels[i])
		g.blockOpen = true
		g.locals = copyMLIRLocals(outerLocals)
		if err := g.bindMatchPattern(subject, arm.Pattern); err != nil {
			g.locals = outerLocals
			return err
		}
		if valueContext {
			armValue, err := g.emitExpressionForTargetUnsigned(arm.Body, targetType, targetUnsigned)
			if err != nil {
				g.locals = outerLocals
				return err
			}
			armValue, err = g.coerceValue(armValue, targetType, targetUnsigned)
			if err != nil {
				g.locals = outerLocals
				return fmt.Errorf("emit-mlir match arm cannot produce %s as %s", armValue.typ, targetType)
			}
			g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", armValue.ref, resultPtr[0], targetType)
		} else if arm.ReturnBody != nil {
			if err := g.emitReturn(arm.ReturnBody); err != nil {
				g.locals = outerLocals
				return err
			}
		} else if arm.BlockBody != nil {
			for _, child := range arm.BlockBody.Statements {
				if err := g.emitStatement(child); err != nil {
					g.locals = outerLocals
					return err
				}
			}
		}
		if g.blockOpen {
			g.write("    llvm.br ^%s\n", endLabel)
			g.blockOpen = false
		}
	}
	g.locals = outerLocals

	g.write("  ^%s:\n", endLabel)
	g.blockOpen = true
	return nil
}

func (g *Generator) emitMatchPatternCondition(subject value, pattern ast.Expression) (value, error) {
	if pattern == nil {
		return value{}, fmt.Errorf("emit-mlir match arm missing pattern")
	}
	if _, ok := pattern.(*ast.Identifier); ok {
		return g.emitBoolConstant(true), nil
	}
	switch pattern.(type) {
	case *ast.OkExpression, *ast.ErrExpression:
		return value{}, fmt.Errorf("emit-mlir does not support Result match patterns yet")
	case *ast.CallExpression:
		return value{}, fmt.Errorf("emit-mlir does not support payload match patterns yet")
	}
	candidate, err := g.emitExpressionForTargetUnsigned(pattern, subject.typ, subject.unsigned)
	if err != nil {
		return value{}, err
	}
	candidate, err = g.coerceValue(candidate, subject.typ, subject.unsigned)
	if err != nil {
		return value{}, fmt.Errorf("emit-mlir match pattern does not match subject type %s", subject.typ)
	}
	return g.emitSwitchEquality(subject, candidate)
}

func (g *Generator) bindMatchPattern(subject value, pattern ast.Expression) error {
	ident, ok := pattern.(*ast.Identifier)
	if !ok || ident.Value == "_" {
		return nil
	}
	g.locals[ident.Value] = local{
		typ:      subject.typ,
		ref:      subject.ref,
		unsigned: subject.unsigned,
		direct:   true,
		enumName: subject.enumName,
	}
	return nil
}

func (g *Generator) initializeDeferFlags() {
	for _, entry := range g.defers {
		if deferBodyIsBareReturnMLIR(entry.stmt.Body) {
			continue
		}
		entry.activePtr = g.emitAlloca("i1")
		inactive := g.emitBoolConstant(false)
		g.write("    llvm.store %s, %s : i1, !llvm.ptr\n", inactive.ref, entry.activePtr)
	}
}

func (g *Generator) emitActiveDefers() error {
	if len(g.defers) == 0 {
		return nil
	}
	for i := len(g.defers) - 1; i >= 0; i-- {
		entry := g.defers[i]
		if entry == nil || !entry.seen || entry.activePtr == "" || deferBodyIsBareReturnMLIR(entry.stmt.Body) {
			continue
		}
		if err := g.emitActiveDefer(entry); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) emitActiveDefer(entry *deferEntry) error {
	active := g.nextTemp()
	bodyLabel := g.nextLabel("defer.body")
	nextLabel := g.nextLabel("defer.next")
	g.write("    %s = llvm.load %s : !llvm.ptr -> i1\n", active, entry.activePtr)
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", active, bodyLabel, nextLabel)
	g.blockOpen = false

	g.write("  ^%s:\n", bodyLabel)
	g.blockOpen = true
	inactive := g.emitBoolConstant(false)
	g.write("    llvm.store %s, %s : i1, !llvm.ptr\n", inactive.ref, entry.activePtr)
	previousLocals := g.locals
	if entry.locals != nil {
		g.locals = copyMLIRLocals(entry.locals)
	}
	if entry.stmt.Body != nil {
		for _, child := range entry.stmt.Body.Statements {
			if err := g.emitStatement(child); err != nil {
				g.locals = previousLocals
				return err
			}
		}
	}
	g.locals = previousLocals
	if g.blockOpen {
		g.write("    llvm.br ^%s\n", nextLabel)
		g.blockOpen = false
	}

	g.write("  ^%s:\n", nextLabel)
	g.blockOpen = true
	return nil
}

func collectFunctionDefers(block *ast.BlockStatement) []*deferEntry {
	var entries []*deferEntry
	collectBlockDefers(block, &entries)
	return entries
}

func collectBlockDefers(block *ast.BlockStatement, entries *[]*deferEntry) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		collectStatementDefers(stmt, entries)
	}
}

func collectStatementDefers(stmt ast.Statement, entries *[]*deferEntry) {
	switch stmt := stmt.(type) {
	case *ast.DeferStatement:
		*entries = append(*entries, &deferEntry{stmt: stmt})
	case *ast.IfStatement:
		collectBlockDefers(stmt.Consequence, entries)
		collectBlockDefers(stmt.Alternative, entries)
	case *ast.ForStatement:
		collectBlockDefers(stmt.Body, entries)
	case *ast.WhileStatement:
		collectBlockDefers(stmt.Body, entries)
	case *ast.SwitchStatement:
		for _, clause := range stmt.Cases {
			if clause != nil {
				collectBlockDefers(clause.Body, entries)
			}
		}
		if stmt.Default != nil {
			collectBlockDefers(stmt.Default.Body, entries)
		}
	case *ast.MatchStatement:
		collectMatchDefers(stmt.Match, entries)
	case *ast.UnsafeStatement:
		collectBlockDefers(stmt.Body, entries)
	}
}

func collectMatchDefers(expr *ast.MatchExpression, entries *[]*deferEntry) {
	if expr == nil {
		return
	}
	for _, arm := range expr.Arms {
		if arm != nil && arm.BlockBody != nil {
			collectBlockDefers(arm.BlockBody, entries)
		}
	}
}

func deferBodyIsBareReturnMLIR(block *ast.BlockStatement) bool {
	if block == nil || len(block.Statements) != 1 {
		return false
	}
	ret, ok := block.Statements[0].(*ast.ReturnStatement)
	return ok && ret.Value == nil
}

func (g *Generator) emitExpression(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return g.emitIdentifier(expr)
	case *ast.IntegerLiteral:
		if expr.Suffix() == "m" {
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
		if expr.Suffix() != "g" {
			return g.emitDecimalLiteral(expr, mlirDecimalType)
		}
		return g.emitFloatConstant(expr.Token.Lexeme, "f64")
	case *ast.BooleanLiteral:
		return g.emitBoolConstant(expr.Value), nil
	case *ast.StringLiteral:
		return g.emitStringLiteral(expr)
	case *ast.ArrayLiteral:
		return g.emitArrayLiteral(expr, "", false)
	case *ast.IndexExpression:
		return g.emitIndexExpression(expr)
	case *ast.PrefixExpression:
		return g.emitPrefixExpression(expr)
	case *ast.InfixExpression:
		return g.emitInfixExpression(expr)
	case *ast.CallExpression:
		return g.emitCallExpression(expr)
	case *ast.ConversionExpression:
		return g.emitConversionExpression(expr)
	case *ast.StructLiteral:
		return g.emitStructLiteral(expr)
	case *ast.MemberExpression:
		return g.emitMemberExpression(expr)
	case *ast.MatchExpression:
		return value{}, fmt.Errorf("emit-mlir match expression requires an expected result type")
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
	if matchExpr, ok := expr.(*ast.MatchExpression); ok {
		return g.emitMatchExpression(matchExpr, targetType, targetUnsigned)
	}
	if isMLIRDecimalType(targetType) {
		if _, _, ok := decimalLiteralParts(expr); ok {
			return g.emitDecimalLiteral(expr, targetType)
		}
	}
	if array, ok := expr.(*ast.ArrayLiteral); ok {
		return g.emitArrayLiteral(array, targetType, targetUnsigned)
	}
	if call, ok := expr.(*ast.CallExpression); ok && callExpressionName(call) == "fill" {
		if _, _, fixedArray := parseMLIRArrayType(targetType); fixedArray {
			return g.emitFixedArrayFill(call, targetType, targetUnsigned)
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

func (g *Generator) emitFixedArrayFill(expr *ast.CallExpression, targetType string, targetUnsigned bool) (value, error) {
	length, elementType, ok := parseMLIRArrayType(targetType)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir fixed-array fill requires an array target, got %s", targetType)
	}
	if len(expr.Arguments) != 1 {
		return value{}, fmt.Errorf("emit-mlir fixed-array fill expects one value, got %d", len(expr.Arguments))
	}

	// The source value is evaluated exactly once. Reusing its SSA value for
	// every insert models the copy semantics established by Sema and does not
	// introduce a runtime helper or dynamic allocation.
	element, err := g.emitExpressionForTargetUnsigned(expr.Arguments[0], elementType, targetUnsigned)
	if err != nil {
		return value{}, err
	}
	element, err = g.coerceValue(element, elementType, targetUnsigned)
	if err != nil {
		return value{}, fmt.Errorf("emit-mlir fixed-array fill value has type %s, expected %s", element.typ, elementType)
	}
	if elementType == "string" || elementType == "void" {
		return value{}, fmt.Errorf("emit-mlir does not support fixed-array fill element type %s yet", elementType)
	}

	aggregate := g.nextTemp()
	g.write("    %s = llvm.mlir.undef : %s\n", aggregate, targetType)
	for index := int64(0); index < length; index++ {
		next := g.nextTemp()
		g.write("    %s = llvm.insertvalue %s, %s[%d] : %s\n", next, element.ref, aggregate, index, targetType)
		aggregate = next
	}
	return value{typ: targetType, ref: aggregate, unsigned: targetUnsigned}, nil
}

func (g *Generator) emitArrayLiteral(expr *ast.ArrayLiteral, targetType string, targetUnsigned bool) (value, error) {
	if expr == nil {
		return value{}, fmt.Errorf("emit-mlir array literal is missing")
	}

	elementType := ""
	expectedLength := int64(len(expr.Elements))
	if targetType != "" {
		length, targetElementType, ok := parseMLIRArrayType(targetType)
		if !ok {
			return value{}, fmt.Errorf("emit-mlir cannot use array literal as %s", targetType)
		}
		if int64(len(expr.Elements)) != length {
			return value{}, fmt.Errorf("emit-mlir array literal has %d elements, expected %d", len(expr.Elements), length)
		}
		expectedLength = length
		elementType = targetElementType
	}
	if len(expr.Elements) == 0 && elementType == "" {
		return value{}, fmt.Errorf("emit-mlir cannot infer empty array literal type")
	}

	elements := make([]value, 0, len(expr.Elements))
	for _, elementExpr := range expr.Elements {
		element, err := g.emitExpressionForTargetUnsigned(elementExpr, elementType, targetUnsigned)
		if err != nil {
			return value{}, err
		}
		if elementType == "" {
			elementType = element.typ
			targetUnsigned = element.unsigned
		}
		element, err = g.coerceValue(element, elementType, targetUnsigned)
		if err != nil {
			return value{}, fmt.Errorf("emit-mlir array literal element has type %s, expected %s", element.typ, elementType)
		}
		if elementType == "string" || elementType == "void" {
			return value{}, fmt.Errorf("emit-mlir does not support array element type %s yet", elementType)
		}
		elements = append(elements, element)
	}

	arrayType := targetType
	if arrayType == "" {
		arrayType = fmt.Sprintf("!llvm.array<%d x %s>", expectedLength, elementType)
	}
	aggregate := g.nextTemp()
	g.write("    %s = llvm.mlir.undef : %s\n", aggregate, arrayType)
	for index, element := range elements {
		next := g.nextTemp()
		g.write("    %s = llvm.insertvalue %s, %s[%d] : %s\n", next, element.ref, aggregate, index, arrayType)
		aggregate = next
	}
	return value{typ: arrayType, ref: aggregate, unsigned: targetUnsigned}, nil
}

func (g *Generator) emitIndexExpression(expr *ast.IndexExpression) (value, error) {
	if expr == nil || expr.Left == nil || expr.Index == nil {
		return value{}, fmt.Errorf("emit-mlir index expression is incomplete")
	}
	array, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	length, elementType, ok := parseMLIRArrayType(array.typ)
	if !ok {
		return value{}, fmt.Errorf("emit-mlir cannot index non-array type %s", array.typ)
	}
	index, err := g.emitArrayIndex(expr.Index, length)
	if err != nil {
		return value{}, err
	}

	arrayPtr := g.emitAlloca(array.typ)
	g.write("    llvm.store %s, %s : %s, !llvm.ptr\n", array.ref, arrayPtr, array.typ)
	elementPtr := g.nextTemp()
	g.write("    %s = llvm.getelementptr %s[0, %s] : (!llvm.ptr, i64) -> !llvm.ptr, %s\n", elementPtr, arrayPtr, index.ref, array.typ)
	result := g.nextTemp()
	g.write("    %s = llvm.load %s : !llvm.ptr -> %s\n", result, elementPtr, elementType)
	return value{
		typ:        elementType,
		ref:        result,
		structName: g.structNameForMLIRType(elementType),
		unsigned:   array.unsigned,
	}, nil
}

func (g *Generator) emitArrayIndex(expr ast.Expression, length int64) (value, error) {
	index, err := g.emitExpression(expr)
	if err != nil {
		return value{}, err
	}
	if !isMLIRIntegerValue(index) {
		return value{}, fmt.Errorf("emit-mlir array index must be integer, got %s", index.typ)
	}
	index, err = g.coerceIntegerStorageValue(index, "i64", index.unsigned)
	if err != nil {
		return value{}, err
	}

	lengthValue := g.emitIntegerConstantUnsigned(strconv.FormatInt(length, 10), "i64", true)
	upper := g.nextTemp()
	g.write("    %s = llvm.icmp \"ult\" %s, %s : i64\n", upper, index.ref, lengthValue.ref)
	validRef := upper
	if !index.unsigned {
		zero := g.emitIntegerConstant("0", "i64")
		nonNegative := g.nextTemp()
		g.write("    %s = llvm.icmp \"sge\" %s, %s : i64\n", nonNegative, index.ref, zero.ref)
		valid := g.nextTemp()
		g.write("    %s = llvm.and %s, %s : i1\n", valid, nonNegative, upper)
		validRef = valid
	}

	validLabel := g.nextLabel("array.index.valid")
	trapLabel := g.nextLabel("array.index.trap")
	g.write("    llvm.cond_br %s, ^%s, ^%s\n", validRef, validLabel, trapLabel)
	g.blockOpen = false
	g.write("  ^%s:\n", trapLabel)
	g.write("    llvm.intr.trap\n")
	g.write("    llvm.unreachable\n")
	g.write("  ^%s:\n", validLabel)
	g.blockOpen = true
	return index, nil
}

func (g *Generator) emitIdentifier(expr *ast.Identifier) (value, error) {
	slot, ok := g.locals[expr.Value]
	if ok {
		if slot.direct {
			return value{typ: slot.typ, ref: slot.ref, len: slot.len, structName: slot.structName, enumName: slot.enumName, unsigned: slot.unsigned}, nil
		}
		return g.loadLocal(slot)
	}
	if constant, exists := g.constants[expr.Value]; exists {
		if constant.boolean != nil {
			return g.emitBoolConstant(*constant.boolean), nil
		}
		if constant.text != nil {
			return g.emitStringValue(*constant.text), nil
		}
		return g.emitIntegerConstantUnsigned(constant.literal, constant.typ, constant.unsigned), nil
	}
	return value{}, fmt.Errorf("emit-mlir unknown identifier %s", expr.Value)
}

func (g *Generator) resolveTopLevelConstant(stmt *ast.LetStatement) (mlirConstant, bool) {
	if stmt == nil || stmt.Value == nil {
		return mlirConstant{}, false
	}
	if boolean, ok := stmt.Value.(*ast.BooleanLiteral); ok {
		value := boolean.Value
		return mlirConstant{typ: "i1", boolean: &value}, true
	}
	if stringLiteral, ok := stmt.Value.(*ast.StringLiteral); ok {
		targetType := ""
		if stmt.Type != nil {
			targetType = g.mlirType(stmt.Type)
		}
		if targetType == "" || targetType == "string" {
			text := stringLiteral.Value
			return mlirConstant{typ: "string", text: &text}, true
		}
		return mlirConstant{}, false
	}
	integer, ok := enumIntegerConstant(stmt.Value, big.NewInt(0))
	if !ok {
		return mlirConstant{}, false
	}
	typ := ""
	unsigned := false
	if stmt.Type != nil {
		typ = g.mlirType(stmt.Type)
		unsigned = g.typeUnsigned(stmt.Type)
	}
	if typ == "" || !isMLIRIntegerType(typ) {
		suffix := ""
		if literal, ok := stmt.Value.(*ast.IntegerLiteral); ok {
			suffix = literal.Suffix()
		}
		typ = inferredIntegerLiteralType(integer, suffix)
		unsigned = suffix == "u"
		if unsigned {
			typ = inferredUnsignedIntegerLiteralType(integer)
		}
	}
	return mlirConstant{literal: integer.String(), typ: typ, unsigned: unsigned}, true
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
		if field.string {
			fieldValue, err = g.packString(fieldValue)
			if err != nil {
				return value{}, err
			}
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
	if typeName, ok := mlirExpressionPath(expr.Object); ok {
		if info, exists := g.enums[typeName]; exists {
			literal, found := info.values[expr.Property.Value]
			if !found {
				return value{}, fmt.Errorf("emit-mlir unknown enum value %s.%s", typeName, expr.Property.Value)
			}
			result := g.emitIntegerConstantUnsigned(literal, info.typ, info.unsigned)
			result.enumName = info.name
			return result, nil
		}
	}
	object, err := g.emitExpression(expr.Object)
	if err != nil {
		return value{}, err
	}
	if object.typ == "string" {
		switch expr.Property.Value {
		case "ptr":
			return value{typ: "!llvm.ptr", ref: object.ref}, nil
		case "len":
			return value{typ: "i64", ref: object.len, unsigned: true}, nil
		default:
			return value{}, fmt.Errorf("emit-mlir unknown string property %s", expr.Property.Value)
		}
	}
	if length, _, ok := parseMLIRArrayType(object.typ); ok {
		if expr.Property.Value != "len" {
			return value{}, fmt.Errorf("emit-mlir unknown array property %s", expr.Property.Value)
		}
		return g.emitIntegerConstantUnsigned(strconv.FormatInt(length, 10), "i64", true), nil
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
	if field.string {
		return g.unpackString(value{typ: mlirStringType, ref: result})
	}
	return value{
		typ:        field.typ,
		ref:        result,
		structName: field.structName,
		enumName:   field.enumName,
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
	case "+":
		return right, nil
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
		return literal.Suffix() == "m"
	case *ast.FloatLiteral:
		return literal.Suffix() != "g"
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
	if info, ok := g.enums[name]; ok {
		return g.emitEnumConversion(expr.Arguments, info)
	}
	if info, ok := g.namedTypes[name]; ok {
		if len(expr.Arguments) != 1 {
			return value{}, fmt.Errorf("emit-mlir conversion to %s expects one argument", name)
		}
		source, err := g.emitExpressionForTargetUnsigned(expr.Arguments[0], info.typ, info.unsigned)
		if err != nil {
			return value{}, err
		}
		return g.coerceValue(source, info.typ, info.unsigned)
	}
	if isMLIRBuiltinNumericTypeName(name) {
		return g.emitBuiltinNumericConversion(expr, name)
	}
	fn, err := g.resolveFunction(name, len(expr.Arguments))
	if err != nil {
		return value{}, err
	}
	if fn == nil {
		return value{}, fmt.Errorf("emit-mlir unknown function %s", name)
	}
	g.reachable[fn] = true

	args := []value{}
	for i, argExpr := range expr.Arguments {
		param := fn.Parameters[i]
		targetType := g.mlirParameterType(param)
		targetUnsigned := g.typeUnsigned(param.Type)
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
	returnUnsigned := g.typeUnsigned(fn.ReturnType)
	callReturnType := returnType
	if returnType == "string" {
		callReturnType = mlirStringType
	}
	result := ""
	if returnType != "void" {
		result = g.nextTemp()
		g.write("    %s = ", result)
	} else {
		g.write("    ")
	}
	g.write("llvm.call @%s(", mlirSymbolName(g.functionName(fn)))
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
	g.write(") -> %s\n", callReturnType)
	if returnType == "string" {
		return g.unpackString(value{typ: mlirStringType, ref: result})
	}
	return value{
		typ:        returnType,
		ref:        result,
		structName: g.structName(fn.ReturnType),
		enumName:   g.enumName(fn.ReturnType),
		unsigned:   returnUnsigned,
	}, nil
}

func (g *Generator) emitConversionExpression(expr *ast.ConversionExpression) (value, error) {
	if expr == nil || expr.Type == nil || expr.Value == nil {
		return value{}, fmt.Errorf("emit-mlir conversion expression is incomplete")
	}
	if info, ok := g.enums[expr.Type.Name]; ok {
		return g.emitEnumConversion([]ast.Expression{expr.Value}, info)
	}
	if info, ok := g.namedTypes[expr.Type.Name]; ok {
		source, err := g.emitExpressionForTargetUnsigned(expr.Value, info.typ, info.unsigned)
		if err != nil {
			return value{}, err
		}
		return g.coerceValue(source, info.typ, info.unsigned)
	}
	targetType := mlirBuiltinNumericType(expr.Type.Name)
	if targetType == "" {
		return value{}, fmt.Errorf("emit-mlir does not support conversion to %s yet", expr.Type.Name)
	}
	source, err := g.emitExpression(expr.Value)
	if err != nil {
		return value{}, err
	}
	result, err := g.coerceValue(source, targetType, isUnsignedBuiltinName(expr.Type.Name))
	if err != nil {
		return value{}, err
	}
	result.enumName = ""
	return result, nil
}

func (g *Generator) emitEnumConversion(arguments []ast.Expression, info *mlirEnum) (value, error) {
	if len(arguments) != 1 {
		return value{}, fmt.Errorf("conversion to %s expects 1 argument", info.name)
	}
	source, err := g.emitExpression(arguments[0])
	if err != nil {
		return value{}, err
	}
	if !isMLIRIntegerValue(source) {
		return value{}, fmt.Errorf("emit-mlir cannot convert %s to enum %s", source.typ, info.name)
	}
	result, err := g.coerceIntegerStorageValue(source, info.typ, info.unsigned)
	if err != nil {
		return value{}, err
	}
	result.enumName = info.name
	result.unsigned = info.unsigned
	return result, nil
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
	if suffix == "g" || suffix == "i" || suffix == "u" || suffix == "t" || suffix == "r" || digits == "" {
		return nil, 0, false
	}
	if integer, ok := ast.ParseIntegerFormNumericLiteralLexeme(lexeme); ok {
		if negative {
			integer.Neg(integer)
		}
		return integer, 0, true
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
	case source.typ == "!llvm.ptr" && isMLIRIntegerType(targetType):
		result := g.nextTemp()
		g.write("    %s = llvm.ptrtoint %s : !llvm.ptr to %s\n", result, source.ref, targetType)
		return value{typ: targetType, ref: result, unsigned: targetUnsigned}, nil
	default:
		result, err := g.coerceValue(source, targetType, targetUnsigned)
		if err != nil {
			return value{}, err
		}
		result.enumName = ""
		return result, nil
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
	if !isMLIRIntegerValue(left) || left.typ != right.typ {
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
	if !isMLIRIntegerValue(left) || left.typ != right.typ {
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

func (g *Generator) registerEnum(declaration *ast.EnumDeclaration, owner string) error {
	if declaration == nil || declaration.Name == nil {
		return fmt.Errorf("emit-mlir enum declaration is incomplete")
	}
	name := declaration.Name.Value
	if owner != "" {
		name = owner + "." + name
	}

	typ := "i32"
	unsigned := false
	if declaration.BitUnderlying {
		if declaration.UnderlyingBitWidth <= 0 || declaration.UnderlyingBitWidth > 256 {
			return fmt.Errorf("emit-mlir enum %s has unsupported bit width %d", name, declaration.UnderlyingBitWidth)
		}
		typ = fmt.Sprintf("i%d", declaration.UnderlyingBitWidth)
		unsigned = true
	} else if declaration.UnderlyingType != nil {
		typ = g.mlirType(declaration.UnderlyingType)
		unsigned = isUnsignedTypeReference(declaration.UnderlyingType)
	}
	if !isMLIRIntegerStorageType(typ) {
		return fmt.Errorf("emit-mlir enum %s requires a supported integer underlying type", name)
	}

	info := &mlirEnum{
		name:     name,
		typ:      typ,
		unsigned: unsigned,
		values:   map[string]string{},
	}
	previous := big.NewInt(-1)
	for index, variant := range declaration.Values {
		if variant == nil || variant.Name == nil {
			return fmt.Errorf("emit-mlir enum %s contains an incomplete value", name)
		}
		next := new(big.Int).Add(previous, big.NewInt(1))
		if variant.Initializer != nil {
			var ok bool
			next, ok = enumIntegerConstant(variant.Initializer, big.NewInt(int64(index)))
			if !ok {
				return fmt.Errorf("emit-mlir enum value %s.%s is not an integer constant", name, variant.Name.Value)
			}
		}
		info.values[variant.Name.Value] = next.String()
		previous = new(big.Int).Set(next)
	}
	g.enums[name] = info
	return nil
}

func enumIntegerConstant(expr ast.Expression, iotaValue *big.Int) (*big.Int, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	case *ast.Identifier:
		if expr.Value == "iota" {
			return new(big.Int).Set(iotaValue), true
		}
		return nil, false
	case *ast.ConversionExpression:
		return enumIntegerConstant(expr.Value, iotaValue)
	case *ast.CallExpression:
		if len(expr.Arguments) != 1 {
			return nil, false
		}
		return enumIntegerConstant(expr.Arguments[0], iotaValue)
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return nil, false
		}
		value, ok := enumIntegerConstant(expr.Right, iotaValue)
		if !ok {
			return nil, false
		}
		return new(big.Int).Neg(value), true
	case *ast.InfixExpression:
		left, ok := enumIntegerConstant(expr.Left, iotaValue)
		if !ok {
			return nil, false
		}
		right, ok := enumIntegerConstant(expr.Right, iotaValue)
		if !ok {
			return nil, false
		}
		result := new(big.Int)
		switch expr.Operator {
		case "+":
			return result.Add(left, right), true
		case "-":
			return result.Sub(left, right), true
		case "*":
			return result.Mul(left, right), true
		case "<<":
			if !right.IsUint64() {
				return nil, false
			}
			return result.Lsh(left, uint(right.Uint64())), true
		case ">>":
			if !right.IsUint64() {
				return nil, false
			}
			return result.Rsh(left, uint(right.Uint64())), true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
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
		if ref.Ref || ref.MutableRef || ref.Slice {
			return nil, fmt.Errorf("emit-mlir struct field %s.%s uses an unsupported compound type", name, declarationField.Name.Value)
		}
		if len(ref.TypeArgs) > 0 && ref.Name != "RawPtr" {
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
		if fieldType == "void" {
			return nil, fmt.Errorf("emit-mlir does not support field type %s in struct %s", ref.Name, name)
		}
		storageType := fieldType
		stringField := fieldType == "string"
		if stringField {
			storageType = mlirStringType
		}
		fieldTypes = append(fieldTypes, storageType)
		fields = append(fields, mlirStructField{
			name:       declarationField.Name.Value,
			typ:        fieldType,
			storageTyp: storageType,
			structName: fieldStructName,
			enumName:   g.enumName(ref),
			unsigned:   g.typeUnsigned(ref),
			string:     stringField,
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

func (g *Generator) enumName(ref *ast.TypeReference) string {
	if ref == nil {
		return ""
	}
	if _, ok := g.enums[ref.Name]; ok {
		return ref.Name
	}
	return ""
}

func (g *Generator) typeUnsigned(ref *ast.TypeReference) bool {
	if ref == nil {
		return false
	}
	if ref.ElementType != nil && !ref.Slice {
		return g.typeUnsigned(ref.ElementType)
	}
	if info, ok := g.enums[ref.Name]; ok {
		return info.unsigned
	}
	if info, ok := g.namedTypes[ref.Name]; ok {
		return info.unsigned
	}
	return isUnsignedTypeReference(ref)
}

func parseMLIRArrayType(typ string) (int64, string, bool) {
	const prefix = "!llvm.array<"
	if !strings.HasPrefix(typ, prefix) || !strings.HasSuffix(typ, ">") {
		return 0, "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(typ, prefix), ">")
	separator := strings.Index(body, " x ")
	if separator <= 0 {
		return 0, "", false
	}
	length, err := strconv.ParseInt(body[:separator], 10, 64)
	if err != nil || length < 0 {
		return 0, "", false
	}
	elementType := body[separator+3:]
	if elementType == "" {
		return 0, "", false
	}
	return length, elementType, true
}

func (g *Generator) structNameForMLIRType(typ string) string {
	for name, info := range g.structs {
		if info.typ == typ {
			return name
		}
	}
	return ""
}

func (g *Generator) mlirType(ref *ast.TypeReference) string {
	if ref == nil {
		return "void"
	}
	if ref.Name == "RawPtr" && len(ref.TypeArgs) == 1 {
		return "!llvm.ptr"
	}
	if ref.ElementType != nil {
		if ref.Slice {
			return "void"
		}
		length, ok := mlirArrayLength(ref)
		if !ok {
			return "void"
		}
		elementType := g.mlirType(ref.ElementType)
		if elementType == "void" || elementType == "string" {
			return "void"
		}
		return fmt.Sprintf("!llvm.array<%d x %s>", length, elementType)
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
		if info, ok := g.namedTypes[ref.Name]; ok {
			return info.typ
		}
		if info, ok := g.enums[ref.Name]; ok {
			return info.typ
		}
		if info, ok := g.structs[ref.Name]; ok {
			return info.typ
		}
		return "void"
	}
}

func mlirArrayLength(ref *ast.TypeReference) (int64, bool) {
	if ref == nil || ref.ElementType == nil || ref.Slice {
		return 0, false
	}
	if ref.ArrayLengthExpression == nil {
		return ref.ArrayLength, ref.ArrayLength >= 0
	}
	value, ok := enumIntegerConstant(ref.ArrayLengthExpression, big.NewInt(0))
	if !ok || !value.IsInt64() || value.Sign() < 0 {
		return 0, false
	}
	return value.Int64(), true
}

func (g *Generator) mlirParameterType(param *ast.Parameter) string {
	if param != nil && param.Ref {
		return "!llvm.ptr"
	}
	if param == nil {
		return "void"
	}
	return g.mlirType(param.Type)
}

func (g *Generator) functionName(fn *ast.FunctionDeclaration) string {
	if symbol, ok := g.functionNames[fn]; ok {
		return symbol
	}
	if fn != nil && fn.Name != nil {
		return fn.Name.Value
	}
	return ""
}

func mlirSymbolName(name string) string {
	if name != "" {
		valid := true
		for i, r := range name {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '$' || r == '.' || r == '_' || (i > 0 && r >= '0' && r <= '9') {
				continue
			}
			valid = false
			break
		}
		if valid {
			return name
		}
	}
	escaped := strings.ReplaceAll(name, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func (g *Generator) resolveFunction(name string, arity int) (*ast.FunctionDeclaration, error) {
	overloads := g.functions[name]
	var match *ast.FunctionDeclaration
	for _, fn := range overloads {
		if len(fn.Parameters) != arity {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("emit-mlir call %s is ambiguous for %d arguments", name, arity)
		}
		match = fn
	}
	if match != nil {
		return match, nil
	}
	if len(overloads) == 0 {
		return nil, nil
	}
	expected := make([]string, 0, len(overloads))
	for _, fn := range overloads {
		expected = append(expected, strconv.Itoa(len(fn.Parameters)))
	}
	return nil, fmt.Errorf("call %s expects one of [%s] arguments, got %d", name, strings.Join(expected, ", "), arity)
}

func (g *Generator) emitStringLiteral(expr *ast.StringLiteral) (value, error) {
	return g.emitStringValue(expr.Value), nil
}

func (g *Generator) emitStringValue(text string) value {
	name := fmt.Sprintf("__sec_str_%d", g.stringID)
	g.stringID++
	g.globals.WriteString(fmt.Sprintf("  llvm.mlir.global internal constant @%s(\"%s\") : !llvm.array<%d x i8>\n", name, mlirCString(text), len([]byte(text))))
	ptr := g.nextTemp()
	g.write("    %s = llvm.mlir.addressof @%s : !llvm.ptr\n", ptr, name)
	lenValue := g.emitIndexConstant(strconv.Itoa(len(text)))
	return value{typ: "string", ref: ptr, len: lenValue.ref}
}

func (g *Generator) packString(input value) (value, error) {
	if input.typ != "string" || input.ref == "" || input.len == "" {
		return value{}, fmt.Errorf("emit-mlir expected string value")
	}
	undef := g.nextTemp()
	g.write("    %s = llvm.mlir.undef : %s\n", undef, mlirStringType)
	withPtr := g.nextTemp()
	g.write("    %s = llvm.insertvalue %s, %s[0] : %s\n", withPtr, input.ref, undef, mlirStringType)
	result := g.nextTemp()
	g.write("    %s = llvm.insertvalue %s, %s[1] : %s\n", result, input.len, withPtr, mlirStringType)
	return value{typ: mlirStringType, ref: result}, nil
}

func (g *Generator) unpackString(descriptor value) (value, error) {
	if descriptor.typ != mlirStringType || descriptor.ref == "" {
		return value{}, fmt.Errorf("emit-mlir expected stored string descriptor")
	}
	ptr := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[0] : %s\n", ptr, descriptor.ref, mlirStringType)
	length := g.nextTemp()
	g.write("    %s = llvm.extractvalue %s[1] : %s\n", length, descriptor.ref, mlirStringType)
	return value{typ: "string", ref: ptr, len: length}, nil
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
	if isMLIRIntegerValue(val) && isMLIRIntegerType(targetType) {
		return g.coerceIntegerStorageValue(val, targetType, targetUnsigned)
	}
	if isMLIRIntegerType(val.typ) && targetType == "i1" {
		return g.coerceIntegerStorageValue(val, targetType, targetUnsigned)
	}
	if isMLIRIntegerValue(val) && isMLIRFloatType(targetType) {
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

func (g *Generator) coerceIntegerStorageValue(val value, targetType string, targetUnsigned bool) (value, error) {
	if !isMLIRIntegerStorageType(val.typ) || !isMLIRIntegerStorageType(targetType) {
		return value{}, fmt.Errorf("cannot convert %s to %s", val.typ, targetType)
	}
	if val.typ == targetType {
		val.unsigned = targetUnsigned
		return val, nil
	}
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

func (g *Generator) zeroValue(typ string) value {
	if _, _, ok := parseMLIRArrayType(typ); ok {
		result := g.nextTemp()
		g.write("    %s = llvm.mlir.undef : %s\n", result, typ)
		return value{typ: typ, ref: result}
	}
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

func isMLIRIntegerStorageType(typ string) bool {
	if !strings.HasPrefix(typ, "i") {
		return false
	}
	width, err := strconv.Atoi(strings.TrimPrefix(typ, "i"))
	return err == nil && width > 0
}

func isMLIRIntegerValue(val value) bool {
	return isMLIRIntegerType(val.typ) || (val.typ == "i1" && val.enumName != "")
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
	if expr.Callee != nil {
		if name, ok := mlirExpressionPath(expr.Callee); ok {
			return name
		}
	}
	if expr.Function != nil {
		return expr.Function.Value
	}
	return ""
}

func mlirExpressionPath(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		left, ok := mlirExpressionPath(expr.Object)
		if !ok || expr.Property == nil {
			return "", false
		}
		return left + "." + expr.Property.Value, true
	default:
		return "", false
	}
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
