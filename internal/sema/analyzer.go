package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type Analyzer struct {
	types                 map[string]Type
	units                 map[string]UnitDefinition
	functions             map[string][]Function
	implBlocks            map[string]lexer.Token
	validImplStatements   map[*ast.ImplStatement]bool
	currentImplTarget     string
	currentModule         string
	genericTypes          map[string]Type
	genericTypeInstances  map[genericInstanceKey]Type
	genericFuncInstances  map[genericInstanceKey]Function
	symbols               map[string]Symbol
	constInts             map[string]*big.Int
	assigned              map[string]bool
	currentFunctionName   string
	currentFunctionReturn Type
	inFunctionBody        bool
	inLambda              bool
	lambdaOuterSymbols    map[string]Symbol
	inUnsafe              bool
	inSwitchCaseBody      bool
	inDeferBlock          bool
	loopDepth             int
	loopBreakAssignments  [][]map[string]bool
	errors                []Error
	warnings              []Error
}

type genericInstanceKey struct {
	Declaration string
	Arguments   string
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		types: builtinTypes(),
		units: builtinUnits(),
	}
}

func (a *Analyzer) Analyze(program *ast.Program) []Error {
	a.errors = nil
	a.warnings = nil
	a.symbols = map[string]Symbol{}
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.functions = map[string][]Function{}
	a.implBlocks = map[string]lexer.Token{}
	a.validImplStatements = map[*ast.ImplStatement]bool{}
	a.currentImplTarget = ""
	a.currentModule = ""
	a.genericTypes = nil
	a.genericTypeInstances = map[genericInstanceKey]Type{}
	a.genericFuncInstances = map[genericInstanceKey]Function{}
	a.currentFunctionName = ""
	a.currentFunctionReturn = Type{}
	a.inFunctionBody = false
	a.inLambda = false
	a.lambdaOuterSymbols = nil
	a.inUnsafe = false
	a.inSwitchCaseBody = false
	a.inDeferBlock = false
	a.loopDepth = 0
	a.loopBreakAssignments = nil
	a.validateModuleDeclaration(program)
	a.registerTypeDeclarations(program)
	a.registerImplTypeDeclarations(program)
	a.analyzeTypeDeclarations(program)
	a.analyzeEnumDeclarations(program)
	a.analyzeImplTypeDeclarations(program)
	a.registerImplDeclarations(program)
	a.registerFunctionDeclarations(program)

	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt.(type) {
		case *ast.TargetDirective, *ast.TypeDeclStatement, *ast.UnitDeclStatement, *ast.EnumDeclaration, *ast.ImplStatement, *ast.FunctionDeclaration:
			return
		}
		if !isAllowedModuleStatement(stmt) {
			a.addTopLevelStatementError(stmt)
			return
		}
		a.analyzeStatement(stmt)
	})

	a.analyzeFunctionBodies(program)
	a.analyzeImplBodies(program)

	return a.errors
}

func (a *Analyzer) Warnings() []Error {
	return a.warnings
}

func (a *Analyzer) withProgramModules(program *ast.Program, visit func(ast.Statement)) {
	previous := a.currentModule
	module := ""
	for _, stmt := range program.Statements {
		if moduleStmt, ok := stmt.(*ast.ModuleStatement); ok {
			module = moduleStmt.Path
			continue
		}
		a.currentModule = module
		visit(stmt)
	}
	a.currentModule = previous
}

func (a *Analyzer) validateModuleDeclaration(program *ast.Program) {
	for _, stmt := range program.Statements {
		if moduleStmt, ok := stmt.(*ast.ModuleStatement); ok && moduleStmt.Path != "" {
			return
		}
	}

	a.errors = append(a.errors, Error{Message: "missing module declaration"})
}

func isAllowedModuleStatement(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.TargetDirective,
		*ast.ModuleStatement,
		*ast.ImportStatement,
		*ast.TypeDeclStatement,
		*ast.UnitDeclStatement,
		*ast.EnumDeclaration,
		*ast.ImplStatement,
		*ast.FunctionDeclaration,
		*ast.StructStatement,
		*ast.LetStatement,
		*ast.LetGroupStatement,
		*ast.CommentStatement,
		*ast.InvalidStatement:
		return true
	default:
		return false
	}
}

func (a *Analyzer) addTopLevelStatementError(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.AssignmentStatement:
		a.addErrorAtToken(stmt.Token, "assignment is not allowed at module scope")
	case *ast.ReturnStatement:
		a.addErrorAtToken(stmt.Token, "return is not allowed at module scope")
	case *ast.DeferStatement:
		a.addErrorAtToken(stmt.Token, "defer is only valid inside functions")
	default:
		a.addErrorAtToken(statementToken(stmt), "code is not allowed at module scope")
	}
}

func (a *Analyzer) registerTypeDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt := stmt.(type) {
		case *ast.TypeDeclStatement:
			if stmt.Name == nil {
				return
			}
			params := a.genericParameterNames(stmt.GenericParameters)
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType, GenericParameters: params}
		case *ast.UnitDeclStatement:
			if stmt.Name == nil {
				return
			}
			dimension := a.parseDimension(stmt.Name.Value)
			if dimension.IsZero() {
				dimension = dimensionFromBase(stmt.Name.Value, 1)
			}
			category := OtherUnit
			if existing, ok := a.units[stmt.Name.Value]; ok {
				category = existing.Category
				dimension = existing.Dimension
			}
			a.units[stmt.Name.Value] = UnitDefinition{Name: stmt.Name.Value, Category: category, Dimension: dimension, Token: stmt.Name.Token}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType}
		case *ast.EnumDeclaration:
			if stmt.Name == nil {
				return
			}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType}
		}
	})
}

func (a *Analyzer) genericParameterNames(parameters []*ast.GenericParameter) []string {
	if len(parameters) == 0 {
		return nil
	}
	names := make([]string, 0, len(parameters))
	seen := map[string]lexer.Token{}
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		if previous, exists := seen[param.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(param.Name.Token, "duplicate generic parameter %q", param.Name.Value)
			continue
		}
		seen[param.Name.Value] = param.Name.Token
		names = append(names, param.Name.Value)
	}
	return names
}

func genericParameterNameValues(parameters []*ast.GenericParameter) []string {
	if len(parameters) == 0 {
		return nil
	}
	names := make([]string, 0, len(parameters))
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		names = append(names, param.Name.Value)
	}
	return names
}

func (a *Analyzer) withGenericTypeParameters(parameters []*ast.GenericParameter, visit func()) {
	previous := a.genericTypes
	current := map[string]Type{}
	for name, typ := range previous {
		current[name] = typ
	}
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		current[param.Name.Value] = Type{
			Name: param.Name.Value,
			Kind: GenericType,
		}
	}
	a.genericTypes = current
	defer func() {
		a.genericTypes = previous
	}()
	visit()
}

func (a *Analyzer) validateGenericParameterConstraints(parameters []*ast.GenericParameter) {
	for _, param := range parameters {
		if param == nil || param.Name == nil || param.Constraint == nil {
			continue
		}
		name := a.resolveTypeName(param.Constraint.Name)
		constraint, ok := a.types[name]
		if !ok {
			a.addErrorAtToken(param.Constraint.Token, "unknown generic constraint %s for %s", param.Constraint.Name, param.Name.Value)
			continue
		}
		if constraint.Kind != InterfaceType {
			a.addErrorAtToken(param.Constraint.Token, "generic constraint %s is not an interface", param.Constraint.Name)
		}
	}
}

func (a *Analyzer) registerImplTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			continue
		}

		target, ok := a.types[impl.Target.Name]
		if !ok {
			a.addErrorAtToken(impl.Target.Token, "unknown impl target %s", impl.Target.Name)
			continue
		}
		if !target.Named && target.Kind != InvalidType {
			a.addErrorAtToken(impl.Target.Token, "impl target %s is not a named type", impl.Target.Name)
			continue
		}
		if !a.validateImplGenericTarget(impl, target) {
			continue
		}
		a.implBlocks[impl.Target.Name] = impl.Target.Token
		a.validImplStatements[impl] = true

		nested := map[string]lexer.Token{}
		for _, member := range impl.Members {
			name, token, ok := implNestedTypeName(member)
			if !ok {
				continue
			}
			if _, exists := nested[name]; exists {
				a.addErrorAtToken(token, "duplicate nested type %q in impl %s", name, impl.Target.Name)
				continue
			}
			nested[name] = token

			qualified := impl.Target.Name + "." + name
			if _, exists := a.types[qualified]; exists {
				a.addErrorAtToken(token, "duplicate nested type %q in impl %s", name, impl.Target.Name)
				continue
			}
			a.types[qualified] = Type{Name: qualified, Kind: InvalidType}
		}
	}
}

func implNestedTypeName(member ast.ImplMember) (string, lexer.Token, bool) {
	switch member := member.(type) {
	case *ast.TypeDeclStatement:
		if member.Name == nil {
			return "", lexer.Token{}, false
		}
		return member.Name.Value, member.Name.Token, true
	case *ast.UnitDeclStatement:
		if member.Name == nil {
			return "", lexer.Token{}, false
		}
		return member.Name.Value, member.Name.Token, true
	case *ast.EnumDeclaration:
		if member.Name == nil {
			return "", lexer.Token{}, false
		}
		return member.Name.Value, member.Name.Token, true
	default:
		return "", lexer.Token{}, false
	}
}

func (a *Analyzer) validateImplGenericTarget(stmt *ast.ImplStatement, target Type) bool {
	if len(target.GenericParameters) == 0 {
		if len(stmt.Target.TypeArgs) > 0 {
			a.addErrorAtToken(stmt.Target.Token, "%s is not generic", stmt.Target.Name)
			return false
		}
		return true
	}

	if len(stmt.Target.TypeArgs) == 0 {
		a.addErrorAtToken(stmt.Target.Token, "%s requires %d generic arguments, got 0", stmt.Target.Name, len(target.GenericParameters))
		return false
	}
	if len(stmt.Target.TypeArgs) != len(target.GenericParameters) {
		a.addErrorAtToken(stmt.Target.Token, "%s requires %d generic arguments, got %d", stmt.Target.Name, len(target.GenericParameters), len(stmt.Target.TypeArgs))
		return false
	}

	ok := true
	for i, arg := range stmt.Target.TypeArgs {
		expected := target.GenericParameters[i]
		if arg == nil || arg.Name != expected || len(arg.TypeArgs) > 0 || arg.ElementType != nil {
			a.addErrorAtToken(typeReferenceToken(arg, stmt.Target.Token), "unknown generic parameter %s in impl target %s", typeReferenceDisplayName(arg), stmt.Target.Name)
			ok = false
		}
	}
	return ok
}

func implGenericParametersForTarget(stmt *ast.ImplStatement, target Type) []*ast.GenericParameter {
	if len(target.GenericParameters) == 0 || len(stmt.Target.TypeArgs) != len(target.GenericParameters) {
		return nil
	}
	params := make([]*ast.GenericParameter, 0, len(stmt.Target.TypeArgs))
	for _, arg := range stmt.Target.TypeArgs {
		if arg == nil || arg.Name == "" || len(arg.TypeArgs) > 0 || arg.ElementType != nil {
			return nil
		}
		params = append(params, &ast.GenericParameter{
			Token: arg.Token,
			Name:  &ast.Identifier{Token: arg.Token, Value: arg.Name},
		})
	}
	return params
}

func typeReferenceToken(ref *ast.TypeReference, fallback lexer.Token) lexer.Token {
	if ref == nil {
		return fallback
	}
	return ref.Token
}

func typeReferenceDisplayName(ref *ast.TypeReference) string {
	if ref == nil {
		return "<nil>"
	}
	if ref.ElementType != nil {
		prefix := "[]"
		if ref.ArrayLength > 0 {
			prefix = fmt.Sprintf("[%d]", ref.ArrayLength)
		}
		return prefix + typeReferenceDisplayName(ref.ElementType)
	}
	out := ref.Name
	if len(ref.TypeArgs) > 0 {
		parts := make([]string, 0, len(ref.TypeArgs))
		for _, arg := range ref.TypeArgs {
			parts = append(parts, typeReferenceDisplayName(arg))
		}
		out += "[" + strings.Join(parts, ", ") + "]"
	}
	return out
}

func (a *Analyzer) analyzeTypeDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt := stmt.(type) {
		case *ast.TypeDeclStatement:
			a.analyzeTypeDeclaration(stmt)
		case *ast.UnitDeclStatement:
			a.analyzeUnitDeclaration(stmt)
		}
	})
}

func (a *Analyzer) analyzeEnumDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		enum, ok := stmt.(*ast.EnumDeclaration)
		if !ok {
			return
		}
		a.types[enum.Name.Value] = a.typeFromEnumDeclaration(enum.Name.Value, enum)
	})
}

func (a *Analyzer) analyzeImplTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			continue
		}
		if !a.validImplStatements[impl] {
			continue
		}
		target, ok := a.types[impl.Target.Name]
		if !ok {
			continue
		}
		genericParams := implGenericParametersForTarget(impl, target)

		for _, member := range impl.Members {
			switch member := member.(type) {
			case *ast.TypeDeclStatement:
				qualified := impl.Target.Name + "." + member.Name.Value
				a.withImplTarget(impl.Target.Name, func() {
					a.withGenericTypeParameters(genericParams, func() {
						a.analyzeNestedTypeDeclaration(qualified, member)
					})
				})
			case *ast.UnitDeclStatement:
				qualified := impl.Target.Name + "." + member.Name.Value
				a.withImplTarget(impl.Target.Name, func() {
					a.withGenericTypeParameters(genericParams, func() {
						a.analyzeNestedUnitDeclaration(qualified, member)
					})
				})
			case *ast.EnumDeclaration:
				qualified := impl.Target.Name + "." + member.Name.Value
				a.withImplTarget(impl.Target.Name, func() {
					a.types[qualified] = a.typeFromEnumDeclaration(qualified, member)
				})
			}
		}
	}
}

func (a *Analyzer) withImplTarget(target string, fn func()) {
	previous := a.currentImplTarget
	a.currentImplTarget = target
	defer func() {
		a.currentImplTarget = previous
	}()
	fn()
}

func (a *Analyzer) analyzeStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		a.analyzeTypeDeclaration(stmt)
	case *ast.UnitDeclStatement:
		a.analyzeUnitDeclaration(stmt)
	case *ast.EnumDeclaration:
		a.types[stmt.Name.Value] = a.typeFromEnumDeclaration(stmt.Name.Value, stmt)
	case *ast.FunctionDeclaration:
		a.registerFunctionDeclaration(stmt)
	case *ast.LetStatement:
		a.analyzeLetStatement(stmt)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			a.analyzeLetStatement(let)
		}
	case *ast.AssignmentStatement:
		a.analyzeAssignmentStatement(stmt, false)
	case *ast.TryAssignmentStatement:
		if stmt.Assignment != nil {
			a.analyzeAssignmentStatement(stmt.Assignment, true)
		}
		if len(stmt.Handlers) > 0 {
			a.analyzeTryAssignmentHandlers(stmt)
		}
	case *ast.DeferStatement:
		a.analyzeDeferStatement(stmt)
	case *ast.DiscardStatement:
		a.analyzeDiscardStatement(stmt)
	case *ast.ExpressionStatement:
		a.analyzeExpressionStatement(stmt)
	case *ast.ReturnStatement:
		if a.inDeferBlock {
			a.addErrorAtToken(stmt.Token, "return is not allowed inside defer")
			return
		}
		if a.inFunctionBody {
			a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, stmt)
		}
	case *ast.IfStatement:
		a.analyzeIfStatement(stmt)
	case *ast.ForStatement:
		a.analyzeForStatement(stmt)
	case *ast.WhileStatement:
		a.analyzeWhileStatement(stmt)
	case *ast.SwitchStatement:
		a.analyzeSwitchStatement(stmt)
	case *ast.MatchStatement:
		a.analyzeMatchStatement(stmt)
	case *ast.FallthroughStatement:
		if a.inDeferBlock {
			a.addErrorAtToken(stmt.Token, "fallthrough is not allowed inside defer")
			return
		}
		if !a.inSwitchCaseBody {
			a.addErrorAtToken(stmt.Token, "fallthrough is only valid directly inside a switch case")
		}
	case *ast.BreakStatement:
		if a.inDeferBlock {
			a.addErrorAtToken(stmt.Token, "break is not allowed inside defer")
			return
		}
		if a.loopDepth == 0 {
			a.addErrorAtToken(stmt.Token, "break is only valid inside a loop")
		} else {
			a.recordLoopBreak()
		}
	case *ast.ContinueStatement:
		if a.inDeferBlock {
			a.addErrorAtToken(stmt.Token, "continue is not allowed inside defer")
			return
		}
		if a.loopDepth == 0 {
			a.addErrorAtToken(stmt.Token, "continue is only valid inside a loop")
		}
	case *ast.UnsafeStatement:
		a.analyzeUnsafeStatement(stmt)
	case *ast.AsmStatement:
		a.analyzeAsmStatement(stmt)
	case *ast.ImplStatement:
		a.registerImplStatement(stmt)
	case *ast.StructStatement:
		for _, field := range stmt.Fields {
			a.resolveType(field.Type)
		}
	case *ast.InvalidStatement:
		return
	}
}

func (a *Analyzer) analyzeBlockStatements(block *ast.BlockStatement) {
	if block == nil {
		return
	}

	unreachable := false
	for _, stmt := range block.Statements {
		if unreachable {
			a.addErrorAtToken(statementToken(stmt), "unreachable code")
			break
		}
		a.analyzeStatement(stmt)
		if a.statementTerminatesBlock(stmt) {
			unreachable = true
		}
	}
}

func (a *Analyzer) analyzeExpressionStatement(stmt *ast.ExpressionStatement) {
	exprType, _ := a.inferExpression(stmt.Expression)
	if a.inDeferBlock && exprType.Kind == ResultType {
		a.addErrorAtToken(expressionToken(stmt.Expression), "unhandled Result inside defer; handle it or discard it explicitly")
	}
}

func (a *Analyzer) analyzeDiscardStatement(stmt *ast.DiscardStatement) {
	if stmt.Name == nil {
		a.addErrorAtToken(stmt.Token, "discard requires identifier")
		return
	}
	if stmt.Name.Value == "_" {
		a.addErrorAtToken(stmt.Name.Token, "discard requires named value")
		return
	}
	if _, ok := a.symbols[stmt.Name.Value]; !ok {
		a.addErrorAtToken(stmt.Name.Token, "undefined variable %s", stmt.Name.Value)
	}
}

func (a *Analyzer) analyzeIfStatement(stmt *ast.IfStatement) {
	if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "if condition must be bool, got %s", typeDisplayName(conditionType))
		}
	}

	before := copyAssigned(a.assigned)
	thenBranch := a.analyzeBranchBlock(stmt.Consequence)
	if stmt.Alternative != nil {
		elseBranch := a.analyzeBranchBlock(stmt.Alternative)
		a.assigned = mergeContinuingAssigned(before, thenBranch, elseBranch)
		return
	}

	a.assigned = mergeContinuingAssigned(before, thenBranch, branchAnalysis{
		assigned:  before,
		continues: true,
	})
}

func (a *Analyzer) analyzeDeferStatement(stmt *ast.DeferStatement) {
	if !a.inFunctionBody {
		a.addErrorAtToken(stmt.Token, "defer is only valid inside functions")
		return
	}
	if a.inDeferBlock {
		a.addErrorAtToken(stmt.Token, "defer is not allowed inside defer")
		return
	}
	if stmt.Body == nil {
		a.addErrorAtToken(stmt.Token, "defer requires a block")
		return
	}
	if a.loopDepth > 0 {
		a.addWarningAtToken(stmt.Token, "defer inside loop registers once per execution and runs at function exit")
	}
	if deferBodyIsBareReturn(stmt.Body) {
		a.addWarningAtToken(stmt.Token, "superfluous defer return")
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousInDeferBlock := a.inDeferBlock
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.inDeferBlock = true
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.inDeferBlock = previousInDeferBlock
	}()

	a.analyzeBlockStatements(stmt.Body)
}

func deferBodyIsBareReturn(block *ast.BlockStatement) bool {
	if block == nil || len(block.Statements) != 1 {
		return false
	}
	ret, ok := block.Statements[0].(*ast.ReturnStatement)
	return ok && ret.Value == nil
}

func (a *Analyzer) analyzeForStatement(stmt *ast.ForStatement) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.loopDepth++

	if len(stmt.Bindings) > 0 || stmt.Iterable != nil {
		a.analyzeForIterable(stmt)
	}

	if stmt.Body != nil {
		a.analyzeBlockStatements(stmt.Body)
	}

	_ = a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	a.assigned = previousAssigned
	a.loopDepth = previousLoopDepth
}

func (a *Analyzer) analyzeWhileStatement(stmt *ast.WhileStatement) {
	if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "while condition must be bool, got %s", typeDisplayName(conditionType))
		}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.loopDepth++

	if stmt.Body != nil {
		a.analyzeBlockStatements(stmt.Body)
	}

	breakAssignments := a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	if isBoolLiteral(stmt.Condition, true) && len(breakAssignments) > 0 {
		a.assigned = mergeBreakAssigned(previousAssigned, breakAssignments)
	} else {
		a.assigned = previousAssigned
	}
	a.loopDepth = previousLoopDepth
}

func (a *Analyzer) analyzeForIterable(stmt *ast.ForStatement) {
	if stmt.Iterable == nil {
		a.addErrorAtToken(stmt.Token, "for loop requires an iterable expression")
		return
	}

	bindingTypes, ok := a.inferForIterableBindingTypes(stmt)
	if !ok {
		return
	}

	if len(stmt.Bindings) != len(bindingTypes) {
		if len(stmt.Bindings) > 0 {
			a.addErrorAtToken(stmt.Bindings[0].Token, "iteration over %s requires %d loop binding(s), got %d", forIterableKind(stmt.Iterable), len(bindingTypes), len(stmt.Bindings))
		}
		return
	}

	for i, binding := range stmt.Bindings {
		if binding.Discard {
			continue
		}
		if a.defineSymbol(binding.Name, bindingTypes[i], false, binding.Token) {
			a.assigned[binding.Name] = true
		}
	}
}

func (a *Analyzer) inferForIterableBindingTypes(stmt *ast.ForStatement) ([]Type, bool) {
	switch iterable := stmt.Iterable.(type) {
	case *ast.RangeExpression:
		bindingType, ok := a.inferForRangeBindingType(iterable, stmt.Step)
		if !ok {
			return nil, false
		}
		return []Type{bindingType}, true
	default:
		if stmt.Step != nil {
			a.addErrorAtToken(expressionToken(stmt.Step), "for step is only valid for range iteration")
			return nil, false
		}
		iterableType, _ := a.inferExpression(iterable)
		if iterableType.Kind == InvalidType {
			return nil, false
		}
		indexType := Type{Name: "int", Kind: IntType}
		if iterableType.Kind == StringType {
			valueType := Type{Name: "rune", Kind: RuneType}
			if len(stmt.Bindings) > 2 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "sequential iteration supports one or two loop bindings, got %d", len(stmt.Bindings))
				return nil, false
			}
			if len(stmt.Bindings) == 2 {
				return []Type{indexType, valueType}, true
			}
			return []Type{valueType}, true
		}
		if (iterableType.Kind == ArrayType || iterableType.Kind == SliceType) && iterableType.Element != nil {
			if len(stmt.Bindings) > 2 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "sequential iteration supports one or two loop bindings, got %d", len(stmt.Bindings))
				return nil, false
			}
			if len(stmt.Bindings) == 2 {
				return []Type{indexType, *iterableType.Element}, true
			}
			return []Type{*iterableType.Element}, true
		}
		a.addErrorAtToken(expressionToken(iterable), "type %s is not iterable", typeDisplayName(iterableType))
		return nil, false
	}
}

func forIterableKind(expr ast.Expression) string {
	if _, ok := expr.(*ast.RangeExpression); ok {
		return "range"
	}
	return "iterable"
}

func (a *Analyzer) inferForRangeBindingType(expr *ast.RangeExpression, step ast.Expression) (Type, bool) {
	if expr.Start == nil || expr.End == nil {
		a.addErrorAtToken(expr.Token, "range used in for loop must be finite")
		return Type{Kind: InvalidType}, false
	}

	startType, _ := a.inferExpression(expr.Start)
	endType, _ := a.inferExpression(expr.End)
	if startType.Kind == InvalidType || endType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if !sameConcreteType(startType, endType) {
		a.addErrorAtToken(expr.Token, "cannot create range with bounds %s and %s", typeDisplayName(startType), typeDisplayName(endType))
		return Type{Kind: InvalidType}, false
	}

	if step != nil {
		stepType, _ := a.inferExpression(step)
		if stepType.Kind == InvalidType {
			return Type{Kind: InvalidType}, false
		}
		if !canInitialize(startType, stepType, step) {
			a.addErrorAtToken(expressionToken(step), "for range step must be %s, got %s", typeDisplayName(startType), typeDisplayName(stepType))
			return Type{Kind: InvalidType}, false
		}
		if value, ok := a.integerConstantValue(step); ok && value.Sign() <= 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must be greater than zero")
			return Type{Kind: InvalidType}, false
		}
		if value, ok := decimalLiteralValue(step); ok && value.Int64 <= 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must be greater than zero")
			return Type{Kind: InvalidType}, false
		}
	}

	if !isNumericType(startType) || !isNumericType(endType) {
		a.addErrorAtToken(expr.Token, "type %s is not iterable", typeDisplayName(startType))
		return Type{Kind: InvalidType}, false
	}

	return startType, true
}

type branchAnalysis struct {
	assigned  map[string]bool
	continues bool
}

func (a *Analyzer) analyzeBranchBlock(block *ast.BlockStatement) branchAnalysis {
	if block == nil {
		return branchAnalysis{assigned: copyAssigned(a.assigned), continues: true}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
	}()

	a.analyzeBlockStatements(block)
	return branchAnalysis{
		assigned:  copyAssigned(a.assigned),
		continues: !blockDefinitelyReturns(block),
	}
}

func mergeContinuingAssigned(before map[string]bool, branches ...branchAnalysis) map[string]bool {
	var merged map[string]bool
	foundContinuing := false
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		if !foundContinuing {
			merged = copyAssigned(branch.assigned)
			foundContinuing = true
			continue
		}
		for name := range before {
			merged[name] = merged[name] && branch.assigned[name]
		}
	}
	if !foundContinuing {
		return copyAssigned(before)
	}
	return merged
}

func (a *Analyzer) analyzeSwitchStatement(stmt *ast.SwitchStatement) {
	before := copyAssigned(a.assigned)
	if stmt.DefaultNotFinalToken.Type != "" {
		a.addErrorAtToken(stmt.DefaultNotFinalToken, "default must be the final switch clause")
	}
	for _, token := range stmt.DuplicateDefaultTokens {
		a.addErrorAtToken(token, "switch may contain only one default clause")
	}

	var subjectType Type
	hasSubject := stmt.Subject != nil
	if hasSubject {
		subjectType, _ = a.inferExpression(stmt.Subject)
		if subjectType.Kind == VoidType {
			a.addErrorAtToken(expressionToken(stmt.Subject), "switch subject cannot be void")
		}
	}

	tracker := newSwitchCoverageTracker()
	tracker.subjectType = subjectType
	clauses := append([]*ast.SwitchCase{}, stmt.Cases...)
	if stmt.Default != nil {
		clauses = append(clauses, stmt.Default)
	}

	branches := make([]branchAnalysis, 0, len(clauses)+1)
	for i, clause := range clauses {
		if clause == nil {
			continue
		}
		a.analyzeSwitchCaseItems(clause, hasSubject, subjectType, tracker)
		a.analyzeSwitchFallthrough(clause, i == len(clauses)-1)
		branches = append(branches, a.analyzeSwitchCaseBody(clause.Body))
	}

	if stmt.Default == nil && !tracker.isExhaustive() {
		branches = append(branches, branchAnalysis{assigned: before, continues: true})
	}
	a.assigned = mergeContinuingAssigned(before, branches...)
}

func (a *Analyzer) analyzeSwitchCaseItems(clause *ast.SwitchCase, hasSubject bool, subjectType Type, tracker *switchCoverageTracker) {
	if clause.Default {
		return
	}

	for _, item := range clause.Items {
		switch item := item.(type) {
		case *ast.SwitchValueCase:
			valueType, _ := a.inferExpression(item.Value)
			if valueType.Kind == InvalidType {
				continue
			}
			if hasSubject {
				if !canCompareEquality(subjectType, valueType) {
					a.addErrorAtToken(expressionToken(item.Value), "switch case must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(valueType))
				}
				a.checkSwitchValueCoverage(item.Value, tracker)
			} else if valueType.Kind != BoolType {
				a.addErrorAtToken(expressionToken(item.Value), "subjectless switch case must be bool, got %s", typeDisplayName(valueType))
			}
		case *ast.SwitchRangeCase:
			if !hasSubject {
				a.addErrorAtToken(item.Token, "subjectless switch case must be bool, got range")
				continue
			}
			a.analyzeSwitchRangeCase(item, subjectType)
			a.checkSwitchRangeCoverage(item.Range, tracker)
		case *ast.SwitchRelationalCase:
			if !hasSubject {
				a.addErrorAtToken(item.Token, "subjectless switch case must be bool, got relational case")
				continue
			}
			if !isOrderedSwitchType(subjectType) {
				a.addErrorAtToken(item.Token, "relational switch case requires ordered subject type")
				continue
			}
			valueType, _ := a.inferExpression(item.Value)
			if valueType.Kind != InvalidType && !canCompareEquality(subjectType, valueType) {
				a.addErrorAtToken(expressionToken(item.Value), "switch case must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(valueType))
			}
			a.checkSwitchRelationalCoverage(item, tracker)
		}
	}
}

type switchCoverageTracker struct {
	subjectType Type
	values      map[string]lexer.Token
	ranges      []switchConstRange
	boolValues  map[bool]lexer.Token
}

type switchConstRange struct {
	min          *big.Int
	minExclusive bool
	max          *big.Int
	maxExclusive bool
	token        lexer.Token
	relational   bool
}

func newSwitchCoverageTracker() *switchCoverageTracker {
	return &switchCoverageTracker{
		values:     map[string]lexer.Token{},
		boolValues: map[bool]lexer.Token{},
	}
}

func (a *Analyzer) analyzeSwitchRangeCase(item *ast.SwitchRangeCase, subjectType Type) {
	if !isOrderedSwitchType(subjectType) {
		a.addErrorAtToken(item.Token, "switch range requires ordered subject type")
		return
	}
	if item.Range == nil {
		return
	}
	if item.Range.Start != nil {
		startType, _ := a.inferExpression(item.Range.Start)
		if startType.Kind != InvalidType && !canRangeBoundType(subjectType, startType, item.Range.Start) {
			a.addErrorAtToken(expressionToken(item.Range.Start), "switch range must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(startType))
		}
	}
	if item.Range.End != nil {
		endType, _ := a.inferExpression(item.Range.End)
		if endType.Kind != InvalidType && !canRangeBoundType(subjectType, endType, item.Range.End) {
			a.addErrorAtToken(expressionToken(item.Range.End), "switch range must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(endType))
		}
	}
}

func (a *Analyzer) checkSwitchValueCoverage(expr ast.Expression, tracker *switchCoverageTracker) {
	if tracker == nil {
		return
	}
	if literal, ok := expr.(*ast.BooleanLiteral); ok && tracker.subjectType.Kind == BoolType {
		if _, exists := tracker.boolValues[literal.Value]; exists {
			a.addErrorAtToken(expressionToken(expr), "duplicate switch case value %t", literal.Value)
			return
		}
		tracker.boolValues[literal.Value] = expressionToken(expr)
		return
	}
	value, ok := constantIntegerValue(expr)
	if !ok {
		return
	}
	key := value.String()
	if _, exists := tracker.values[key]; exists {
		a.addErrorAtToken(expressionToken(expr), "duplicate switch case value %s", key)
		return
	}
	for _, previous := range tracker.ranges {
		if previous.contains(value) {
			a.addErrorAtToken(expressionToken(expr), "switch case value %s is already covered by previous case", key)
			return
		}
	}
	tracker.values[key] = expressionToken(expr)
}

func (t *switchCoverageTracker) isExhaustive() bool {
	return t != nil && t.subjectType.Kind == BoolType && len(t.boolValues) == 2
}

func (a *Analyzer) checkSwitchRangeCoverage(expr *ast.RangeExpression, tracker *switchCoverageTracker) {
	if tracker == nil || expr == nil {
		return
	}
	current, ok := switchConstRangeFromExpression(expr)
	if !ok {
		return
	}
	for _, previous := range tracker.ranges {
		if previous.relational && !current.coveredBy(previous) {
			continue
		}
		if current.overlaps(previous) {
			a.addErrorAtToken(expr.Token, "%s", switchCoverageOverlapMessage(current, previous))
			return
		}
	}
	for key := range tracker.values {
		value, ok := new(big.Int).SetString(key, 10)
		if ok && current.contains(value) {
			a.addErrorAtToken(expr.Token, "switch case range overlaps previous case")
			return
		}
	}
	tracker.ranges = append(tracker.ranges, current)
}

func (a *Analyzer) checkSwitchRelationalCoverage(item *ast.SwitchRelationalCase, tracker *switchCoverageTracker) {
	if tracker == nil || item == nil {
		return
	}
	current, ok := switchConstRangeFromRelationalCase(item)
	if !ok {
		return
	}
	for _, previous := range tracker.ranges {
		if previous.relational {
			if current.coveredBy(previous) {
				a.addErrorAtToken(item.Token, "unreachable switch case; previous case already covers this condition")
				return
			}
			continue
		}
		if current.overlaps(previous) {
			a.addErrorAtToken(item.Token, "unreachable switch case; previous case already covers this condition")
			return
		}
	}
	tracker.ranges = append(tracker.ranges, current)
}

func switchCoverageOverlapMessage(current switchConstRange, previous switchConstRange) string {
	if current.relational || previous.relational {
		return "unreachable switch case; previous case already covers this condition"
	}
	return "switch case range overlaps previous case"
}

func switchConstRangeFromExpression(expr *ast.RangeExpression) (switchConstRange, bool) {
	out := switchConstRange{token: expr.Token, maxExclusive: expr.Exclusive}
	if expr.Start != nil {
		value, ok := constantIntegerValue(expr.Start)
		if !ok {
			return switchConstRange{}, false
		}
		out.min = value
	}
	if expr.End != nil {
		value, ok := constantIntegerValue(expr.End)
		if !ok {
			return switchConstRange{}, false
		}
		out.max = value
	}
	out.normalizeBounds()
	return out, true
}

func switchConstRangeFromRelationalCase(item *ast.SwitchRelationalCase) (switchConstRange, bool) {
	value, ok := constantIntegerValue(item.Value)
	if !ok {
		return switchConstRange{}, false
	}
	out := switchConstRange{token: item.Token, relational: true}
	switch item.Operator {
	case "<":
		out.max = value
		out.maxExclusive = true
	case "<=":
		out.max = value
	case ">":
		out.min = value
		out.minExclusive = true
	case ">=":
		out.min = value
	default:
		return switchConstRange{}, false
	}
	return out, true
}

func (r switchConstRange) contains(value *big.Int) bool {
	if r.min != nil {
		cmp := value.Cmp(r.min)
		if cmp < 0 || (cmp == 0 && r.minExclusive) {
			return false
		}
	}
	if r.max != nil {
		cmp := value.Cmp(r.max)
		if cmp > 0 || (cmp == 0 && r.maxExclusive) {
			return false
		}
	}
	return true
}

func (r switchConstRange) overlaps(other switchConstRange) bool {
	if r.max != nil && other.min != nil {
		cmp := r.max.Cmp(other.min)
		if cmp < 0 || (cmp == 0 && (r.maxExclusive || other.minExclusive)) {
			return false
		}
	}
	if other.max != nil && r.min != nil {
		cmp := other.max.Cmp(r.min)
		if cmp < 0 || (cmp == 0 && (other.maxExclusive || r.minExclusive)) {
			return false
		}
	}
	return true
}

func (r *switchConstRange) normalizeBounds() {
	if r == nil || r.min == nil || r.max == nil || r.min.Cmp(r.max) <= 0 {
		return
	}
	r.min, r.max = r.max, r.min
	r.minExclusive, r.maxExclusive = r.maxExclusive, r.minExclusive
}

func (r switchConstRange) coveredBy(other switchConstRange) bool {
	return lowerBoundCovers(other, r) && upperBoundCovers(other, r)
}

func lowerBoundCovers(outer switchConstRange, inner switchConstRange) bool {
	if outer.min == nil {
		return true
	}
	if inner.min == nil {
		return false
	}
	cmp := outer.min.Cmp(inner.min)
	if cmp < 0 {
		return true
	}
	if cmp > 0 {
		return false
	}
	return !outer.minExclusive || inner.minExclusive
}

func upperBoundCovers(outer switchConstRange, inner switchConstRange) bool {
	if outer.max == nil {
		return true
	}
	if inner.max == nil {
		return false
	}
	cmp := outer.max.Cmp(inner.max)
	if cmp > 0 {
		return true
	}
	if cmp < 0 {
		return false
	}
	return !outer.maxExclusive || inner.maxExclusive
}

func (a *Analyzer) analyzeSwitchFallthrough(clause *ast.SwitchCase, isFinal bool) {
	if clause == nil || clause.Body == nil {
		return
	}
	fallthroughIndex := -1
	for i, stmt := range clause.Body.Statements {
		if _, ok := stmt.(*ast.FallthroughStatement); ok {
			fallthroughIndex = i
		}
	}
	if fallthroughIndex == -1 {
		return
	}
	if clause.Default || isFinal {
		a.addErrorAtToken(clause.Body.Statements[fallthroughIndex].(*ast.FallthroughStatement).Token, "fallthrough is not allowed in the final switch case")
	}
	for i := fallthroughIndex + 1; i < len(clause.Body.Statements); i++ {
		if _, ok := clause.Body.Statements[i].(*ast.CommentStatement); ok {
			continue
		}
		a.addErrorAtToken(clause.Body.Statements[fallthroughIndex].(*ast.FallthroughStatement).Token, "fallthrough must be the final statement in a switch case")
		return
	}
}

func (a *Analyzer) analyzeSwitchCaseBody(block *ast.BlockStatement) branchAnalysis {
	if block == nil {
		return branchAnalysis{assigned: copyAssigned(a.assigned), continues: true}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousInSwitchCaseBody := a.inSwitchCaseBody
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.inSwitchCaseBody = true
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.inSwitchCaseBody = previousInSwitchCaseBody
	}()

	hasFallthrough := blockEndsWithFallthrough(block)
	a.analyzeBlockStatements(block)
	return branchAnalysis{
		assigned:  copyAssigned(a.assigned),
		continues: !blockDefinitelyReturns(block) && !hasFallthrough,
	}
}

func isOrderedSwitchType(typ Type) bool {
	return isNumericType(typ) || typ.Kind == StringType || typ.Kind == CharType || typ.Kind == RuneType
}

func (a *Analyzer) analyzeUnsafeStatement(stmt *ast.UnsafeStatement) {
	if stmt.Body == nil {
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousUnsafe := a.inUnsafe

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.inUnsafe = true

	a.analyzeBlockStatements(stmt.Body)
	if unsafeAsmReturns(stmt) && a.currentFunctionReturn.Kind != InvalidType && a.currentFunctionReturn.Kind != VoidType && a.currentFunctionReturn.Name != "int64" {
		a.addErrorAtToken(stmt.Token, "asm output rax cannot return %s", typeDisplayName(a.currentFunctionReturn))
	}

	updatedAssigned := a.assigned
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	a.assigned = previousAssigned
	a.inUnsafe = previousUnsafe

	for name := range previousAssigned {
		if updatedAssigned[name] {
			a.assigned[name] = true
		}
	}
}

func (a *Analyzer) analyzeAsmStatement(stmt *ast.AsmStatement) {
	if !a.inUnsafe {
		a.addErrorAtToken(stmt.Token, "asm is only allowed inside unsafe")
		return
	}
	if stmt.Block != nil {
		a.analyzeAsmBlock(stmt)
		return
	}
	if stmt.Template == nil {
		a.addErrorAtToken(stmt.Token, "asm statement requires string template")
	}
}

func unsafeAsmReturns(stmt *ast.UnsafeStatement) bool {
	if stmt == nil || stmt.Body == nil || len(stmt.Body.Statements) != 1 {
		return false
	}
	asmStmt, ok := stmt.Body.Statements[0].(*ast.AsmStatement)
	return ok && asmStmt.Block != nil && len(asmStmt.Block.Outputs) > 0
}

func (a *Analyzer) analyzeAsmBlock(stmt *ast.AsmStatement) {
	if stmt.Block.Template == nil {
		a.addErrorAtToken(stmt.Token, "asm block requires string template")
	}
	for _, input := range stmt.Block.Inputs {
		if input.Value == nil {
			a.addErrorAtToken(stmt.Token, "asm input %s missing value", input.Register)
			continue
		}
		a.inferExpression(input.Value)
	}
	if len(stmt.Block.Outputs) == 0 {
		a.addErrorAtToken(stmt.Token, "asm block requires at least one output")
	}
	for _, output := range stmt.Block.Outputs {
		if output.Name == "" {
			continue
		}
		outputType := a.currentFunctionReturn
		if outputType.Kind == InvalidType || outputType.Kind == VoidType {
			outputType = Type{Name: "int64", Kind: IntType}
		}
		if a.defineSymbol(output.Name, outputType, false, stmt.Token) {
			a.assigned[output.Name] = true
		}
	}
}

func (a *Analyzer) registerFunctionDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			return
		}
		a.registerFunctionDeclaration(fn)
	})
}

func (a *Analyzer) registerFunctionDeclaration(fn *ast.FunctionDeclaration) {
	a.registerFunctionDeclarationNamed(fn, fn.Name.Value)
}

func (a *Analyzer) registerFunctionDeclarationNamed(fn *ast.FunctionDeclaration, name string) {
	if len(fn.GenericParameters) > 0 {
		a.genericParameterNames(fn.GenericParameters)
		a.validateGenericParameterConstraints(fn.GenericParameters)
		a.withGenericTypeParameters(fn.GenericParameters, func() {
			a.registerFunctionDeclarationBody(fn, name)
		})
		return
	}
	a.registerFunctionDeclarationBody(fn, name)
}

func (a *Analyzer) registerFunctionDeclarationBody(fn *ast.FunctionDeclaration, name string) {
	function := Function{
		Name:              name,
		Module:            a.currentModule,
		GenericParameters: genericParameterNameValues(fn.GenericParameters),
		Token:             fn.Name.Token,
		Extern:            fn.Extern,
		ABI:               fn.ABI,
	}
	if fn.Extern && !isSupportedExternABI(fn.ABI) {
		a.addErrorAtToken(fn.Token, "unknown extern ABI %q", fn.ABI)
	}

	seenParams := map[string]lexer.Token{}
	for _, param := range fn.Parameters {
		if _, exists := seenParams[param.Name.Value]; exists {
			a.addErrorAtToken(param.Name.Token, "duplicate parameter %q", param.Name.Value)
			continue
		}
		seenParams[param.Name.Value] = param.Name.Token

		paramType, ok := a.resolveType(param.Type)
		if !ok {
			continue
		}
		function.Parameters = append(function.Parameters, FunctionParameter{
			Name:  param.Name.Value,
			Type:  paramType,
			Token: param.Name.Token,
			Ref:   param.Ref,
		})
	}

	returnType, ok := a.resolveType(fn.ReturnType)
	if ok {
		function.ReturnType = returnType
	} else {
		function.ReturnType = Type{Kind: InvalidType}
	}

	for _, existing := range a.functions[name] {
		if sameFunctionSignature(existing, function) {
			a.addErrorAtToken(fn.Name.Token, "duplicate function %q with same signature", name)
			return
		}
	}

	a.functions[name] = append(a.functions[name], function)
}

func sameFunctionSignature(left Function, right Function) bool {
	if len(left.Parameters) != len(right.Parameters) {
		return false
	}
	for i := range left.Parameters {
		if !sameConcreteType(left.Parameters[i].Type, right.Parameters[i].Type) {
			return false
		}
	}
	return true
}

func isSupportedExternABI(abi string) bool {
	switch abi {
	case "Sec", "C", "system":
		return true
	default:
		return false
	}
}

func (a *Analyzer) validateExternFunction(function Function) {
	for i, param := range function.Parameters {
		if !isFFICompatibleType(param.Type) {
			a.addErrorAtToken(param.Token, "extern %s parameter %d %s has non-ABI-compatible type %s", function.ABI, i+1, param.Name, typeDisplayName(param.Type))
		}
	}
	if function.ReturnType.Kind != VoidType && !isFFICompatibleType(function.ReturnType) {
		a.addErrorAtToken(function.Token, "extern %s function %s has non-ABI-compatible return type %s", function.ABI, function.Name, typeDisplayName(function.ReturnType))
	}
}

func isFFICompatibleType(typ Type) bool {
	switch typ.Kind {
	case IntType, UintType, FloatType, RawPtrType, VoidType:
		return true
	case EnumType:
		return typ.Underlying != "" && typ.Underlying != "enum"
	default:
		return false
	}
}

func (a *Analyzer) lookupFunctionByToken(name string, token lexer.Token) (Function, bool) {
	for _, function := range a.functions[name] {
		if function.Token.Line == token.Line && function.Token.Column == token.Column {
			return function, true
		}
	}
	return Function{}, false
}

func (a *Analyzer) analyzeFunctionBodies(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			return
		}
		a.analyzeFunctionBody(fn)
	})
}

func (a *Analyzer) analyzeFunctionBody(fn *ast.FunctionDeclaration) {
	if len(fn.GenericParameters) > 0 {
		a.withGenericTypeParameters(fn.GenericParameters, func() {
			a.analyzeFunctionBodyInScope(fn)
		})
		return
	}
	a.analyzeFunctionBodyInScope(fn)
}

func (a *Analyzer) analyzeFunctionBodyInScope(fn *ast.FunctionDeclaration) {
	function, ok := a.lookupFunctionByToken(fn.Name.Value, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	if function.Extern {
		a.validateExternFunction(function)
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousFunctionName := a.currentFunctionName
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousUnsafe := a.inUnsafe
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.currentFunctionName = fn.Name.Value
	a.currentFunctionReturn = function.ReturnType
	a.inFunctionBody = true
	if fn.Unsafe {
		a.inUnsafe = true
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.currentFunctionName = previousFunctionName
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.inUnsafe = previousUnsafe
	}()

	for _, param := range function.Parameters {
		a.symbols[param.Name] = Symbol{Name: param.Name, Type: param.Type, Mutable: false, Token: param.Token}
		delete(a.constInts, param.Name)
		a.assigned[param.Name] = true
	}

	a.analyzeBlockStatements(fn.Body)

	if !a.blockDefinitelyReturns(fn.Body) && function.ReturnType.Kind != VoidType {
		a.addErrorAtToken(fn.Name.Token, "function %s must return %s", fn.Name.Value, typeDisplayName(function.ReturnType))
	}
}

func (a *Analyzer) blockDefinitelyReturns(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}

	for _, stmt := range block.Statements {
		if a.statementDefinitelyReturns(stmt) {
			return true
		}
	}
	return false
}

func blockDefinitelyReturns(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}

	for _, stmt := range block.Statements {
		if statementDefinitelyReturns(stmt) {
			return true
		}
	}
	return false
}

func statementDefinitelyReturns(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStatement:
		return true
	case *ast.IfStatement:
		if stmt.Alternative == nil {
			return false
		}
		return blockDefinitelyReturns(stmt.Consequence) && blockDefinitelyReturns(stmt.Alternative)
	case *ast.SwitchStatement:
		return switchDefinitelyReturns(stmt)
	case *ast.ForStatement:
		return len(stmt.Bindings) == 0 && stmt.Iterable == nil && !blockContainsBreak(stmt.Body)
	case *ast.WhileStatement:
		return isBoolLiteral(stmt.Condition, true) && !blockContainsBreak(stmt.Body)
	case *ast.UnsafeStatement:
		if unsafeAsmReturns(stmt) {
			return true
		}
		return blockDefinitelyReturns(stmt.Body)
	case *ast.MatchStatement:
		return false
	default:
		return false
	}
}

func (a *Analyzer) statementDefinitelyReturns(stmt ast.Statement) bool {
	if matchStmt, ok := stmt.(*ast.MatchStatement); ok {
		return a.matchStatementDefinitelyReturns(matchStmt)
	}
	return statementDefinitelyReturns(stmt)
}

func (a *Analyzer) matchStatementDefinitelyReturns(stmt *ast.MatchStatement) bool {
	if stmt == nil || stmt.Match == nil || stmt.Match.Subject == nil || len(stmt.Match.Arms) == 0 {
		return false
	}

	subjectType, _ := a.inferExpression(stmt.Match.Subject)
	if subjectType.Kind == InvalidType {
		return false
	}

	seenKinds := map[string]bool{}
	seenVariants := map[string]bool{}
	catchAll := false

	for _, arm := range stmt.Match.Arms {
		if arm == nil || !matchArmDefinitelyReturns(arm) {
			return false
		}
		if arm.Guard != nil {
			continue
		}
		info, ok := a.matchPatternInfoNoDiagnostics(arm.Pattern, subjectType)
		if !ok {
			return false
		}
		if info.Kind == "catchall" {
			catchAll = true
		}
		if info.Kind != "" {
			seenKinds[info.Kind] = true
		}
		if info.Variant != "" {
			seenVariants[info.Variant] = true
		}
	}

	if catchAll {
		return true
	}
	if subjectType.Kind == ResultType && len(subjectType.TypeArgs) == 2 {
		return seenKinds["Ok"] && seenKinds["Err"]
	}
	if enumValues, ok := a.enumValuesForType(subjectType); ok {
		for _, variant := range enumValues {
			if !seenVariants[variant] {
				return false
			}
		}
		return true
	}
	return false
}

func matchArmDefinitelyReturns(arm *ast.MatchArm) bool {
	if arm.ReturnBody != nil {
		return true
	}
	return blockDefinitelyReturns(arm.BlockBody)
}

func (a *Analyzer) matchPatternInfoNoDiagnostics(pattern ast.Expression, subjectType Type) (matchPatternInfo, bool) {
	switch pattern := pattern.(type) {
	case *ast.OkExpression:
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "Ok"}, true
	case *ast.ErrExpression:
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "Err"}, true
	case *ast.MemberExpression:
		patternType, ok := a.inferMemberExpression(pattern)
		if !ok || patternType.Kind == InvalidType || !sameConcreteType(patternType, subjectType) {
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "variant", Variant: pattern.Property.Value}, true
	case *ast.Identifier:
		return matchPatternInfo{BindingName: pattern.Value, BindingType: subjectType, Kind: "catchall"}, true
	default:
		patternType, _ := a.inferExpression(pattern)
		if patternType.Kind == InvalidType || !canInitialize(subjectType, patternType, pattern) {
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "literal"}, true
	}
}

func (a *Analyzer) statementTerminatesBlock(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.BreakStatement, *ast.ContinueStatement:
		return a.loopDepth > 0
	default:
		return a.statementDefinitelyReturns(stmt)
	}
}

func blockContainsBreak(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if statementContainsBreak(stmt) {
			return true
		}
	}
	return false
}

func statementContainsBreak(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.BreakStatement:
		return true
	case *ast.IfStatement:
		return blockContainsBreak(stmt.Consequence) || blockContainsBreak(stmt.Alternative)
	case *ast.ForStatement:
		return blockContainsBreak(stmt.Body)
	case *ast.WhileStatement:
		return blockContainsBreak(stmt.Body)
	case *ast.SwitchStatement:
		for _, clause := range stmt.Cases {
			if blockContainsBreak(clause.Body) {
				return true
			}
		}
		return stmt.Default != nil && blockContainsBreak(stmt.Default.Body)
	case *ast.UnsafeStatement:
		return blockContainsBreak(stmt.Body)
	default:
		return false
	}
}

func isBoolLiteral(expr ast.Expression, value bool) bool {
	lit, ok := expr.(*ast.BooleanLiteral)
	return ok && lit.Value == value
}

func switchDefinitelyReturns(stmt *ast.SwitchStatement) bool {
	if stmt.Default == nil && !switchCoversBoolLiterals(stmt) {
		return false
	}

	nextTerminates := false
	if stmt.Default != nil {
		nextTerminates = blockDefinitelyReturns(stmt.Default.Body)
		if !nextTerminates {
			return false
		}
	}

	for i := len(stmt.Cases) - 1; i >= 0; i-- {
		clause := stmt.Cases[i]
		if clause == nil {
			return false
		}
		terminates := blockDefinitelyReturns(clause.Body) ||
			(blockEndsWithFallthrough(clause.Body) && nextTerminates)
		if !terminates {
			return false
		}
		nextTerminates = terminates
	}
	return true
}

func blockEndsWithFallthrough(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for i := len(block.Statements) - 1; i >= 0; i-- {
		if _, ok := block.Statements[i].(*ast.CommentStatement); ok {
			continue
		}
		_, ok := block.Statements[i].(*ast.FallthroughStatement)
		return ok
	}
	return false
}

func switchCoversBoolLiterals(stmt *ast.SwitchStatement) bool {
	if stmt == nil || stmt.Subject == nil {
		return false
	}
	seen := map[bool]bool{}
	for _, clause := range stmt.Cases {
		for _, item := range clause.Items {
			valueCase, ok := item.(*ast.SwitchValueCase)
			if !ok {
				continue
			}
			literal, ok := valueCase.Value.(*ast.BooleanLiteral)
			if ok {
				seen[literal.Value] = true
			}
		}
	}
	return seen[true] && seen[false]
}

func (a *Analyzer) analyzeReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if stmt.Value == nil {
		if returnType.Kind != VoidType {
			if functionName == "lambda" {
				a.addErrorAtToken(stmt.Token, "lambda must return %s", typeDisplayName(returnType))
				return
			}
			a.addErrorAtToken(stmt.Token, "function %s must return %s", functionName, typeDisplayName(returnType))
		}
		return
	}

	if returnType.Kind == ResultType {
		a.analyzeResultReturnStatement(functionName, returnType, stmt)
		return
	}

	valueType, _ := a.inferExpressionWithExpected(stmt.Value, returnType)
	if valueType.Kind == InvalidType {
		return
	}

	if returnType.Kind == VoidType {
		if functionName == "lambda" {
			a.addErrorAtToken(expressionToken(stmt.Value), "lambda must return void, got %s", typeDisplayName(valueType))
			return
		}
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s must return void, got %s", functionName, typeDisplayName(valueType))
		return
	}

	if !canInitialize(returnType, valueType, stmt.Value) {
		if functionName == "lambda" {
			a.addErrorAtToken(expressionToken(stmt.Value), "lambda must return %s, got %s", typeDisplayName(returnType), typeDisplayName(valueType))
			return
		}
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s must return %s, got %s", functionName, typeDisplayName(returnType), typeDisplayName(valueType))
	}
}

func (a *Analyzer) analyzeResultReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if len(returnType.TypeArgs) != 2 {
		return
	}

	switch expr := stmt.Value.(type) {
	case *ast.OkExpression:
		expected := returnType.TypeArgs[0]
		if expr.Value == nil {
			if expected.Kind != VoidType {
				a.addErrorAtToken(expr.Token, "function %s must return Ok(%s), got Ok()", functionName, typeDisplayName(expected))
			}
			return
		}
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind == InvalidType {
			return
		}
		if !canInitialize(expected, valueType, expr.Value) {
			a.addErrorAtToken(expressionToken(expr.Value), "function %s must return Ok(%s), got Ok(%s)", functionName, typeDisplayName(expected), typeDisplayName(valueType))
		}
	case *ast.ErrExpression:
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind == InvalidType {
			return
		}
		expected := returnType.TypeArgs[1]
		if !canInitialize(expected, valueType, expr.Value) {
			a.addErrorAtToken(expressionToken(expr.Value), "function %s must return Err(%s), got Err(%s)", functionName, typeDisplayName(expected), typeDisplayName(valueType))
		}
	default:
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s returning %s must return Ok(...) or Err(...)", functionName, typeDisplayName(returnType))
	}
}

func copySymbols(in map[string]Symbol) map[string]Symbol {
	out := make(map[string]Symbol, len(in))
	for name, symbol := range in {
		out[name] = symbol
	}
	return out
}

func copyConstInts(in map[string]*big.Int) map[string]*big.Int {
	out := make(map[string]*big.Int, len(in))
	for name, value := range in {
		out[name] = new(big.Int).Set(value)
	}
	return out
}

func copyAssigned(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for name, assigned := range in {
		out[name] = assigned
	}
	return out
}

func (a *Analyzer) pushLoopBreakFrame() int {
	a.loopBreakAssignments = append(a.loopBreakAssignments, nil)
	return len(a.loopBreakAssignments) - 1
}

func (a *Analyzer) popLoopBreakFrame(frame int) []map[string]bool {
	if frame < 0 || frame >= len(a.loopBreakAssignments) {
		return nil
	}
	breaks := a.loopBreakAssignments[frame]
	a.loopBreakAssignments = a.loopBreakAssignments[:frame]
	return breaks
}

func (a *Analyzer) recordLoopBreak() {
	if len(a.loopBreakAssignments) == 0 {
		return
	}
	top := len(a.loopBreakAssignments) - 1
	a.loopBreakAssignments[top] = append(a.loopBreakAssignments[top], copyAssigned(a.assigned))
}

func mergeBreakAssigned(before map[string]bool, breaks []map[string]bool) map[string]bool {
	merged := copyAssigned(before)
	if len(breaks) == 0 {
		return merged
	}

	for name := range before {
		assigned := true
		for _, breakAssigned := range breaks {
			if !breakAssigned[name] {
				assigned = false
				break
			}
		}
		merged[name] = assigned
	}
	return merged
}

func (a *Analyzer) analyzeTypeDeclaration(stmt *ast.TypeDeclStatement) {
	if len(stmt.GenericParameters) > 0 {
		a.validateGenericParameterConstraints(stmt.GenericParameters)
		a.withGenericTypeParameters(stmt.GenericParameters, func() {
			a.analyzeTypeDeclarationBody(stmt)
		})
		return
	}
	a.analyzeTypeDeclarationBody(stmt)
}

func (a *Analyzer) analyzeUnitDeclaration(stmt *ast.UnitDeclStatement) {
	if stmt.Name == nil || stmt.BaseType == nil {
		return
	}

	baseType, ok := a.resolveType(stmt.BaseType)
	if !ok {
		return
	}
	if !isNumericType(baseType) {
		a.addErrorAtToken(stmt.BaseType.Token, "unit %s must use numeric storage, got %s", stmt.Name.Value, typeDisplayName(baseType))
		return
	}

	unitName := stmt.Name.Value
	unit := a.units[unitName]
	if unit.Name == "" {
		unit = UnitDefinition{Name: unitName, Category: OtherUnit, Dimension: dimensionFromBase(unitName, 1), Token: stmt.Name.Token}
	}
	if stmt.Category != "" {
		unit.Category = UnitCategory(stmt.Category)
	}
	if stmt.BaseType.Unit != "" {
		unit.Dimension = a.parseDimension(stmt.BaseType.Unit)
	}
	a.units[unitName] = unit

	typ := baseType
	typ.Name = unitName
	typ.Module = a.currentModule
	typ.Named = true
	typ.Declared = true
	typ.Underlying = baseType.Name
	typ.Unit = unitName
	typ.Dimension = unit.Dimension
	a.types[unitName] = typ
}

func (a *Analyzer) analyzeTypeDeclarationBody(stmt *ast.TypeDeclStatement) {
	if stmt.Union {
		a.types[stmt.Name.Value] = a.typeFromUnionDeclaration(stmt.Name.Value, stmt)
		return
	}

	if stmt.StructType != nil {
		a.types[stmt.Name.Value] = a.typeFromStructDeclaration(stmt)
		return
	}
	if stmt.RegisterType != nil {
		a.types[stmt.Name.Value] = a.typeFromRegisterDeclaration(stmt.Name.Value, stmt)
		return
	}
	if len(stmt.Variants) > 0 {
		a.types[stmt.Name.Value] = a.typeFromVariantDeclaration(stmt.Name.Value, stmt.Variants)
		return
	}

	var baseType Type
	var baseTypeOK bool
	if stmt.BaseType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.BaseType)
	}

	if stmt.AssignedType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.AssignedType)
	}

	if contract, ok := stmt.Contract.(*ast.RangeContract); ok && baseTypeOK {
		if contract.Min != nil {
			a.checkIntegerLiteralRange(baseType, contract.Min)
		}
		if contract.Max != nil {
			a.checkIntegerLiteralRange(baseType, contract.Max)
		}
	}

	if baseTypeOK {
		a.types[stmt.Name.Value] = a.typeFromDeclaration(stmt, baseType)
	}
}

func (a *Analyzer) analyzeNestedTypeDeclaration(qualifiedName string, stmt *ast.TypeDeclStatement) {
	if stmt.Union {
		a.types[qualifiedName] = a.typeFromUnionDeclaration(qualifiedName, stmt)
		return
	}
	if stmt.StructType != nil {
		a.types[qualifiedName] = a.typeFromStructDeclarationWithName(qualifiedName, stmt)
		return
	}
	if len(stmt.Variants) > 0 {
		a.types[qualifiedName] = a.typeFromVariantDeclaration(qualifiedName, stmt.Variants)
		return
	}

	var baseType Type
	var baseTypeOK bool
	if stmt.BaseType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.BaseType)
	}

	if stmt.AssignedType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.AssignedType)
	}

	if contract, ok := stmt.Contract.(*ast.RangeContract); ok && baseTypeOK {
		if contract.Min != nil {
			a.checkIntegerLiteralRange(baseType, contract.Min)
		}
		if contract.Max != nil {
			a.checkIntegerLiteralRange(baseType, contract.Max)
		}
	}

	if baseTypeOK {
		a.types[qualifiedName] = a.typeFromDeclarationWithName(qualifiedName, stmt, baseType)
	}
}

func (a *Analyzer) analyzeNestedUnitDeclaration(qualifiedName string, stmt *ast.UnitDeclStatement) {
	if stmt.Name == nil || stmt.BaseType == nil {
		return
	}
	originalName := stmt.Name.Value
	stmt.Name.Value = qualifiedName
	defer func() {
		stmt.Name.Value = originalName
	}()
	a.analyzeUnitDeclaration(stmt)
}

func (a *Analyzer) typeFromStructDeclaration(stmt *ast.TypeDeclStatement) Type {
	return a.typeFromStructDeclarationWithName(stmt.Name.Value, stmt)
}

func (a *Analyzer) typeFromStructDeclarationWithName(name string, stmt *ast.TypeDeclStatement) Type {
	typ := Type{
		Name:              name,
		Module:            a.currentModule,
		Kind:              StructType,
		Named:             true,
		Declared:          true,
		Underlying:        "struct",
		GenericParameters: genericParameterNameValues(stmt.GenericParameters),
	}

	seen := map[string]lexer.Token{}
	for _, field := range stmt.StructType.Fields {
		if previous, exists := seen[field.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct %s", field.Name.Value, name)
			continue
		}
		seen[field.Name.Value] = field.Name.Token

		if len(stmt.GenericParameters) > 0 {
			switch genericRecursiveStorageKind(name, genericParameterNameValues(stmt.GenericParameters), field.Type) {
			case "direct":
				a.addErrorAtToken(field.Token, "recursive generic type %s has infinite size", genericDeclarationDisplayName(name, stmt.GenericParameters))
				continue
			case "nonconverging":
				a.addErrorAtToken(field.Token, "recursive generic instantiation does not converge for %s", name)
				continue
			}
		}

		fieldType, ok := a.resolveType(field.Type)
		if !ok {
			continue
		}
		if contract, ok := field.Contract.(*ast.RangeContract); ok {
			if contract.Min != nil {
				a.checkIntegerLiteralRange(fieldType, contract.Min)
			}
			if contract.Max != nil {
				a.checkIntegerLiteralRange(fieldType, contract.Max)
			}
			fieldType = applyRangeContract(fieldType, field.Contract)
		}

		typ.Fields = append(typ.Fields, StructField{
			Name:  field.Name.Value,
			Type:  fieldType,
			Token: field.Name.Token,
			Tags:  semaStructTags(field.Tags),
		})
	}

	return typ
}

func (a *Analyzer) typeFromRegisterDeclaration(name string, stmt *ast.TypeDeclStatement) Type {
	typ := Type{
		Name:              name,
		Module:            a.currentModule,
		Kind:              RegisterType,
		Named:             true,
		Declared:          true,
		Underlying:        "register",
		RegisterWidth:     stmt.RegisterType.Width,
		GenericParameters: genericParameterNameValues(stmt.GenericParameters),
	}

	if stmt.RegisterType.Width <= 0 {
		a.addErrorAtToken(stmt.RegisterType.Token, "register %s width must be positive", name)
	}

	used := int64(0)
	invalidFieldWidth := false
	seen := map[string]lexer.Token{}
	for _, field := range stmt.RegisterType.Fields {
		if field == nil || field.Name == nil {
			continue
		}
		if field.Width <= 0 {
			a.addErrorAtToken(field.Token, "register field %s.%s width must be positive", name, field.Name.Value)
			invalidFieldWidth = true
			continue
		}
		used += field.Width

		if field.Name.Value != "_" {
			if previous, exists := seen[field.Name.Value]; exists {
				_ = previous
				a.addErrorAtToken(field.Name.Token, "duplicate register field %q in %s", field.Name.Value, name)
				continue
			}
			seen[field.Name.Value] = field.Name.Token
		}

		fieldType := a.registerFieldType(field)
		if field.Unit != "" {
			if _, ok := a.units[field.Unit]; !ok {
				a.addErrorAtToken(field.Token, "unknown unit %s on register field %s.%s", field.Unit, name, field.Name.Value)
			}
		}

		typ.RegisterFields = append(typ.RegisterFields, RegisterField{
			Name:  field.Name.Value,
			Width: field.Width,
			Unit:  field.Unit,
			Type:  fieldType,
			Token: field.Name.Token,
		})
	}

	if stmt.RegisterType.Width > 0 && !invalidFieldWidth && used != stmt.RegisterType.Width {
		a.addErrorAtToken(stmt.RegisterType.Token, "register %s declares %d bits but its fields occupy %d bits", name, stmt.RegisterType.Width, used)
	}

	return typ
}

func (a *Analyzer) registerFieldType(field *ast.RegisterField) Type {
	if field.Width == 1 && field.Unit == "" {
		return Type{Name: "bool", Kind: BoolType}
	}
	max := uint64(0)
	if field.Width >= 64 {
		max = ^uint64(0)
	} else if field.Width > 0 {
		max = 1<<uint(field.Width) - 1
	}
	typ := unsignedType("uint", max)
	if field.Unit != "" {
		if unitType, ok := a.types[field.Unit]; ok && unitType.Kind != InvalidType {
			typ = unitType
			min := uint64(0)
			typ.MinUint = &min
			typ.MaxUint = &max
		} else {
			typ.Named = true
			typ.Unit = field.Unit
			typ.Dimension = a.parseDimension(field.Unit)
		}
	}
	return typ
}

func (a *Analyzer) typeFromUnionDeclaration(name string, stmt *ast.TypeDeclStatement) Type {
	typ := Type{
		Name:              name,
		Module:            a.currentModule,
		Kind:              UnionType,
		Named:             true,
		Declared:          true,
		Underlying:        "union",
		GenericParameters: genericParameterNameValues(stmt.GenericParameters),
	}

	if len(stmt.UnionVariants) == 0 {
		a.addErrorAtToken(stmt.Name.Token, "union %s must declare at least one variant", name)
	}

	seen := map[string]lexer.Token{}
	for _, variant := range stmt.UnionVariants {
		if variant == nil || variant.Name == nil {
			continue
		}
		if previous, exists := seen[variant.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(variant.Name.Token, "duplicate union variant %q in %s", variant.Name.Value, name)
			continue
		}
		seen[variant.Name.Value] = variant.Name.Token

		unionVariant := UnionVariant{
			Name:  variant.Name.Value,
			Token: variant.Name.Token,
		}
		if variant.Payload != nil {
			payload, ok := a.resolveType(variant.Payload)
			if !ok {
				continue
			}
			if sameConcreteType(typ, payload) {
				a.addErrorAtToken(variant.Name.Token, "recursive union %s has infinite size", typeDisplayName(typ))
				continue
			}
			unionVariant.Payload = &payload
		}
		if len(variant.PayloadFields) > 0 {
			fieldSeen := map[string]lexer.Token{}
			for _, field := range variant.PayloadFields {
				if field == nil || field.Name == nil {
					continue
				}
				if _, exists := fieldSeen[field.Name.Value]; exists {
					a.addErrorAtToken(field.Name.Token, "duplicate payload field %s in %s.%s", field.Name.Value, name, variant.Name.Value)
					continue
				}
				fieldSeen[field.Name.Value] = field.Name.Token

				fieldType, ok := a.resolveType(field.Type)
				if !ok {
					continue
				}
				if sameConcreteType(typ, fieldType) {
					a.addErrorAtToken(field.Name.Token, "recursive union %s has infinite size", typeDisplayName(typ))
					continue
				}
				unionVariant.PayloadFields = append(unionVariant.PayloadFields, StructField{
					Name:  field.Name.Value,
					Type:  fieldType,
					Token: field.Name.Token,
					Tags:  semaStructTags(field.Tags),
				})
			}
		}
		typ.UnionVariants = append(typ.UnionVariants, unionVariant)
	}

	return typ
}

func genericRecursiveStorageKind(owner string, parameters []string, ref *ast.TypeReference) string {
	if ref == nil {
		return ""
	}

	if ref.ElementType != nil {
		if ref.ArrayLength > 0 {
			return genericRecursiveStorageKind(owner, parameters, ref.ElementType)
		}
		return ""
	}

	if ref.Name != owner {
		return ""
	}

	if genericTypeArgsMatchParameters(ref.TypeArgs, parameters) {
		return "direct"
	}
	return "nonconverging"
}

func genericTypeArgsMatchParameters(args []*ast.TypeReference, parameters []string) bool {
	if len(args) != len(parameters) {
		return false
	}
	for i, arg := range args {
		if arg == nil || len(arg.TypeArgs) > 0 || arg.ElementType != nil || arg.Name != parameters[i] {
			return false
		}
	}
	return true
}

func genericDeclarationDisplayName(name string, parameters []*ast.GenericParameter) string {
	names := genericParameterNameValues(parameters)
	if len(names) == 0 {
		return name
	}
	return name + "[" + strings.Join(names, ", ") + "]"
}

func (a *Analyzer) typeFromVariantDeclaration(name string, variants []*ast.Identifier) Type {
	enum := &ast.EnumDeclaration{
		Token: lexer.Token{Type: lexer.ENUM, Lexeme: "enum"},
		Name:  &ast.Identifier{Token: variants[0].Token, Value: name},
	}
	for _, variant := range variants {
		enum.Values = append(enum.Values, &ast.EnumValue{
			Token: variant.Token,
			Name:  variant,
		})
	}
	return a.typeFromEnumDeclaration(name, enum)
}

func (a *Analyzer) typeFromEnumDeclaration(name string, enum *ast.EnumDeclaration) Type {
	underlying := a.types["int"]
	if enum.UnderlyingType != nil {
		resolved, ok := a.resolveType(enum.UnderlyingType)
		if !ok {
			return Type{Name: name, Kind: InvalidType}
		}
		underlying = resolved
	}

	if underlying.Kind != IntType && underlying.Kind != UintType {
		token := enum.Name.Token
		if enum.UnderlyingType != nil {
			token = enum.UnderlyingType.Token
		}
		a.addErrorAtToken(token, "enum %s underlying type must be integer, got %s", name, typeDisplayName(underlying))
		return Type{Name: name, Kind: InvalidType}
	}

	typ := Type{
		Name:       name,
		Module:     a.currentModule,
		Kind:       EnumType,
		Named:      true,
		Declared:   true,
		Underlying: underlying.Name,
		EnumConsts: map[string]EnumValue{},
	}

	seen := map[string]lexer.Token{}
	previous := big.NewInt(-1)
	for i, value := range enum.Values {
		if _, exists := seen[value.Name.Value]; exists {
			a.addErrorAtToken(value.Token, "duplicate enum value %q in enum %s", value.Name.Value, name)
			continue
		}
		seen[value.Name.Value] = value.Token

		next := new(big.Int).Add(previous, big.NewInt(1))
		if value.Initializer != nil {
			constValue, ok := a.enumInitializerIntegerValue(value.Initializer, i)
			if !ok {
				a.addErrorAtToken(expressionToken(value.Initializer), "enum value %s.%s initializer must be integer constant", name, value.Name.Value)
				continue
			}
			next = constValue
		}

		a.checkIntegerValueRange(underlying, next, value.Token)
		previous = new(big.Int).Set(next)
		typ.EnumValues = append(typ.EnumValues, value.Name.Value)
		typ.EnumConsts[value.Name.Value] = EnumValue{
			Name:  value.Name.Value,
			Value: new(big.Int).Set(next),
			Token: value.Token,
		}
	}

	return typ
}

func (a *Analyzer) enumInitializerIntegerValue(expr ast.Expression, iotaIndex int) (*big.Int, bool) {
	previous, hadPrevious := a.constInts["iota"]
	a.constInts["iota"] = big.NewInt(int64(iotaIndex))
	defer func() {
		if hadPrevious {
			a.constInts["iota"] = previous
			return
		}
		delete(a.constInts, "iota")
	}()

	return a.integerConstantValue(expr)
}

func semaStructTags(tags []ast.StructTag) []StructTag {
	out := make([]StructTag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, StructTag{Key: tag.Key, Value: tag.Value})
	}
	return out
}

func (a *Analyzer) registerImplDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			continue
		}
		a.registerImplStatement(impl)
	}
}

func (a *Analyzer) registerImplStatement(stmt *ast.ImplStatement) {
	if !a.validImplStatements[stmt] {
		return
	}
	target, ok := a.types[stmt.Target.Name]
	if !ok {
		return
	}

	if !target.Named && target.Kind != InvalidType {
		return
	}
	if !a.validateImplGenericTarget(stmt, target) {
		return
	}
	genericParams := implGenericParametersForTarget(stmt, target)

	properties := map[string]lexer.Token{}
	for _, property := range target.Properties {
		properties[property.Name] = property.Token
	}

	targetChanged := false
	for _, member := range stmt.Members {
		if fn, ok := member.(*ast.FunctionDeclaration); ok {
			if len(fn.GenericParameters) > 0 {
				a.addErrorAtToken(fn.Name.Token, "generic methods with additional type parameters are not supported yet")
				continue
			}
			a.withImplTarget(stmt.Target.Name, func() {
				a.withGenericTypeParameters(genericParams, func() {
					a.registerFunctionDeclarationNamed(fn, stmt.Target.Name+"."+fn.Name.Value)
				})
			})
			continue
		}

		property, ok := member.(*ast.PropertyDeclaration)
		if !ok {
			continue
		}

		if previous, exists := properties[property.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(property.Name.Token, "duplicate property %q in impl %s", property.Name.Value, stmt.Target.Name)
			continue
		}
		properties[property.Name.Value] = property.Name.Token

		var propertyType Type
		var typeOK bool
		a.withImplTarget(stmt.Target.Name, func() {
			a.withGenericTypeParameters(genericParams, func() {
				propertyType, typeOK = a.resolveType(property.Type)
			})
		})
		if !typeOK {
			continue
		}

		var errorType *Type
		if property.Setter != nil && property.Setter.Fallible {
			if inferred, ok := a.inferPropertySetterErrorType(target, property, propertyType); ok {
				errorType = &inferred
			}
		}

		target.Properties = append(target.Properties, Property{
			Name:     property.Name.Value,
			Type:     propertyType,
			Token:    property.Name.Token,
			Fallible: property.Setter != nil && property.Setter.Fallible,
			Error:    errorType,
		})
		targetChanged = true
	}

	if targetChanged {
		a.types[target.Name] = target
	}
}

func (a *Analyzer) analyzeImplBodies(program *ast.Program) {
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			continue
		}
		a.analyzeImplBody(impl)
	}
}

func (a *Analyzer) analyzeImplBody(stmt *ast.ImplStatement) {
	if !a.validImplStatements[stmt] {
		return
	}
	target, ok := a.types[stmt.Target.Name]
	if !ok || !target.Named {
		return
	}
	genericParams := implGenericParametersForTarget(stmt, target)

	for _, member := range stmt.Members {
		property, ok := member.(*ast.PropertyDeclaration)
		if !ok {
			continue
		}

		registeredProperty, ok := lookupPropertyByToken(target, property.Name.Value, property.Name.Token)
		if !ok {
			continue
		}

		a.withImplTarget(stmt.Target.Name, func() {
			a.withGenericTypeParameters(genericParams, func() {
				a.analyzePropertyBody(target, property, registeredProperty.Type)
			})
		})
	}
}

func (a *Analyzer) analyzePropertyBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if property.Getter != nil {
		a.analyzeGetterBody(target, property, propertyType)
	}
	if property.Setter != nil {
		a.analyzeSetterBody(target, property, propertyType)
	}
}

func (a *Analyzer) analyzeGetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	foundReturn := false
	for i, token := range property.Getter.Tokens {
		if token.Type != lexer.RETURN || i+1 >= len(property.Getter.Tokens) {
			continue
		}
		foundReturn = true

		returnToken := property.Getter.Tokens[i+1]
		returnExpr := parseBodyExpression(property.Getter.Tokens[i+1:])
		if returnExpr == nil {
			continue
		}

		returnType, ok := a.inferPropertyBodyExpression(target, nil, propertyType, returnExpr)
		if ok && returnType.Kind != InvalidType && !canInitialize(propertyType, returnType, returnExpr) {
			a.addErrorAtToken(returnToken, "getter %s must return %s, got %s", property.Name.Value, typeDisplayName(propertyType), typeDisplayName(returnType))
		}
		return
	}

	if !foundReturn {
		a.addErrorAtToken(property.Name.Token, "getter %s must return %s", property.Name.Value, typeDisplayName(propertyType))
	}
}

func (a *Analyzer) analyzeSetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if !property.Setter.Fallible && blockReturnsErr(property.Setter.Body) {
		a.addErrorAtToken(property.Setter.Token, "non-fallible setter %s cannot return Err", property.Name.Value)
	}

	for i, token := range property.Setter.Body.Tokens {
		if token.Type != lexer.IDENT || i+2 >= len(property.Setter.Body.Tokens) {
			continue
		}
		if property.Setter.Body.Tokens[i+1].Type != lexer.ASSIGN {
			continue
		}

		targetType, ok := a.resolveBodyValueType(target, property.Setter, propertyType, token)
		if !ok {
			continue
		}

		valueType, ok := a.resolveBodyValueType(target, property.Setter, propertyType, property.Setter.Body.Tokens[i+2])
		if ok && !sameConcreteType(targetType, valueType) {
			a.addErrorAtToken(property.Setter.Body.Tokens[i+2], "cannot assign %s to %s in setter %s", typeDisplayName(valueType), typeDisplayName(targetType), property.Name.Value)
		}
	}
}

func blockReturnsErr(block *ast.BlockStatement) bool {
	for i, token := range block.Tokens {
		if token.Type == lexer.RETURN && i+1 < len(block.Tokens) && block.Tokens[i+1].Type == lexer.IDENT && block.Tokens[i+1].Lexeme == "Err" {
			return true
		}
	}
	return false
}

func (a *Analyzer) inferPropertySetterErrorType(target Type, property *ast.PropertyDeclaration, propertyType Type) (Type, bool) {
	if property.Setter == nil || property.Setter.Body == nil {
		return Type{}, false
	}
	for i, token := range property.Setter.Body.Tokens {
		if token.Type != lexer.IDENT || token.Lexeme != "Err" || i+2 >= len(property.Setter.Body.Tokens) {
			continue
		}
		if property.Setter.Body.Tokens[i+1].Type != lexer.LPAREN {
			continue
		}
		expr := parseBodyExpression(property.Setter.Body.Tokens[i+2:])
		if expr == nil {
			continue
		}
		errExpr := &ast.ErrExpression{Token: token, Value: expr}
		valueType, ok := a.inferPropertyBodyExpression(target, property.Setter, propertyType, errExpr.Value)
		if ok && valueType.Kind != InvalidType {
			return valueType, true
		}
	}
	return Type{}, false
}

func parseBodyExpression(tokens []lexer.Token) ast.Expression {
	exprTokens := collectBodyExpressionTokens(tokens)
	if len(exprTokens) == 0 {
		return nil
	}

	source := "let value := " + tokensSource(exprTokens)
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || len(program.Statements) != 1 {
		return nil
	}

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		return nil
	}

	return stmt.Value
}

func collectBodyExpressionTokens(tokens []lexer.Token) []lexer.Token {
	if len(tokens) == 0 {
		return nil
	}

	line := tokens[0].Line
	depth := 0
	out := []lexer.Token{}
	for _, token := range tokens {
		if depth == 0 && token.Line != line {
			break
		}

		switch token.Type {
		case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
			depth++
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			if depth == 0 {
				return out
			}
			depth--
		case lexer.SEMICOLON, lexer.RETURN:
			if depth == 0 {
				return out
			}
		}

		out = append(out, token)
	}

	return out
}

func tokensSource(tokens []lexer.Token) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token.Lexeme)
	}
	return strings.Join(parts, " ")
}

func (a *Analyzer) inferPropertyBodyExpression(target Type, setter *ast.PropertySetter, setterType Type, expr ast.Expression) (Type, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if expr.Value == "self" {
			return target, true
		}
		if setter != nil && setter.Parameter != nil && expr.Value == setter.Parameter.Value {
			return setterType, true
		}
		if fieldType, ok := lookupStructField(target, expr.Value); ok {
			return fieldType, true
		}
		if property, ok := lookupProperty(target, expr.Value); ok {
			return property.Type, true
		}
		if symbol, ok := a.symbols[expr.Value]; ok {
			return symbol.Type, true
		}
		return Type{Kind: InvalidType}, false
	case *ast.ConversionExpression:
		targetType, ok := a.resolveType(expr.Type)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		valueType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Value)
		if !ok || valueType.Kind == InvalidType {
			return Type{Kind: InvalidType}, ok
		}
		if !canExplicitConvert(targetType, valueType) {
			a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
			return Type{Kind: InvalidType}, false
		}
		return targetType, true
	case *ast.CallExpression:
		typ, _ := a.inferCallExpression(expr)
		return typ, typ.Kind != InvalidType
	case *ast.RuntimeCallExpression:
		typ, _ := a.inferRuntimeCallExpression(expr)
		return typ, typ.Kind != InvalidType
	case *ast.OkExpression, *ast.ErrExpression:
		return Type{Kind: InvalidType}, false
	case *ast.TryExpression:
		typ, _ := a.inferTryExpression(expr)
		return typ, typ.Kind != InvalidType
	case *ast.PrefixExpression:
		rightType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Right)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		switch expr.Operator {
		case "-":
			if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
				return rightType, true
			}
		case "!":
			return Type{Name: "bool", Kind: BoolType}, true
		}
		return Type{Kind: InvalidType}, false
	case *ast.InfixExpression:
		return a.inferPropertyBodyInfixExpression(target, setter, setterType, expr)
	case *ast.MemberExpression:
		if enumType, ok := a.inferEnumValueExpression(expr); ok {
			return enumType, true
		}
		objectType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Object)
		if !ok || objectType.Kind == InvalidType {
			return Type{Kind: InvalidType}, ok
		}
		if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
			return fieldType, true
		}
		if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
			return property.Type, true
		}
		a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
		return Type{Kind: InvalidType}, false
	default:
		typ, _ := a.inferExpression(expr)
		return typ, typ.Kind != InvalidType
	}
}

func (a *Analyzer) inferPropertyBodyInfixExpression(target Type, setter *ast.PropertySetter, setterType Type, expr *ast.InfixExpression) (Type, bool) {
	leftType, leftOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Left)
	rightType, rightOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Right)
	if !leftOK || !rightOK || leftType.Kind == InvalidType || rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if isLogicalOperator(expr.Operator) {
		if leftType.Kind != BoolType || rightType.Kind != BoolType {
			a.addErrorAtToken(expr.Token, "operator %s requires bool operands", expr.Operator)
			return Type{Kind: InvalidType}, false
		}
		return Type{Name: "bool", Kind: BoolType}, true
	}

	if isComparisonOperator(expr.Operator) {
		return Type{Name: "bool", Kind: BoolType}, true
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		typ, _ := a.inferDecimalInfixExpression(expr, leftType, rightType)
		return typ, typ.Kind != InvalidType
	}

	if leftType.Kind == rightType.Kind {
		return leftType, true
	}

	if leftType.Kind == UintType && rightType.Kind == IntType {
		return leftType, true
	}

	if leftType.Kind == IntType && rightType.Kind == UintType {
		return rightType, true
	}

	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) resolveBodyValueType(target Type, setter *ast.PropertySetter, setterType Type, token lexer.Token) (Type, bool) {
	if setter != nil && setter.Parameter != nil && token.Type == lexer.IDENT && token.Lexeme == setter.Parameter.Value {
		return setterType, true
	}

	if token.Type == lexer.SELF && token.Lexeme == "self" {
		return target, true
	}

	if token.Type == lexer.IDENT {
		for _, field := range target.Fields {
			if field.Name == token.Lexeme {
				return field.Type, true
			}
		}
		if property, ok := lookupProperty(target, token.Lexeme); ok {
			return property.Type, true
		}
	}

	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) analyzeLetStatement(stmt *ast.LetStatement) {
	var declaredType Type
	var ok bool

	if stmt.Type != nil {
		declaredType, ok = a.resolveType(stmt.Type)
	} else if stmt.Value != nil {
		declaredType, _ = a.inferExpression(stmt.Value)
		ok = declaredType.Kind != InvalidType
	}

	defined := false
	if ok {
		defined = a.defineSymbol(stmt.Name.Value, declaredType, stmt.Mutable, stmt.Name.Token)
		if defined {
			a.assigned[stmt.Name.Value] = stmt.Value != nil
			if stmt.Address != nil {
				a.analyzeAddressedLetStatement(stmt, declaredType)
			}
		}
	}

	if ok && stmt.Value == nil && !stmt.Mutable && stmt.Address == nil {
		a.addErrorAtToken(stmt.Name.Token, "immutable variable %s requires initializer", stmt.Name.Value)
		return
	}

	if !ok || stmt.Value == nil || stmt.Type == nil {
		if defined && stmt.Value != nil {
			a.setConstInt(stmt.Name.Value, stmt.Value)
		}
		return
	}

	var exprType Type
	if declaredType.Kind == FunctionType {
		if fnType, resolved := a.resolveFunctionValueInitializer(declaredType, stmt.Value); resolved {
			exprType = fnType
		}
	}
	if exprType.Kind == "" && declaredType.Kind == ResultType {
		if resultType, resolved := a.resolveResultValueInitializer(declaredType, stmt.Value); resolved {
			exprType = resultType
		}
	}
	if exprType.Kind == "" {
		exprType, _ = a.inferExpressionWithExpected(stmt.Value, declaredType)
	}
	if exprType.Kind == InvalidType {
		return
	}

	if a.checkIntegerExpressionRange(declaredType, stmt.Value) {
		return
	}

	if a.checkInitializerType(declaredType, exprType, stmt.Value) && defined {
		a.setConstInt(stmt.Name.Value, stmt.Value)
	}
}

func (a *Analyzer) analyzeAddressedLetStatement(stmt *ast.LetStatement, declaredType Type) {
	if stmt.Type == nil {
		a.addErrorAtToken(stmt.AddressToken, "@address requires an explicit register type")
		return
	}
	if declaredType.Kind != RegisterType {
		a.addErrorAtToken(stmt.Type.Token, "@address requires register type, got %s", typeDisplayName(declaredType))
		return
	}
	if stmt.Value != nil {
		a.addErrorAtToken(expressionToken(stmt.Value), "addressed register %s cannot have initializer", stmt.Name.Value)
	}

	if _, ok := stmt.Address.(*ast.IntegerLiteral); !ok {
		a.addErrorAtToken(expressionToken(stmt.Address), "@address requires compile-time integer address")
	}

	symbol := a.symbols[stmt.Name.Value]
	symbol.Addressed = true
	symbol.Volatile = true
	symbol.Address = stmt.Address.String()
	a.symbols[stmt.Name.Value] = symbol
	a.assigned[stmt.Name.Value] = true
}

func (a *Analyzer) resolveResultValueInitializer(resultType Type, expr ast.Expression) (Type, bool) {
	if len(resultType.TypeArgs) != 2 {
		return Type{Kind: InvalidType}, false
	}
	switch expr := expr.(type) {
	case *ast.OkExpression:
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind != InvalidType && !canInitialize(resultType.TypeArgs[0], valueType, expr.Value) {
			a.addErrorAtToken(expressionToken(expr.Value), "cannot initialize %s with Ok(%s)", typeDisplayName(resultType), typeDisplayName(valueType))
		}
		return resultType, true
	case *ast.ErrExpression:
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind != InvalidType && !canInitialize(resultType.TypeArgs[1], valueType, expr.Value) {
			a.addErrorAtToken(expressionToken(expr.Value), "cannot initialize %s with Err(%s)", typeDisplayName(resultType), typeDisplayName(valueType))
		}
		return resultType, true
	default:
		return Type{}, false
	}
}

func (a *Analyzer) analyzeAssignmentStatement(stmt *ast.AssignmentStatement, allowFallible bool) {
	if member, ok := stmt.Target.(*ast.MemberExpression); ok {
		a.analyzeMemberAssignmentStatement(stmt, member, allowFallible)
		return
	}

	target, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		a.addErrorAtToken(expressionToken(stmt.Target), "invalid assignment target")
		return
	}

	symbol, ok := a.symbols[target.Value]
	if !ok {
		a.addErrorAtToken(target.Token, "undefined variable %s", target.Value)
		return
	}

	if !symbol.Mutable {
		a.addErrorAtToken(target.Token, "cannot assign to immutable variable %s", target.Value)
		return
	}

	if hasContracts(symbol.Type) && !allowFallible {
		a.addErrorAtToken(target.Token, "assigning variable %s requires try because %s has contracts", target.Value, typeDisplayName(symbol.Type))
		return
	}

	exprType, _ := a.inferExpressionWithExpected(stmt.Value, symbol.Type)
	if exprType.Kind == InvalidType {
		return
	}

	if stmt.Operator != "=" && !canInitialize(symbol.Type, exprType, stmt.Value) {
		a.addErrorAtToken(
			expressionToken(stmt.Value),
			"cannot %s %s to %s",
			assignmentVerb(stmt.Operator),
			typeDisplayName(exprType),
			typeDisplayName(symbol.Type),
		)
		return
	}

	if a.checkIntegerAssignmentRange(symbol, stmt) {
		return
	}

	if a.checkInitializerType(symbol.Type, exprType, stmt.Value) {
		a.updateAssignedConstInt(symbol.Name, stmt)
		a.assigned[symbol.Name] = true
	}
}

func (a *Analyzer) analyzeMemberAssignmentStatement(stmt *ast.AssignmentStatement, member *ast.MemberExpression, allowFallible bool) {
	targetType, ok := a.inferMemberExpression(member)
	if !ok {
		return
	}

	if symbol, ok := a.symbolForMemberObject(member.Object); ok && symbol.Type.Kind == RegisterType {
		if !symbol.Mutable {
			a.addErrorAtToken(member.Property.Token, "cannot assign to field %s on read-only addressed register %s", member.Property.Value, symbol.Name)
			return
		}
	}

	if property, ok := a.lookupPropertyOnMember(member); ok && property.Fallible && !allowFallible {
		a.addErrorAtToken(member.Property.Token, "assigning fallible property %s requires try", member.Property.Value)
		return
	}

	valueType, _ := a.inferExpressionWithExpected(stmt.Value, targetType)
	if valueType.Kind == InvalidType {
		return
	}

	if a.checkIntegerExpressionRange(targetType, stmt.Value) {
		return
	}

	if !canInitialize(targetType, valueType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
	}
}

func (a *Analyzer) analyzeTryAssignmentHandlers(stmt *ast.TryAssignmentStatement) {
	if stmt.Assignment == nil {
		return
	}
	errorType, ok := a.tryAssignmentErrorType(stmt.Assignment)
	if !ok {
		a.addErrorAtToken(stmt.Token, "try assignment handlers require a known error type")
		return
	}
	resultType := Type{
		Name:     "Result",
		Kind:     ResultType,
		TypeArgs: []Type{{Name: "void", Kind: VoidType}, errorType},
	}
	expr := &ast.TryExpression{
		Token:      stmt.Token,
		Expression: &ast.Identifier{Token: stmt.Token, Value: "__try_assignment"},
		Handlers:   stmt.Handlers,
	}
	a.analyzeTryHandlers(expr, resultType)
}

func (a *Analyzer) tryAssignmentErrorType(stmt *ast.AssignmentStatement) (Type, bool) {
	member, ok := stmt.Target.(*ast.MemberExpression)
	if !ok {
		return Type{}, false
	}
	property, ok := a.lookupPropertyOnMember(member)
	if !ok || property.Error == nil {
		return Type{}, false
	}
	return *property.Error, true
}

func (a *Analyzer) resolveType(ref *ast.TypeReference) (Type, bool) {
	if ref == nil {
		return Type{Kind: InvalidType}, false
	}

	if ref.Ref {
		innerRef := *ref
		innerRef.Ref = false
		innerRef.MutableRef = false
		inner, ok := a.resolveType(&innerRef)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		name := "ref " + typeDisplayName(inner)
		if ref.MutableRef {
			name = "ref mut " + typeDisplayName(inner)
		}
		return Type{
			Name:             name,
			Kind:             ReferenceType,
			Element:          &inner,
			ReferenceMutable: ref.MutableRef,
		}, true
	}

	if ref.Name == "fn" || ref.FunctionReturnType != nil {
		return a.resolveFunctionType(ref)
	}

	if ref.ElementType != nil {
		element, ok := a.resolveType(ref.ElementType)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		if ref.ArrayLength > 0 {
			return Type{
				Name:        fmt.Sprintf("[%d]%s", ref.ArrayLength, typeDisplayName(element)),
				Kind:        ArrayType,
				Element:     &element,
				ArrayLength: ref.ArrayLength,
			}, true
		}
		return Type{
			Name:    "[]" + typeDisplayName(element),
			Kind:    SliceType,
			Element: &element,
		}, true
	}

	if genericType, ok := a.genericTypes[ref.Name]; ok {
		if len(ref.TypeArgs) > 0 {
			a.addErrorAtToken(ref.Token, "generic parameter %s does not take type arguments", ref.Name)
			return Type{Kind: InvalidType}, false
		}
		return genericType, true
	}

	typeArgs := make([]Type, 0, len(ref.TypeArgs))
	for _, arg := range ref.TypeArgs {
		argType, ok := a.resolveType(arg)
		if ok {
			typeArgs = append(typeArgs, argType)
		}
	}

	name := a.resolveTypeName(ref.Name)

	typ, ok := a.types[name]
	if !ok {
		a.addErrorAtToken(ref.Token, "unknown type %s", ref.Name)
		return Type{Kind: InvalidType}, false
	}
	if !a.canAccessDeclaredName(typ.Name, typ.Module) {
		a.addErrorAtToken(ref.Token, "type %s is not accessible from module %s", ref.Name, a.currentModule)
		return Type{Kind: InvalidType}, false
	}

	if typ.Kind == ResultType && len(ref.TypeArgs) != 2 {
		a.addErrorAtToken(ref.Token, "Result requires exactly 2 type arguments, got %d", len(ref.TypeArgs))
		return Type{Kind: InvalidType}, false
	}
	if typ.Kind != ResultType && len(typ.GenericParameters) == 0 && len(ref.TypeArgs) > 0 {
		a.addErrorAtToken(ref.Token, "%s is not generic", ref.Name)
		return Type{Kind: InvalidType}, false
	}
	if typ.Kind != ResultType && len(typ.GenericParameters) > 0 && len(ref.TypeArgs) == 0 {
		a.addErrorAtToken(ref.Token, "%s requires %d generic arguments, got 0", ref.Name, len(typ.GenericParameters))
		return Type{Kind: InvalidType}, false
	}
	if len(typ.GenericParameters) > 0 && len(ref.TypeArgs) != len(typ.GenericParameters) {
		a.addErrorAtToken(ref.Token, "%s requires %d generic arguments, got %d", ref.Name, len(typ.GenericParameters), len(ref.TypeArgs))
		return Type{Kind: InvalidType}, false
	}
	if len(typeArgs) != len(ref.TypeArgs) {
		return Type{Kind: InvalidType}, false
	}

	typ.TypeArgs = typeArgs
	if (typ.Kind == StructType || typ.Kind == UnionType) && len(typ.GenericParameters) > 0 {
		typ = a.instantiateGenericType(typ)
	}
	return typ, true
}

func (a *Analyzer) instantiateGenericType(typ Type) Type {
	key := genericTypeInstanceKey(typ)
	if existing, ok := a.genericTypeInstances[key]; ok {
		return existing
	}

	substitution := map[string]Type{}
	for i, name := range typ.GenericParameters {
		if i < len(typ.TypeArgs) {
			substitution[name] = typ.TypeArgs[i]
		}
	}

	out := typ
	out.Fields = make([]StructField, 0, len(typ.Fields))
	out.Properties = make([]Property, 0, len(typ.Properties))
	out.UnionVariants = make([]UnionVariant, 0, len(typ.UnionVariants))
	out.GenericParameters = nil
	recursive := false
	for _, field := range typ.Fields {
		field.Type = substituteGenericType(field.Type, substitution)
		if genericStructFieldHasDirectRecursiveStorage(out, field.Type) {
			if !recursive {
				a.addErrorAtToken(field.Token, "recursive generic type %s has infinite size", typeDisplayName(out))
			}
			recursive = true
			continue
		}
		out.Fields = append(out.Fields, field)
	}
	for _, property := range typ.Properties {
		property.Type = substituteGenericType(property.Type, substitution)
		if property.Error != nil {
			errorType := substituteGenericType(*property.Error, substitution)
			property.Error = &errorType
		}
		out.Properties = append(out.Properties, property)
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil {
			payload := substituteGenericType(*variant.Payload, substitution)
			variant.Payload = &payload
		}
		if len(variant.PayloadFields) > 0 {
			fields := make([]StructField, 0, len(variant.PayloadFields))
			for _, field := range variant.PayloadFields {
				field.Type = substituteGenericType(field.Type, substitution)
				fields = append(fields, field)
			}
			variant.PayloadFields = fields
		}
		out.UnionVariants = append(out.UnionVariants, variant)
	}
	a.genericTypeInstances[key] = out
	return out
}

func genericStructFieldHasDirectRecursiveStorage(owner Type, field Type) bool {
	if sameConcreteType(owner, field) {
		return true
	}
	switch field.Kind {
	case ArrayType:
		return field.Element != nil && genericStructFieldHasDirectRecursiveStorage(owner, *field.Element)
	default:
		return false
	}
}

func substituteGenericType(typ Type, substitution map[string]Type) Type {
	if typ.Kind == GenericType {
		if concrete, ok := substitution[typ.Name]; ok {
			return concrete
		}
		return typ
	}
	if len(typ.TypeArgs) > 0 {
		out := typ
		out.TypeArgs = make([]Type, 0, len(typ.TypeArgs))
		for _, arg := range typ.TypeArgs {
			out.TypeArgs = append(out.TypeArgs, substituteGenericType(arg, substitution))
		}
		return out
	}
	if typ.Element != nil {
		out := typ
		element := substituteGenericType(*typ.Element, substitution)
		out.Element = &element
		return out
	}
	if typ.Kind == FunctionType {
		out := typ
		out.FunctionParameterTypes = make([]Type, 0, len(typ.FunctionParameterTypes))
		for _, param := range typ.FunctionParameterTypes {
			out.FunctionParameterTypes = append(out.FunctionParameterTypes, substituteGenericType(param, substitution))
		}
		if typ.FunctionReturnType != nil {
			returnType := substituteGenericType(*typ.FunctionReturnType, substitution)
			out.FunctionReturnType = &returnType
		}
		return out
	}
	return typ
}

func (a *Analyzer) resolveFunctionType(ref *ast.TypeReference) (Type, bool) {
	params := make([]Type, 0, len(ref.FunctionParameterTypes))
	ok := true
	for _, paramRef := range ref.FunctionParameterTypes {
		paramType, paramOK := a.resolveType(paramRef)
		if !paramOK {
			ok = false
			continue
		}
		params = append(params, paramType)
	}

	if ref.FunctionReturnType == nil {
		a.addErrorAtToken(ref.Token, "function type return type is required")
		return Type{Kind: InvalidType}, false
	}

	returnType, returnOK := a.resolveType(ref.FunctionReturnType)
	if !returnOK {
		ok = false
	}
	if !ok {
		return Type{Kind: InvalidType}, false
	}

	return Type{
		Name:                   functionTypeName(params, returnType),
		Kind:                   FunctionType,
		FunctionParameterTypes: params,
		FunctionReturnType:     &returnType,
	}, true
}

type expressionValue struct {
	Display  string
	Negative bool
}

func (a *Analyzer) inferExpression(expr ast.Expression) (Type, expressionValue) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		switch expr.Suffix() {
		case "u":
			return Type{Name: "uint", Kind: UintType}, expressionValue{Display: expr.String()}
		case "f":
			return Type{Name: "float", Kind: FloatType}, expressionValue{Display: expr.String()}
		case "d":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "int", Kind: IntType}, expressionValue{Display: expr.String()}
	case *ast.FloatLiteral:
		switch expr.Suffix() {
		case "f":
			return Type{Name: "float", Kind: FloatType}, expressionValue{Display: expr.String()}
		case "d":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
	case *ast.StringLiteral, *ast.InterpolatedStringLiteral:
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	case *ast.BooleanLiteral:
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok {
			if functions := a.accessibleFunctions(a.functions[expr.Value]); len(functions) > 0 {
				if len(functions) == 1 {
					return functionTypeFromFunction(functions[0]), expressionValue{Display: expr.String()}
				}
				a.addErrorAtToken(expr.Token, "ambiguous function value %s; explicit function type required", expr.Value)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			if a.inLambda {
				if _, outer := a.lambdaOuterSymbols[expr.Value]; outer {
					a.addErrorAtToken(expr.Token, "lambda cannot access outer variable %s without explicit capture", expr.Value)
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
			}
			a.addErrorAtToken(expr.Token, "undefined variable %s", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if assigned, ok := a.assigned[expr.Value]; ok && !assigned {
			a.addErrorAtToken(expr.Token, "variable %s is unassigned", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return symbol.Type, expressionValue{Display: expr.String()}
	case *ast.PrefixExpression:
		return a.inferPrefixExpression(expr)
	case *ast.InfixExpression:
		return a.inferInfixExpression(expr)
	case *ast.ConversionExpression:
		return a.inferConversionExpression(expr)
	case *ast.CallExpression:
		return a.inferCallExpression(expr)
	case *ast.LambdaExpression:
		return a.inferLambdaExpression(expr)
	case *ast.RuntimeCallExpression:
		return a.inferRuntimeCallExpression(expr)
	case *ast.OkExpression:
		a.addErrorAtToken(expr.Token, "Ok can only be returned from Result-returning function")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.ErrExpression:
		a.addErrorAtToken(expr.Token, "Err can only be returned from Result-returning function")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.TryExpression:
		return a.inferTryExpression(expr)
	case *ast.SpawnExpression:
		if expr.Body != nil {
			a.analyzeBlockStatements(expr.Body)
		}
		return Type{Name: "Task", Kind: StructType}, expressionValue{Display: expr.String()}
	case *ast.AwaitExpression:
		if expr.Value != nil {
			a.inferExpression(expr.Value)
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}
	case *ast.MatchExpression:
		return a.inferMatchExpression(expr)
	case *ast.MemberExpression:
		typ, ok := a.inferMemberExpression(expr)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return typ, expressionValue{Display: expr.String()}
	case *ast.StructLiteral:
		return a.inferStructLiteral(expr)
	default:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferExpressionWithExpected(expr ast.Expression, expected Type) (Type, expressionValue) {
	call, ok := expr.(*ast.CallExpression)
	if !ok || expected.Kind == InvalidType || expected.Kind == "" {
		return a.inferExpression(expr)
	}
	if typ, value, ok := a.inferCallAsUnionVariantConstructor(call, &expected); ok {
		return typ, value
	}
	if len(call.GenericArguments) > 0 {
		return a.inferExpression(expr)
	}
	if callExpressionName(call) == "" {
		return a.inferExpression(expr)
	}
	if typ, value, ok := a.inferCallExpressionWithExpected(call, expected); ok {
		return typ, value
	}
	return a.inferExpression(expr)
}

func (a *Analyzer) inferStructLiteral(expr *ast.StructLiteral) (Type, expressionValue) {
	if unionType, value, unionOK := a.inferStructLiteralAsUnionVariant(expr); unionOK {
		return unionType, value
	}

	typ, ok := a.resolveType(expr.Type)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if typ.Kind != StructType {
		a.addErrorAtToken(expr.Token, "%s is not a struct type", typ.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	seen := map[string]lexer.Token{}
	for _, field := range expr.Fields {
		if _, exists := seen[field.Name.Value]; exists {
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct literal %s", field.Name.Value, typ.Name)
			continue
		}
		seen[field.Name.Value] = field.Name.Token

		fieldType, ok := lookupStructField(typ, field.Name.Value)
		if !ok {
			a.addErrorAtToken(field.Name.Token, "unknown field %q in struct %s", field.Name.Value, typ.Name)
			continue
		}

		valueType, _ := a.inferExpression(field.Value)
		if valueType.Kind != InvalidType && !canInitialize(fieldType, valueType, field.Value) {
			a.addErrorAtToken(expressionToken(field.Value), "cannot initialize field %s with %s", field.Name.Value, typeDisplayName(valueType))
			continue
		}
		if valueType.Kind != InvalidType {
			a.checkIntegerExpressionRange(fieldType, field.Value)
		}
	}

	return typ, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferStructLiteralAsUnionVariant(expr *ast.StructLiteral) (Type, expressionValue, bool) {
	if expr.Type == nil || !strings.Contains(expr.Type.Name, ".") {
		return Type{}, expressionValue{}, false
	}

	unionName, variantName, ok := splitUnionVariantTypeName(expr.Type.Name)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	unionName = a.resolveTypeName(unionName)
	unionType, ok := a.types[unionName]
	if !ok || unionType.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}

	variant, ok := lookupUnionVariant(unionType, variantName)
	if !ok {
		a.addErrorAtToken(expr.Token, "unknown union variant %s.%s", unionType.Name, variantName)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(variant.PayloadFields) == 0 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s requires unnamed payload construction", typeDisplayName(unionType), variant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	a.checkUnionPayloadFields(unionType, variant, expr.Fields, expr.Token)
	return unionType, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) checkUnionPayloadFields(unionType Type, variant UnionVariant, fields []*ast.StructLiteralField, token lexer.Token) {
	expected := map[string]StructField{}
	for _, field := range variant.PayloadFields {
		expected[field.Name] = field
	}

	seen := map[string]lexer.Token{}
	for _, field := range fields {
		if field == nil || field.Name == nil {
			continue
		}
		name := field.Name.Value
		if _, exists := seen[name]; exists {
			a.addErrorAtToken(field.Name.Token, "duplicate payload field %s for %s.%s", name, typeDisplayName(unionType), variant.Name)
			continue
		}
		seen[name] = field.Name.Token

		expectedField, ok := expected[name]
		if !ok {
			a.addErrorAtToken(field.Name.Token, "unknown payload field %s for %s.%s", name, typeDisplayName(unionType), variant.Name)
			continue
		}

		valueType, _ := a.inferExpressionWithExpected(field.Value, expectedField.Type)
		if valueType.Kind != InvalidType && !canInitialize(expectedField.Type, valueType, field.Value) {
			a.addErrorAtToken(expressionToken(field.Value), "payload field %s for %s.%s must be %s, got %s", name, typeDisplayName(unionType), variant.Name, typeDisplayName(expectedField.Type), typeDisplayName(valueType))
		}
	}

	for _, field := range variant.PayloadFields {
		if _, ok := seen[field.Name]; !ok {
			a.addErrorAtToken(token, "missing payload field %s for %s.%s", field.Name, typeDisplayName(unionType), variant.Name)
		}
	}
}

func (a *Analyzer) inferLambdaExpression(expr *ast.LambdaExpression) (Type, expressionValue) {
	if len(expr.Captures) > 0 {
		a.analyzeUnsupportedCaptures(expr)
	}

	returnType, ok := a.resolveType(expr.ReturnType)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	params := make([]Type, 0, len(expr.Parameters))
	seenParams := map[string]lexer.Token{}
	for _, param := range expr.Parameters {
		if _, exists := seenParams[param.Name.Value]; exists {
			a.addErrorAtToken(param.Name.Token, "duplicate parameter %q", param.Name.Value)
			continue
		}
		seenParams[param.Name.Value] = param.Name.Token

		paramType, paramOK := a.resolveType(param.Type)
		if !paramOK {
			continue
		}
		params = append(params, paramType)
	}

	lambdaType := Type{
		Name:                   functionTypeName(params, returnType),
		Kind:                   FunctionType,
		FunctionParameterTypes: params,
		FunctionReturnType:     &returnType,
	}
	if len(params) != len(expr.Parameters) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousFunctionName := a.currentFunctionName
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousInLambda := a.inLambda
	previousLambdaOuterSymbols := a.lambdaOuterSymbols
	previousLoopDepth := a.loopDepth

	a.symbols = map[string]Symbol{}
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.currentFunctionName = "lambda"
	a.currentFunctionReturn = returnType
	a.inFunctionBody = true
	a.inLambda = true
	a.lambdaOuterSymbols = previousSymbols
	a.loopDepth = 0
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.currentFunctionName = previousFunctionName
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.inLambda = previousInLambda
		a.lambdaOuterSymbols = previousLambdaOuterSymbols
		a.loopDepth = previousLoopDepth
	}()

	for i, param := range expr.Parameters {
		if !a.defineSymbol(param.Name.Value, params[i], false, param.Name.Token) {
			continue
		}
		a.assigned[param.Name.Value] = true
	}

	a.analyzeBlockStatements(expr.Body)

	if !blockDefinitelyReturns(expr.Body) && returnType.Kind != VoidType {
		a.addErrorAtToken(expr.Token, "lambda must return %s", typeDisplayName(returnType))
	}

	return lambdaType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) analyzeUnsupportedCaptures(expr *ast.LambdaExpression) {
	seen := map[string]lexer.Token{}
	for _, capture := range expr.Captures {
		if capture.Name == nil {
			continue
		}
		name := capture.Name.Value
		if _, exists := seen[name]; exists {
			a.addErrorAtToken(capture.Name.Token, "duplicate capture %s", name)
			continue
		}
		seen[name] = capture.Name.Token
		if _, ok := a.symbols[name]; !ok {
			a.addErrorAtToken(capture.Name.Token, "undefined capture %s", name)
			continue
		}
		a.addErrorAtToken(capture.Name.Token, "capturing lambdas are not supported yet")
	}
}

func (a *Analyzer) inferMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	if enumType, ok := a.inferEnumValueExpression(expr); ok {
		return enumType, true
	}
	if unionType, ok := a.inferUnionVariantExpression(expr); ok {
		return unionType, true
	}

	objectType, _ := a.inferExpression(expr.Object)
	if objectType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if objectType.Kind == StringType {
		switch expr.Property.Value {
		case "ptr":
			return a.types["byte"], true
		case "len":
			return a.types["int64"], true
		}
	}

	if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
		return fieldType, true
	}
	if fieldType, ok := lookupRegisterField(objectType, expr.Property.Value); ok {
		return fieldType, true
	}
	if objectType.Kind == RegisterType && expr.Property.Value == "_" {
		a.addErrorAtToken(expr.Property.Token, "reserved register field _ cannot be accessed")
		return Type{Kind: InvalidType}, false
	}

	if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
		return property.Type, true
	}

	a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) inferUnionVariantExpression(expr *ast.MemberExpression) (Type, bool) {
	typeName, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Type{}, false
	}
	typeName = a.resolveTypeName(typeName)

	typ, ok := a.types[typeName]
	if !ok || typ.Kind != UnionType {
		return Type{}, false
	}
	if len(typ.GenericParameters) > 0 && a.currentFunctionReturn.Kind == UnionType && a.currentFunctionReturn.Name == typ.Name {
		typ = a.currentFunctionReturn
	}
	for _, variant := range typ.UnionVariants {
		if variant.Name != expr.Property.Value {
			continue
		}
		if variant.Payload != nil {
			a.addErrorAtToken(expr.Property.Token, "union variant %s.%s requires payload", typeDisplayName(typ), variant.Name)
			return Type{Kind: InvalidType}, true
		}
		return typ, true
	}
	a.addErrorAtToken(expr.Property.Token, "unknown union variant %s.%s", typ.Name, expr.Property.Value)
	return Type{Kind: InvalidType}, true
}

func (a *Analyzer) inferEnumValueExpression(expr *ast.MemberExpression) (Type, bool) {
	typeName, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Type{}, false
	}
	typeName = a.resolveTypeName(typeName)

	typ, ok := a.types[typeName]
	if !ok || typ.Kind != EnumType {
		return Type{}, false
	}
	if _, ok := typ.EnumConsts[expr.Property.Value]; !ok {
		a.addErrorAtToken(expr.Property.Token, "unknown enum value %s.%s", typeName, expr.Property.Value)
		return Type{Kind: InvalidType}, true
	}
	return typ, true
}

func (a *Analyzer) resolveTypeName(name string) string {
	if a.currentImplTarget == "" || strings.Contains(name, ".") {
		return name
	}

	qualified := a.currentImplTarget + "." + name
	if _, ok := a.types[qualified]; ok {
		return qualified
	}

	return name
}

func typePathFromExpression(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		left, ok := typePathFromExpression(expr.Object)
		if !ok {
			return "", false
		}
		return left + "." + expr.Property.Value, true
	default:
		return "", false
	}
}

func (a *Analyzer) lookupPropertyOnMember(expr *ast.MemberExpression) (Property, bool) {
	objectType, _ := a.inferExpression(expr.Object)
	if objectType.Kind == InvalidType {
		return Property{}, false
	}
	return lookupProperty(objectType, expr.Property.Value)
}

func (a *Analyzer) symbolForMemberObject(expr ast.Expression) (Symbol, bool) {
	ident, ok := expr.(*ast.Identifier)
	if !ok {
		return Symbol{}, false
	}
	symbol, ok := a.symbols[ident.Value]
	return symbol, ok
}

func lookupStructField(typ Type, name string) (Type, bool) {
	for _, field := range typ.Fields {
		if field.Name == name {
			return field.Type, true
		}
	}
	return Type{}, false
}

func lookupRegisterField(typ Type, name string) (Type, bool) {
	if typ.Kind != RegisterType || name == "_" {
		return Type{}, false
	}
	for _, field := range typ.RegisterFields {
		if field.Name == name {
			return field.Type, true
		}
	}
	return Type{}, false
}

func lookupProperty(typ Type, name string) (Property, bool) {
	for _, property := range typ.Properties {
		if property.Name == name {
			return property, true
		}
	}
	return Property{}, false
}

func lookupPropertyByToken(typ Type, name string, token lexer.Token) (Property, bool) {
	for _, property := range typ.Properties {
		if property.Name == name && property.Token.Line == token.Line && property.Token.Column == token.Column {
			return property, true
		}
	}
	return Property{}, false
}

func (a *Analyzer) inferConversionExpression(expr *ast.ConversionExpression) (Type, expressionValue) {
	targetType, ok := a.resolveType(expr.Type)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if (targetType.Kind == RawPtrType || valueType.Kind == RawPtrType) && !a.inUnsafe {
		a.addErrorAtToken(expr.Token, "conversion involving RawPtr requires unsafe")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferCallExpression(expr *ast.CallExpression) (Type, expressionValue) {
	name := callExpressionName(expr)
	if name == "" {
		return a.inferFunctionValueCall(expr, Type{Kind: InvalidType})
	}

	functions, ok := a.functions[name]
	if !ok || len(functions) == 0 {
		if methodName, methodOK := a.methodCallName(expr); methodOK {
			if methodFunctions := a.functions[methodName]; len(methodFunctions) > 0 {
				name = methodName
				functions = methodFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		if symbol, exists := a.symbols[name]; exists && symbol.Type.Kind == FunctionType {
			return a.inferFunctionValueCall(expr, symbol.Type)
		}
		if typ, value, ok := a.inferCallAsUnionVariantConstructor(expr, nil); ok {
			return typ, value
		}
		return a.inferCallAsConversion(expr)
	}
	functions = a.accessibleFunctions(functions)
	if len(functions) == 0 {
		a.addErrorAtToken(expr.Token, "function %s is not accessible from module %s", name, a.currentModule)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	argTypes := make([]Type, 0, len(expr.Arguments))
	for _, arg := range expr.Arguments {
		argType, _ := a.inferExpression(arg)
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		argTypes = append(argTypes, argType)
	}

	arityMatches := []Function{}
	for _, function := range functions {
		if len(function.Parameters) == len(expr.Arguments) {
			arityMatches = append(arityMatches, function)
		}
	}

	if len(arityMatches) == 0 {
		a.addErrorAtToken(expr.Token, "function %s expects %s arguments, got %d", name, formatFunctionArities(functions), len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	matches := []overloadMatch{}
	hadGenericArityMatch := false
	hadGenericInference := false
	hadExplicitGenericCall := len(expr.GenericArguments) > 0
	hadGenericFunctionForExplicitCall := false
	hadExplicitGenericArityMatch := false
	for _, function := range arityMatches {
		if hadExplicitGenericCall {
			if len(function.GenericParameters) == 0 {
				continue
			}
			hadGenericFunctionForExplicitCall = true
			instantiated, ok := a.explicitGenericFunctionInstance(function, expr.GenericArguments)
			if !ok {
				if len(function.GenericParameters) == len(expr.GenericArguments) {
					hadExplicitGenericArityMatch = true
				}
				continue
			}
			hadExplicitGenericArityMatch = true
			function = instantiated
		} else if len(function.GenericParameters) > 0 {
			hadGenericArityMatch = true
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			hadGenericInference = true
			function = instantiated
		}
		matchesArguments := true
		rank := 0
		for i, arg := range expr.Arguments {
			if !canInitialize(function.Parameters[i].Type, argTypes[i], arg) {
				matchesArguments = false
				break
			}
			rank += overloadArgumentRank(function.Parameters[i].Type, argTypes[i])
		}
		if matchesArguments {
			matches = append(matches, overloadMatch{Function: function, Rank: rank})
		}
	}

	best := bestOverloadMatches(matches)
	if len(best) == 1 {
		if best[0].Function.Extern && !a.inUnsafe {
			a.addErrorAtToken(expr.Token, "calling extern function %s requires unsafe", name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}
	}

	if len(best) > 1 {
		a.addErrorAtToken(expr.Token, "ambiguous call to %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if hadExplicitGenericCall && !hadGenericFunctionForExplicitCall {
		a.addErrorAtToken(expr.Token, "function %s is not generic", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if hadExplicitGenericCall && !hadExplicitGenericArityMatch {
		for _, function := range arityMatches {
			if len(function.GenericParameters) > 0 {
				a.addErrorAtToken(expr.Token, "%s requires %d explicit generic arguments, got %d", name, len(function.GenericParameters), len(expr.GenericArguments))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
	}

	if hadGenericArityMatch && !hadGenericInference {
		a.addErrorAtToken(expr.Token, "cannot infer generic arguments for %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	for _, function := range arityMatches {
		displayName := name
		if hadExplicitGenericCall {
			if len(function.GenericParameters) == 0 {
				continue
			}
			instantiated, ok := a.explicitGenericFunctionInstance(function, expr.GenericArguments)
			if !ok {
				continue
			}
			function = instantiated
			displayName = genericFunctionDisplayName(name, function)
		} else if len(function.GenericParameters) > 0 {
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			function = instantiated
			displayName = genericFunctionDisplayName(name, function)
		}
		for i, arg := range expr.Arguments {
			param := function.Parameters[i]
			if !canInitialize(param.Type, argTypes[i], arg) {
				a.addErrorAtToken(expressionToken(arg), "argument %d to %s must be %s, got %s", i+1, displayName, typeDisplayName(param.Type), typeDisplayName(argTypes[i]))
			}
		}
		break
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) methodCallName(expr *ast.CallExpression) (string, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return "", false
	}
	if typeName, ok := typePathFromExpression(member.Object); ok {
		if _, exists := a.types[a.resolveTypeName(typeName)]; exists {
			return "", false
		}
	}
	objectType, _ := a.inferExpression(member.Object)
	if objectType.Kind == InvalidType || objectType.Name == "" {
		return "", false
	}
	return objectType.Name + "." + member.Property.Value, true
}

func (a *Analyzer) inferCallAsUnionVariantConstructor(expr *ast.CallExpression, expected *Type) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}

	typeName, ok := typePathFromExpression(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	typeName = a.resolveTypeName(typeName)

	template, ok := a.types[typeName]
	if !ok || template.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}

	variant, ok := lookupUnionVariant(template, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown union variant %s.%s", template.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	unionType := template
	if len(expr.GenericArguments) > 0 {
		explicit, ok := a.resolveType(&ast.TypeReference{
			Token:    expr.Token,
			Name:     typeName,
			TypeArgs: expr.GenericArguments,
		})
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = explicit
	} else if expected != nil && expected.Kind == UnionType && expected.Name == template.Name {
		unionType = *expected
	} else if len(template.GenericParameters) > 0 {
		concrete, ok := a.inferGenericUnionVariantInstance(template, variant, expr)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = concrete
	}

	concreteVariant, ok := lookupUnionVariant(unionType, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown union variant %s.%s", unionType.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if len(concreteVariant.PayloadFields) > 0 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s requires named payload fields", typeDisplayName(unionType), concreteVariant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if concreteVariant.Payload == nil {
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s expects 0 arguments, got %d", typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return unionType, expressionValue{Display: expr.String()}, true
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	payloadType := *concreteVariant.Payload
	valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], payloadType)
	if valueType.Kind != InvalidType && !canInitialize(payloadType, valueType, expr.Arguments[0]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "union variant %s.%s payload must be %s, got %s", typeDisplayName(unionType), concreteVariant.Name, typeDisplayName(payloadType), typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	return unionType, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferGenericUnionVariantInstance(template Type, variant UnionVariant, expr *ast.CallExpression) (Type, bool) {
	if variant.Payload == nil {
		a.addErrorAtToken(expr.Token, "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
		return Type{}, false
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(template), variant.Name, len(expr.Arguments))
		return Type{}, false
	}

	argType, _ := a.inferExpression(expr.Arguments[0])
	if argType.Kind == InvalidType {
		return Type{}, false
	}

	substitution := map[string]Type{}
	if !inferGenericTypeSubstitution(*variant.Payload, argType, substitution) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
		return Type{}, false
	}

	typeArgs := make([]Type, 0, len(template.GenericParameters))
	for _, name := range template.GenericParameters {
		arg, ok := substitution[name]
		if !ok {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "cannot infer generic arguments for %s.%s", template.Name, variant.Name)
			return Type{}, false
		}
		typeArgs = append(typeArgs, arg)
	}

	concrete := template
	concrete.TypeArgs = typeArgs
	return a.instantiateGenericType(concrete), true
}

func lookupUnionVariant(typ Type, name string) (UnionVariant, bool) {
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return variant, true
		}
	}
	return UnionVariant{}, false
}

func splitUnionVariantTypeName(name string) (string, string, bool) {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx == len(name)-1 {
		return "", "", false
	}
	return name[:idx], name[idx+1:], true
}

func (a *Analyzer) inferCallExpressionWithExpected(expr *ast.CallExpression, expected Type) (Type, expressionValue, bool) {
	name := callExpressionName(expr)
	functions, ok := a.functions[name]
	if !ok || len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}
	functions = a.accessibleFunctions(functions)
	if len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}

	argTypes := make([]Type, 0, len(expr.Arguments))
	for _, arg := range expr.Arguments {
		argType, _ := a.inferExpression(arg)
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		argTypes = append(argTypes, argType)
	}

	matches := []overloadMatch{}
	for _, function := range functions {
		if len(function.Parameters) != len(expr.Arguments) || len(function.GenericParameters) == 0 {
			continue
		}
		instantiated, ok := a.inferGenericFunctionInstanceWithExpected(function, argTypes, expected)
		if !ok {
			continue
		}

		matchesArguments := true
		rank := 0
		for i, arg := range expr.Arguments {
			if !canInitialize(instantiated.Parameters[i].Type, argTypes[i], arg) {
				matchesArguments = false
				break
			}
			rank += overloadArgumentRank(instantiated.Parameters[i].Type, argTypes[i])
		}
		if matchesArguments {
			matches = append(matches, overloadMatch{Function: instantiated, Rank: rank})
		}
	}

	best := bestOverloadMatches(matches)
	if len(best) == 1 {
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}, true
	}
	if len(best) > 1 {
		a.addErrorAtToken(expr.Token, "ambiguous call to %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return Type{}, expressionValue{}, false
}

func (a *Analyzer) explicitGenericFunctionInstance(function Function, refs []*ast.TypeReference) (Function, bool) {
	if len(function.GenericParameters) != len(refs) {
		return Function{}, false
	}

	substitution := map[string]Type{}
	for i, ref := range refs {
		typ, ok := a.resolveType(ref)
		if !ok {
			return Function{}, false
		}
		substitution[function.GenericParameters[i]] = typ
	}

	return a.instantiateGenericFunction(function, substitution), true
}

func (a *Analyzer) inferGenericFunctionInstance(function Function, argTypes []Type) (Function, bool) {
	if len(function.Parameters) != len(argTypes) {
		return Function{}, false
	}

	substitution := map[string]Type{}
	for i, param := range function.Parameters {
		if !inferGenericTypeSubstitution(param.Type, argTypes[i], substitution) {
			return Function{}, false
		}
	}
	for _, name := range function.GenericParameters {
		if _, ok := substitution[name]; !ok {
			return Function{}, false
		}
	}

	return a.instantiateGenericFunction(function, substitution), true
}

func (a *Analyzer) inferGenericFunctionInstanceWithExpected(function Function, argTypes []Type, expected Type) (Function, bool) {
	if len(function.Parameters) != len(argTypes) {
		return Function{}, false
	}

	substitution := map[string]Type{}
	for i, param := range function.Parameters {
		if !inferGenericTypeSubstitution(param.Type, argTypes[i], substitution) {
			return Function{}, false
		}
	}

	if expected.Kind != InvalidType && expected.Kind != "" {
		before := len(substitution)
		if !inferGenericTypeSubstitution(function.ReturnType, expected, substitution) {
			if before < len(function.GenericParameters) {
				return Function{}, false
			}
		}
	}

	for _, name := range function.GenericParameters {
		if _, ok := substitution[name]; !ok {
			return Function{}, false
		}
	}

	return a.instantiateGenericFunction(function, substitution), true
}

func (a *Analyzer) instantiateGenericFunction(function Function, substitution map[string]Type) Function {
	key := genericFunctionInstanceKey(function, substitution)
	if existing, ok := a.genericFuncInstances[key]; ok {
		return existing
	}

	out := function
	out.GenericParameters = nil
	out.Parameters = make([]FunctionParameter, 0, len(function.Parameters))
	for _, param := range function.Parameters {
		param.Type = substituteGenericType(param.Type, substitution)
		out.Parameters = append(out.Parameters, param)
	}
	out.ReturnType = substituteGenericType(function.ReturnType, substitution)
	a.genericFuncInstances[key] = out
	return out
}

func genericTypeInstanceKey(typ Type) genericInstanceKey {
	return genericInstanceKey{
		Declaration: typeDeclarationIdentity(typ),
		Arguments:   canonicalTypeArgumentsKey(typ.TypeArgs),
	}
}

func genericFunctionInstanceKey(function Function, substitution map[string]Type) genericInstanceKey {
	args := make([]Type, 0, len(function.GenericParameters))
	for _, name := range function.GenericParameters {
		args = append(args, substitution[name])
	}
	return genericInstanceKey{
		Declaration: functionDeclarationIdentity(function),
		Arguments:   canonicalTypeArgumentsKey(args),
	}
}

func typeDeclarationIdentity(typ Type) string {
	return typ.Module + ":" + string(typ.Kind) + ":" + typ.Name
}

func functionDeclarationIdentity(function Function) string {
	return fmt.Sprintf("%s:fn:%s:%d:%d", function.Module, function.Name, function.Token.Line, function.Token.Column)
}

func canonicalTypeArgumentsKey(args []Type) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, canonicalTypeIdentity(arg))
	}
	return strings.Join(parts, ";")
}

func canonicalTypeIdentity(typ Type) string {
	switch typ.Kind {
	case ArrayType:
		if typ.Element == nil {
			return fmt.Sprintf("array:%d:<nil>", typ.ArrayLength)
		}
		return fmt.Sprintf("array:%d:%s", typ.ArrayLength, canonicalTypeIdentity(*typ.Element))
	case SliceType:
		if typ.Element == nil {
			return "slice:<nil>"
		}
		return "slice:" + canonicalTypeIdentity(*typ.Element)
	case FunctionType:
		params := make([]string, 0, len(typ.FunctionParameterTypes))
		for _, param := range typ.FunctionParameterTypes {
			params = append(params, canonicalTypeIdentity(param))
		}
		returnType := "<nil>"
		if typ.FunctionReturnType != nil {
			returnType = canonicalTypeIdentity(*typ.FunctionReturnType)
		}
		return "fn:(" + strings.Join(params, ",") + ")->" + returnType
	default:
		identity := typeDeclarationIdentity(typ)
		if identity == "::" || typ.Name == "" {
			identity = string(typ.Kind)
		}
		if len(typ.TypeArgs) > 0 {
			identity += "[" + canonicalTypeArgumentsKey(typ.TypeArgs) + "]"
		}
		return identity
	}
}

func inferGenericTypeSubstitution(pattern Type, concrete Type, substitution map[string]Type) bool {
	if pattern.Kind == GenericType {
		if existing, ok := substitution[pattern.Name]; ok {
			return sameConcreteType(existing, concrete)
		}
		substitution[pattern.Name] = concrete
		return true
	}

	if pattern.Kind == FunctionType || concrete.Kind == FunctionType {
		if pattern.Kind != FunctionType || concrete.Kind != FunctionType {
			return false
		}
		if len(pattern.FunctionParameterTypes) != len(concrete.FunctionParameterTypes) {
			return false
		}
		for i := range pattern.FunctionParameterTypes {
			if !inferGenericTypeSubstitution(pattern.FunctionParameterTypes[i], concrete.FunctionParameterTypes[i], substitution) {
				return false
			}
		}
		if pattern.FunctionReturnType == nil || concrete.FunctionReturnType == nil {
			return pattern.FunctionReturnType == nil && concrete.FunctionReturnType == nil
		}
		return inferGenericTypeSubstitution(*pattern.FunctionReturnType, *concrete.FunctionReturnType, substitution)
	}

	if pattern.Element != nil || concrete.Element != nil {
		if pattern.Element == nil || concrete.Element == nil || pattern.Kind != concrete.Kind || pattern.ArrayLength != concrete.ArrayLength {
			return false
		}
		return inferGenericTypeSubstitution(*pattern.Element, *concrete.Element, substitution)
	}

	if len(pattern.TypeArgs) > 0 || len(concrete.TypeArgs) > 0 {
		if pattern.Name != concrete.Name || len(pattern.TypeArgs) != len(concrete.TypeArgs) {
			return false
		}
		for i := range pattern.TypeArgs {
			if !inferGenericTypeSubstitution(pattern.TypeArgs[i], concrete.TypeArgs[i], substitution) {
				return false
			}
		}
		return true
	}

	return canInitialize(pattern, concrete, nil)
}

func genericFunctionDisplayName(name string, function Function) string {
	if len(function.Parameters) == 0 {
		return name
	}
	return name
}

func (a *Analyzer) inferFunctionValueCall(expr *ast.CallExpression, calleeType Type) (Type, expressionValue) {
	if calleeType.Kind == InvalidType {
		calleeType, _ = a.inferExpression(expr.Callee)
	}
	if calleeType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if calleeType.Kind != FunctionType || calleeType.FunctionReturnType == nil {
		a.addErrorAtToken(expr.Token, "cannot call %s", typeDisplayName(calleeType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if len(calleeType.FunctionParameterTypes) != len(expr.Arguments) {
		a.addErrorAtToken(expr.Token, "function value expects %d arguments, got %d", len(calleeType.FunctionParameterTypes), len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	for i, arg := range expr.Arguments {
		argType, _ := a.inferExpression(arg)
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		expected := calleeType.FunctionParameterTypes[i]
		if !canInitialize(expected, argType, arg) {
			a.addErrorAtToken(expressionToken(arg), "argument %d must be %s, got %s", i+1, typeDisplayName(expected), typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}

	return *calleeType.FunctionReturnType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) resolveFunctionValueInitializer(target Type, expr ast.Expression) (Type, bool) {
	ident, ok := expr.(*ast.Identifier)
	if !ok {
		return Type{}, false
	}

	functions := a.accessibleFunctions(a.functions[ident.Value])
	if len(functions) == 0 {
		return Type{}, false
	}

	matches := []Type{}
	for _, function := range functions {
		fnType := functionTypeFromFunction(function)
		if sameFunctionType(target, fnType) {
			matches = append(matches, fnType)
		}
	}

	if len(matches) == 1 {
		return matches[0], true
	}
	if len(matches) > 1 {
		a.addErrorAtToken(ident.Token, "ambiguous function value %s; explicit function type required", ident.Value)
		return Type{Kind: InvalidType}, true
	}

	return functionTypeFromFunction(functions[0]), true
}

func functionTypeFromFunction(function Function) Type {
	params := make([]Type, 0, len(function.Parameters))
	for _, param := range function.Parameters {
		params = append(params, param.Type)
	}
	return Type{
		Name:                   functionTypeName(params, function.ReturnType),
		Kind:                   FunctionType,
		FunctionParameterTypes: params,
		FunctionReturnType:     &function.ReturnType,
	}
}

func (a *Analyzer) accessibleFunctions(functions []Function) []Function {
	out := make([]Function, 0, len(functions))
	for _, function := range functions {
		if a.canAccessDeclaredName(function.Name, function.Module) {
			out = append(out, function)
		}
	}
	return out
}

func (a *Analyzer) canAccessDeclaredName(name string, declarationModule string) bool {
	base := visibilityBaseName(name)
	if !strings.HasPrefix(base, "_") {
		return true
	}
	if declarationModule == "" || a.currentModule == "" {
		return declarationModule == a.currentModule
	}
	if moduleRoot(a.currentModule) == "io" && moduleRoot(declarationModule) == "platform" {
		return true
	}
	if strings.HasPrefix(base, "__") {
		return a.currentModule == declarationModule
	}
	return moduleRoot(a.currentModule) == moduleRoot(declarationModule)
}

func visibilityBaseName(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func moduleRoot(module string) string {
	if idx := strings.Index(module, "."); idx >= 0 {
		return module[:idx]
	}
	return module
}

func callExpressionName(expr *ast.CallExpression) string {
	if expr.Callee != nil {
		if name, ok := typePathFromExpression(expr.Callee); ok {
			return name
		}
	}
	if expr.Function != nil {
		return expr.Function.Value
	}
	return ""
}

func (a *Analyzer) inferRuntimeCallExpression(expr *ast.RuntimeCallExpression) (Type, expressionValue) {
	// TODO: Replace hard-coded runtime hooks with proper runtime library metadata.
	switch expr.Name {
	case "runtime.PrintlnString":
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "@runtime.PrintlnString expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		argType, _ := a.inferExpression(expr.Arguments[0])
		if argType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if argType.Kind != StringType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "@runtime.PrintlnString argument must be string, got %s", typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "unknown runtime function @%s", expr.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

type overloadMatch struct {
	Function Function
	Rank     int
}

func overloadArgumentRank(param Type, arg Type) int {
	if sameConcreteType(param, arg) {
		return 0
	}
	return 1
}

func bestOverloadMatches(matches []overloadMatch) []overloadMatch {
	if len(matches) == 0 {
		return nil
	}

	bestRank := matches[0].Rank
	for _, match := range matches[1:] {
		if match.Rank < bestRank {
			bestRank = match.Rank
		}
	}

	best := []overloadMatch{}
	for _, match := range matches {
		if match.Rank == bestRank {
			best = append(best, match)
		}
	}
	return best
}

func formatFunctionArities(functions []Function) string {
	seen := map[int]bool{}
	out := ""
	for _, function := range functions {
		arity := len(function.Parameters)
		if seen[arity] {
			continue
		}
		seen[arity] = true
		if out != "" {
			out += " or "
		}
		out += fmt.Sprintf("%d", arity)
	}
	return out
}

func (a *Analyzer) inferCallAsConversion(expr *ast.CallExpression) (Type, expressionValue) {
	name := callExpressionName(expr)
	typeName := a.resolveTypeName(name)
	if _, exists := a.types[typeName]; !exists {
		a.addErrorAtToken(expr.Token, "unknown function or type %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	targetRef := &ast.TypeReference{
		Token:    expr.Token,
		Name:     name,
		TypeArgs: expr.GenericArguments,
	}
	targetType, ok := a.resolveType(targetRef)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "conversion to %s expects 1 argument, got %d", name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Arguments[0])
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if (targetType.Kind == RawPtrType || valueType.Kind == RawPtrType) && !a.inUnsafe {
		a.addErrorAtToken(expr.Token, "conversion involving RawPtr requires unsafe")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferTryExpression(expr *ast.TryExpression) (Type, expressionValue) {
	valueType, _ := a.inferExpression(expr.Expression)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if valueType.Kind != ResultType || len(valueType.TypeArgs) != 2 {
		a.addErrorAtToken(expr.Token, "try requires Result expression")
		return valueType, expressionValue{Display: expr.String()}
	}

	if a.inDeferBlock && len(expr.Handlers) == 0 {
		a.addErrorAtToken(expr.Token, "try cannot propagate from inside defer")
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if len(expr.Handlers) > 0 {
		a.analyzeTryHandlers(expr, valueType)
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if !a.inFunctionBody {
		a.addErrorAtToken(expr.Token, "cannot use try outside function")
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addErrorAtToken(expr.Token, "cannot use try in function returning %s", typeDisplayName(a.currentFunctionReturn))
		return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
	}

	valueErrorType := valueType.TypeArgs[1]
	functionErrorType := a.currentFunctionReturn.TypeArgs[1]
	if !sameConcreteType(valueErrorType, functionErrorType) {
		a.addErrorAtToken(expr.Token, "cannot propagate %s from function returning %s", typeDisplayName(valueErrorType), typeDisplayName(a.currentFunctionReturn))
	}

	return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
}

func (a *Analyzer) analyzeTryHandlers(expr *ast.TryExpression, resultType Type) {
	successType := resultType.TypeArgs[0]
	errorType := resultType.TypeArgs[1]
	errorCatchAllSeen := false
	okSeen := false
	matchedVariants := map[string]lexer.Token{}

	for _, handler := range expr.Handlers {
		kind, bindingName, variantName, bindingType, ok := a.analyzeTryHandlerPattern(handler, successType, errorType)
		if !ok {
			continue
		}

		if kind == "Ok" {
			if okSeen {
				a.addErrorAtToken(handler.Token, "unreachable try handler")
				continue
			}
			okSeen = true
			a.analyzeTryHandlerBody(handler, successType, bindingType, bindingName)
			continue
		}

		if errorCatchAllSeen {
			a.addErrorAtToken(handler.Token, "unreachable try handler")
			continue
		}

		if bindingName != "" {
			errorCatchAllSeen = true
		}
		if variantName != "" {
			matchedVariants[variantName] = handler.Token
		}

		a.analyzeTryHandlerBody(handler, successType, bindingType, bindingName)
	}

	if errorCatchAllSeen {
		return
	}

	if errorType.Kind == EnumType {
		if len(matchedVariants) < len(errorType.EnumValues) {
			a.addErrorAtToken(expr.Token, "non-exhaustive try handlers for %s", typeDisplayName(errorType))
		}
		return
	}

	a.addErrorAtToken(expr.Token, "non-exhaustive try handlers for %s", typeDisplayName(errorType))
}

func (a *Analyzer) analyzeTryHandlerPattern(handler *ast.TryHandler, successType Type, errorType Type) (kind string, bindingName string, variantName string, bindingType Type, ok bool) {
	switch pattern := handler.Pattern.(type) {
	case *ast.OkExpression:
		name, patternOK := tryHandlerBindingName(pattern.Value)
		if !patternOK {
			a.addErrorAtToken(expressionToken(pattern.Value), "try handler Ok pattern must be identifier")
			return "", "", "", Type{}, false
		}
		return "Ok", name, "", successType, true
	case *ast.ErrExpression:
		return a.analyzeTryErrHandlerPattern(pattern, errorType)
	default:
		a.addErrorAtToken(expressionToken(handler.Pattern), "try handler pattern must be Ok(...) or Err(...)")
		return "", "", "", Type{}, false
	}
}

func (a *Analyzer) analyzeTryErrHandlerPattern(errPattern *ast.ErrExpression, errorType Type) (kind string, bindingName string, variantName string, bindingType Type, ok bool) {
	switch pattern := errPattern.Value.(type) {
	case *ast.Identifier:
		if pattern.Value == "_" {
			a.addErrorAtToken(pattern.Token, "Err payload must be named; use discard name inside the handler")
			return "", "", "", Type{}, false
		}
		if enumHasValue(errorType, pattern.Value) {
			return "Err", "", pattern.Value, errorType, true
		}
		return "Err", pattern.Value, "", errorType, true
	case *ast.MemberExpression:
		patternType, ok := a.inferMemberExpression(pattern)
		if !ok || patternType.Kind == InvalidType {
			return "", "", "", Type{}, false
		}
		if !sameConcreteType(patternType, errorType) {
			a.addErrorAtToken(expressionToken(pattern), "try handler pattern must match %s, got %s", typeDisplayName(errorType), typeDisplayName(patternType))
			return "", "", "", Type{}, false
		}
		return "Err", "", pattern.Property.Value, errorType, true
	default:
		a.addErrorAtToken(expressionToken(errPattern.Value), "try handler pattern must be enum variant or identifier")
		return "", "", "", Type{}, false
	}
}

func enumHasValue(typ Type, name string) bool {
	if typ.Kind != EnumType || typ.EnumConsts == nil {
		return false
	}
	_, ok := typ.EnumConsts[name]
	return ok
}

func tryHandlerBindingName(expr ast.Expression) (string, bool) {
	ident, ok := expr.(*ast.Identifier)
	if !ok {
		return "", false
	}
	if ident.Value == "_" {
		return "", true
	}
	return ident.Value, true
}

func (a *Analyzer) analyzeTryHandlerBody(handler *ast.TryHandler, successType Type, errorType Type, bindingName string) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	if bindingName != "" {
		a.symbols[bindingName] = Symbol{Name: bindingName, Type: errorType, Mutable: false, Token: handler.Token}
		delete(a.constInts, bindingName)
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
	}()

	if handler.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, handler.ReturnBody)
		return
	}

	if handler.BlockBody != nil {
		a.analyzeBlockStatements(handler.BlockBody)
		if successType.Kind == VoidType {
			return
		}
		if !blockContainsReturn(handler.BlockBody) {
			a.addErrorAtToken(handler.Token, "try handler must return, propagate, terminate or produce %s", typeDisplayName(successType))
		}
		return
	}

	if handler.Body == nil {
		a.addErrorAtToken(handler.Token, "try handler must return, propagate, terminate or produce %s", typeDisplayName(successType))
		return
	}

	bodyType, _ := a.inferExpression(handler.Body)
	if bodyType.Kind == InvalidType {
		return
	}
	if !canInitialize(successType, bodyType, handler.Body) {
		a.addErrorAtToken(expressionToken(handler.Body), "try handler must produce %s, got %s", typeDisplayName(successType), typeDisplayName(bodyType))
	}
}

func blockContainsReturn(block *ast.BlockStatement) bool {
	for _, token := range block.Tokens {
		if token.Type == lexer.RETURN {
			return true
		}
	}
	return false
}

func (a *Analyzer) analyzeMatchStatement(stmt *ast.MatchStatement) {
	if stmt.Match == nil {
		return
	}
	a.analyzeMatch(stmt.Match, false)
}

func (a *Analyzer) inferMatchExpression(expr *ast.MatchExpression) (Type, expressionValue) {
	typ := a.analyzeMatch(expr, true)
	return typ, expressionValue{Display: expr.String()}
}

type matchPatternInfo struct {
	BindingName string
	BindingType Type
	Kind        string
	Variant     string
}

func (a *Analyzer) analyzeMatch(expr *ast.MatchExpression, valueContext bool) Type {
	subjectType, _ := a.inferExpression(expr.Subject)
	if subjectType.Kind == InvalidType {
		return Type{Kind: InvalidType}
	}
	if len(expr.Arms) == 0 {
		a.addErrorAtToken(expr.Token, "match requires at least one branch")
		return Type{Kind: InvalidType}
	}

	seenKinds := map[string]bool{}
	seenVariants := map[string]bool{}
	catchAll := false
	var resultType Type
	hasResultType := false
	beforeAssigned := copyAssigned(a.assigned)
	branches := []branchAnalysis{}
	patternError := false

	for _, arm := range expr.Arms {
		info, ok := a.analyzeMatchPattern(arm.Pattern, subjectType)
		if !ok {
			patternError = true
			continue
		}
		if catchAll {
			a.addErrorAtToken(arm.Token, "unreachable match arm")
			continue
		}
		guarded := arm.Guard != nil
		if guarded && matchPatternAlreadyCovered(info, seenKinds, seenVariants) {
			a.addErrorAtToken(arm.Token, "unreachable match arm")
			continue
		}
		if !guarded {
			if info.Kind == "catchall" {
				catchAll = true
			}
			if info.Kind != "" && info.Variant == "" && info.Kind != "catchall" {
				if seenKinds[info.Kind] {
					a.addErrorAtToken(arm.Token, "duplicate match arm for %s.%s", typeDisplayName(subjectType), info.Kind)
					continue
				}
				seenKinds[info.Kind] = true
			} else if info.Kind != "" {
				seenKinds[info.Kind] = true
			}
			if info.Variant != "" {
				if seenVariants[info.Variant] {
					a.addErrorAtToken(arm.Token, "duplicate match arm for %s.%s", typeDisplayName(subjectType), info.Variant)
					continue
				}
				seenVariants[info.Variant] = true
			}
		}

		armType, branch := a.analyzeMatchArmBody(arm, info)
		branches = append(branches, branch)
		if !valueContext || armType.Kind == InvalidType {
			continue
		}
		if !hasResultType {
			resultType = armType
			hasResultType = true
			continue
		}
		if !canInitialize(resultType, armType, arm.Body) && !canInitialize(armType, resultType, arm.Body) {
			a.addErrorAtToken(expressionToken(arm.Body), "match arms must produce compatible types, got %s and %s", typeDisplayName(resultType), typeDisplayName(armType))
		}
	}

	if catchAll && subjectType.Kind == ResultType && !seenKinds["Err"] {
		a.addErrorAtToken(expr.Token, "catch-all pattern may not hide Err")
	}
	exhaustive := true
	if !patternError {
		exhaustive = a.checkMatchExhaustive(expr, subjectType, catchAll, seenKinds, seenVariants)
	}
	if !valueContext {
		if !exhaustive {
			branches = append(branches, branchAnalysis{assigned: beforeAssigned, continues: true})
		}
		a.assigned = mergeContinuingAssigned(beforeAssigned, branches...)
	}

	if valueContext {
		if !hasResultType {
			a.addErrorAtToken(expr.Token, "match expression must produce a value")
			return Type{Kind: InvalidType}
		}
		return resultType
	}

	return Type{Name: "void", Kind: VoidType}
}

func matchPatternAlreadyCovered(info matchPatternInfo, seenKinds map[string]bool, seenVariants map[string]bool) bool {
	if info.Variant != "" {
		return seenVariants[info.Variant]
	}
	if info.Kind != "" {
		return seenKinds[info.Kind]
	}
	return false
}

func (a *Analyzer) analyzeMatchPattern(pattern ast.Expression, subjectType Type) (matchPatternInfo, bool) {
	switch pattern := pattern.(type) {
	case *ast.OkExpression:
		if subjectType.Kind == UnionType {
			args := []ast.Expression{}
			if pattern.Value != nil {
				args = append(args, pattern.Value)
			}
			return a.analyzeUnionPayloadPattern("Ok", args, pattern.Token, subjectType)
		}
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			a.addErrorAtToken(pattern.Token, "Ok pattern requires Result subject")
			return matchPatternInfo{}, false
		}
		info := matchPatternInfo{Kind: "Ok"}
		if ident, ok := pattern.Value.(*ast.Identifier); ok {
			if ident.Value != "_" {
				info.BindingName = ident.Value
				info.BindingType = subjectType.TypeArgs[0]
			}
		}
		return info, true
	case *ast.ErrExpression:
		if subjectType.Kind == UnionType {
			args := []ast.Expression{}
			if pattern.Value != nil {
				args = append(args, pattern.Value)
			}
			return a.analyzeUnionPayloadPattern("Err", args, pattern.Token, subjectType)
		}
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			a.addErrorAtToken(pattern.Token, "Err pattern requires Result subject")
			return matchPatternInfo{}, false
		}
		info := matchPatternInfo{Kind: "Err"}
		if ident, ok := pattern.Value.(*ast.Identifier); ok {
			if ident.Value == "_" {
				a.addErrorAtToken(ident.Token, "Err payload must be named; use discard name inside the handler")
				return matchPatternInfo{}, false
			}
			info.BindingName = ident.Value
			info.BindingType = subjectType.TypeArgs[1]
		}
		return info, true
	case *ast.MemberExpression:
		patternType, ok := a.inferMemberExpression(pattern)
		if !ok || patternType.Kind == InvalidType {
			return matchPatternInfo{}, false
		}
		if !sameConcreteType(patternType, subjectType) {
			a.addErrorAtToken(expressionToken(pattern), "match pattern must match %s, got %s", typeDisplayName(subjectType), typeDisplayName(patternType))
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "variant", Variant: pattern.Property.Value}, true
	case *ast.CallExpression:
		if subjectType.Kind == UnionType {
			return a.analyzeUnionPayloadMatchPattern(pattern, subjectType)
		}
		patternType, _ := a.inferExpression(pattern)
		if patternType.Kind == InvalidType {
			return matchPatternInfo{}, false
		}
		if !canInitialize(subjectType, patternType, pattern) {
			a.addErrorAtToken(expressionToken(pattern), "match pattern must match %s, got %s", typeDisplayName(subjectType), typeDisplayName(patternType))
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "literal"}, true
	case *ast.Identifier:
		if subjectType.Kind == UnionType {
			if variant, ok := lookupUnionVariant(subjectType, pattern.Value); ok {
				if variant.Payload != nil || len(variant.PayloadFields) > 0 {
					a.addErrorAtToken(pattern.Token, "union variant %s.%s requires payload binding", typeDisplayName(subjectType), variant.Name)
					return matchPatternInfo{}, false
				}
				return matchPatternInfo{Kind: "variant", Variant: variant.Name}, true
			}
		}
		return matchPatternInfo{BindingName: pattern.Value, BindingType: subjectType, Kind: "catchall"}, true
	default:
		patternType, _ := a.inferExpression(pattern)
		if patternType.Kind == InvalidType {
			return matchPatternInfo{}, false
		}
		if !canInitialize(subjectType, patternType, pattern) {
			a.addErrorAtToken(expressionToken(pattern), "match pattern must match %s, got %s", typeDisplayName(subjectType), typeDisplayName(patternType))
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "literal"}, true
	}
}

func (a *Analyzer) analyzeUnionPayloadMatchPattern(pattern *ast.CallExpression, subjectType Type) (matchPatternInfo, bool) {
	variantName := ""
	switch callee := pattern.Callee.(type) {
	case *ast.Identifier:
		variantName = callee.Value
	case *ast.MemberExpression:
		typeName, ok := typePathFromExpression(callee.Object)
		if !ok {
			a.addErrorAtToken(pattern.Token, "match pattern must match %s", typeDisplayName(subjectType))
			return matchPatternInfo{}, false
		}
		typeName = a.resolveTypeName(typeName)
		if typeName != subjectType.Name {
			a.addErrorAtToken(pattern.Token, "match pattern must match %s", typeDisplayName(subjectType))
			return matchPatternInfo{}, false
		}
		variantName = callee.Property.Value
	default:
		a.addErrorAtToken(pattern.Token, "match pattern must match %s", typeDisplayName(subjectType))
		return matchPatternInfo{}, false
	}

	return a.analyzeUnionPayloadPattern(variantName, pattern.Arguments, pattern.Token, subjectType)
}

func (a *Analyzer) analyzeUnionPayloadPattern(variantName string, arguments []ast.Expression, token lexer.Token, subjectType Type) (matchPatternInfo, bool) {
	variant, ok := lookupUnionVariant(subjectType, variantName)
	if !ok {
		a.addErrorAtToken(token, "unknown union variant %s.%s", typeDisplayName(subjectType), variantName)
		return matchPatternInfo{}, false
	}
	if variant.Payload == nil && len(variant.PayloadFields) == 0 {
		a.addErrorAtToken(token, "payload-less union variant %s.%s must not bind a payload", typeDisplayName(subjectType), variant.Name)
		return matchPatternInfo{}, false
	}
	if len(arguments) != 1 {
		a.addErrorAtToken(token, "union variant %s.%s requires 1 payload binding, got %d", typeDisplayName(subjectType), variant.Name, len(arguments))
		return matchPatternInfo{}, false
	}
	binding, ok := arguments[0].(*ast.Identifier)
	if !ok {
		a.addErrorAtToken(expressionToken(arguments[0]), "union payload pattern must bind an identifier")
		return matchPatternInfo{}, false
	}

	info := matchPatternInfo{Kind: "variant", Variant: variant.Name}
	if binding.Value == "_" {
		return info, true
	}
	info.BindingName = binding.Value
	if variant.Payload != nil {
		info.BindingType = *variant.Payload
	} else {
		info.BindingType = Type{
			Name:   typeDisplayName(subjectType) + "." + variant.Name,
			Kind:   StructType,
			Fields: variant.PayloadFields,
		}
	}
	return info, true
}

func (a *Analyzer) analyzeMatchArmBody(arm *ast.MatchArm, info matchPatternInfo) (Type, branchAnalysis) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	if info.BindingName != "" {
		a.symbols[info.BindingName] = Symbol{Name: info.BindingName, Type: info.BindingType, Mutable: false, Token: arm.Token}
		a.assigned[info.BindingName] = true
		delete(a.constInts, info.BindingName)
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
	}()

	if arm.Guard != nil {
		guardType, _ := a.inferExpression(arm.Guard)
		if guardType.Kind != InvalidType && guardType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(arm.Guard), "match guard must be bool, got %s", typeDisplayName(guardType))
		}
	}

	if arm.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, arm.ReturnBody)
		return Type{Kind: InvalidType}, branchAnalysis{assigned: copyAssigned(a.assigned), continues: false}
	}
	if arm.BlockBody != nil {
		a.analyzeBlockStatements(arm.BlockBody)
		return Type{Kind: InvalidType}, branchAnalysis{
			assigned:  copyAssigned(a.assigned),
			continues: !a.blockDefinitelyReturns(arm.BlockBody),
		}
	}
	if arm.Body == nil {
		return Type{Kind: InvalidType}, branchAnalysis{assigned: copyAssigned(a.assigned), continues: true}
	}
	bodyType, _ := a.inferExpression(arm.Body)
	return bodyType, branchAnalysis{assigned: copyAssigned(a.assigned), continues: true}
}

func (a *Analyzer) checkMatchExhaustive(expr *ast.MatchExpression, subjectType Type, catchAll bool, seenKinds map[string]bool, seenVariants map[string]bool) bool {
	if catchAll {
		return true
	}

	if subjectType.Kind == ResultType && len(subjectType.TypeArgs) == 2 {
		ok := true
		if !seenKinds["Ok"] {
			a.addErrorAtToken(expr.Token, "non-exhaustive match for %s: missing Ok", typeDisplayName(subjectType))
			ok = false
		}
		if !seenKinds["Err"] {
			a.addErrorAtToken(expr.Token, "non-exhaustive match for %s: missing Err", typeDisplayName(subjectType))
			ok = false
		}
		return ok
	}

	if enumValues, ok := a.enumValuesForType(subjectType); ok {
		for _, variant := range enumValues {
			if !seenVariants[variant] {
				a.addErrorAtToken(expr.Token, "non-exhaustive match for %s", typeDisplayName(subjectType))
				return false
			}
		}
		return true
	}

	if subjectType.Kind == UnionType {
		missing := []string{}
		for _, variant := range subjectType.UnionVariants {
			if !seenVariants[variant.Name] {
				missing = append(missing, variant.Name)
			}
		}
		if len(missing) > 0 {
			a.addErrorAtToken(expr.Token, "non-exhaustive match for %s: missing %s", typeDisplayName(subjectType), strings.Join(missing, ", "))
			return false
		}
		return true
	}
	return false
}

func (a *Analyzer) enumValuesForType(typ Type) ([]string, bool) {
	if typ.Kind == EnumType && len(typ.EnumValues) > 0 {
		return typ.EnumValues, true
	}
	if typ.Name == "" {
		return nil, false
	}
	registered, ok := a.types[typ.Name]
	if !ok || registered.Kind != EnumType || len(registered.EnumValues) == 0 {
		return nil, false
	}
	return registered.EnumValues, true
}

func (a *Analyzer) inferInfixExpression(expr *ast.InfixExpression) (Type, expressionValue) {
	if isComparisonOperator(expr.Operator) && containsComparisonExpression(expr.Left) {
		a.addErrorAtToken(expr.Token, "comparison chaining is not supported")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	leftType, _ := a.inferExpression(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if expr.Operator == "in" {
		return a.inferMembershipExpression(expr, leftType)
	}

	rightType, _ := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if isLogicalOperator(expr.Operator) {
		if leftType.Kind != BoolType || rightType.Kind != BoolType {
			a.addErrorAtToken(expr.Token, "operator %s requires bool operands", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isEqualityOperator(expr.Operator) {
		if !canCompareEquality(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot compare %s and %s", typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isOrderedComparisonOperator(expr.Operator) {
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		return a.inferDecimalInfixExpression(expr, leftType, rightType)
	}

	if leftType.Kind == rightType.Kind {
		return leftType, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == UintType && rightType.Kind == IntType {
		return leftType, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == IntType && rightType.Kind == UintType {
		return rightType, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferMembershipExpression(expr *ast.InfixExpression, leftType Type) (Type, expressionValue) {
	rangeExpr, ok := expr.Right.(*ast.RangeExpression)
	if !ok {
		a.addErrorAtToken(expr.Token, "membership requires range expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if rangeExpr.Start != nil {
		startType, _ := a.inferExpression(rangeExpr.Start)
		if startType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !canRangeBoundType(leftType, startType, rangeExpr.Start) {
			a.addErrorAtToken(expressionToken(rangeExpr.Start), "cannot test %s in range of %s", typeDisplayName(leftType), typeDisplayName(startType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}

	if rangeExpr.End != nil {
		endType, _ := a.inferExpression(rangeExpr.End)
		if endType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !canRangeBoundType(leftType, endType, rangeExpr.End) {
			a.addErrorAtToken(expressionToken(rangeExpr.End), "cannot test %s in range of %s", typeDisplayName(leftType), typeDisplayName(endType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}

	return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
}

func canRangeBoundType(value Type, bound Type, expr ast.Expression) bool {
	if value.Kind == InvalidType || bound.Kind == InvalidType {
		return true
	}
	if isNominal(value) || isNominal(bound) {
		if isNominal(value) && !isNominal(bound) && isUntypedNumericExpression(expr) && sameUnderlyingNumericKind(value, bound) {
			return true
		}
		return sameConcreteType(value, bound)
	}
	return canInitialize(value, bound, expr) || sameConcreteType(value, bound)
}

func sameUnderlyingNumericKind(value Type, bound Type) bool {
	if value.Kind == bound.Kind {
		return true
	}
	switch value.Underlying {
	case "int":
		return bound.Kind == IntType
	case "uint":
		return bound.Kind == UintType
	case "float", "float32", "float64":
		return bound.Kind == FloatType
	case "decimal":
		return bound.Kind == DecimalType
	default:
		return false
	}
}

func canCompareEquality(left Type, right Type) bool {
	if left.Kind == InvalidType || right.Kind == InvalidType {
		return true
	}

	if isNominal(left) || isNominal(right) {
		return sameConcreteType(left, right)
	}

	if left.Kind == right.Kind {
		return true
	}

	if isIntegerType(left) && isIntegerType(right) {
		return true
	}

	if isNumericType(left) && isNumericType(right) {
		return true
	}

	return false
}

func (a *Analyzer) inferDecimalInfixExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if isComparisonOperator(expr.Operator) {
		if isEqualityOperator(expr.Operator) && !canCompareEquality(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot compare %s and %s", typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == DecimalType && (rightType.Kind == IntType || rightType.Kind == UintType) {
		switch expr.Operator {
		case "*", "/":
			return leftType, expressionValue{Display: expr.String()}
		}
	}

	if (leftType.Kind == IntType || leftType.Kind == UintType) && rightType.Kind == DecimalType {
		switch expr.Operator {
		case "*":
			return rightType, expressionValue{Display: expr.String()}
		}
	}

	if leftType.Kind != DecimalType || rightType.Kind != DecimalType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case "+", "-":
		if sameConcreteType(leftType, rightType) {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) {
			return a.typeForDimension(DecimalType, leftType.Dimension), expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(
			expr.Token,
			"cannot %s %s to %s",
			infixVerb(expr.Operator),
			typeDisplayName(rightType),
			typeDisplayName(leftType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case "*":
		if leftType.Dimension.IsZero() && !rightType.Dimension.IsZero() {
			return rightType, expressionValue{Display: expr.String()}
		}
		if rightType.Dimension.IsZero() && !leftType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.IsZero() && rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) && leftType.Dimension.HasCurrencyBase() {
			a.addErrorAtToken(
				expr.Token,
				"cannot multiply %s by %s",
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
	case "/":
		return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferPrefixExpression(expr *ast.PrefixExpression) (Type, expressionValue) {
	rightType, rightValue := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "-":
		if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
			return rightType, expressionValue{
				Display:  "-" + rightValue.Display,
				Negative: true,
			}
		}
	case "!":
		if rightType.Kind != BoolType {
			a.addErrorAtToken(expr.Token, "operator ! requires bool operand")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) addError(format string, args ...any) {
	a.errors = append(a.errors, Error{Message: fmt.Sprintf(format, args...)})
}

func (a *Analyzer) addErrorAtToken(token lexer.Token, format string, args ...any) {
	a.errors = append(a.errors, Error{
		Message: fmt.Sprintf(format, args...),
		Line:    token.Line,
		Column:  token.Column,
	})
}

func (a *Analyzer) addWarningAtToken(token lexer.Token, format string, args ...any) {
	a.warnings = append(a.warnings, Error{
		Message: fmt.Sprintf(format, args...),
		Line:    token.Line,
		Column:  token.Column,
	})
}

func (a *Analyzer) defineSymbol(name string, typ Type, mutable bool, token lexer.Token) bool {
	if previous, exists := a.symbols[name]; exists {
		a.errors = append(a.errors, Error{
			Message:        fmt.Sprintf("variable %q already declared", name),
			Line:           token.Line,
			Column:         token.Column,
			PreviousLine:   previous.Token.Line,
			PreviousColumn: previous.Token.Column,
		})
		return false
	}

	a.symbols[name] = Symbol{Name: name, Type: typ, Mutable: mutable, Token: token}
	return true
}

func (a *Analyzer) checkInitializerType(target Type, value Type, expr ast.Expression) bool {
	if canInitialize(target, value, expr) {
		return true
	}

	a.addErrorAtToken(
		expressionToken(expr),
		"cannot initialize %s with %s",
		typeDisplayName(target),
		typeDisplayName(value),
	)
	return false
}

func canInitialize(target Type, value Type, expr ast.Expression) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return true
	}

	if target.Kind == FunctionType || value.Kind == FunctionType {
		return sameFunctionType(target, value)
	}

	if target.Kind == ReferenceType {
		if target.Element == nil {
			return false
		}
		if value.Kind == ReferenceType {
			return sameConcreteType(target, value)
		}
		return canInitialize(*target.Element, value, expr)
	}

	if target.Kind == EnumType || value.Kind == EnumType {
		return target.Kind == EnumType && value.Kind == EnumType && target.Name == value.Name
	}

	if target.Kind == StructType || value.Kind == StructType {
		return target.Kind == StructType && value.Kind == StructType && sameConcreteType(target, value)
	}

	if target.Kind == RegisterType || value.Kind == RegisterType {
		return target.Kind == RegisterType && value.Kind == RegisterType && sameConcreteType(target, value)
	}

	if target.Kind == UnionType || value.Kind == UnionType {
		return target.Kind == UnionType && value.Kind == UnionType && sameConcreteType(target, value)
	}

	if len(target.TypeArgs) > 0 || len(value.TypeArgs) > 0 {
		return sameConcreteType(target, value)
	}

	if isNominal(target) && target.Name != value.Name {
		return isUntypedNumericExpression(expr)
	}

	if target.Kind == value.Kind {
		return true
	}

	if target.Kind == UintType && value.Kind == IntType {
		return true
	}

	if target.Kind == DecimalType && isNumericLiteral(expr) {
		_, ok := decimalLiteralValue(expr)
		return ok
	}

	if target.Kind == FloatType && value.Kind == DecimalType && isNumericLiteral(expr) {
		return true
	}

	return false
}

func canExplicitConvert(target Type, value Type) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return false
	}

	if target.Kind == EnumType && isIntegerType(value) {
		return true
	}

	if isIntegerType(target) && isIntegerType(value) {
		return true
	}

	if isIntegerType(target) && value.Kind == EnumType {
		return true
	}

	if target.Kind == BoolType && isNumericType(value) {
		return true
	}

	if target.Kind == RawPtrType && (value.Kind == UintType || value.Kind == RawPtrType || value.Kind == ReferenceType) {
		return true
	}
	if target.Kind == UintType && value.Kind == RawPtrType {
		return true
	}

	if target.Kind == StringType && isNumericType(value) {
		return true
	}

	if target.Kind == UnionType || value.Kind == UnionType {
		return target.Kind == UnionType && value.Kind == UnionType && sameConcreteType(target, value)
	}

	if target.Kind == value.Kind {
		return true
	}

	if target.Kind == DecimalType && (value.Kind == IntType || value.Kind == FloatType) {
		return true
	}

	return false
}

func hasContracts(typ Type) bool {
	return len(typ.Contracts) > 0
}

func isNominal(typ Type) bool {
	return typ.Named && (typ.Kind == EnumType || typ.Kind == UnionType || hasContracts(typ) || !typ.Dimension.IsZero())
}

func isIntegerType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType
}

func isNumericType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType || typ.Kind == FloatType || typ.Kind == DecimalType
}

func sameConcreteType(left Type, right Type) bool {
	if left.Kind == FunctionType || right.Kind == FunctionType {
		return sameFunctionType(left, right)
	}
	if left.Name != "" || right.Name != "" {
		return left.Name == right.Name && sameTypeArguments(left.TypeArgs, right.TypeArgs)
	}
	return left.Kind == right.Kind
}

func sameTypeArguments(left []Type, right []Type) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !sameConcreteType(left[i], right[i]) {
			return false
		}
	}
	return true
}

func sameFunctionType(left Type, right Type) bool {
	if left.Kind != FunctionType || right.Kind != FunctionType {
		return false
	}
	if left.FunctionReturnType == nil || right.FunctionReturnType == nil {
		return false
	}
	if len(left.FunctionParameterTypes) != len(right.FunctionParameterTypes) {
		return false
	}
	for i := range left.FunctionParameterTypes {
		if !sameConcreteType(left.FunctionParameterTypes[i], right.FunctionParameterTypes[i]) {
			return false
		}
	}
	return sameConcreteType(*left.FunctionReturnType, *right.FunctionReturnType)
}

func assignmentVerb(operator string) string {
	switch operator {
	case "+=":
		return "add"
	case "-=":
		return "subtract"
	case "*=":
		return "multiply"
	case "/=":
		return "divide"
	default:
		return "assign"
	}
}

func infixVerb(operator string) string {
	switch operator {
	case "+":
		return "add"
	case "-":
		return "subtract"
	default:
		return "combine"
	}
}

func isComparisonOperator(operator string) bool {
	switch operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func containsComparisonExpression(expr ast.Expression) bool {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return false
	}
	return isComparisonOperator(infix.Operator)
}

func isEqualityOperator(operator string) bool {
	return operator == "==" || operator == "!="
}

func isOrderedComparisonOperator(operator string) bool {
	switch operator {
	case "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func isLogicalOperator(operator string) bool {
	return operator == "&&" || operator == "||"
}

func isUntypedNumericExpression(expr ast.Expression) bool {
	if isNumericLiteral(expr) {
		return true
	}

	_, ok := constantIntegerValue(expr)
	return ok
}

func (a *Analyzer) isExplicitConversionExpression(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.ConversionExpression:
		return true
	case *ast.CallExpression:
		if len(expr.Arguments) != 1 {
			return false
		}
		_, ok := a.types[a.resolveTypeName(callExpressionName(expr))]
		return ok
	default:
		return false
	}
}

func typeDisplayName(typ Type) string {
	if typ.Kind == ReferenceType && typ.Element != nil {
		if typ.ReferenceMutable {
			return "ref mut " + typeDisplayName(*typ.Element)
		}
		return "ref " + typeDisplayName(*typ.Element)
	}
	if typ.Kind == ArrayType && typ.Element != nil {
		return fmt.Sprintf("[%d]%s", typ.ArrayLength, typeDisplayName(*typ.Element))
	}
	if typ.Kind == SliceType && typ.Element != nil {
		return "[]" + typeDisplayName(*typ.Element)
	}
	if typ.Kind == FunctionType {
		return functionTypeName(typ.FunctionParameterTypes, functionReturnType(typ))
	}
	if typ.Name != "" && len(typ.TypeArgs) > 0 {
		out := typ.Name + "["
		for i, arg := range typ.TypeArgs {
			if i > 0 {
				out += ", "
			}
			out += typeDisplayName(arg)
		}
		out += "]"
		return out
	}
	if typ.Name != "" {
		return typ.Name
	}
	return string(typ.Kind)
}

func functionReturnType(typ Type) Type {
	if typ.FunctionReturnType == nil {
		return Type{Kind: InvalidType}
	}
	return *typ.FunctionReturnType
}

func functionTypeName(params []Type, returnType Type) string {
	out := "fn("
	for i, param := range params {
		if i > 0 {
			out += ", "
		}
		out += typeDisplayName(param)
	}
	out += ") " + typeDisplayName(returnType)
	return out
}

func statementToken(stmt ast.Statement) lexer.Token {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		return stmt.Token
	case *ast.ImportStatement:
		return stmt.Token
	case *ast.TypeDeclStatement:
		return stmt.Token
	case *ast.EnumDeclaration:
		return stmt.Token
	case *ast.ImplStatement:
		return stmt.Token
	case *ast.FunctionDeclaration:
		return stmt.Token
	case *ast.StructStatement:
		return stmt.Token
	case *ast.LetStatement:
		return stmt.Token
	case *ast.LetGroupStatement:
		return stmt.Token
	case *ast.AssignmentStatement:
		return stmt.Token
	case *ast.TryAssignmentStatement:
		return stmt.Token
	case *ast.DeferStatement:
		return stmt.Token
	case *ast.DiscardStatement:
		return stmt.Token
	case *ast.ExpressionStatement:
		return stmt.Token
	case *ast.ReturnStatement:
		return stmt.Token
	case *ast.IfStatement:
		return stmt.Token
	case *ast.ForStatement:
		return stmt.Token
	case *ast.WhileStatement:
		return stmt.Token
	case *ast.SwitchStatement:
		return stmt.Token
	case *ast.FallthroughStatement:
		return stmt.Token
	case *ast.BreakStatement:
		return stmt.Token
	case *ast.ContinueStatement:
		return stmt.Token
	case *ast.UnsafeStatement:
		return stmt.Token
	case *ast.AsmStatement:
		return stmt.Token
	case *ast.MatchStatement:
		return stmt.Token
	case *ast.CommentStatement:
		return stmt.Token
	case *ast.InvalidStatement:
		return stmt.Token
	default:
		return lexer.Token{}
	}
}

func expressionToken(expr ast.Expression) lexer.Token {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Token
	case *ast.IntegerLiteral:
		return expr.Token
	case *ast.FloatLiteral:
		return expr.Token
	case *ast.StringLiteral:
		return expr.Token
	case *ast.BooleanLiteral:
		return expr.Token
	case *ast.InterpolatedStringLiteral:
		return expr.Token
	case *ast.PrefixExpression:
		return expr.Token
	case *ast.InfixExpression:
		return expr.Token
	case *ast.ConversionExpression:
		return expr.Token
	case *ast.CallExpression:
		return expr.Token
	case *ast.RuntimeCallExpression:
		return expr.Token
	case *ast.TryExpression:
		return expr.Token
	case *ast.MatchExpression:
		return expr.Token
	case *ast.RangeExpression:
		return expr.Token
	case *ast.MemberExpression:
		return expr.Token
	case *ast.StructLiteral:
		return expr.Token
	case *ast.LambdaExpression:
		return expr.Token
	default:
		return lexer.Token{}
	}
}
