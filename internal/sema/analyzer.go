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
	functions             map[string][]Function
	implBlocks            map[string]lexer.Token
	currentImplTarget     string
	symbols               map[string]Symbol
	constInts             map[string]*big.Int
	assigned              map[string]bool
	currentFunctionName   string
	currentFunctionReturn Type
	inFunctionBody        bool
	errors                []Error
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		types: builtinTypes(),
	}
}

func (a *Analyzer) Analyze(program *ast.Program) []Error {
	a.errors = nil
	a.symbols = map[string]Symbol{}
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.functions = map[string][]Function{}
	a.implBlocks = map[string]lexer.Token{}
	a.currentImplTarget = ""
	a.currentFunctionName = ""
	a.currentFunctionReturn = Type{}
	a.inFunctionBody = false
	a.validateModuleDeclaration(program)
	a.registerTypeDeclarations(program)
	a.registerImplTypeDeclarations(program)
	a.analyzeTypeDeclarations(program)
	a.analyzeEnumDeclarations(program)
	a.analyzeImplTypeDeclarations(program)
	a.registerImplDeclarations(program)
	a.registerFunctionDeclarations(program)

	for _, stmt := range program.Statements {
		switch stmt.(type) {
		case *ast.TypeDeclStatement, *ast.EnumDeclaration, *ast.ImplStatement, *ast.FunctionDeclaration:
			continue
		}
		if !isAllowedModuleStatement(stmt) {
			a.addTopLevelStatementError(stmt)
			continue
		}
		a.analyzeStatement(stmt)
	}

	a.analyzeFunctionBodies(program)
	a.analyzeImplBodies(program)

	return a.errors
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
	case *ast.ModuleStatement,
		*ast.ImportStatement,
		*ast.TypeDeclStatement,
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
	default:
		a.addErrorAtToken(statementToken(stmt), "code is not allowed at module scope")
	}
}

func (a *Analyzer) registerTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.TypeDeclStatement:
			if stmt.Name == nil {
				continue
			}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Kind: InvalidType}
		case *ast.EnumDeclaration:
			if stmt.Name == nil {
				continue
			}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Kind: InvalidType}
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
		if previous, exists := a.implBlocks[impl.Target.Name]; exists {
			_ = previous
			a.addErrorAtToken(impl.Target.Token, "duplicate impl block for %s", impl.Target.Name)
			continue
		}
		a.implBlocks[impl.Target.Name] = impl.Target.Token

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
	case *ast.EnumDeclaration:
		if member.Name == nil {
			return "", lexer.Token{}, false
		}
		return member.Name.Value, member.Name.Token, true
	default:
		return "", lexer.Token{}, false
	}
}

func (a *Analyzer) analyzeTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		typeDecl, ok := stmt.(*ast.TypeDeclStatement)
		if !ok {
			continue
		}

		a.analyzeTypeDeclaration(typeDecl)
	}
}

func (a *Analyzer) analyzeEnumDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		enum, ok := stmt.(*ast.EnumDeclaration)
		if !ok {
			continue
		}
		a.types[enum.Name.Value] = a.typeFromEnumDeclaration(enum.Name.Value, enum)
	}
}

func (a *Analyzer) analyzeImplTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			continue
		}
		if _, ok := a.implBlocks[impl.Target.Name]; !ok {
			continue
		}

		for _, member := range impl.Members {
			switch member := member.(type) {
			case *ast.TypeDeclStatement:
				qualified := impl.Target.Name + "." + member.Name.Value
				a.withImplTarget(impl.Target.Name, func() {
					a.analyzeNestedTypeDeclaration(qualified, member)
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
		a.analyzeAssignmentStatement(stmt)
	case *ast.ExpressionStatement:
		a.inferExpression(stmt.Expression)
	case *ast.ReturnStatement:
		if a.inFunctionBody {
			a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, stmt)
		}
	case *ast.IfStatement:
		a.analyzeIfStatement(stmt)
	case *ast.SwitchStatement:
		a.analyzeSwitchStatement(stmt)
	case *ast.MatchStatement:
		a.analyzeMatchStatement(stmt)
	case *ast.FallthroughStatement:
		a.addErrorAtToken(stmt.Token, "fallthrough is only valid directly inside a switch case")
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

	for _, stmt := range block.Statements {
		a.analyzeStatement(stmt)
	}
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
	var subjectType Type
	hasSubject := stmt.Subject != nil
	if hasSubject {
		subjectType, _ = a.inferExpression(stmt.Subject)
		if subjectType.Kind == VoidType {
			a.addErrorAtToken(expressionToken(stmt.Subject), "switch subject cannot be void")
		}
	}

	seenInts := map[string]lexer.Token{}
	clauses := append([]*ast.SwitchCase{}, stmt.Cases...)
	if stmt.Default != nil {
		clauses = append(clauses, stmt.Default)
	}

	branches := make([]branchAnalysis, 0, len(clauses)+1)
	for i, clause := range clauses {
		if clause == nil {
			continue
		}
		if clause.Default && i != len(clauses)-1 {
			a.addErrorAtToken(clause.Token, "default must be the final switch clause")
		}
		a.analyzeSwitchCaseItems(clause, hasSubject, subjectType, seenInts)
		a.analyzeSwitchFallthrough(clause, i == len(clauses)-1)
		branches = append(branches, a.analyzeSwitchCaseBody(clause.Body))
	}

	if stmt.Default == nil {
		branches = append(branches, branchAnalysis{assigned: before, continues: true})
	}
	a.assigned = mergeContinuingAssigned(before, branches...)
}

func (a *Analyzer) analyzeSwitchCaseItems(clause *ast.SwitchCase, hasSubject bool, subjectType Type, seenInts map[string]lexer.Token) {
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
				a.checkDuplicateSwitchValue(item.Value, seenInts)
			} else if valueType.Kind != BoolType {
				a.addErrorAtToken(expressionToken(item.Value), "subjectless switch case must be bool, got %s", typeDisplayName(valueType))
			}
		case *ast.SwitchRangeCase:
			if !hasSubject {
				a.addErrorAtToken(item.Token, "subjectless switch case must be bool, got range")
				continue
			}
			a.analyzeSwitchRangeCase(item, subjectType)
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
		}
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

func (a *Analyzer) checkDuplicateSwitchValue(expr ast.Expression, seen map[string]lexer.Token) {
	value, ok := constantIntegerValue(expr)
	if !ok {
		return
	}
	key := value.String()
	if _, exists := seen[key]; exists {
		a.addErrorAtToken(expressionToken(expr), "duplicate switch case value %s", key)
		return
	}
	seen[key] = expressionToken(expr)
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
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
	}()

	hasFallthrough := false
	for _, stmt := range block.Statements {
		if _, ok := stmt.(*ast.FallthroughStatement); ok {
			hasFallthrough = true
			continue
		}
		a.analyzeStatement(stmt)
	}
	return branchAnalysis{
		assigned:  copyAssigned(a.assigned),
		continues: !blockDefinitelyReturns(block) && !hasFallthrough,
	}
}

func isOrderedSwitchType(typ Type) bool {
	return isNumericType(typ) || typ.Kind == StringType || typ.Kind == CharType || typ.Kind == RuneType
}

func (a *Analyzer) registerFunctionDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		a.registerFunctionDeclaration(fn)
	}
}

func (a *Analyzer) registerFunctionDeclaration(fn *ast.FunctionDeclaration) {
	function := Function{Name: fn.Name.Value, Token: fn.Name.Token}

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
		})
	}

	returnType, ok := a.resolveType(fn.ReturnType)
	if ok {
		function.ReturnType = returnType
	} else {
		function.ReturnType = Type{Kind: InvalidType}
	}

	for _, existing := range a.functions[fn.Name.Value] {
		if sameFunctionSignature(existing, function) {
			a.addErrorAtToken(fn.Name.Token, "duplicate function %q with same signature", fn.Name.Value)
			return
		}
	}

	a.functions[fn.Name.Value] = append(a.functions[fn.Name.Value], function)
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

func (a *Analyzer) lookupFunctionByToken(name string, token lexer.Token) (Function, bool) {
	for _, function := range a.functions[name] {
		if function.Token.Line == token.Line && function.Token.Column == token.Column {
			return function, true
		}
	}
	return Function{}, false
}

func (a *Analyzer) analyzeFunctionBodies(program *ast.Program) {
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		a.analyzeFunctionBody(fn)
	}
}

func (a *Analyzer) analyzeFunctionBody(fn *ast.FunctionDeclaration) {
	function, ok := a.lookupFunctionByToken(fn.Name.Value, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousFunctionName := a.currentFunctionName
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.currentFunctionName = fn.Name.Value
	a.currentFunctionReturn = function.ReturnType
	a.inFunctionBody = true
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.currentFunctionName = previousFunctionName
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
	}()

	for _, param := range function.Parameters {
		a.symbols[param.Name] = Symbol{Name: param.Name, Type: param.Type, Mutable: false, Token: param.Token}
		delete(a.constInts, param.Name)
		a.assigned[param.Name] = true
	}

	for _, stmt := range fn.Body.Statements {
		a.analyzeStatement(stmt)
	}

	if !blockDefinitelyReturns(fn.Body) && function.ReturnType.Kind != VoidType {
		a.addErrorAtToken(fn.Name.Token, "function %s must return %s", fn.Name.Value, typeDisplayName(function.ReturnType))
	}
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
	case *ast.MatchStatement:
		return false
	default:
		return false
	}
}

func switchDefinitelyReturns(stmt *ast.SwitchStatement) bool {
	if stmt.Default == nil {
		return false
	}
	for _, clause := range stmt.Cases {
		if clause == nil || !blockDefinitelyReturns(clause.Body) {
			return false
		}
	}
	return blockDefinitelyReturns(stmt.Default.Body)
}

func (a *Analyzer) analyzeReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if stmt.Value == nil {
		if returnType.Kind != VoidType {
			a.addErrorAtToken(stmt.Token, "function %s must return %s", functionName, typeDisplayName(returnType))
		}
		return
	}

	if returnType.Kind == ResultType {
		a.analyzeResultReturnStatement(functionName, returnType, stmt)
		return
	}

	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}

	if returnType.Kind == VoidType {
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s must return void, got %s", functionName, typeDisplayName(valueType))
		return
	}

	if !canInitialize(returnType, valueType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s must return %s, got %s", functionName, typeDisplayName(returnType), typeDisplayName(valueType))
	}
}

func (a *Analyzer) analyzeResultReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if len(returnType.TypeArgs) != 2 {
		return
	}

	switch expr := stmt.Value.(type) {
	case *ast.OkExpression:
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind == InvalidType {
			return
		}
		expected := returnType.TypeArgs[0]
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

func (a *Analyzer) analyzeTypeDeclaration(stmt *ast.TypeDeclStatement) {
	if stmt.StructType != nil {
		a.types[stmt.Name.Value] = a.typeFromStructDeclaration(stmt)
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
	if stmt.StructType != nil {
		a.types[qualifiedName] = a.typeFromStructDeclarationWithName(qualifiedName, stmt)
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

func (a *Analyzer) typeFromStructDeclaration(stmt *ast.TypeDeclStatement) Type {
	return a.typeFromStructDeclarationWithName(stmt.Name.Value, stmt)
}

func (a *Analyzer) typeFromStructDeclarationWithName(name string, stmt *ast.TypeDeclStatement) Type {
	typ := Type{
		Name:       name,
		Kind:       StructType,
		Named:      true,
		Declared:   true,
		Underlying: "struct",
	}

	seen := map[string]lexer.Token{}
	for _, field := range stmt.StructType.Fields {
		if previous, exists := seen[field.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct %s", field.Name.Value, name)
			continue
		}
		seen[field.Name.Value] = field.Name.Token

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
		Kind:       EnumType,
		Named:      true,
		Declared:   true,
		Underlying: underlying.Name,
		EnumConsts: map[string]EnumValue{},
	}

	seen := map[string]lexer.Token{}
	previous := big.NewInt(-1)
	for _, value := range enum.Values {
		if _, exists := seen[value.Name.Value]; exists {
			a.addErrorAtToken(value.Token, "duplicate enum value %q in enum %s", value.Name.Value, name)
			continue
		}
		seen[value.Name.Value] = value.Token

		next := new(big.Int).Add(previous, big.NewInt(1))
		if value.Initializer != nil {
			constValue, ok := constantIntegerValue(value.Initializer)
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
	target, ok := a.types[stmt.Target.Name]
	if !ok {
		return
	}

	if !target.Named && target.Kind != InvalidType {
		return
	}

	properties := map[string]lexer.Token{}
	for _, property := range target.Properties {
		properties[property.Name] = property.Token
	}

	targetChanged := false
	for _, member := range stmt.Members {
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
			propertyType, typeOK = a.resolveType(property.Type)
		})
		if !typeOK {
			continue
		}

		target.Properties = append(target.Properties, Property{
			Name:     property.Name.Value,
			Type:     propertyType,
			Token:    property.Name.Token,
			Fallible: property.Setter != nil && property.Setter.Fallible,
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
	target, ok := a.types[stmt.Target.Name]
	if !ok || !target.Named {
		return
	}

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
			a.analyzePropertyBody(target, property, registeredProperty.Type)
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
		}
	}

	if !ok || stmt.Value == nil || stmt.Type == nil {
		if defined && stmt.Value != nil {
			a.setConstInt(stmt.Name.Value, stmt.Value)
		}
		return
	}

	exprType, _ := a.inferExpression(stmt.Value)
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

func (a *Analyzer) analyzeAssignmentStatement(stmt *ast.AssignmentStatement) {
	if member, ok := stmt.Target.(*ast.MemberExpression); ok {
		a.analyzeMemberAssignmentStatement(stmt, member)
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

	exprType, _ := a.inferExpression(stmt.Value)
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

func (a *Analyzer) analyzeMemberAssignmentStatement(stmt *ast.AssignmentStatement, member *ast.MemberExpression) {
	targetType, ok := a.inferMemberExpression(member)
	if !ok {
		return
	}

	if property, ok := a.lookupPropertyOnMember(member); ok && property.Fallible {
		a.addErrorAtToken(member.Property.Token, "assigning fallible property %s requires try", member.Property.Value)
		return
	}

	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}

	if !canInitialize(targetType, valueType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
	}
}

func (a *Analyzer) resolveType(ref *ast.TypeReference) (Type, bool) {
	if ref == nil {
		return Type{Kind: InvalidType}, false
	}

	if ref.ElementType != nil {
		_, ok := a.resolveType(ref.ElementType)
		return Type{Name: "slice", Kind: InvalidType}, ok
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

	typ.TypeArgs = typeArgs
	if typ.Kind == ResultType && len(ref.TypeArgs) != 2 {
		a.addErrorAtToken(ref.Token, "Result requires exactly 2 type arguments, got %d", len(ref.TypeArgs))
		return Type{Kind: InvalidType}, false
	}
	if len(typeArgs) != len(ref.TypeArgs) {
		return Type{Kind: InvalidType}, false
	}

	return typ, true
}

type expressionValue struct {
	Display  string
	Negative bool
}

func (a *Analyzer) inferExpression(expr ast.Expression) (Type, expressionValue) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return Type{Name: "int", Kind: IntType}, expressionValue{Display: expr.String()}
	case *ast.FloatLiteral:
		return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
	case *ast.StringLiteral, *ast.InterpolatedStringLiteral:
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	case *ast.BooleanLiteral:
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok {
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

func (a *Analyzer) inferStructLiteral(expr *ast.StructLiteral) (Type, expressionValue) {
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

func (a *Analyzer) inferMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	if enumType, ok := a.inferEnumValueExpression(expr); ok {
		return enumType, true
	}

	objectType, _ := a.inferExpression(expr.Object)
	if objectType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
		return fieldType, true
	}

	if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
		return property.Type, true
	}

	a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
	return Type{Kind: InvalidType}, false
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

func lookupStructField(typ Type, name string) (Type, bool) {
	for _, field := range typ.Fields {
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

	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferCallExpression(expr *ast.CallExpression) (Type, expressionValue) {
	name := callExpressionName(expr)
	if name == "" {
		a.addErrorAtToken(expr.Token, "unsupported call expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	functions, ok := a.functions[name]
	if !ok || len(functions) == 0 {
		return a.inferCallAsConversion(expr)
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
	for _, function := range arityMatches {
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
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}
	}

	if len(best) > 1 {
		a.addErrorAtToken(expr.Token, "ambiguous call to %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	for _, function := range arityMatches {
		for i, arg := range expr.Arguments {
			param := function.Parameters[i]
			if !canInitialize(param.Type, argTypes[i], arg) {
				a.addErrorAtToken(expressionToken(arg), "argument %d to %s must be %s, got %s", i+1, name, typeDisplayName(param.Type), typeDisplayName(argTypes[i]))
			}
		}
		break
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
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
	targetType, ok := a.types[typeName]
	if !ok {
		a.addErrorAtToken(expr.Token, "unknown function or type %s", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Function.Token, "conversion to %s expects 1 argument, got %d", expr.Function.Value, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Arguments[0])
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Function.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
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
	catchAllSeen := false
	matchedVariants := map[string]lexer.Token{}

	for _, handler := range expr.Handlers {
		bindingName, variantName, ok := a.analyzeTryHandlerPattern(handler, errorType)
		if !ok {
			continue
		}

		if catchAllSeen {
			a.addErrorAtToken(handler.Token, "unreachable try handler")
			continue
		}

		if bindingName != "" {
			catchAllSeen = true
		}
		if variantName != "" {
			matchedVariants[variantName] = handler.Token
		}

		a.analyzeTryHandlerBody(handler, successType, errorType, bindingName)
	}

	if catchAllSeen {
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

func (a *Analyzer) analyzeTryHandlerPattern(handler *ast.TryHandler, errorType Type) (string, string, bool) {
	errPattern, ok := handler.Pattern.(*ast.ErrExpression)
	if !ok {
		a.addErrorAtToken(expressionToken(handler.Pattern), "try handler pattern must be Err(...)")
		return "", "", false
	}

	switch pattern := errPattern.Value.(type) {
	case *ast.Identifier:
		return pattern.Value, "", true
	case *ast.MemberExpression:
		patternType, ok := a.inferMemberExpression(pattern)
		if !ok || patternType.Kind == InvalidType {
			return "", "", false
		}
		if !sameConcreteType(patternType, errorType) {
			a.addErrorAtToken(expressionToken(pattern), "try handler pattern must match %s, got %s", typeDisplayName(errorType), typeDisplayName(patternType))
			return "", "", false
		}
		return "", pattern.Property.Value, true
	default:
		a.addErrorAtToken(expressionToken(errPattern.Value), "try handler pattern must be enum variant or identifier")
		return "", "", false
	}
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

	seenKinds := map[string]bool{}
	seenVariants := map[string]bool{}
	catchAll := false
	var resultType Type
	hasResultType := false

	for _, arm := range expr.Arms {
		info, ok := a.analyzeMatchPattern(arm.Pattern, subjectType)
		if !ok {
			continue
		}
		if catchAll {
			a.addErrorAtToken(arm.Token, "unreachable match arm")
			continue
		}
		if info.Kind == "catchall" {
			catchAll = true
		}
		if info.Kind != "" {
			seenKinds[info.Kind] = true
		}
		if info.Variant != "" {
			if seenVariants[info.Variant] {
				a.addErrorAtToken(arm.Token, "duplicate match arm for %s.%s", typeDisplayName(subjectType), info.Variant)
				continue
			}
			seenVariants[info.Variant] = true
		}

		armType := a.analyzeMatchArmBody(arm, info)
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

	a.checkMatchExhaustive(expr, subjectType, catchAll, seenKinds, seenVariants)

	if valueContext {
		if !hasResultType {
			a.addErrorAtToken(expr.Token, "match expression must produce a value")
			return Type{Kind: InvalidType}
		}
		return resultType
	}

	return Type{Name: "void", Kind: VoidType}
}

func (a *Analyzer) analyzeMatchPattern(pattern ast.Expression, subjectType Type) (matchPatternInfo, bool) {
	switch pattern := pattern.(type) {
	case *ast.OkExpression:
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			a.addErrorAtToken(pattern.Token, "Ok pattern requires Result subject")
			return matchPatternInfo{}, false
		}
		info := matchPatternInfo{Kind: "Ok"}
		if ident, ok := pattern.Value.(*ast.Identifier); ok {
			info.BindingName = ident.Value
			info.BindingType = subjectType.TypeArgs[0]
		}
		return info, true
	case *ast.ErrExpression:
		if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
			a.addErrorAtToken(pattern.Token, "Err pattern requires Result subject")
			return matchPatternInfo{}, false
		}
		info := matchPatternInfo{Kind: "Err"}
		if ident, ok := pattern.Value.(*ast.Identifier); ok {
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
	case *ast.Identifier:
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

func (a *Analyzer) analyzeMatchArmBody(arm *ast.MatchArm, info matchPatternInfo) Type {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	if info.BindingName != "" {
		a.symbols[info.BindingName] = Symbol{Name: info.BindingName, Type: info.BindingType, Mutable: false, Token: arm.Token}
		delete(a.constInts, info.BindingName)
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
	}()

	if arm.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, arm.ReturnBody)
		return Type{Kind: InvalidType}
	}
	if arm.BlockBody != nil {
		return Type{Kind: InvalidType}
	}
	if arm.Body == nil {
		return Type{Kind: InvalidType}
	}
	bodyType, _ := a.inferExpression(arm.Body)
	return bodyType
}

func (a *Analyzer) checkMatchExhaustive(expr *ast.MatchExpression, subjectType Type, catchAll bool, seenKinds map[string]bool, seenVariants map[string]bool) {
	if catchAll {
		return
	}

	if subjectType.Kind == ResultType && len(subjectType.TypeArgs) == 2 {
		if !seenKinds["Ok"] {
			a.addErrorAtToken(expr.Token, "non-exhaustive match for %s: missing Ok", typeDisplayName(subjectType))
		}
		if !seenKinds["Err"] {
			a.addErrorAtToken(expr.Token, "non-exhaustive match for %s: missing Err", typeDisplayName(subjectType))
		}
		return
	}

	if subjectType.Kind == EnumType {
		for _, variant := range subjectType.EnumValues {
			if !seenVariants[variant] {
				a.addErrorAtToken(expr.Token, "non-exhaustive match for %s", typeDisplayName(subjectType))
				return
			}
		}
	}
}

func (a *Analyzer) inferInfixExpression(expr *ast.InfixExpression) (Type, expressionValue) {
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
		return sameConcreteType(value, bound)
	}
	return canInitialize(value, bound, expr) || sameConcreteType(value, bound)
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

	if target.Kind == EnumType || value.Kind == EnumType {
		return target.Kind == EnumType && value.Kind == EnumType && target.Name == value.Name
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

	if isIntegerType(target) && value.Kind == EnumType {
		return true
	}

	if target.Kind == BoolType && isNumericType(value) {
		return true
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
	return typ.Named && (typ.Kind == EnumType || hasContracts(typ) || !typ.Dimension.IsZero())
}

func isIntegerType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType
}

func isNumericType(typ Type) bool {
	return typ.Kind == IntType || typ.Kind == UintType || typ.Kind == FloatType || typ.Kind == DecimalType
}

func sameConcreteType(left Type, right Type) bool {
	if left.Name != "" || right.Name != "" {
		return left.Name == right.Name
	}
	return left.Kind == right.Kind
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
	case *ast.ExpressionStatement:
		return stmt.Token
	case *ast.ReturnStatement:
		return stmt.Token
	case *ast.IfStatement:
		return stmt.Token
	case *ast.SwitchStatement:
		return stmt.Token
	case *ast.FallthroughStatement:
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
	default:
		return lexer.Token{}
	}
}
