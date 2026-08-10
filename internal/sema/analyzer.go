package sema

import (
	"fmt"
	"math/big"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type Analyzer struct {
	analysisDepth               AnalysisDepth
	analysisBudget              AnalysisBudget
	types                       map[string]Type
	units                       map[string]UnitDefinition
	functions                   map[string][]Function
	externSymbols               map[string]Function
	implBlocks                  map[string]lexer.Token
	implBlockModules            map[string]string
	validImplStatements         map[*ast.ImplStatement]bool
	currentImplTarget           string
	currentModule               string
	genericTypes                map[string]Type
	genericTypeInstances        map[genericInstanceKey]Type
	genericFuncInstances        map[genericInstanceKey]Function
	symbols                     map[string]Symbol
	completionSymbols           map[string]Symbol
	expressionTypes             map[ast.Expression]Type
	bindingIDs                  map[sourceTokenKey]BindingID
	bindingFacts                map[sourceTokenKey]ResolvedBinding
	resolvedCalls               map[*ast.CallExpression]ResolvedCall
	resolvedOperators           map[ast.Expression]ResolvedOperator
	resolvedTries               map[*ast.TryExpression]ResolvedTry
	resolvedTryPlans            map[*ast.TryExpression]ResolvedTryPlan
	nextBindingID               BindingID
	definitionTokens            map[sourceTokenKey][]lexer.Token
	callGraph                   *CallGraph
	escapeAnalysis              *EscapeAnalysis
	parameterUsageAnalysis      *ParameterUsageAnalysis
	currentCallable             CallableID
	callGraphPathReachable      bool
	spawnCallExpression         *ast.CallExpression
	spawnCallExecution          CallExecutionRelation
	typeDefinitionTokens        map[string]lexer.Token
	genericTypeDefinitions      map[string]lexer.Token
	invalidInterfaceInheritance map[string]bool
	constInts                   map[string]*big.Int
	assigned                    map[string]bool
	moved                       map[string]lexer.Token
	moveReasons                 map[string]string
	closedResources             map[string]lexer.Token
	borrows                     map[string][]borrowRecord
	localRefContainers          map[string]localReferenceOrigin
	expressionReferenceOrigins  map[ast.Expression]localReferenceOrigin
	arenaGenerations            map[string]int
	currentFunctionName         string
	currentFunctionReturn       Type
	currentFunctionToken        lexer.Token
	currentFunctionMetadata     Function
	currentFunctionSummary      localReferenceOrigin
	hasCurrentFunctionSummary   bool
	summaryPass                 bool
	inFunctionBody              bool
	inLambda                    bool
	lambdaOuterSymbols          map[string]Symbol
	inUnsafe                    bool
	inSwitchCaseBody            bool
	inDeferBlock                bool
	deferOuterSymbols           map[string]Symbol
	deferCaptures               map[string]lexer.Token
	suppressPlaceRootRead       int
	loopBackedgePlaces          map[string]bool
	cancellableDepth            int
	loopDepth                   int
	loopBreakFrames             []loopBreakFrame
	scopeDepth                  int
	allocationContext           AllocationContext
	errors                      []Error
	errorKeys                   map[string]bool
	warnings                    []Error
}

type borrowKind string

const (
	sharedBorrow  borrowKind = "shared"
	mutableBorrow borrowKind = "mutable"
	deferredUse   borrowKind = "deferred-use"
)

const dynamicArrayLength int64 = -1
const maxReferenceOriginAlternatives = 16

type borrowRecord struct {
	Root        string
	Place       Place
	Holder      string
	Kind        borrowKind
	Token       lexer.Token
	LoopCarried bool
}

type localReferenceOrigin struct {
	Name        string
	Token       lexer.Token
	Local       bool
	MatchScoped bool
	Mutable     bool
	Ambiguous   bool
	Unknown     bool
	Place       Place
	Places      []Place
	HasPlace    bool
	// Contained tracks reference origins stored below an aggregate root. Keys
	// are canonical relative access paths such as ".field" and "[2].view".
	Contained map[string]localReferenceOrigin
}

type loopBreakFrame struct {
	assignments                []map[string]bool
	moved                      []map[string]lexer.Token
	moveReasons                []map[string]string
	continueMoved              []map[string]lexer.Token
	continueReasons            []map[string]string
	closedResources            []map[string]lexer.Token
	continueClosedResources    []map[string]lexer.Token
	borrows                    []map[string][]borrowRecord
	localRefContainers         []map[string]localReferenceOrigin
	continueBorrows            []map[string][]borrowRecord
	continueLocalRefContainers []map[string]localReferenceOrigin
	arenaGenerations           []map[string]int
}

type genericInstanceKey struct {
	Declaration    string
	Arguments      string
	ConstArguments string
}

type sourceTokenKey struct {
	File   string
	Line   int
	Column int
}

func NewAnalyzer() *Analyzer {
	return NewAnalyzerWithDepth(AnalysisStandard)
}

func NewAnalyzerWithDepth(depth AnalysisDepth) *Analyzer {
	if _, err := ParseAnalysisDepth(string(depth)); err != nil {
		depth = AnalysisStandard
	}
	return &Analyzer{
		analysisDepth:  depth,
		analysisBudget: analysisBudget(depth),
		types:          builtinTypes(),
		units:          builtinUnits(),
	}
}

func (a *Analyzer) AnalysisDepth() AnalysisDepth { return a.analysisDepth }

func (a *Analyzer) AnalysisBudget() AnalysisBudget { return a.analysisBudget }

func (a *Analyzer) Analyze(program *ast.Program) []Error {
	a.errors = nil
	a.errorKeys = map[string]bool{}
	a.warnings = nil
	a.symbols = map[string]Symbol{}
	a.completionSymbols = map[string]Symbol{}
	a.expressionTypes = map[ast.Expression]Type{}
	a.bindingIDs = map[sourceTokenKey]BindingID{}
	a.bindingFacts = map[sourceTokenKey]ResolvedBinding{}
	a.resolvedCalls = map[*ast.CallExpression]ResolvedCall{}
	a.resolvedOperators = map[ast.Expression]ResolvedOperator{}
	a.resolvedTries = map[*ast.TryExpression]ResolvedTry{}
	a.resolvedTryPlans = map[*ast.TryExpression]ResolvedTryPlan{}
	a.nextBindingID = 1
	a.definitionTokens = map[sourceTokenKey][]lexer.Token{}
	a.callGraph = newCallGraph()
	a.escapeAnalysis = newEscapeAnalysis()
	a.parameterUsageAnalysis = newParameterUsageAnalysis()
	a.currentCallable = ""
	a.callGraphPathReachable = true
	a.spawnCallExpression = nil
	a.spawnCallExecution = ""
	a.typeDefinitionTokens = map[string]lexer.Token{}
	a.genericTypeDefinitions = nil
	a.invalidInterfaceInheritance = map[string]bool{}
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.moved = map[string]lexer.Token{}
	a.moveReasons = map[string]string{}
	a.closedResources = map[string]lexer.Token{}
	a.borrows = map[string][]borrowRecord{}
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.expressionReferenceOrigins = map[ast.Expression]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.functions = map[string][]Function{}
	a.externSymbols = map[string]Function{}
	a.registerCompilerKnownFunctions()
	a.implBlocks = map[string]lexer.Token{}
	a.implBlockModules = map[string]string{}
	a.validImplStatements = map[*ast.ImplStatement]bool{}
	a.currentImplTarget = ""
	a.currentModule = ""
	a.genericTypes = nil
	a.genericTypeInstances = map[genericInstanceKey]Type{}
	a.genericFuncInstances = map[genericInstanceKey]Function{}
	a.currentFunctionName = ""
	a.currentFunctionReturn = Type{}
	a.currentFunctionToken = lexer.Token{}
	a.currentFunctionMetadata = Function{}
	a.currentFunctionSummary = localReferenceOrigin{}
	a.hasCurrentFunctionSummary = false
	a.summaryPass = false
	a.inFunctionBody = false
	a.inLambda = false
	a.lambdaOuterSymbols = nil
	a.inUnsafe = false
	a.inSwitchCaseBody = false
	a.inDeferBlock = false
	a.suppressPlaceRootRead = 0
	a.loopBackedgePlaces = nil
	a.cancellableDepth = 0
	a.loopDepth = 0
	a.loopBreakFrames = nil
	a.scopeDepth = 0
	a.allocationContext = AllocationContext{Available: false, Origin: StorageOriginUnknown}
	a.validateModuleDeclaration(program)
	a.validateModuleDeclarationNamespace(program)
	a.registerTypeDeclarations(program)
	a.registerImplTypeDeclarations(program)
	a.analyzeInterfaceDeclarations(program)
	a.analyzeEarlyEnumDeclarations(program)
	a.analyzeTypeDeclarations(program)
	a.analyzeEnumDeclarations(program)
	a.analyzeImplTypeDeclarations(program)
	a.analyzeUnitMetadata(program)
	a.registerImplDeclarations(program)
	a.registerFunctionDeclarations(program)
	a.validateInterfaceConformance()
	a.inferFunctionReferenceSummaries(program)
	a.expressionTypes = map[ast.Expression]Type{}
	a.expressionReferenceOrigins = map[ast.Expression]localReferenceOrigin{}

	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt.(type) {
		case *ast.TargetDirective, *ast.TypeDeclStatement, *ast.UnitDeclStatement, *ast.EnumDeclaration, *ast.InterfaceDeclaration, *ast.ImplStatement, *ast.FunctionDeclaration:
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
	a.parameterUsageAnalysis = buildParameterUsageAnalysis(program, a)

	return a.errors
}

func (a *Analyzer) Warnings() []Error {
	return a.warnings
}

func (a *Analyzer) TypeOf(expr ast.Expression) (Type, bool) {
	if expr == nil {
		return Type{}, false
	}
	if inferred, ok := a.expressionTypes[expr]; ok && inferred.Kind != InvalidType {
		return inferred, true
	}
	inferred, _ := a.inferExpression(expr)
	if inferred.Kind == InvalidType {
		return Type{}, false
	}
	return inferred, true
}

// DefinitionsAt returns the declarations resolved by Sema for one source
// token. Most valid uses have exactly one declaration; unresolved or ambiguous
// source may have none or several.
func (a *Analyzer) DefinitionsAt(file string, line int, column int) []lexer.Token {
	definitions := a.definitionTokens[sourceTokenKey{File: file, Line: line, Column: column}]
	return append([]lexer.Token(nil), definitions...)
}

// CallGraph returns an immutable snapshot of the graph produced by the most
// recent analysis.
func (a *Analyzer) CallGraph() *CallGraph {
	return a.callGraph.clone()
}

// EscapeAnalysis returns an immutable snapshot of escape facts and callable
// summaries produced by the most recent analysis.
func (a *Analyzer) EscapeAnalysis() *EscapeAnalysis {
	return a.escapeAnalysis.clone()
}

// ParameterUsageAnalysis returns an immutable snapshot of the demands derived
// from the most recent completed semantic analysis.
func (a *Analyzer) ParameterUsageAnalysis() *ParameterUsageAnalysis {
	return a.parameterUsageAnalysis.clone()
}

func (a *Analyzer) recordArenaEffect(kind ArenaEffectKind, arena string, source lexer.Token, mayAllocate bool) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{
		Kind:        kind,
		Arena:       arena,
		Source:      source,
		MayAllocate: mayAllocate,
	})
}

func (a *Analyzer) recordDefinition(token lexer.Token) {
	if !validDefinitionToken(token) {
		return
	}
	a.setDefinitions(token, token)
}

func (a *Analyzer) bindDefinition(use lexer.Token, declaration lexer.Token) {
	if !validDefinitionToken(use) || !validDefinitionToken(declaration) {
		return
	}
	if _, exists := a.definitionTokens[sourceTokenLocation(use)]; exists {
		return
	}
	a.setDefinitions(use, declaration)
}

func (a *Analyzer) bindDefinitions(use lexer.Token, declarations []lexer.Token) {
	if !validDefinitionToken(use) {
		return
	}
	valid := make([]lexer.Token, 0, len(declarations))
	seen := map[sourceTokenKey]bool{}
	for _, declaration := range declarations {
		if !validDefinitionToken(declaration) {
			continue
		}
		key := sourceTokenLocation(declaration)
		if seen[key] {
			continue
		}
		seen[key] = true
		valid = append(valid, declaration)
	}
	if len(valid) == 0 {
		return
	}
	a.definitionTokens[sourceTokenLocation(use)] = valid
}

func (a *Analyzer) setDefinitions(use lexer.Token, declarations ...lexer.Token) {
	a.bindDefinitions(use, declarations)
}

func sourceTokenLocation(token lexer.Token) sourceTokenKey {
	return sourceTokenKey{File: token.File, Line: token.Line, Column: token.Column}
}

func validDefinitionToken(token lexer.Token) bool {
	return token.Line > 0 && token.Column > 0
}

func (a *Analyzer) registerTypeDefinition(name string, token lexer.Token) {
	if name == "" || !validDefinitionToken(token) {
		return
	}
	a.typeDefinitionTokens[name] = token
	a.recordDefinition(token)
}

func (a *Analyzer) Types() map[string]Type {
	out := make(map[string]Type, len(a.types))
	for name, typ := range a.types {
		out[name] = typ
	}
	return out
}

func (a *Analyzer) IntrinsicTypes() map[string]Type {
	out := map[string]Type{}
	for name, typ := range a.types {
		if typ.Intrinsic {
			out[name] = typ
		}
	}
	return out
}

func (a *Analyzer) Functions() map[string][]Function {
	out := make(map[string][]Function, len(a.functions))
	for name, functions := range a.functions {
		out[name] = append([]Function(nil), functions...)
	}
	return out
}

func (a *Analyzer) Symbols() map[string]Symbol {
	out := make(map[string]Symbol, len(a.symbols)+len(a.completionSymbols))
	for name, symbol := range a.completionSymbols {
		out[name] = symbol
	}
	for name, symbol := range a.symbols {
		out[name] = symbol
	}
	return out
}

func (a *Analyzer) withProgramModules(program *ast.Program, visit func(ast.Statement)) {
	previous := a.currentModule
	module := ""
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
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
	firstByFileAndPath := map[string]*ast.ModuleStatement{}
	found := false
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
		if moduleStmt, ok := stmt.(*ast.ModuleStatement); ok {
			if moduleStmt == nil || moduleStmt.Path == "" {
				continue
			}
			found = true
			key := moduleStmt.Token.File + "\x00" + moduleStmt.Path
			if previous := firstByFileAndPath[key]; previous != nil {
				a.addErrorAtTokenWithPreviousID(moduleStmt.Token, previous.Token, diagnostics.DuplicateModuleDeclaration, "duplicate module declaration %s", moduleStmt.Path)
				continue
			}
			firstByFileAndPath[key] = moduleStmt
		}
	}

	if !found {
		a.appendError(Error{
			ID:       diagnostics.MissingModuleDeclaration,
			Severity: diagnostics.SeverityError,
			Message:  "missing module declaration",
		})
	}
}

func isAllowedModuleStatement(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.TargetDirective,
		*ast.ModuleStatement,
		*ast.ImportStatement,
		*ast.TypeDeclStatement,
		*ast.UnitDeclStatement,
		*ast.EnumDeclaration,
		*ast.InterfaceDeclaration,
		*ast.ImplStatement,
		*ast.FunctionDeclaration,
		*ast.StructStatement,
		*ast.LetStatement,
		*ast.LetGroupStatement,
		*ast.CommentStatement,
		*ast.InvalidDeclaration,
		*ast.InvalidStatement:
		return true
	default:
		return false
	}
}

func isNilStatement(stmt ast.Statement) bool {
	if stmt == nil {
		return true
	}
	switch stmt := stmt.(type) {
	case *ast.TargetDirective:
		return stmt == nil
	case *ast.ModuleStatement:
		return stmt == nil
	case *ast.ImportStatement:
		return stmt == nil
	case *ast.TypeDeclStatement:
		return stmt == nil
	case *ast.UnitDeclStatement:
		return stmt == nil
	case *ast.EnumDeclaration:
		return stmt == nil
	case *ast.InterfaceDeclaration:
		return stmt == nil
	case *ast.ImplStatement:
		return stmt == nil
	case *ast.FunctionDeclaration:
		return stmt == nil
	case *ast.StructStatement:
		return stmt == nil
	case *ast.LetStatement:
		return stmt == nil
	case *ast.LetGroupStatement:
		return stmt == nil
	case *ast.CommentStatement:
		return stmt == nil
	case *ast.InvalidStatement:
		return stmt == nil
	case *ast.InvalidDeclaration:
		return stmt == nil
	case *ast.AssignmentStatement:
		return stmt == nil
	case *ast.TryAssignmentStatement:
		return stmt == nil
	case *ast.DeferStatement:
		return stmt == nil
	case *ast.DiscardStatement:
		return stmt == nil
	case *ast.DetachStatement:
		return stmt == nil
	case *ast.CancelStatement:
		return stmt == nil
	case *ast.ExpressionStatement:
		return stmt == nil
	case *ast.ReturnStatement:
		return stmt == nil
	case *ast.IfStatement:
		return stmt == nil
	case *ast.ForStatement:
		return stmt == nil
	case *ast.WhileStatement:
		return stmt == nil
	case *ast.SwitchStatement:
		return stmt == nil
	case *ast.SelectStatement:
		return stmt == nil
	case *ast.FallthroughStatement:
		return stmt == nil
	case *ast.BreakStatement:
		return stmt == nil
	case *ast.ContinueStatement:
		return stmt == nil
	case *ast.UnsafeStatement:
		return stmt == nil
	case *ast.AsmStatement:
		return stmt == nil
	case *ast.MatchStatement:
		return stmt == nil
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

type moduleDeclarationKind string

const (
	moduleDeclarationFunction  moduleDeclarationKind = "function"
	moduleDeclarationType      moduleDeclarationKind = "type"
	moduleDeclarationUnit      moduleDeclarationKind = "unit"
	moduleDeclarationEnum      moduleDeclarationKind = "enum"
	moduleDeclarationInterface moduleDeclarationKind = "interface"
	moduleDeclarationVariable  moduleDeclarationKind = "variable"
)

type moduleDeclaration struct {
	Name  string
	Kind  moduleDeclarationKind
	Token lexer.Token
}

func (a *Analyzer) validateModuleDeclarationNamespace(program *ast.Program) {
	declared := map[string]moduleDeclaration{}
	a.withProgramModules(program, func(stmt ast.Statement) {
		for _, decl := range moduleDeclarationsFromStatement(stmt) {
			if decl.Name == "" {
				continue
			}
			key := a.currentModule + "\x00" + decl.Name
			previous, exists := declared[key]
			if !exists {
				declared[key] = decl
				continue
			}
			if previous.Kind == moduleDeclarationFunction && decl.Kind == moduleDeclarationFunction {
				continue
			}
			if previous.Kind == moduleDeclarationVariable && decl.Kind == moduleDeclarationVariable {
				continue
			}
			if previous.Kind == decl.Kind {
				a.addErrorAtTokenWithPreviousID(decl.Token, previous.Token, diagnostics.ModuleDeclarationConflict, "duplicate declaration %s in module %s", decl.Name, moduleDisplayName(a.currentModule))
				continue
			}
			a.addErrorAtTokenWithPreviousID(decl.Token, previous.Token, diagnostics.ModuleDeclarationConflict, "%s %s conflicts with %s %s declared here", decl.Kind, decl.Name, previous.Kind, previous.Name)
		}
	})
}

func moduleDeclarationsFromStatement(stmt ast.Statement) []moduleDeclaration {
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationType, Token: stmt.Name.Token}}
	case *ast.UnitDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationUnit, Token: stmt.Name.Token}}
	case *ast.EnumDeclaration:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationEnum, Token: stmt.Name.Token}}
	case *ast.InterfaceDeclaration:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationInterface, Token: stmt.Name.Token}}
	case *ast.StructStatement:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationType, Token: stmt.Name.Token}}
	case *ast.FunctionDeclaration:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationFunction, Token: stmt.Name.Token}}
	case *ast.LetStatement:
		if stmt == nil || stmt.Name == nil {
			return nil
		}
		return []moduleDeclaration{{Name: stmt.Name.Value, Kind: moduleDeclarationVariable, Token: stmt.Name.Token}}
	case *ast.LetGroupStatement:
		if stmt == nil {
			return nil
		}
		decls := []moduleDeclaration{}
		for _, let := range stmt.Lets {
			if let == nil || let.Name == nil {
				continue
			}
			decls = append(decls, moduleDeclaration{Name: let.Name.Value, Kind: moduleDeclarationVariable, Token: let.Name.Token})
		}
		return decls
	default:
		return nil
	}
}

func moduleDisplayName(module string) string {
	if module == "" {
		return "<unnamed>"
	}
	return module
}

func hasAttribute(attributes []*ast.Attribute, name string) bool {
	for _, attribute := range attributes {
		if attribute != nil && attribute.Name != nil && attribute.Name.Value == name {
			return true
		}
	}
	return false
}

func (a *Analyzer) registerTypeDeclarations(program *ast.Program) {
	seenUnits := map[string]lexer.Token{}
	a.withProgramModules(program, func(stmt ast.Statement) {
		switch stmt := stmt.(type) {
		case *ast.TypeDeclStatement:
			if stmt.Name == nil {
				return
			}
			a.registerTypeDefinition(stmt.Name.Value, stmt.Name.Token)
			params := a.genericParameterNames(stmt.GenericParameters)
			noCopy := hasAttribute(stmt.Attributes, "noCopy")
			origin := ""
			if noCopy {
				origin = stmt.Name.Value
			}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType, GenericParameters: params, ExplicitlyNonCopyable: noCopy, NoCopyPolicyOrigin: origin}
		case *ast.UnitDeclStatement:
			if stmt.Name == nil {
				return
			}
			a.registerTypeDefinition(stmt.Name.Value, stmt.Name.Token)
			if previous, exists := seenUnits[stmt.Name.Value]; exists {
				a.addErrorAtTokenWithPrevious(stmt.Name.Token, previous, "unit %s already declared", stmt.Name.Value)
				return
			}
			seenUnits[stmt.Name.Value] = stmt.Name.Token
			dimension := a.parseDimension(stmt.Name.Value)
			if dimension.IsZero() {
				dimension = dimensionFromBase(stmt.Name.Value, 1)
			}
			category := OtherUnit
			defaultNumeric := "decimal"
			status := StatusActive
			if existing, ok := a.units[stmt.Name.Value]; ok {
				category = existing.Category
				dimension = existing.Dimension
				if existing.DefaultNumeric != "" {
					defaultNumeric = existing.DefaultNumeric
				}
				if existing.Status != "" {
					status = existing.Status
				}
			}
			a.units[stmt.Name.Value] = UnitDefinition{Name: stmt.Name.Value, Category: category, Dimension: dimension, DefaultNumeric: defaultNumeric, Status: status, Token: stmt.Name.Token}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType}
		case *ast.EnumDeclaration:
			if stmt.Name == nil {
				return
			}
			a.registerTypeDefinition(stmt.Name.Value, stmt.Name.Token)
			noCopy := hasAttribute(stmt.Attributes, "noCopy")
			origin := ""
			if noCopy {
				origin = stmt.Name.Value
			}
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InvalidType, ExplicitlyNonCopyable: noCopy, NoCopyPolicyOrigin: origin}
		case *ast.InterfaceDeclaration:
			if stmt.Name == nil {
				return
			}
			a.registerTypeDefinition(stmt.Name.Value, stmt.Name.Token)
			params := a.genericParameterNames(stmt.GenericParameters)
			a.types[stmt.Name.Value] = Type{Name: stmt.Name.Value, Module: a.currentModule, Kind: InterfaceType, Named: true, Declared: true, Underlying: "interface", GenericParameters: params}
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
	previousDefinitions := a.genericTypeDefinitions
	current := map[string]Type{}
	currentDefinitions := map[string]lexer.Token{}
	for name, typ := range previous {
		current[name] = typ
	}
	for name, token := range previousDefinitions {
		currentDefinitions[name] = token
	}
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		current[param.Name.Value] = Type{
			Name: param.Name.Value,
			Kind: GenericType,
		}
		currentDefinitions[param.Name.Value] = param.Name.Token
		a.recordDefinition(param.Name.Token)
	}
	a.genericTypes = current
	a.genericTypeDefinitions = currentDefinitions
	defer func() {
		a.genericTypes = previous
		a.genericTypeDefinitions = previousDefinitions
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
	// Register the single primary impl for each target first. Extension validity
	// must not depend on which source file was appended to the module program
	// first.
	a.withProgramModules(program, func(stmt ast.Statement) {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || impl.Extends {
			return
		}
		if _, ok := a.validateImplTarget(impl); !ok {
			return
		}
		if previous, exists := a.implBlocks[impl.Target.Name]; exists {
			a.addErrorAtTokenWithPrevious(impl.Target.Token, previous, "duplicate impl block for %s; additional blocks must use impl extends %s", impl.Target.Name, impl.Target.Name)
			return
		}
		a.implBlocks[impl.Target.Name] = impl.Target.Token
		a.implBlockModules[impl.Target.Name] = a.currentModule
		a.validImplStatements[impl] = true
	})

	// Extensions are validated only after all primaries are known. This permits
	// the primary and its extensions to live in any files of the same module.
	a.withProgramModules(program, func(stmt ast.Statement) {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || !impl.Extends {
			return
		}
		if _, ok := a.validateImplTarget(impl); !ok {
			return
		}
		primary, exists := a.implBlocks[impl.Target.Name]
		if !exists {
			a.addErrorAtToken(impl.Target.Token, "impl extends %s requires a primary impl %s block in the same module", impl.Target.Name, impl.Target.Name)
			return
		}
		primaryModule := a.implBlockModules[impl.Target.Name]
		if primaryModule != a.currentModule {
			a.addErrorAtTokenWithPrevious(impl.Target.Token, primary, "impl extension for %s must be in module %s", impl.Target.Name, moduleDisplayName(primaryModule))
			return
		}
		a.validImplStatements[impl] = true
	})

	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || !a.validImplStatements[impl] {
			continue
		}
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
			a.registerTypeDefinition(qualified, token)
		}
	}
}

func (a *Analyzer) validateImplTarget(impl *ast.ImplStatement) (Type, bool) {
	if impl == nil || impl.Target == nil {
		return Type{}, false
	}
	target, ok := a.types[impl.Target.Name]
	if !ok {
		a.addErrorAtToken(impl.Target.Token, "unknown impl target %s", impl.Target.Name)
		return Type{}, false
	}
	if definition, exists := a.typeDefinitionTokens[impl.Target.Name]; exists {
		a.bindDefinition(impl.Target.Token, definition)
	}
	if !target.Named && target.Kind != InvalidType && !isAllowedCoreBuiltinImpl(impl.Target.Name, impl.Target.Token) {
		a.addErrorAtToken(impl.Target.Token, "impl target %s is not a named type", impl.Target.Name)
		return Type{}, false
	}
	if target.Kind == InterfaceType {
		a.addErrorAtToken(impl.Target.Token, "interface %s cannot have an ordinary impl block", impl.Target.Name)
		return Type{}, false
	}
	if !a.validateImplGenericTarget(impl, target) {
		return Type{}, false
	}
	return target, true
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

func isAllowedCoreBuiltinImpl(target string, token lexer.Token) bool {
	if token.File == "" {
		return false
	}
	path := filepath.ToSlash(filepath.Clean(token.File))
	if !strings.Contains(path, "/sec/core/") && !strings.HasPrefix(path, "sec/core/") {
		return false
	}
	return isCoreBuiltinImplTarget(target)
}

func isCoreBuiltinImplTarget(target string) bool {
	switch target {
	case "bool",
		"byte",
		"char",
		"rune",
		"string",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"int128",
		"int256",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uint128",
		"uint256",
		"float",
		"float32",
		"float64",
		"decimal",
		"decimal128",
		"RawPtr",
		"Option",
		"Result":
		return true
	default:
		return false
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
		element := typeReferenceDisplayName(ref.ElementType)
		if ref.Slice {
			return element + "[]"
		}
		if ref.ArrayLengthExpression != nil {
			return fmt.Sprintf("%s[%s]", element, ref.ArrayLengthExpression.String())
		}
		return fmt.Sprintf("%s[%d]", element, ref.ArrayLength)
	}
	out := ref.Name
	if len(ref.TypeArgs) > 0 || len(ref.ConstArgs) > 0 {
		parts := make([]string, 0, len(ref.TypeArgs)+len(ref.ConstArgs))
		for _, arg := range ref.TypeArgs {
			parts = append(parts, typeReferenceDisplayName(arg))
		}
		for _, arg := range ref.ConstArgs {
			parts = append(parts, arg.String())
		}
		if ref.EventCapacitySet {
			parts = append(parts, fmt.Sprintf("%d", ref.EventCapacity))
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
		if !ok || enum.Name == nil {
			return
		}
		if enum.BitUnderlying {
			return
		}
		if existing := a.types[enum.Name.Value]; existing.Kind == EnumType {
			return
		}
		a.types[enum.Name.Value] = a.typeFromEnumDeclaration(enum.Name.Value, enum)
	})
}

func (a *Analyzer) analyzeEarlyEnumDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		enum, ok := stmt.(*ast.EnumDeclaration)
		if !ok || enum.Name == nil || !a.enumCanAnalyzeEarly(enum) {
			return
		}
		a.types[enum.Name.Value] = a.typeFromEnumDeclaration(enum.Name.Value, enum)
	})
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || !a.validImplStatements[impl] || impl.Target == nil {
			continue
		}
		for _, member := range impl.Members {
			enum, ok := member.(*ast.EnumDeclaration)
			if !ok || enum.Name == nil || !a.enumCanAnalyzeEarly(enum) {
				continue
			}
			qualified := impl.Target.Name + "." + enum.Name.Value
			a.types[qualified] = a.typeFromEnumDeclaration(qualified, enum)
		}
	}
}

func (a *Analyzer) enumCanAnalyzeEarly(enum *ast.EnumDeclaration) bool {
	if enum.BitUnderlying || enum.UnderlyingType == nil {
		return true
	}
	underlying, ok := a.types[enum.UnderlyingType.Name]
	return ok && (underlying.Kind == IntType || underlying.Kind == UintType)
}

func (a *Analyzer) analyzeImplTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
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
				if member.BitUnderlying {
					continue
				}
				if existing := a.types[qualified]; existing.Kind == EnumType {
					continue
				}
				a.withImplTarget(impl.Target.Name, func() {
					a.types[qualified] = a.typeFromEnumDeclaration(qualified, member)
				})
			}
		}
	}
}

func (a *Analyzer) analyzeUnitMetadata(program *ast.Program) {
	for _, stmt := range program.Statements {
		if isNilStatement(stmt) {
			continue
		}
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || !a.validImplStatements[impl] || impl.Target == nil {
			continue
		}
		unit, ok := a.units[impl.Target.Name]
		if !ok {
			continue
		}
		changed := false
		for _, member := range impl.Members {
			metadata, ok := member.(*ast.UnitMetadataDeclaration)
			if !ok {
				continue
			}
			value := unitMetadataValueTokens(metadata.Value)
			switch normalizeUnitMetadataName(metadata.Name) {
			case "dimension":
				dimension, ok := parseUnitMetadataDimension(value)
				if !ok {
					a.addErrorAtToken(metadata.Token, "invalid dimension metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.Dimension = dimension
				changed = true
			case "system":
				if len(value) != 1 || value[0].Type != lexer.IDENT {
					a.addErrorAtToken(metadata.Token, "invalid system metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.System = value[0].Lexeme
				changed = true
			case "scale":
				if len(value) == 0 {
					a.addErrorAtToken(metadata.Token, "invalid scale metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.Scale = tokensDisplay(value)
				changed = true
			case "longname":
				text, ok := parseUnitMetadataString(value)
				if !ok {
					a.addErrorAtToken(metadata.Token, "invalid long_name metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.LongName = text
				changed = true
			case "symbol":
				text, ok := parseUnitMetadataString(value)
				if !ok {
					a.addErrorAtToken(metadata.Token, "invalid symbol metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.Symbol = text
				changed = true
			case "baseunit":
				baseUnit, ok := parseUnitMetadataBool(value)
				if !ok {
					a.addErrorAtToken(metadata.Token, "invalid base_unit metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.IsBaseUnit = baseUnit
				changed = true
			case "status":
				status, ok := parseUnitMetadataStatus(value)
				if !ok {
					a.addErrorAtToken(metadata.Token, "invalid status metadata for unit %s", impl.Target.Name)
					continue
				}
				unit.Status = status
				changed = true
			}
		}
		if !changed {
			continue
		}
		a.units[impl.Target.Name] = unit
		typ := a.types[impl.Target.Name]
		if typ.Kind != InvalidType {
			typ.Dimension = unit.Dimension
			a.types[impl.Target.Name] = typ
		}
	}
}

func unitMetadataValueTokens(tokens []lexer.Token) []lexer.Token {
	out := make([]lexer.Token, 0, len(tokens))
	for _, token := range tokens {
		if token.Type == lexer.COMMENT {
			continue
		}
		out = append(out, token)
	}
	return out
}

func normalizeUnitMetadataName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

func parseUnitMetadataString(tokens []lexer.Token) (string, bool) {
	if len(tokens) != 1 {
		return "", false
	}
	switch tokens[0].Type {
	case lexer.STRING:
		value, err := strconv.Unquote(tokens[0].Lexeme)
		if err != nil {
			return "", false
		}
		return value, true
	case lexer.IDENT:
		return tokens[0].Lexeme, true
	default:
		return "", false
	}
}

func parseUnitMetadataBool(tokens []lexer.Token) (bool, bool) {
	if len(tokens) != 1 {
		return false, false
	}
	switch tokens[0].Type {
	case lexer.TRUE:
		return true, true
	case lexer.FALSE:
		return false, true
	default:
		return false, false
	}
}

func parseUnitMetadataStatus(tokens []lexer.Token) (UnitStatus, bool) {
	if len(tokens) != 1 || tokens[0].Type != lexer.IDENT {
		return "", false
	}
	status := UnitStatus(tokens[0].Lexeme)
	switch status {
	case StatusActive, StatusDeprecated, StatusObsolete:
		return status, true
	default:
		return "", false
	}
}

func tokensDisplay(tokens []lexer.Token) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token.Lexeme)
	}
	return strings.Join(parts, " ")
}

func parseUnitMetadataDimension(tokens []lexer.Token) (Dimension, bool) {
	if len(tokens) < 2 || tokens[0].Type != lexer.LBRACKET || tokens[len(tokens)-1].Type != lexer.RBRACKET {
		return Dimension{}, false
	}
	dimension := Dimension{Base: map[string]int{}}
	i := 1
	for i < len(tokens)-1 {
		if tokens[i].Type != lexer.IDENT {
			return Dimension{}, false
		}
		axis := tokens[i].Lexeme
		i++
		for i+1 < len(tokens)-1 && tokens[i].Type == lexer.DOT && tokens[i+1].Type == lexer.IDENT {
			axis += "." + tokens[i+1].Lexeme
			i += 2
		}
		if i >= len(tokens)-1 || tokens[i].Type != lexer.BIT_XOR {
			return Dimension{}, false
		}
		i++
		sign := 1
		if i < len(tokens)-1 && tokens[i].Type == lexer.MINUS {
			sign = -1
			i++
		}
		if i >= len(tokens)-1 || tokens[i].Type != lexer.INT {
			return Dimension{}, false
		}
		exponent, ok := parseSmallInt(tokens[i].Lexeme)
		if !ok || exponent == 0 {
			return Dimension{}, false
		}
		if _, exists := dimension.Base[axis]; exists {
			return Dimension{}, false
		}
		dimension.Base[axis] = sign * exponent
		i++
		if i == len(tokens)-1 {
			break
		}
		if tokens[i].Type != lexer.COMMA {
			return Dimension{}, false
		}
		i++
	}
	return dimension, true
}

func parseSmallInt(value string) (int, bool) {
	n := 0
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
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
	case *ast.InterfaceDeclaration:
		a.analyzeInterfaceDeclaration(stmt)
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
	case *ast.DetachStatement:
		a.analyzeDetachStatement(stmt)
	case *ast.CancelStatement:
		a.analyzeCancelStatement(stmt)
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
	case *ast.SelectStatement:
		a.analyzeSelectStatement(stmt)
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
		} else {
			a.recordLoopContinue()
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
		if stmt.Message != "" {
			a.addErrorAtToken(stmt.Token, "%s", stmt.Message)
		}
		return
	case *ast.InvalidDeclaration:
		if stmt.Message != "" {
			a.addErrorAtToken(stmt.Token, "%s", stmt.Message)
		}
		return
	}
}

func (a *Analyzer) analyzeBlockStatements(block *ast.BlockStatement) {
	if block == nil {
		return
	}

	symbolsBefore := map[string]bool{}
	for name := range a.symbols {
		symbolsBefore[name] = true
	}
	a.scopeDepth++
	defer func() {
		newNames := make([]string, 0)
		for name := range a.symbols {
			if !symbolsBefore[name] {
				newNames = append(newNames, name)
			}
		}
		sort.Strings(newNames)
		for _, name := range newNames {
			a.checkUnresolvedTaskAtScopeExit(name)
			a.checkUnclosedResourceAtScopeExit(name)
			a.clearScopedBindingFromLoopEdges(name)
			a.endBorrowsHeldBy(name)
			delete(a.localRefContainers, name)
		}
		a.scopeDepth--
	}()

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
	if _, ok := stmt.Expression.(*ast.SpawnExpression); ok && exprType.Kind != InvalidType {
		a.addErrorAtToken(stmt.Token, "spawned task result must be owned, awaited, joined or detached")
		return
	}
	if exprType.Kind == InvalidType || exprType.Kind == VoidType || !isCallLikeExpression(stmt.Expression) {
		return
	}
	if !isMustUseType(exprType) {
		// Ordinary call results are implicitly discarded at statement end. The
		// configurable advisory for this acknowledged loss is tracked separately.
		return
	}
	if a.inDeferBlock && exprType.Kind == ResultType {
		a.addErrorAtTokenWithMetadata(
			expressionToken(stmt.Expression),
			diagnostics.UnhandledMustUseResult,
			"use try, match, binding, return, or explicit discard",
			"unhandled Result inside defer; handle it or discard it explicitly",
		)
		return
	}
	a.addErrorAtTokenWithMetadata(
		expressionToken(stmt.Expression),
		diagnostics.UnhandledMustUseResult,
		"use try, match, binding, return, or explicit discard",
		"result of %s has type %s and must be handled explicitly",
		expressionStatementDisplayName(stmt.Expression),
		typeDisplayName(exprType),
	)
}

func isCallLikeExpression(expr ast.Expression) bool {
	_, call := expr.(*ast.CallExpression)
	return call
}

func isMustUseType(typ Type) bool {
	return typeRequiresExplicitHandling(typ, map[string]bool{})
}

func typeRequiresExplicitHandling(typ Type, visiting map[string]bool) bool {
	if typ.Kind == ReferenceType {
		// A reference does not own the lifecycle value it views.
		return false
	}
	typ = dereferenceType(typ)
	if typ.Kind == ResultType || isTaskType(typ) || isThreadType(typ) {
		return true
	}
	key := typeObligationKey(typ)
	if key != "" {
		if visiting[key] {
			return false
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	for _, argument := range typ.TypeArgs {
		if typeRequiresExplicitHandling(argument, visiting) {
			return true
		}
	}
	if typ.Element != nil && typeRequiresExplicitHandling(*typ.Element, visiting) {
		return true
	}
	for _, field := range typ.Fields {
		if typeRequiresExplicitHandling(field.Type, visiting) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && typeRequiresExplicitHandling(*variant.Payload, visiting) {
			return true
		}
		for _, field := range variant.PayloadFields {
			if typeRequiresExplicitHandling(field.Type, visiting) {
				return true
			}
		}
	}
	return false
}

func isDiscardableType(typ Type) bool {
	return typeIsDiscardable(typ, map[string]bool{})
}

func typeIsDiscardable(typ Type, visiting map[string]bool) bool {
	if typ.Kind == ReferenceType {
		return true
	}
	typ = dereferenceType(typ)
	if isTaskType(typ) || isThreadType(typ) {
		return false
	}
	key := typeObligationKey(typ)
	if key != "" {
		if visiting[key] {
			return true
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	for _, argument := range typ.TypeArgs {
		if !typeIsDiscardable(argument, visiting) {
			return false
		}
	}
	if typ.Element != nil && !typeIsDiscardable(*typ.Element, visiting) {
		return false
	}
	for _, field := range typ.Fields {
		if !typeIsDiscardable(field.Type, visiting) {
			return false
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && !typeIsDiscardable(*variant.Payload, visiting) {
			return false
		}
		for _, field := range variant.PayloadFields {
			if !typeIsDiscardable(field.Type, visiting) {
				return false
			}
		}
	}
	return true
}

func typeObligationKey(typ Type) string {
	if typ.Name != "" {
		return typeDisplayName(typ)
	}
	if typ.Kind == StructType || typ.Kind == UnionType || typ.Kind == ArrayType || typ.Kind == SliceType {
		return string(typ.Kind)
	}
	return ""
}

func expressionStatementDisplayName(expr ast.Expression) string {
	if call, ok := expr.(*ast.CallExpression); ok {
		if name := callExpressionName(call); name != "" {
			return name
		}
	}
	return expr.String()
}

func (a *Analyzer) checkUnresolvedTaskAtScopeExit(name string) {
	symbol, ok := a.symbols[name]
	if !ok || (!isTaskType(symbol.Type) && !isThreadType(symbol.Type)) {
		return
	}
	if _, moved := a.moved[name]; moved {
		return
	}
	kind := "task"
	if isThreadType(symbol.Type) {
		kind = "thread"
	}
	a.addErrorAtToken(symbol.Token, "owned %s %s is unresolved at scope exit", kind, name)
}

func (a *Analyzer) checkUnclosedResourceAtScopeExit(name string) {
	symbol, ok := a.symbols[name]
	if !ok || !isCloseTrackedResourceType(symbol.Type) {
		return
	}
	if _, moved := a.moved[name]; moved {
		return
	}
	if _, closed := a.closedResources[name]; closed {
		return
	}
	resource := closeTrackedResourceName(symbol.Type)
	a.addErrorAtToken(symbol.Token, "owned %s %s is still open at scope exit; call %s.Close() or return it to transfer ownership", resource, name, name)
}

func isCloseTrackedResourceType(typ Type) bool {
	typ = dereferenceType(typ)
	if typ.Kind != StructType {
		return false
	}
	baseName := visibilityBaseName(typ.Name)
	if baseName != "File" && baseName != "Directory" {
		return false
	}
	return typ.Module == "io" || typ.Name == "io."+baseName
}

func closeTrackedResourceName(typ Type) string {
	if visibilityBaseName(dereferenceType(typ).Name) == "Directory" {
		return "directory"
	}
	return "file"
}

func (a *Analyzer) markClosedResourceCall(function Function, receiver methodReceiverInfo, isMethodCall bool, token lexer.Token) {
	if !isMethodCall || !strings.HasSuffix(function.Name, ".Close") {
		return
	}
	if receiver.Symbol == nil || !isCloseTrackedResourceType(receiver.Symbol.Type) {
		return
	}
	if !isCloseTrackedResourceType(receiver.Type) && !isCloseTrackedResourceName(function.ImplTarget) {
		return
	}
	a.closedResources[receiver.Symbol.Name] = token
}

func isCloseTrackedResourceName(name string) bool {
	baseName := visibilityBaseName(name)
	if baseName != "File" && baseName != "Directory" {
		return false
	}
	return name == baseName || name == "io."+baseName
}

func (a *Analyzer) analyzeDiscardStatement(stmt *ast.DiscardStatement) {
	if stmt.Value == nil {
		a.addErrorAtToken(stmt.Token, "discard requires expression")
		return
	}
	if ident, ok := stmt.Value.(*ast.Identifier); ok && ident.Value == "_" {
		a.addErrorAtToken(ident.Token, "discard requires named value")
		return
	}
	if _, ok := stmt.Value.(*ast.SpawnExpression); ok {
		typ, _ := a.inferExpression(stmt.Value)
		if typ.Kind != InvalidType {
			a.addErrorAtToken(stmt.Token, "cannot discard spawn result because successful creation would abandon %s", typeDisplayName(typ))
		}
		return
	}

	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}
	if isTaskType(valueType) || isThreadType(valueType) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot discard unresolved %s; await, join or detach it explicitly", typeDisplayName(valueType))
		return
	}
	if !isDiscardableType(valueType) {
		a.addErrorAtTokenWithMetadata(
			expressionToken(stmt.Value),
			diagnostics.NonDiscardableValue,
			"handle the value and resolve every contained task or thread lifecycle",
			"cannot discard %s because it may contain an unresolved lifecycle handle",
			typeDisplayName(valueType),
		)
		return
	}

	ident, ok := stmt.Value.(*ast.Identifier)
	if !ok {
		return
	}
	symbol, exists := a.symbols[ident.Value]
	if !exists {
		return
	}
	if a.checkDeferredUse(ident.Value, ident.Token, "discard") || a.checkBorrowedMove(ident.Value, ident.Token) {
		return
	}
	a.moved[ident.Value] = ident.Token
	a.moveReasons[ident.Value] = "discarded"
	delete(a.constInts, ident.Value)
	a.endBorrowsHeldBy(ident.Value)
	if symbol.Type.Kind == ReferenceType {
		delete(a.localRefContainers, ident.Value)
	}
}

func (a *Analyzer) analyzeDetachStatement(stmt *ast.DetachStatement) {
	if stmt.Value == nil {
		a.addErrorAtToken(stmt.Token, "detach requires task or thread handle")
		return
	}
	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}
	if !isTaskType(valueType) && !isThreadType(valueType) {
		a.addErrorAtToken(expressionToken(stmt.Value), "detach requires Task[T] or Thread[T], got %s", typeDisplayName(valueType))
		return
	}
	if len(valueType.TypeArgs) != 1 {
		return
	}
	resultType := valueType.TypeArgs[0]
	if resultType.Kind != VoidType && !stmt.DiscardResult {
		a.addErrorAtToken(expressionToken(stmt.Value), "detaching %s with non-void result requires explicit discard", typeDisplayName(valueType))
		return
	}
	if a.markMoveSource(stmt.Value) {
		if ident, ok := stmt.Value.(*ast.Identifier); ok {
			a.moveReasons[ident.Value] = "detached"
		}
	}
}

func (a *Analyzer) analyzeCancelStatement(stmt *ast.CancelStatement) {
	if a.inDeferBlock {
		a.addErrorAtToken(stmt.Token, "cancel is not allowed inside defer")
		return
	}
	if a.cancellableDepth > 0 {
		return
	}
	a.addErrorAtToken(stmt.Token, "cancel is not valid outside a task or explicit thread context")
}

func (a *Analyzer) analyzeIfStatement(stmt *ast.IfStatement) {
	if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "if condition must be bool, got %s", typeDisplayName(conditionType))
		}
	}

	before := copyAssigned(a.assigned)
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)
	thenReachable := true
	elseReachable := true
	if isBoolLiteral(stmt.Condition, true) {
		elseReachable = false
	} else if isBoolLiteral(stmt.Condition, false) {
		thenReachable = false
	}
	thenBranch := a.analyzeBranchBlockWithCallGraphReachability(stmt.Consequence, thenReachable)
	if !thenReachable {
		thenBranch.continues = false
	}
	if stmt.Alternative != nil {
		elseBranch := a.analyzeBranchBlockWithCallGraphReachability(stmt.Alternative, elseReachable)
		if !elseReachable {
			elseBranch.continues = false
		}
		a.assigned = mergeContinuingAssigned(before, thenBranch, elseBranch)
		a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, thenBranch, elseBranch)
		a.closedResources = mergeContinuingClosedResources(beforeClosedResources, thenBranch, elseBranch)
		a.borrows = mergeContinuingBorrows(beforeBorrows, thenBranch, elseBranch)
		a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, thenBranch, elseBranch)
		a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, thenBranch, elseBranch)
		return
	}

	fallthroughBranch := branchAnalysis{
		assigned:           before,
		moved:              beforeMoved,
		moveReasons:        beforeMoveReasons,
		closedResources:    beforeClosedResources,
		borrows:            beforeBorrows,
		localRefContainers: beforeLocalRefContainers,
		arenaGenerations:   beforeArenaGenerations,
		continues:          elseReachable,
	}
	a.assigned = mergeContinuingAssigned(before, thenBranch, fallthroughBranch)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, thenBranch, fallthroughBranch)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, thenBranch, fallthroughBranch)
	a.borrows = mergeContinuingBorrows(beforeBorrows, thenBranch, fallthroughBranch)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, thenBranch, fallthroughBranch)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, thenBranch, fallthroughBranch)
}

func (a *Analyzer) analyzeBranchBlockWithCallGraphReachability(block *ast.BlockStatement, reachable bool) branchAnalysis {
	previous := a.callGraphPathReachable
	a.callGraphPathReachable = previous && reachable
	defer func() {
		a.callGraphPathReachable = previous
	}()
	return a.analyzeBranchBlock(block)
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
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousInDeferBlock := a.inDeferBlock
	previousDeferOuterSymbols := a.deferOuterSymbols
	previousDeferCaptures := a.deferCaptures
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.inDeferBlock = true
	a.deferOuterSymbols = previousSymbols
	a.deferCaptures = map[string]lexer.Token{}
	defer func() {
		for name, token := range a.deferCaptures {
			place, _ := a.rootPlace(name)
			previousBorrows[name] = append(previousBorrows[name], borrowRecord{
				Root:   name,
				Place:  place,
				Holder: "$defer",
				Kind:   deferredUse,
				Token:  token,
			})
		}
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.inDeferBlock = previousInDeferBlock
		a.deferOuterSymbols = previousDeferOuterSymbols
		a.deferCaptures = previousDeferCaptures
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
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.loopDepth++

	if len(stmt.Bindings) > 0 || stmt.Iterable != nil {
		a.analyzeForIterable(stmt)
	}
	iterationEntry := a.captureLoopIterationAnalysisState()

	if stmt.Body != nil {
		a.analyzeBlockStatements(stmt.Body)
	}

	loopMoved := a.moved
	loopMoveReasons := a.moveReasons
	loopClosedResources := a.closedResources
	loopBorrows := a.borrows
	loopLocalRefContainers := a.localRefContainers
	loopArenaGenerations := a.arenaGenerations
	frameState := a.loopBreakFrames[frame]
	for _, binding := range stmt.Bindings {
		if binding.Discard {
			continue
		}
		clearRootPlaceStateMaps(loopMoved, loopMoveReasons, binding.Name)
		for index := range frameState.moved {
			clearRootPlaceStateMaps(frameState.moved[index], frameState.moveReasons[index], binding.Name)
		}
		for index := range frameState.continueMoved {
			clearRootPlaceStateMaps(frameState.continueMoved[index], frameState.continueReasons[index], binding.Name)
		}
		delete(loopClosedResources, binding.Name)
		for index := range frameState.closedResources {
			delete(frameState.closedResources[index], binding.Name)
		}
		for index := range frameState.continueClosedResources {
			delete(frameState.continueClosedResources[index], binding.Name)
		}
		clearLoopBindingReferenceState(loopBorrows, loopLocalRefContainers, binding.Name)
		for index := range frameState.borrows {
			clearLoopBindingReferenceState(frameState.borrows[index], frameState.localRefContainers[index], binding.Name)
		}
		for index := range frameState.continueBorrows {
			clearLoopBindingReferenceState(frameState.continueBorrows[index], frameState.continueLocalRefContainers[index], binding.Name)
		}
	}
	a.loopBreakFrames[frame] = frameState
	headerMoved, headerReasons := loopBackedgeMoveState(iterationEntry.moved, iterationEntry.moveReasons, loopMoved, loopMoveReasons, frameState, blockCanFallThrough(stmt.Body))
	headerClosedResources := loopBackedgeClosedResourceState(iterationEntry.closedResources, loopClosedResources, frameState, blockCanFallThrough(stmt.Body))
	headerBorrows := loopBackedgeBorrowState(iterationEntry.borrows, loopBorrows, frameState, blockCanFallThrough(stmt.Body))
	headerLocalRefContainers := loopBackedgeReferenceState(iterationEntry.localRefContainers, loopLocalRefContainers, frameState, blockCanFallThrough(stmt.Body))
	a.checkLoopBackedgeFixedPoint(nil, stmt.Body, iterationEntry, headerMoved, headerReasons, headerClosedResources, headerBorrows, headerLocalRefContainers)
	breakFrame := a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	a.assigned = previousAssigned
	a.moved, a.moveReasons = mergeLoopMoveState(previousMoved, previousMoveReasons, loopMoved, loopMoveReasons, breakFrame)
	a.closedResources = mergeLoopClosedResourceState(previousClosedResources, loopClosedResources, breakFrame, blockCanFallThrough(stmt.Body), false)
	a.borrows = mergeLoopBorrowState(previousBorrows, loopBorrows, breakFrame)
	a.localRefContainers = mergeLoopReferenceState(previousLocalRefContainers, loopLocalRefContainers, breakFrame)
	a.arenaGenerations = mergeLoopArenaGenerations(previousArenaGenerations, loopArenaGenerations, breakFrame.arenaGenerations)
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
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.loopDepth++
	iterationEntry := a.captureLoopIterationAnalysisState()

	if stmt.Body != nil {
		a.analyzeBlockStatements(stmt.Body)
	}

	loopConstInts := a.constInts
	loopMoved := a.moved
	loopMoveReasons := a.moveReasons
	loopClosedResources := a.closedResources
	loopBorrows := a.borrows
	loopLocalRefContainers := a.localRefContainers
	loopArenaGenerations := a.arenaGenerations
	frameState := a.loopBreakFrames[frame]
	headerMoved, headerReasons := loopBackedgeMoveState(iterationEntry.moved, iterationEntry.moveReasons, loopMoved, loopMoveReasons, frameState, blockCanFallThrough(stmt.Body))
	headerClosedResources := loopBackedgeClosedResourceState(iterationEntry.closedResources, loopClosedResources, frameState, blockCanFallThrough(stmt.Body))
	headerBorrows := loopBackedgeBorrowState(iterationEntry.borrows, loopBorrows, frameState, blockCanFallThrough(stmt.Body))
	headerLocalRefContainers := loopBackedgeReferenceState(iterationEntry.localRefContainers, loopLocalRefContainers, frameState, blockCanFallThrough(stmt.Body))
	a.checkLoopBackedgeFixedPoint(stmt.Condition, stmt.Body, iterationEntry, headerMoved, headerReasons, headerClosedResources, headerBorrows, headerLocalRefContainers)
	breakFrame := a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	for name, previousValue := range previousConstInts {
		currentValue, exists := loopConstInts[name]
		if !exists || currentValue.Cmp(previousValue) != 0 {
			delete(a.constInts, name)
		}
	}
	if isBoolLiteral(stmt.Condition, true) && len(breakFrame.assignments) > 0 {
		a.assigned = mergeBreakAssigned(previousAssigned, breakFrame.assignments)
	} else {
		a.assigned = previousAssigned
	}
	a.moved, a.moveReasons = mergeLoopMoveState(previousMoved, previousMoveReasons, loopMoved, loopMoveReasons, breakFrame)
	a.closedResources = mergeLoopClosedResourceState(previousClosedResources, loopClosedResources, breakFrame, blockCanFallThrough(stmt.Body), isBoolLiteral(stmt.Condition, true))
	a.borrows = mergeLoopBorrowState(previousBorrows, loopBorrows, breakFrame)
	a.localRefContainers = mergeLoopReferenceState(previousLocalRefContainers, loopLocalRefContainers, breakFrame)
	a.arenaGenerations = mergeLoopArenaGenerations(previousArenaGenerations, loopArenaGenerations, breakFrame.arenaGenerations)
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
		if iterableType.Kind == ReferenceType && iterableType.Element != nil &&
			(iterableType.Element.Kind == ArrayType || iterableType.Element.Kind == SliceType || isForCollectionFamily(*iterableType.Element)) {
			iterableType = *iterableType.Element
		}
		indexType := Type{Name: "int", Kind: IntType}
		if iterableType.Kind == StringType {
			return a.inferSequentialForBindingTypes(stmt, Type{Name: "rune", Kind: RuneType}, indexType)
		}
		if (iterableType.Kind == ArrayType || iterableType.Kind == SliceType) && iterableType.Element != nil {
			return a.inferSequentialForBindingTypes(stmt, *iterableType.Element, indexType)
		}
		if (iterableType.Name == "Vec" || iterableType.Name == "list") && len(iterableType.TypeArgs) == 1 {
			return a.inferSequentialForBindingTypes(stmt, iterableType.TypeArgs[0], indexType)
		}
		if iterableType.Name == "vector" && len(iterableType.TypeArgs) == 1 && len(iterableType.ConstArgs) == 1 {
			return a.inferSequentialForBindingTypes(stmt, iterableType.TypeArgs[0], indexType)
		}
		if (iterableType.Name == "Set" || iterableType.Name == "set") && len(iterableType.TypeArgs) == 1 {
			if len(stmt.Bindings) > 1 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "set iteration supports one loop binding, got %d", len(stmt.Bindings))
				return nil, false
			}
			return []Type{iterableType.TypeArgs[0]}, true
		}
		if (iterableType.Name == "Map" || iterableType.Name == "map") && len(iterableType.TypeArgs) == 2 {
			if len(stmt.Bindings) != 2 {
				a.addErrorAtToken(stmt.Bindings[0].Token, "map iteration requires key and value bindings, got %d", len(stmt.Bindings))
				return nil, false
			}
			return []Type{iterableType.TypeArgs[0], iterableType.TypeArgs[1]}, true
		}
		a.addErrorAtToken(expressionToken(iterable), "type %s is not iterable", typeDisplayName(iterableType))
		return nil, false
	}
}

func (a *Analyzer) inferSequentialForBindingTypes(stmt *ast.ForStatement, valueType Type, indexType Type) ([]Type, bool) {
	if len(stmt.Bindings) > 2 {
		a.addErrorAtToken(stmt.Bindings[0].Token, "sequential iteration supports one or two loop bindings, got %d", len(stmt.Bindings))
		return nil, false
	}
	if len(stmt.Bindings) == 2 {
		return []Type{indexType, valueType}, true
	}
	return []Type{valueType}, true
}

func isForCollectionFamily(typ Type) bool {
	switch typ.Name {
	case "Vec", "Set", "Map", "list", "set", "map", "vector":
		return true
	default:
		return false
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
		if value, ok := a.integerConstantValue(step); ok && value.Sign() == 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must not be zero")
			return Type{Kind: InvalidType}, false
		}
		if startValue, startOK := a.integerConstantValue(expr.Start); startOK {
			if endValue, endOK := a.integerConstantValue(expr.End); endOK {
				if stepValue, stepOK := a.integerConstantValue(step); stepOK {
					if startValue.Cmp(endValue) < 0 && stepValue.Sign() < 0 {
						a.addErrorAtToken(expressionToken(step), "for ascending range step must be positive")
						return Type{Kind: InvalidType}, false
					}
					if startValue.Cmp(endValue) > 0 && stepValue.Sign() > 0 {
						a.addErrorAtToken(expressionToken(step), "for descending range step must be negative")
						return Type{Kind: InvalidType}, false
					}
				}
			}
		}
		if value, ok := decimalLiteralValue(step); ok && value.Int64 == 0 {
			a.addErrorAtToken(expressionToken(step), "for range step must not be zero")
			return Type{Kind: InvalidType}, false
		}
		if startValue, startOK := decimalLiteralValue(expr.Start); startOK {
			if endValue, endOK := decimalLiteralValue(expr.End); endOK {
				if stepValue, stepOK := decimalLiteralValue(step); stepOK {
					if startValue.Int64 < endValue.Int64 && stepValue.Int64 < 0 {
						a.addErrorAtToken(expressionToken(step), "for ascending range step must be positive")
						return Type{Kind: InvalidType}, false
					}
					if startValue.Int64 > endValue.Int64 && stepValue.Int64 > 0 {
						a.addErrorAtToken(expressionToken(step), "for descending range step must be negative")
						return Type{Kind: InvalidType}, false
					}
				}
			}
		}
	}

	if !isNumericType(startType) || !isNumericType(endType) {
		a.addErrorAtToken(expr.Token, "type %s is not iterable", typeDisplayName(startType))
		return Type{Kind: InvalidType}, false
	}

	return startType, true
}

type branchAnalysis struct {
	assigned           map[string]bool
	moved              map[string]lexer.Token
	moveReasons        map[string]string
	closedResources    map[string]lexer.Token
	borrows            map[string][]borrowRecord
	localRefContainers map[string]localReferenceOrigin
	arenaGenerations   map[string]int
	continues          bool
}

type loopIterationAnalysisState struct {
	symbols            map[string]Symbol
	completionSymbols  map[string]Symbol
	definitionTokens   map[sourceTokenKey][]lexer.Token
	constInts          map[string]*big.Int
	assigned           map[string]bool
	moved              map[string]lexer.Token
	moveReasons        map[string]string
	closedResources    map[string]lexer.Token
	borrows            map[string][]borrowRecord
	localRefContainers map[string]localReferenceOrigin
	arenaGenerations   map[string]int
	scopeDepth         int
}

func (a *Analyzer) captureLoopIterationAnalysisState() loopIterationAnalysisState {
	return loopIterationAnalysisState{
		symbols:            copySymbols(a.symbols),
		completionSymbols:  copySymbols(a.completionSymbols),
		definitionTokens:   copyDefinitionTokens(a.definitionTokens),
		constInts:          copyConstInts(a.constInts),
		assigned:           copyAssigned(a.assigned),
		moved:              copyMoved(a.moved),
		moveReasons:        copyMoveReasons(a.moveReasons),
		closedResources:    copyMoved(a.closedResources),
		borrows:            copyBorrows(a.borrows),
		localRefContainers: copyLocalRefContainers(a.localRefContainers),
		arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
		scopeDepth:         a.scopeDepth,
	}
}

func (a *Analyzer) analyzeBranchBlock(block *ast.BlockStatement) branchAnalysis {
	if block == nil {
		return branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: true}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
	}()

	a.analyzeBlockStatements(block)
	return branchAnalysis{
		assigned:           copyAssigned(a.assigned),
		moved:              copyMoved(a.moved),
		moveReasons:        copyMoveReasons(a.moveReasons),
		closedResources:    copyMoved(a.closedResources),
		borrows:            copyBorrows(a.borrows),
		localRefContainers: copyLocalRefContainers(a.localRefContainers),
		arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
		continues:          blockCanFallThrough(block),
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

func mergeContinuingMoveState(beforeMoved map[string]lexer.Token, beforeReasons map[string]string, branches ...branchAnalysis) (map[string]lexer.Token, map[string]string) {
	mergedMoved := map[string]lexer.Token{}
	mergedReasons := map[string]string{}
	foundContinuing := false
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		foundContinuing = true
		for place, token := range branch.moved {
			if _, exists := mergedMoved[place]; exists {
				continue
			}
			mergedMoved[place] = token
			if reason := branch.moveReasons[place]; reason != "" {
				mergedReasons[place] = reason
			}
		}
	}
	if !foundContinuing {
		return copyMoved(beforeMoved), copyMoveReasons(beforeReasons)
	}
	return mergedMoved, mergedReasons
}

func mergeContinuingClosedResources(before map[string]lexer.Token, branches ...branchAnalysis) map[string]lexer.Token {
	states := make([]map[string]lexer.Token, 0, len(branches))
	for _, branch := range branches {
		if branch.continues {
			states = append(states, branch.closedResources)
		}
	}
	if len(states) == 0 {
		return copyMoved(before)
	}
	return intersectTokenStates(states...)
}

func intersectTokenStates(states ...map[string]lexer.Token) map[string]lexer.Token {
	if len(states) == 0 {
		return map[string]lexer.Token{}
	}
	intersection := copyMoved(states[0])
	for name := range intersection {
		for _, state := range states[1:] {
			if _, exists := state[name]; !exists {
				delete(intersection, name)
				break
			}
		}
	}
	return intersection
}

func mergeContinuingBorrows(before map[string][]borrowRecord, branches ...branchAnalysis) map[string][]borrowRecord {
	merged := map[string][]borrowRecord{}
	foundContinuing := false
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		foundContinuing = true
		for root, records := range branch.borrows {
			for _, record := range records {
				if containsBorrowRecord(merged[root], record) {
					continue
				}
				merged[root] = append(merged[root], record)
			}
		}
	}
	if !foundContinuing {
		return copyBorrows(before)
	}
	return merged
}

func containsBorrowRecord(records []borrowRecord, target borrowRecord) bool {
	for _, record := range records {
		if record.Root == target.Root &&
			record.Holder == target.Holder &&
			record.Kind == target.Kind &&
			record.Token.File == target.Token.File &&
			record.Token.Line == target.Token.Line &&
			record.Token.Column == target.Token.Column &&
			samePlaceIdentity(record.Place, target.Place) {
			return true
		}
	}
	return false
}

func mergeContinuingLocalRefContainers(before map[string]localReferenceOrigin, branches ...branchAnalysis) map[string]localReferenceOrigin {
	states := make([]map[string]localReferenceOrigin, 0, len(branches))
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		states = append(states, branch.localRefContainers)
	}
	if len(states) == 0 {
		return copyLocalRefContainers(before)
	}
	return mergeReferenceOriginStates(states...)
}

func mergeContinuingArenaGenerations(before map[string]int, branches ...branchAnalysis) map[string]int {
	merged := copyArenaGenerations(before)
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		for name, generation := range branch.arenaGenerations {
			if generation > merged[name] {
				merged[name] = generation
			}
		}
	}
	return merged
}

func (a *Analyzer) analyzeSwitchStatement(stmt *ast.SwitchStatement) {
	before := copyAssigned(a.assigned)
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)
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
	if stmt.Default == nil {
		a.warnIncompleteEnumSwitch(stmt, tracker)
	}

	if stmt.Default == nil && !tracker.isExhaustive() {
		branches = append(branches, branchAnalysis{assigned: before, moved: beforeMoved, moveReasons: beforeMoveReasons, closedResources: beforeClosedResources, borrows: beforeBorrows, localRefContainers: beforeLocalRefContainers, arenaGenerations: beforeArenaGenerations, continues: true})
	}
	a.assigned = mergeContinuingAssigned(before, branches...)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, branches...)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, branches...)
	a.borrows = mergeContinuingBorrows(beforeBorrows, branches...)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, branches...)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, branches...)
}

func (a *Analyzer) analyzeSelectStatement(stmt *ast.SelectStatement) {
	beforeAssigned := copyAssigned(a.assigned)
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)

	if len(stmt.Branches) == 0 {
		a.addErrorAtToken(stmt.Token, "select requires at least one branch")
		return
	}
	if stmt.DefaultNotFinalToken.Type != "" {
		a.addErrorAtToken(stmt.DefaultNotFinalToken, "default branch must be last in select")
	}
	for _, token := range stmt.DuplicateDefaultTokens {
		a.addErrorAtToken(token, "select may contain only one default branch")
	}
	if stmt.UnreachableTimeoutToken.Type != "" {
		a.addErrorAtToken(stmt.UnreachableTimeoutToken, "timeout branch is unreachable because default executes immediately")
	}
	a.checkMutexGuardsAcrossSelect(stmt.Token)

	seenExclusive := map[string]selectResource{}
	seenMovedMessages := map[string]selectResource{}
	branches := make([]branchAnalysis, 0, len(stmt.Branches)+1)
	for _, branch := range stmt.Branches {
		if branch == nil {
			continue
		}
		if resource, ok := a.selectExclusiveResource(branch); ok {
			if previous, exists := seenExclusive[resource.Root]; exists {
				kind := resource.Kind
				if previous.Kind == kind {
					a.addErrorAtTokenWithPrevious(branch.Token, previous.Token, "%s %s is used by more than one branch in the same select", kind, resource.Root)
				} else {
					a.addErrorAtTokenWithPrevious(branch.Token, previous.Token, "resource %s is used by more than one branch in the same select as %s and %s", resource.Root, previous.Kind, kind)
				}
			} else {
				seenExclusive[resource.Root] = resource
			}
		}
		if resource, ok := a.selectMovedMessageResource(branch); ok {
			if previous, exists := seenMovedMessages[resource.Root]; exists {
				a.addErrorAtTokenWithPrevious(branch.Token, previous.Token, "message value %s is moved by multiple select branches", resource.Root)
			} else {
				seenMovedMessages[resource.Root] = resource
			}
		}
		branches = append(branches, a.analyzeSelectBranchBody(branch))
	}
	if !selectHasDefault(stmt) {
		branches = append(branches, branchAnalysis{assigned: beforeAssigned, moved: beforeMoved, moveReasons: beforeMoveReasons, closedResources: beforeClosedResources, borrows: beforeBorrows, localRefContainers: beforeLocalRefContainers, arenaGenerations: beforeArenaGenerations, continues: true})
	}
	a.assigned = mergeContinuingAssigned(beforeAssigned, branches...)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, branches...)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, branches...)
	a.borrows = mergeContinuingBorrows(beforeBorrows, branches...)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, branches...)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, branches...)
}

func (a *Analyzer) checkMutexGuardsAcrossSelect(token lexer.Token) {
	for name, symbol := range a.symbols {
		if !isMutexGuardType(symbol.Type) {
			continue
		}
		if _, moved := a.moved[name]; moved {
			continue
		}
		a.addErrorAtTokenWithPrevious(token, symbol.Token, "mutex guard %s remains active across select", name)
	}
}

func selectHasDefault(stmt *ast.SelectStatement) bool {
	for _, branch := range stmt.Branches {
		if branch != nil && branch.Kind == ast.SelectDefaultBranch {
			return true
		}
	}
	return false
}

func (a *Analyzer) selectBranchResultType(branch *ast.SelectBranch) (Type, bool) {
	switch branch.Kind {
	case ast.SelectDefaultBranch:
		return Type{Name: "void", Kind: VoidType}, true
	case ast.SelectTimeoutBranch:
		if branch.Value == nil {
			return Type{Kind: InvalidType}, false
		}
		durationType, _ := a.inferExpression(branch.Value)
		if durationType.Kind != InvalidType && !isNumericType(durationType) {
			a.addErrorAtToken(expressionToken(branch.Value), "after duration must be duration-compatible numeric value, got %s", typeDisplayName(durationType))
			return Type{Kind: InvalidType}, false
		}
		return Type{Name: "void", Kind: VoidType}, true
	case ast.SelectOperationBranch:
		return a.selectableOperationType(branch.Value)
	default:
		return Type{Kind: InvalidType}, false
	}
}

func (a *Analyzer) selectableOperationType(expr ast.Expression) (Type, bool) {
	switch expr := expr.(type) {
	case *ast.AwaitExpression:
		typ, _ := a.inferAwaitExpression(expr)
		return typ, true
	case *ast.CallExpression:
		member, ok := expr.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil {
			a.inferExpression(expr)
			return Type{Kind: InvalidType}, false
		}
		receiverType, ok := a.compilerKnownReceiverType(member.Object)
		if !ok {
			a.inferExpression(expr)
			return Type{Kind: InvalidType}, false
		}
		if isReceiverType(receiverType) {
			switch member.Property.Value {
			case "Receive", "TryReceive":
				typ, _ := a.inferExpression(expr)
				return typ, true
			}
		}
		if isSenderType(receiverType) {
			switch member.Property.Value {
			case "Send", "TrySend", "SendRevocable":
				typ, _ := a.inferExpression(expr)
				return typ, true
			}
		}
		a.inferExpression(expr)
		return Type{Kind: InvalidType}, false
	default:
		a.inferExpression(expr)
		return Type{Kind: InvalidType}, false
	}
}

type selectResource struct {
	Root  string
	Kind  string
	Token lexer.Token
}

func (a *Analyzer) selectExclusiveResource(branch *ast.SelectBranch) (selectResource, bool) {
	if branch == nil || branch.Value == nil || branch.Kind != ast.SelectOperationBranch {
		return selectResource{}, false
	}
	switch expr := branch.Value.(type) {
	case *ast.AwaitExpression:
		root, ok := borrowRootName(expr.Value)
		if !ok {
			return selectResource{}, false
		}
		return selectResource{Root: root, Kind: "task", Token: branch.Token}, true
	case *ast.CallExpression:
		member, ok := expr.Callee.(*ast.MemberExpression)
		if !ok {
			return selectResource{}, false
		}
		receiverType, ok := a.compilerKnownReceiverType(member.Object)
		if !ok {
			return selectResource{}, false
		}
		root, ok := borrowRootName(member.Object)
		if !ok {
			return selectResource{}, false
		}
		if isReceiverType(receiverType) {
			return selectResource{Root: root, Kind: "receiver", Token: branch.Token}, true
		}
		if isSenderType(receiverType) {
			return selectResource{Root: root, Kind: "sender", Token: branch.Token}, true
		}
	}
	return selectResource{}, false
}

func (a *Analyzer) selectMovedMessageResource(branch *ast.SelectBranch) (selectResource, bool) {
	if branch == nil || branch.Value == nil || branch.Kind != ast.SelectOperationBranch {
		return selectResource{}, false
	}
	call, ok := branch.Value.(*ast.CallExpression)
	if !ok || len(call.Arguments) == 0 {
		return selectResource{}, false
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return selectResource{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok || !isSenderType(receiverType) {
		return selectResource{}, false
	}
	switch member.Property.Value {
	case "Send", "TrySend", "SendRevocable":
	default:
		return selectResource{}, false
	}
	root, ok := borrowRootName(call.Arguments[0])
	if !ok {
		return selectResource{}, false
	}
	symbol, ok := a.symbols[root]
	if !ok || !requiresOwnershipTransfer(symbol.Type) {
		return selectResource{}, false
	}
	return selectResource{Root: root, Kind: "message", Token: branch.Token}, true
}

func (a *Analyzer) analyzeSelectBranchBody(branch *ast.SelectBranch) branchAnalysis {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
	}()

	resultType, selectable := a.selectBranchResultType(branch)
	if branch.Kind != ast.SelectDefaultBranch && !selectable {
		a.addErrorAtToken(branch.Token, "operation is not selectable")
	}
	if branch.Binding != nil && resultType.Kind != InvalidType && resultType.Kind != VoidType {
		if a.defineSymbol(branch.Binding.Value, resultType, false, branch.Binding.Token) {
			a.assigned[branch.Binding.Value] = true
		}
	} else if branch.Binding != nil && resultType.Kind == VoidType {
		a.addErrorAtToken(branch.Binding.Token, "select branch binding requires non-void operation result")
	}
	a.analyzeBlockStatements(branch.Body)
	return branchAnalysis{
		assigned:           copyAssigned(a.assigned),
		moved:              copyMoved(a.moved),
		moveReasons:        copyMoveReasons(a.moveReasons),
		closedResources:    copyMoved(a.closedResources),
		borrows:            copyBorrows(a.borrows),
		localRefContainers: copyLocalRefContainers(a.localRefContainers),
		arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
		continues:          blockCanFallThrough(branch.Body),
	}
}

func (a *Analyzer) analyzeSwitchCaseItems(clause *ast.SwitchCase, hasSubject bool, subjectType Type, tracker *switchCoverageTracker) {
	if clause.Default {
		return
	}

	for _, item := range clause.Items {
		switch item := item.(type) {
		case *ast.SwitchValueCase:
			valueType, _ := a.inferExpressionWithExpected(item.Value, subjectType)
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
			valueType, _ := a.inferExpressionWithExpected(item.Value, subjectType)
			if valueType.Kind != InvalidType && !canCompareEquality(subjectType, valueType) {
				a.addErrorAtToken(expressionToken(item.Value), "switch case must be compatible with subject type %s, got %s", typeDisplayName(subjectType), typeDisplayName(valueType))
			}
			a.checkSwitchRelationalCoverage(item, tracker)
		}
	}
}

type switchCoverageTracker struct {
	subjectType  Type
	values       map[string]lexer.Token
	ranges       []switchConstRange
	boolValues   map[bool]lexer.Token
	stringValues map[string]lexer.Token
	enumValues   map[string]lexer.Token
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
		values:       map[string]lexer.Token{},
		boolValues:   map[bool]lexer.Token{},
		stringValues: map[string]lexer.Token{},
		enumValues:   map[string]lexer.Token{},
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
	if literal, ok := expr.(*ast.StringLiteral); ok && tracker.subjectType.Kind == StringType {
		if _, exists := tracker.stringValues[literal.Value]; exists {
			a.addErrorAtTokenWithMetadata(
				expressionToken(expr),
				diagnostics.DuplicateSwitchCase,
				"Remove the duplicate case or combine its body with the first case for this string value.",
				"duplicate switch case value %q",
				literal.Value,
			)
			return
		}
		tracker.stringValues[literal.Value] = expressionToken(expr)
		return
	}
	if variant, ok := a.switchEnumCaseVariant(expr, tracker.subjectType); ok {
		tracker.enumValues[variant] = expressionToken(expr)
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

func (a *Analyzer) switchEnumCaseVariant(expr ast.Expression, subjectType Type) (string, bool) {
	if subjectType.Kind != EnumType {
		return "", false
	}
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return "", false
	}
	typeName, ok := typePathFromExpression(member.Object)
	if !ok {
		return "", false
	}
	typeName = a.resolveTypeName(typeName)
	typ, ok := a.types[typeName]
	if !ok || typ.Kind != EnumType || !sameConcreteType(subjectType, typ) {
		return "", false
	}
	if _, ok := typ.EnumConsts[member.Property.Value]; !ok {
		return "", false
	}
	return member.Property.Value, true
}

func (a *Analyzer) warnIncompleteEnumSwitch(stmt *ast.SwitchStatement, tracker *switchCoverageTracker) {
	if stmt == nil || tracker == nil || tracker.subjectType.Kind != EnumType {
		return
	}
	values, ok := a.enumValuesForType(tracker.subjectType)
	if !ok {
		return
	}
	missing := make([]string, 0, len(values))
	for _, value := range values {
		if _, covered := tracker.enumValues[value]; !covered {
			missing = append(missing, value)
		}
	}
	if len(missing) == 0 {
		return
	}
	a.addWarningAtTokenWithMetadata(
		expressionToken(stmt.Subject),
		diagnostics.IncompleteEnumSwitch,
		"Handle the missing values, add default, or use match when exhaustive variant handling is required.",
		"switch over %s omits known values: %s",
		typeDisplayName(tracker.subjectType),
		strings.Join(missing, ", "),
	)
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
		return branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: true}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousInSwitchCaseBody := a.inSwitchCaseBody
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.inSwitchCaseBody = true
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.inSwitchCaseBody = previousInSwitchCaseBody
	}()

	hasFallthrough := blockEndsWithFallthrough(block)
	a.analyzeBlockStatements(block)
	return branchAnalysis{
		assigned:           copyAssigned(a.assigned),
		moved:              copyMoved(a.moved),
		moveReasons:        copyMoveReasons(a.moveReasons),
		closedResources:    copyMoved(a.closedResources),
		borrows:            copyBorrows(a.borrows),
		localRefContainers: copyLocalRefContainers(a.localRefContainers),
		arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
		continues:          blockCanFallThrough(block) && !hasFallthrough,
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
	if isCompilerKnownFunctionName(name) {
		a.addErrorAtToken(fn.Name.Token, "function %s is compiler-known and cannot be declared", name)
		return
	}
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

func (a *Analyzer) registerCompilerKnownFunctions() {
	a.functions["len"] = []Function{{
		Name:       "len",
		Module:     "core",
		ReturnType: a.types["int"],
	}}
}

func isCompilerKnownFunctionName(name string) bool {
	return name == "len"
}

func (a *Analyzer) registerFunctionDeclarationBody(fn *ast.FunctionDeclaration, name string) {
	a.recordDefinition(fn.Name.Token)
	function := Function{
		Name:              name,
		Module:            a.currentModule,
		GenericParameters: genericParameterNameValues(fn.GenericParameters),
		Token:             fn.Name.Token,
		Extern:            fn.Extern,
		ABI:               fn.ABI,
		LinkName:          fn.LinkName,
		AllocationEffect:  AllocationEffectNone,
	}
	if a.currentImplTarget != "" {
		function.ImplTarget = a.currentImplTarget
		if target, ok := a.types[a.currentImplTarget]; ok {
			function.ReceiverMutable = a.functionBodyWritesTargetMember(fn.Body, target, functionParameterNames(fn))
		}
	}
	if fn.Extern && !isSupportedExternABI(fn.ABI) {
		a.addErrorAtToken(fn.Token, "unknown extern ABI %q", fn.ABI)
	}

	seenParams := map[string]lexer.Token{}
	a.rejectExplicitImplSelfParameter(fn)
	for _, param := range fn.Parameters {
		if a.currentImplTarget != "" && param.Name != nil && param.Name.Value == "self" {
			continue
		}
		if _, exists := seenParams[param.Name.Value]; exists {
			a.addErrorAtToken(param.Name.Token, "duplicate parameter %q", param.Name.Value)
			continue
		}
		seenParams[param.Name.Value] = param.Name.Token
		a.recordDefinition(param.Name.Token)

		paramType, ok := a.resolveType(param.Type)
		if !ok {
			continue
		}
		if isBareSliceType(paramType) {
			a.addErrorAtToken(param.Type.Token, "bare slice type %s must be used behind ref", typeDisplayName(paramType))
			continue
		}
		a.warnLargeByValueParameter(param.Name.Value, paramType, param.Ref, param.MutableRef, param.Name.Token)
		function.Parameters = append(function.Parameters, FunctionParameter{
			Name:       param.Name.Value,
			Type:       paramType,
			Token:      param.Name.Token,
			Ref:        param.Ref,
			MutableRef: param.MutableRef,
		})
	}

	returnType, ok := a.resolveType(fn.ReturnType)
	if ok {
		if isBareSliceType(returnType) {
			a.addErrorAtToken(fn.ReturnType.Token, "bare slice type %s must be used behind ref", typeDisplayName(returnType))
			returnType = Type{Kind: InvalidType}
		}
		function.ReturnType = returnType
	} else {
		function.ReturnType = Type{Kind: InvalidType}
	}

	for _, existing := range a.functions[name] {
		if sameFunctionSignature(existing, function) {
			a.addErrorAtTokenWithPrevious(fn.Name.Token, existing.Token, "duplicate function %q with same signature", name)
			return
		}
	}
	if function.Extern && function.LinkName != "" {
		foreignName := function.LinkName
		if previous, exists := a.externSymbols[foreignName]; exists {
			a.addErrorAtTokenWithPrevious(fn.Name.Token, previous.Token, "duplicate extern symbol %q", foreignName)
			return
		}
		a.externSymbols[foreignName] = function
	}

	a.functions[name] = append(a.functions[name], function)
	nodeID := a.callGraph.addCallable(function)
	if isProgramEntryFunction(function) {
		a.callGraph.addRoot(CallRootProgramEntry, nodeID, function.Token)
	}
}

func isProgramEntryFunction(function Function) bool {
	if function.Module != "main" || function.Name != "main" || function.Extern || len(function.Parameters) != 0 {
		return false
	}
	return function.ReturnType.Kind == VoidType || (function.ReturnType.Kind == IntType && function.ReturnType.Name == "int")
}

func sameFunctionSignature(left Function, right Function) bool {
	if len(left.Parameters) != len(right.Parameters) {
		return false
	}
	for i := range left.Parameters {
		if left.Parameters[i].Ref != right.Parameters[i].Ref || left.Parameters[i].MutableRef != right.Parameters[i].MutableRef {
			return false
		}
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

const largeByValueParameterThresholdBytes int64 = 64

func (a *Analyzer) warnLargeByValueParameter(name string, typ Type, ref bool, mutableRef bool, token lexer.Token) {
	if ref || mutableRef || typ.Kind == ReferenceType || typ.Kind == SliceType || typ.Kind == VoidType || typ.Kind == InvalidType {
		return
	}
	size, ok := estimatedTypeSizeBytes(typ, map[string]bool{})
	if !ok || size < largeByValueParameterThresholdBytes {
		return
	}
	if typ.Kind == ArrayType {
		a.addWarningAtTokenWithMetadata(token, diagnostics.LargeValueParameter, "Pass the parameter by shared reference when the function does not need to own or copy the whole value.", "parameter %q passes large array %s by value; consider ref %s or ref %s[]", name, typeDisplayName(typ), typeDisplayName(typ), arrayElementDisplayName(typ))
		return
	}
	a.addWarningAtTokenWithMetadata(token, diagnostics.LargeValueParameter, "Pass the parameter by shared reference when the function does not need to own or copy the whole value.", "parameter %q passes large value %s by value; consider ref %s", name, typeDisplayName(typ), typeDisplayName(typ))
}

func estimatedTypeSizeBytes(typ Type, visiting map[string]bool) (int64, bool) {
	switch typ.Kind {
	case BoolType, CharType:
		return 1, true
	case RuneType:
		return 4, true
	case IntType, UintType, FloatType:
		return numericTypeSizeBytes(typ), true
	case DecimalType:
		if typ.Name == "decimal128" {
			return 16, true
		}
		return 16, true
	case EnumType, RegisterType:
		if typ.BitWidth > 0 {
			return maxInt64(1, (typ.BitWidth+7)/8), true
		}
		if typ.RegisterWidth > 0 {
			return maxInt64(1, (typ.RegisterWidth+7)/8), true
		}
		return 4, true
	case StringType, ReferenceType, RawPtrType, SliceType, FunctionType:
		return 16, true
	case ArrayType:
		if typ.Element == nil || typ.ArrayLength < 0 {
			return 0, false
		}
		elementSize, ok := estimatedTypeSizeBytes(*typ.Element, visiting)
		if !ok {
			return 0, false
		}
		return elementSize * typ.ArrayLength, true
	case StructType:
		key := typeDisplayName(typ)
		if visiting[key] {
			return 0, false
		}
		visiting[key] = true
		var total int64
		for _, field := range typ.Fields {
			fieldSize, ok := estimatedTypeSizeBytes(field.Type, visiting)
			if !ok {
				delete(visiting, key)
				return 0, false
			}
			total += fieldSize
		}
		delete(visiting, key)
		return total, true
	case ResultType, UnionType:
		var maxPayload int64
		for _, arg := range typ.TypeArgs {
			size, ok := estimatedTypeSizeBytes(arg, visiting)
			if ok && size > maxPayload {
				maxPayload = size
			}
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				size, ok := estimatedTypeSizeBytes(*variant.Payload, visiting)
				if ok && size > maxPayload {
					maxPayload = size
				}
			}
			var fields int64
			fieldsOK := len(variant.PayloadFields) > 0
			for _, field := range variant.PayloadFields {
				size, ok := estimatedTypeSizeBytes(field.Type, visiting)
				if !ok {
					fieldsOK = false
					break
				}
				fields += size
			}
			if fieldsOK && fields > maxPayload {
				maxPayload = fields
			}
		}
		return 8 + maxPayload, true
	default:
		return 0, false
	}
}

func numericTypeSizeBytes(typ Type) int64 {
	switch typ.Name {
	case "int8", "uint8", "byte":
		return 1
	case "int16", "uint16":
		return 2
	case "int32", "uint32", "float32":
		return 4
	case "int128", "uint128":
		return 16
	case "int256", "uint256":
		return 32
	default:
		return 8
	}
}

func arrayElementDisplayName(typ Type) string {
	if typ.Kind == ArrayType && typ.Element != nil {
		return typeDisplayName(*typ.Element)
	}
	return typeDisplayName(typ)
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
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

func (a *Analyzer) inferFunctionReferenceSummaries(program *ast.Program) {
	functionCount := 0
	for _, overloads := range a.functions {
		functionCount += len(overloads)
	}
	if functionCount == 0 {
		return
	}
	a.summaryPass = true
	defer func() { a.summaryPass = false }()
	iterationLimit := functionCount + 1
	if configured := a.analysisBudget.MaxSummaryIterations; configured > 0 && configured < iterationLimit {
		iterationLimit = configured
	}
	converged := false
	for iteration := 0; iteration < iterationLimit; iteration++ {
		before := copyFunctionReferenceSummaries(a.functions)
		a.analyzeFunctionBodies(program)
		a.analyzeImplBodies(program)
		if functionReferenceSummariesEqual(before, a.functions) {
			converged = true
			break
		}
	}
	if !converged {
		a.widenFunctionReferenceSummaries()
	}
}

// widenFunctionReferenceSummaries preserves soundness when an interactive
// resource limit is reached: unresolved reference-returning calls become
// unknown rather than retaining a potentially incomplete proof.
func (a *Analyzer) widenFunctionReferenceSummaries() {
	for name, overloads := range a.functions {
		for index := range overloads {
			if !typeContainsReference(overloads[index].ReturnType, map[string]bool{}) {
				continue
			}
			overloads[index].HasReturnOrigin = true
			overloads[index].ReturnOrigin = localReferenceOrigin{Unknown: true, Ambiguous: true}
		}
		a.functions[name] = overloads
	}
}

func copyFunctionReferenceSummaries(functions map[string][]Function) map[string][]Function {
	out := make(map[string][]Function, len(functions))
	for name, overloads := range functions {
		out[name] = make([]Function, len(overloads))
		for index, function := range overloads {
			function.ReturnOrigin = cloneLocalReferenceOrigin(function.ReturnOrigin)
			out[name][index] = function
		}
	}
	return out
}

func functionReferenceSummariesEqual(left, right map[string][]Function) bool {
	if len(left) != len(right) {
		return false
	}
	for name, leftOverloads := range left {
		rightOverloads, ok := right[name]
		if !ok || len(leftOverloads) != len(rightOverloads) {
			return false
		}
		for index, leftFunction := range leftOverloads {
			rightFunction := rightOverloads[index]
			if leftFunction.HasReturnOrigin != rightFunction.HasReturnOrigin {
				return false
			}
			if leftFunction.HasReturnOrigin && !sameReferenceOrigin(leftFunction.ReturnOrigin, rightFunction.ReturnOrigin) {
				return false
			}
		}
	}
	return true
}

func (a *Analyzer) analyzeFunctionBody(fn *ast.FunctionDeclaration) {
	a.analyzeFunctionBodyNamed(fn, fn.Name.Value)
}

func (a *Analyzer) analyzeFunctionBodyNamed(fn *ast.FunctionDeclaration, name string) {
	if len(fn.GenericParameters) > 0 {
		a.withGenericTypeParameters(fn.GenericParameters, func() {
			a.analyzeFunctionBodyInScope(fn, name)
		})
		return
	}
	a.analyzeFunctionBodyInScope(fn, name)
}

func (a *Analyzer) analyzeFunctionBodyInScope(fn *ast.FunctionDeclaration, name string) {
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	if a.summaryPass && !typeContainsReference(function.ReturnType, map[string]bool{}) {
		return
	}
	if function.Extern {
		a.validateExternFunction(function)
		if fn.Body == nil {
			return
		}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousFunctionName := a.currentFunctionName
	previousCallable := a.currentCallable
	previousFunctionReturn := a.currentFunctionReturn
	previousFunctionToken := a.currentFunctionToken
	previousFunctionMetadata := a.currentFunctionMetadata
	previousFunctionSummary := a.currentFunctionSummary
	previousHasFunctionSummary := a.hasCurrentFunctionSummary
	previousInFunctionBody := a.inFunctionBody
	previousUnsafe := a.inUnsafe
	previousScopeDepth := a.scopeDepth
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = map[string]lexer.Token{}
	a.moveReasons = map[string]string{}
	a.closedResources = map[string]lexer.Token{}
	a.borrows = map[string][]borrowRecord{}
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.scopeDepth = 0
	a.currentFunctionName = fn.Name.Value
	a.currentCallable = a.callGraph.addCallable(function)
	a.currentFunctionReturn = function.ReturnType
	a.currentFunctionToken = function.Token
	a.currentFunctionMetadata = function
	a.currentFunctionSummary = localReferenceOrigin{}
	a.hasCurrentFunctionSummary = false
	a.inFunctionBody = true
	if fn.Unsafe {
		a.inUnsafe = true
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.borrows = previousBorrows
		a.closedResources = previousClosedResources
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.currentFunctionName = previousFunctionName
		a.currentCallable = previousCallable
		a.currentFunctionReturn = previousFunctionReturn
		a.currentFunctionToken = previousFunctionToken
		a.currentFunctionMetadata = previousFunctionMetadata
		a.currentFunctionSummary = previousFunctionSummary
		a.hasCurrentFunctionSummary = previousHasFunctionSummary
		a.inFunctionBody = previousInFunctionBody
		a.inUnsafe = previousUnsafe
		a.scopeDepth = previousScopeDepth
	}()

	for _, param := range function.Parameters {
		symbol := Symbol{Name: param.Name, Type: param.Type, Mutable: param.MutableRef, Token: param.Token, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
		a.symbols[param.Name] = symbol
		completionSymbol := symbol
		completionSymbol.Local = true
		a.completionSymbols[param.Name] = completionSymbol
		delete(a.constInts, param.Name)
		a.assigned[param.Name] = true
		a.recordDefinition(param.Token)
		a.recordBinding(param.Token, BindingParameter, param.Name, param.Type, param.MutableRef)
		a.seedParameterReferenceOrigin(function, param)
	}
	a.defineImplicitImplInstanceSymbols(fn.Body, function.ReceiverMutable)

	a.analyzeBlockStatements(fn.Body)
	a.setFunctionReferenceSummary(function)

	if !a.blockDefinitelyReturns(fn.Body) && function.ReturnType.Kind != VoidType && function.ReturnType.Kind != NeverType {
		a.addErrorAtToken(fn.Name.Token, "function %s must return %s", fn.Name.Value, typeDisplayName(function.ReturnType))
	}
}

func (a *Analyzer) rejectExplicitImplSelfParameter(fn *ast.FunctionDeclaration) {
	if a.currentImplTarget == "" || fn == nil {
		return
	}
	for _, param := range fn.Parameters {
		if param.Name != nil && param.Name.Value == "self" {
			a.addErrorAtToken(param.Name.Token, "impl methods have implicit self; remove self from the parameter list")
		}
	}
}

func (a *Analyzer) defineImplicitImplSelfSymbol(block *ast.BlockStatement, mutableSelf bool) {
	if a.currentImplTarget == "" {
		return
	}
	target, ok := a.types[a.currentImplTarget]
	if !ok || target.Kind == InvalidType {
		return
	}
	a.defineInstanceSymbols(target, mutableSelf, lexer.Token{})
}

func (a *Analyzer) defineImplicitImplInstanceSymbols(block *ast.BlockStatement, mutableSelf bool) {
	a.defineImplicitImplSelfSymbol(block, mutableSelf)
}

func (a *Analyzer) defineInstanceSymbols(target Type, mutableSelf bool, selfToken lexer.Token) {
	a.symbols["self"] = Symbol{Name: "self", Type: target, Mutable: mutableSelf, Token: selfToken, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
	a.assigned["self"] = true
	delete(a.constInts, "self")
	for _, field := range target.Fields {
		if _, exists := a.symbols[field.Name]; exists {
			continue
		}
		a.symbols[field.Name] = Symbol{Name: field.Name, Type: field.Type, Mutable: mutableSelf, ImplicitMember: true, Token: field.Token, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
		a.assigned[field.Name] = true
		delete(a.constInts, field.Name)
	}
	for _, field := range target.RegisterFields {
		if field.Name == "_" {
			continue
		}
		if _, exists := a.symbols[field.Name]; exists {
			continue
		}
		a.symbols[field.Name] = Symbol{Name: field.Name, Type: field.Type, Mutable: mutableSelf, ImplicitMember: true, Token: field.Token, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
		a.assigned[field.Name] = true
		delete(a.constInts, field.Name)
	}
	for _, property := range target.Properties {
		if _, exists := a.symbols[property.Name]; exists {
			continue
		}
		a.symbols[property.Name] = Symbol{Name: property.Name, Type: property.Type, Mutable: mutableSelf && property.HasSetter, ImplicitMember: true, Token: property.Token, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
		a.assigned[property.Name] = true
		delete(a.constInts, property.Name)
	}
	for _, event := range target.Events {
		if _, exists := a.symbols[event.Name]; exists {
			continue
		}
		a.symbols[event.Name] = Symbol{Name: event.Name, Type: event.Type, Mutable: false, ImplicitMember: true, Token: event.Token, Storage: StorageOriginInline, Local: false, ScopeDepth: 0}
		a.assigned[event.Name] = true
		delete(a.constInts, event.Name)
	}
}

func functionBodyUsesSelf(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if statementUsesSelf(stmt) {
			return true
		}
	}
	return false
}

func functionBodyWritesSelf(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if statementWritesSelf(stmt) {
			return true
		}
	}
	return false
}

func functionParameterNames(fn *ast.FunctionDeclaration) map[string]bool {
	names := map[string]bool{}
	if fn == nil {
		return names
	}
	for _, param := range fn.Parameters {
		if param != nil && param.Name != nil {
			names[param.Name.Value] = true
		}
	}
	return names
}

func (a *Analyzer) functionBodyWritesTargetMember(block *ast.BlockStatement, target Type, shadowed map[string]bool) bool {
	if block == nil {
		return false
	}
	return a.blockWritesTargetMember(block, target, targetAssignableMemberNames(target), shadowed)
}

func targetAssignableMemberNames(target Type) map[string]bool {
	names := map[string]bool{}
	for _, field := range target.Fields {
		names[field.Name] = true
	}
	for _, field := range target.RegisterFields {
		if field.Name != "_" {
			names[field.Name] = true
		}
	}
	for _, property := range target.Properties {
		if property.HasSetter {
			names[property.Name] = true
		}
	}
	return names
}

func statementUsesSelf(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		return expressionUsesSelf(stmt.Value) || expressionUsesSelf(stmt.Address)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			if statementUsesSelf(let) {
				return true
			}
		}
	case *ast.AssignmentStatement:
		return expressionUsesSelf(stmt.Target) || expressionUsesSelf(stmt.Value)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && statementUsesSelf(stmt.Assignment)
	case *ast.ExpressionStatement:
		return expressionUsesSelf(stmt.Expression)
	case *ast.DetachStatement:
		return expressionUsesSelf(stmt.Value)
	case *ast.ReturnStatement:
		return expressionUsesSelf(stmt.Value)
	case *ast.IfStatement:
		return expressionUsesSelf(stmt.Condition) || functionBodyUsesSelf(stmt.Consequence) || functionBodyUsesSelf(stmt.Alternative)
	case *ast.ForStatement:
		return expressionUsesSelf(stmt.Iterable) || expressionUsesSelf(stmt.Step) || functionBodyUsesSelf(stmt.Body)
	case *ast.WhileStatement:
		return expressionUsesSelf(stmt.Condition) || functionBodyUsesSelf(stmt.Body)
	case *ast.DeferStatement:
		return functionBodyUsesSelf(stmt.Body)
	case *ast.UnsafeStatement:
		return functionBodyUsesSelf(stmt.Body)
	case *ast.MatchStatement:
		return expressionUsesSelf(stmt.Match)
	}
	return false
}

func statementWritesSelf(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.AssignmentStatement:
		return assignmentTargetUsesSelf(stmt.Target)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && statementWritesSelf(stmt.Assignment)
	case *ast.LetGroupStatement:
		return false
	case *ast.IfStatement:
		return functionBodyWritesSelf(stmt.Consequence) || functionBodyWritesSelf(stmt.Alternative)
	case *ast.ForStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.WhileStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.DeferStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.UnsafeStatement:
		return functionBodyWritesSelf(stmt.Body)
	case *ast.MatchStatement:
		if stmt.Match == nil {
			return false
		}
		for _, arm := range stmt.Match.Arms {
			if arm != nil && functionBodyWritesSelf(arm.BlockBody) {
				return true
			}
		}
	}
	return false
}

func (a *Analyzer) blockWritesTargetMember(block *ast.BlockStatement, target Type, memberNames map[string]bool, inheritedShadowed map[string]bool) bool {
	if block == nil {
		return false
	}
	shadowed := copyBoolMap(inheritedShadowed)
	for _, stmt := range block.Statements {
		if a.statementWritesTargetMember(stmt, target, memberNames, shadowed) {
			return true
		}
		addStatementDeclarationsToShadowed(stmt, shadowed)
	}
	return false
}

func (a *Analyzer) statementWritesTargetMember(stmt ast.Statement, target Type, memberNames map[string]bool, shadowed map[string]bool) bool {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Value, target)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			if let != nil && a.expressionCallsMutableTargetMethod(let.Value, target) {
				return true
			}
		}
		return false
	case *ast.AssignmentStatement:
		return assignmentTargetUsesSelf(stmt.Target) || assignmentTargetUsesUnshadowedMember(stmt.Target, memberNames, shadowed) || a.expressionCallsMutableTargetMethod(stmt.Value, target)
	case *ast.TryAssignmentStatement:
		return stmt.Assignment != nil && a.statementWritesTargetMember(stmt.Assignment, target, memberNames, shadowed)
	case *ast.ExpressionStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Expression, target)
	case *ast.DiscardStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Value, target)
	case *ast.ReturnStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Value, target)
	case *ast.IfStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Condition, target) || a.blockWritesTargetMember(stmt.Consequence, target, memberNames, shadowed) || a.blockWritesTargetMember(stmt.Alternative, target, memberNames, shadowed)
	case *ast.ForStatement:
		loopShadowed := copyBoolMap(shadowed)
		for _, binding := range stmt.Bindings {
			if !binding.Discard {
				loopShadowed[binding.Name] = true
			}
		}
		return a.expressionCallsMutableTargetMethod(stmt.Iterable, target) || a.expressionCallsMutableTargetMethod(stmt.Step, target) || a.blockWritesTargetMember(stmt.Body, target, memberNames, loopShadowed)
	case *ast.WhileStatement:
		return a.expressionCallsMutableTargetMethod(stmt.Condition, target) || a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.DeferStatement:
		return a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.UnsafeStatement:
		return a.blockWritesTargetMember(stmt.Body, target, memberNames, shadowed)
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			if a.expressionCallsMutableTargetMethod(branch.Value, target) {
				return true
			}
			branchShadowed := copyBoolMap(shadowed)
			if branch.Binding != nil {
				branchShadowed[branch.Binding.Value] = true
			}
			if a.blockWritesTargetMember(branch.Body, target, memberNames, branchShadowed) {
				return true
			}
		}
	}
	return false
}

// expressionCallsMutableTargetMethod reports calls that require this impl
// method's implicit self receiver to be writable. Every evaluated expression
// retains that requirement, including returned and discarded call results.
func (a *Analyzer) expressionCallsMutableTargetMethod(expr ast.Expression, target Type) bool {
	switch expr := expr.(type) {
	case nil, *ast.Identifier, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.BooleanLiteral, *ast.CharLiteral, *ast.StringLiteral:
		return false
	case *ast.PrefixExpression:
		return a.expressionCallsMutableTargetMethod(expr.Right, target)
	case *ast.InfixExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target) || a.expressionCallsMutableTargetMethod(expr.Right, target)
	case *ast.RangeExpression:
		return a.expressionCallsMutableTargetMethod(expr.Start, target) || a.expressionCallsMutableTargetMethod(expr.End, target)
	case *ast.ConversionExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target)
	case *ast.OkExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target)
	case *ast.ErrExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target)
	case *ast.TryExpression:
		return a.expressionCallsMutableTargetMethod(expr.Expression, target)
	case *ast.MemberExpression:
		return a.expressionCallsMutableTargetMethod(expr.Object, target)
	case *ast.IndexExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target) || a.expressionCallsMutableTargetMethod(expr.Index, target)
	case *ast.SliceExpression:
		return a.expressionCallsMutableTargetMethod(expr.Left, target) || a.expressionCallsMutableTargetMethod(expr.Start, target) || a.expressionCallsMutableTargetMethod(expr.End, target)
	case *ast.RefExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target)
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if a.expressionCallsMutableTargetMethod(element, target) {
				return true
			}
		}
	case *ast.SpreadExpression:
		return a.expressionCallsMutableTargetMethod(expr.Value, target)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil && a.expressionCallsMutableTargetMethod(field.Value, target) {
				return true
			}
		}
	case *ast.CallExpression:
		if a.callRequiresMutableTargetReceiver(expr, target) {
			return true
		}
		if a.expressionCallsMutableTargetMethod(expr.Callee, target) {
			return true
		}
		for _, argument := range expr.Arguments {
			if a.expressionCallsMutableTargetMethod(argument, target) {
				return true
			}
		}
	}
	return false
}

func (a *Analyzer) callRequiresMutableTargetReceiver(call *ast.CallExpression, target Type) bool {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return false
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok || receiver.Value != "self" {
		return false
	}
	for _, function := range a.functions[target.Name+"."+member.Property.Value] {
		if function.ImplTarget == target.Name && function.ReceiverMutable {
			return true
		}
	}
	return false
}

func addStatementDeclarationsToShadowed(stmt ast.Statement, shadowed map[string]bool) {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		if stmt != nil && stmt.Name != nil {
			shadowed[stmt.Name.Value] = true
		}
	case *ast.LetGroupStatement:
		if stmt == nil {
			return
		}
		for _, let := range stmt.Lets {
			if let != nil && let.Name != nil {
				shadowed[let.Name.Value] = true
			}
		}
	}
}

func assignmentTargetUsesSelf(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		return expressionUsesSelf(expr.Object)
	case *ast.IndexExpression:
		return assignmentTargetUsesSelf(expr.Left)
	}
	return false
}

func assignmentTargetUsesUnshadowedMember(expr ast.Expression, memberNames map[string]bool, shadowed map[string]bool) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return memberNames[expr.Value] && !shadowed[expr.Value]
	case *ast.MemberExpression:
		return expressionUsesSelf(expr.Object)
	case *ast.IndexExpression:
		return assignmentTargetUsesUnshadowedMember(expr.Left, memberNames, shadowed)
	}
	return false
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for name, value := range in {
		out[name] = value
	}
	return out
}

func expressionUsesSelf(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case nil:
		return false
	case *ast.Identifier:
		return expr.Value == "self"
	case *ast.PrefixExpression:
		return expressionUsesSelf(expr.Right)
	case *ast.InfixExpression:
		return expressionUsesSelf(expr.Left) || expressionUsesSelf(expr.Right)
	case *ast.RangeExpression:
		return expressionUsesSelf(expr.Start) || expressionUsesSelf(expr.End)
	case *ast.ConversionExpression:
		return expressionUsesSelf(expr.Value)
	case *ast.CallExpression:
		if expressionUsesSelf(expr.Callee) {
			return true
		}
		for _, arg := range expr.Arguments {
			if expressionUsesSelf(arg) {
				return true
			}
		}
	case *ast.OkExpression:
		return expressionUsesSelf(expr.Value)
	case *ast.ErrExpression:
		return expressionUsesSelf(expr.Value)
	case *ast.TryExpression:
		return expressionUsesSelf(expr.Expression)
	case *ast.MatchExpression:
		if expressionUsesSelf(expr.Subject) {
			return true
		}
		for _, arm := range expr.Arms {
			if arm == nil {
				continue
			}
			if expressionUsesSelf(arm.Pattern) || expressionUsesSelf(arm.Guard) || expressionUsesSelf(arm.Body) {
				return true
			}
			if arm.ReturnBody != nil && statementUsesSelf(arm.ReturnBody) {
				return true
			}
			if functionBodyUsesSelf(arm.BlockBody) {
				return true
			}
		}
	case *ast.MemberExpression:
		return expressionUsesSelf(expr.Object)
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if expressionUsesSelf(element) {
				return true
			}
		}
	case *ast.SpreadExpression:
		return expressionUsesSelf(expr.Value)
	case *ast.IndexExpression:
		return expressionUsesSelf(expr.Left) || expressionUsesSelf(expr.Index)
	case *ast.SliceExpression:
		return expressionUsesSelf(expr.Left) || expressionUsesSelf(expr.Start) || expressionUsesSelf(expr.End)
	case *ast.RefExpression:
		return expressionUsesSelf(expr.Value)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil && expressionUsesSelf(field.Value) {
				return true
			}
		}
	case *ast.SpawnExpression:
		return expressionUsesSelf(expr.Value) || functionBodyUsesSelf(expr.Body)
	case *ast.AwaitExpression:
		return expressionUsesSelf(expr.Value)
	}
	return false
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
	case *ast.SelectStatement:
		return selectDefinitelyReturns(stmt)
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

	subjectType, ok := a.expressionTypes[stmt.Match.Subject]
	if !ok || subjectType.Kind == InvalidType {
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
	case *ast.CallExpression:
		if subjectType.Kind != UnionType {
			return matchPatternInfo{}, false
		}
		variantName := ""
		switch callee := pattern.Callee.(type) {
		case *ast.Identifier:
			variantName = callee.Value
		case *ast.MemberExpression:
			variantName = callee.Property.Value
		}
		variant, ok := lookupUnionVariant(subjectType, variantName)
		if !ok || len(pattern.Arguments) != 1 || variant.Payload == nil && len(variant.PayloadFields) == 0 {
			return matchPatternInfo{}, false
		}
		return matchPatternInfo{Kind: "variant", Variant: variant.Name}, true
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
	case *ast.CancelStatement:
		return true
	default:
		return a.statementDefinitelyReturns(stmt)
	}
}

func blockCanFallThrough(block *ast.BlockStatement) bool {
	if block == nil {
		return true
	}
	for _, stmt := range block.Statements {
		if !statementCanFallThrough(stmt) {
			return false
		}
	}
	return true
}

func statementCanFallThrough(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStatement, *ast.BreakStatement, *ast.ContinueStatement, *ast.CancelStatement, *ast.FallthroughStatement:
		return false
	case *ast.IfStatement:
		if isBoolLiteral(stmt.Condition, true) {
			return blockCanFallThrough(stmt.Consequence)
		}
		if isBoolLiteral(stmt.Condition, false) {
			return blockCanFallThrough(stmt.Alternative)
		}
		if stmt.Alternative == nil {
			return true
		}
		return blockCanFallThrough(stmt.Consequence) || blockCanFallThrough(stmt.Alternative)
	case *ast.SwitchStatement:
		if stmt.Default == nil {
			return true
		}
		for _, clause := range stmt.Cases {
			if clause != nil && blockCanFallThrough(clause.Body) {
				return true
			}
		}
		return blockCanFallThrough(stmt.Default.Body)
	case *ast.SelectStatement:
		if len(stmt.Branches) == 0 {
			return true
		}
		for _, branch := range stmt.Branches {
			if branch != nil && blockCanFallThrough(branch.Body) {
				return true
			}
		}
		return false
	case *ast.UnsafeStatement:
		return blockCanFallThrough(stmt.Body)
	default:
		return !statementDefinitelyReturns(stmt)
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
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch != nil && blockContainsBreak(branch.Body) {
				return true
			}
		}
		return false
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

func selectDefinitelyReturns(stmt *ast.SelectStatement) bool {
	if stmt == nil || len(stmt.Branches) == 0 || !selectHasDefault(stmt) {
		return false
	}
	for _, branch := range stmt.Branches {
		if branch == nil || !blockDefinitelyReturns(branch.Body) {
			return false
		}
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

	if returnType.Kind == FunctionType {
		if a.checkReturningLambdaCapturingLocalReference(functionName, stmt.Value) {
			return
		}
	}

	if returnType.Kind == FunctionType && containsCapturedLambdaExpression(stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "escaping captured lambda is not supported yet")
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
		return
	}
	a.recordFunctionReturnOrigin(returnType, stmt.Value)
	if a.checkReturningReferenceToLocal(functionName, returnType, valueType, stmt.Value) {
		return
	}
	if a.checkExpressionEscapesLocalReference(functionName, stmt.Value) {
		return
	}
	if a.checkExpressionEscapesMatchPayload(functionName, stmt.Value) {
		return
	}
	a.markResourceTransfer(stmt.Value)
	a.markMoveSource(stmt.Value)
}

func (a *Analyzer) seedParameterReferenceOrigin(function Function, param FunctionParameter) {
	if !typeCarriesReferenceOrigin(param.Type) {
		return
	}
	placeType := param.Type
	if param.Type.Kind == ReferenceType && param.Type.Element != nil {
		placeType = *param.Type.Element
	}
	place := Place{
		Root:        param.Name,
		RootToken:   param.Token,
		Type:        placeType,
		Mutable:     param.Type.ReferenceMutable || param.MutableRef,
		Addressable: true,
	}
	a.localRefContainers[param.Name] = localOriginWithPlaces(localReferenceOrigin{
		Name:    param.Name,
		Token:   param.Token,
		Mutable: param.Type.ReferenceMutable,
	}, []Place{place})
}

func (a *Analyzer) recordFunctionReturnOrigin(returnType Type, expr ast.Expression) {
	if expr == nil {
		return
	}
	if !typeContainsReference(returnType, map[string]bool{}) {
		raw, symbolic := a.ownedReturnEscapeOrigins(expr)
		a.recordReturnEscapeFact(returnType, expr, raw, symbolic)
		return
	}
	candidate := localReferenceOrigin{}
	if typeCarriesReferenceOrigin(returnType) {
		origin, ok := a.directReferenceOrigin(expr)
		if !ok {
			candidate.Unknown = true
			candidate.Ambiguous = true
		} else {
			candidate = origin
		}
	} else {
		candidate.Contained = a.containedReferenceOrigins(expr)
		if len(candidate.Contained) == 0 {
			candidate.Unknown = true
			candidate.Ambiguous = true
		} else {
			recomputeContainedOriginSummary(&candidate)
		}
	}
	rawCandidate := cloneLocalReferenceOrigin(candidate)
	candidate = a.symbolizeFunctionReturnOrigin(candidate)
	a.recordReturnEscapeFact(returnType, expr, rawCandidate, candidate)
	if !a.hasCurrentFunctionSummary {
		a.currentFunctionSummary = candidate
		a.hasCurrentFunctionSummary = true
		return
	}
	a.currentFunctionSummary = mergeLocalReferenceOrigins([]localReferenceOrigin{a.currentFunctionSummary, candidate}, false)
}

func typeContainsReference(typ Type, visiting map[string]bool) bool {
	if typeCarriesReferenceOrigin(typ) {
		return true
	}
	identity := typeDeclarationIdentity(typ)
	if identity != "" && visiting[identity] {
		return false
	}
	if identity != "" {
		visiting[identity] = true
		defer delete(visiting, identity)
	}
	if typ.Element != nil && typeContainsReference(*typ.Element, visiting) {
		return true
	}
	for _, field := range typ.Fields {
		if typeContainsReference(field.Type, visiting) {
			return true
		}
	}
	for _, arg := range typ.TypeArgs {
		if typeContainsReference(arg, visiting) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && typeContainsReference(*variant.Payload, visiting) {
			return true
		}
	}
	return false
}

func (a *Analyzer) symbolizeFunctionReturnOrigin(origin localReferenceOrigin) localReferenceOrigin {
	origin = cloneLocalReferenceOrigin(origin)
	places := localOriginPlaces(origin)
	symbolic := make([]Place, 0, len(places))
	unknown := origin.Unknown
	for _, place := range places {
		root, ok := a.symbolicFunctionOriginRoot(place.Root)
		if !ok {
			unknown = true
			continue
		}
		place.Root = root
		symbolic = append(symbolic, place)
	}
	origin = localOriginWithPlaces(origin, symbolic)
	origin.Local = false
	origin.MatchScoped = false
	origin.Unknown = origin.Unknown || unknown
	origin.Ambiguous = origin.Ambiguous || origin.Unknown
	for path, child := range origin.Contained {
		origin.Contained[path] = a.symbolizeFunctionReturnOrigin(child)
		if origin.Contained[path].Unknown {
			origin.Unknown = true
			origin.Ambiguous = true
		}
	}
	return origin
}

func (a *Analyzer) symbolicFunctionOriginRoot(root string) (string, bool) {
	function := a.currentFunctionMetadata
	for index, param := range function.Parameters {
		if param.Name == root {
			return "$param:" + strconv.Itoa(index), true
		}
	}
	if root == "self" && function.ImplTarget != "" {
		return "$receiver", true
	}
	if symbol, exists := a.symbols[root]; exists && !symbol.Local {
		return "$static:" + root, true
	}
	return "", false
}

func (a *Analyzer) setFunctionReferenceSummary(function Function) {
	overloads := a.functions[function.Name]
	for index := range overloads {
		if overloads[index].Token.Line != function.Token.Line || overloads[index].Token.Column != function.Token.Column {
			continue
		}
		overloads[index].HasReturnOrigin = a.hasCurrentFunctionSummary
		overloads[index].ReturnOrigin = cloneLocalReferenceOrigin(a.currentFunctionSummary)
		a.functions[function.Name] = overloads
		return
	}
}

func containsCapturedLambdaExpression(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.LambdaExpression:
		return len(expr.Captures) > 0
	case *ast.PrefixExpression:
		return containsCapturedLambdaExpression(expr.Right)
	case *ast.InfixExpression:
		return containsCapturedLambdaExpression(expr.Left) || containsCapturedLambdaExpression(expr.Right)
	case *ast.CallExpression:
		if containsCapturedLambdaExpression(expr.Callee) {
			return true
		}
		for _, arg := range expr.Arguments {
			if containsCapturedLambdaExpression(arg) {
				return true
			}
		}
	case *ast.ConversionExpression:
		return containsCapturedLambdaExpression(expr.Value)
	}
	return false
}

func (a *Analyzer) checkReturningReferenceToLocal(functionName string, returnType Type, valueType Type, expr ast.Expression) bool {
	if !typeCarriesReferenceOrigin(returnType) || !typeCarriesReferenceOrigin(valueType) {
		return false
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if origin, tracked := a.expressionReferenceOrigins[call]; tracked {
			return a.checkTrackedReturnedReferenceOrigin(functionName, expr, origin)
		}
	}
	if origin, ok := a.containedOriginForAccess(expr); ok {
		return a.checkTrackedReturnedReferenceOrigin(functionName, expr, origin)
	}
	if valueType.ReferenceOriginMatchScoped {
		a.addErrorAtTokenWithPrevious(expressionToken(expr), valueType.ReferenceOriginToken, "function %s cannot return a branch-scoped union payload reference", functionName)
		return true
	}
	if !valueType.ReferenceOriginLocal {
		return false
	}
	originName := valueType.ReferenceOriginName
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.addErrorAtTokenWithPrevious(expressionToken(expr), valueType.ReferenceOriginToken, "lambda cannot return reference to local variable %s", originName)
		return true
	}
	a.addErrorAtTokenWithPrevious(expressionToken(expr), valueType.ReferenceOriginToken, "function %s cannot return reference to local variable %s", functionName, originName)
	return true
}

func (a *Analyzer) checkTrackedReturnedReferenceOrigin(functionName string, expr ast.Expression, origin localReferenceOrigin) bool {
	if origin.Unknown {
		a.addErrorAtToken(expressionToken(expr), "function %s cannot return reference with unknown control-flow provenance", functionName)
		return true
	}
	if origin.MatchScoped {
		a.addErrorAtTokenWithPrevious(expressionToken(expr), origin.Token, "function %s cannot return a branch-scoped union payload reference", functionName)
		return true
	}
	if !origin.Local {
		// The aggregate/call result can be local while the referenced storage
		// is exclusively caller-owned.
		return false
	}
	originName := origin.Name
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.addErrorAtTokenWithPrevious(expressionToken(expr), origin.Token, "lambda cannot return reference to local variable %s", originName)
		return true
	}
	a.addErrorAtTokenWithPrevious(expressionToken(expr), origin.Token, "function %s cannot return reference to local variable %s", functionName, originName)
	return true
}

func (a *Analyzer) checkExpressionEscapesLocalReference(functionName string, expr ast.Expression) bool {
	originName, originToken, ok := a.localReferenceOriginInExpression(expr)
	if !ok {
		return false
	}
	if originName == "" {
		originName = "local value"
	}
	if functionName == "lambda" {
		a.addErrorAtTokenWithPrevious(expressionToken(expr), originToken, "lambda cannot return value containing reference to local variable %s", originName)
		return true
	}
	a.addErrorAtTokenWithPrevious(expressionToken(expr), originToken, "function %s cannot return value containing reference to local variable %s", functionName, originName)
	return true
}

func (a *Analyzer) checkExpressionEscapesMatchPayload(functionName string, expr ast.Expression) bool {
	_, originToken, ok := a.matchScopedReferenceOriginInExpression(expr)
	if !ok {
		return false
	}
	a.addErrorAtTokenWithPrevious(expressionToken(expr), originToken, "function %s cannot return a value containing a branch-scoped union payload reference", functionName)
	return true
}

func (a *Analyzer) checkAssignmentEscapesLocalReference(target ast.Expression, value ast.Expression) bool {
	if a.checkAssignmentEscapesMatchPayload(target, value) {
		return true
	}
	originName, originToken, ok := a.localReferenceOriginInExpression(value)
	if !ok {
		return false
	}
	targetRoot, ok := borrowRootName(target)
	if !ok {
		return false
	}
	targetSymbol, ok := a.symbols[targetRoot]
	if !ok {
		return false
	}
	if targetSymbol.Local {
		return false
	}
	if originName == "" {
		originName = "local value"
	}
	a.recordOuterPlaceEscapeFact(value, originName, originToken)
	a.addErrorAtTokenWithPrevious(expressionToken(value), originToken, "cannot store reference to local variable %s into %s", originName, targetRoot)
	return true
}

func (a *Analyzer) checkAssignmentEscapesMatchPayload(target ast.Expression, value ast.Expression) bool {
	_, originToken, ok := a.matchScopedReferenceOriginInExpression(value)
	if !ok {
		return false
	}
	targetRoot, rootOK := borrowRootName(target)
	if rootOK {
		if symbol, symbolOK := a.symbols[targetRoot]; symbolOK && symbol.Local && symbol.ScopeDepth >= a.scopeDepth {
			return false
		}
	}
	a.addErrorAtTokenWithPrevious(expressionToken(value), originToken, "cannot store branch-scoped union payload reference outside its match arm")
	return true
}

func (a *Analyzer) markLocalRefContainerFromValue(holder string, value ast.Expression) {
	if holder == "" {
		return
	}
	symbol, ok := a.symbols[holder]
	if !ok || !symbol.Local {
		return
	}
	origin, hasOrigin := a.localReferenceOriginForExpression(value)
	contained := a.containedReferenceOrigins(value)
	if !hasOrigin && len(contained) == 0 {
		return
	}
	origin.Contained = contained
	if !hasOrigin {
		recomputeContainedOriginSummary(&origin)
	}
	a.localRefContainers[holder] = origin
}

func cloneLocalReferenceOrigin(origin localReferenceOrigin) localReferenceOrigin {
	origin.Place = clonePlace(origin.Place)
	origin.Places = clonePlaces(origin.Places)
	if len(origin.Contained) > 0 {
		contained := origin.Contained
		origin.Contained = make(map[string]localReferenceOrigin, len(contained))
		for path, child := range contained {
			origin.Contained[path] = cloneLocalReferenceOrigin(child)
		}
	}
	return origin
}

func recomputeContainedOriginSummary(origin *localReferenceOrigin) {
	if origin == nil {
		return
	}
	origin.Local = false
	origin.MatchScoped = false
	origin.Name = ""
	origin.Token = lexer.Token{}
	for _, child := range origin.Contained {
		if child.Local && !origin.Local {
			origin.Local, origin.Name, origin.Token = true, child.Name, child.Token
		}
		if child.MatchScoped && !origin.MatchScoped {
			origin.MatchScoped = true
			if origin.Name == "" {
				origin.Name, origin.Token = child.Name, child.Token
			}
		}
	}
}

func prefixContainedOrigins(prefix string, origins map[string]localReferenceOrigin, out map[string]localReferenceOrigin) {
	for path, origin := range origins {
		out[prefix+path] = cloneLocalReferenceOrigin(origin)
	}
}

func (a *Analyzer) containedReferenceOrigins(expr ast.Expression) map[string]localReferenceOrigin {
	out := map[string]localReferenceOrigin{}
	switch expr := expr.(type) {
	case *ast.Identifier:
		if origin, ok := a.localRefContainers[expr.Value]; ok {
			prefixContainedOrigins("", origin.Contained, out)
		}
	case *ast.CallExpression:
		if origin, ok := a.expressionReferenceOrigins[expr]; ok {
			prefixContainedOrigins("", origin.Contained, out)
		}
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field == nil || field.Value == nil {
				continue
			}
			if field.Spread {
				prefixContainedOrigins("", a.containedReferenceOrigins(field.Value), out)
				continue
			}
			path := "." + field.Name.Value
			if origin, ok := a.directReferenceOrigin(field.Value); ok {
				out[path] = origin
			}
			prefixContainedOrigins(path, a.containedReferenceOrigins(field.Value), out)
		}
	case *ast.ArrayLiteral:
		for index, element := range expr.Elements {
			if spread, ok := element.(*ast.SpreadExpression); ok {
				for childPath, child := range a.containedReferenceOrigins(spread.Value) {
					if close := strings.IndexByte(childPath, ']'); strings.HasPrefix(childPath, "[") && close >= 0 {
						out["[*]"+childPath[close+1:]] = cloneLocalReferenceOrigin(child)
					}
				}
				continue
			}
			path := "[" + strconv.Itoa(index) + "]"
			if origin, ok := a.directReferenceOrigin(element); ok {
				out[path] = origin
			}
			prefixContainedOrigins(path, a.containedReferenceOrigins(element), out)
		}
	case *ast.SpreadExpression:
		return a.containedReferenceOrigins(expr.Value)
	case *ast.ConversionExpression:
		return a.containedReferenceOrigins(expr.Value)
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.containedReferenceOrigins(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.containedReferenceOrigins(expr.Value)
		}
	}
	return out
}

func (a *Analyzer) directReferenceOrigin(expr ast.Expression) (localReferenceOrigin, bool) {
	if spread, ok := expr.(*ast.SpreadExpression); ok {
		return a.directReferenceOrigin(spread.Value)
	}
	typ, ok := a.expressionTypes[expr]
	if !ok {
		typ, _ = a.inferExpression(expr)
	}
	if !typeCarriesReferenceOrigin(typ) {
		return localReferenceOrigin{}, false
	}
	origin := localReferenceOrigin{
		Name:        typ.ReferenceOriginName,
		Token:       typ.ReferenceOriginToken,
		Local:       typ.ReferenceOriginLocal,
		MatchScoped: typ.ReferenceOriginMatchScoped,
		Mutable:     typ.ReferenceMutable,
	}
	if a.referencePlaceOriginIsAmbiguous(expr) {
		origin.Unknown = true
		origin.Ambiguous = true
		return origin, true
	}
	if place, placeOK := a.referencePlaceOrigin(expr); placeOK {
		origin = localOriginWithPlaces(origin, placeOriginAlternatives(place))
	} else if place, placeOK := a.resolvePlace(expr); placeOK {
		// A reference parameter has no local holder entry, but its parameter
		// place is still a stable compile-time provenance root.
		origin = localOriginWithPlaces(origin, placeOriginAlternatives(place))
	}
	if !origin.Local && !origin.MatchScoped && len(localOriginPlaces(origin)) == 0 {
		return localReferenceOrigin{}, false
	}
	return origin, true
}

func (a *Analyzer) localReferenceOriginForExpression(value ast.Expression) (localReferenceOrigin, bool) {
	originName, originToken, ok := a.localReferenceOriginInExpression(value)
	origin := localReferenceOrigin{Name: originName, Token: originToken, Local: ok}
	if !ok {
		originName, originToken, ok = a.matchScopedReferenceOriginInExpression(value)
		if !ok {
			return localReferenceOrigin{}, false
		}
		origin.Name = originName
		origin.Token = originToken
		origin.MatchScoped = true
	}
	if place, placeOK := a.referencePlaceOrigin(value); placeOK {
		origin = localOriginWithPlaces(origin, placeOriginAlternatives(place))
	}
	return origin, true
}

func (a *Analyzer) matchScopedReferenceOriginInExpression(expr ast.Expression) (string, lexer.Token, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if origin, ok := a.localRefContainers[expr.Value]; ok && origin.MatchScoped {
			return origin.Name, origin.Token, true
		}
		if symbol, ok := a.symbols[expr.Value]; ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginMatchScoped {
			return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, true
		}
	case *ast.CallExpression:
		if origin, ok := a.expressionReferenceOrigins[expr]; ok && origin.MatchScoped {
			return origin.Name, origin.Token, true
		}
	case *ast.RefExpression:
		return a.matchScopedReferenceOriginInExpression(expr.Value)
	case *ast.MemberExpression:
		if origin, ok := a.containedOriginForAccess(expr); ok {
			if origin.MatchScoped {
				return origin.Name, origin.Token, true
			}
			return "", lexer.Token{}, false
		}
		if valueType, ok := a.expressionTypes[expr]; ok && typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginMatchScoped {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
		return a.matchScopedReferenceOriginInExpression(expr.Object)
	case *ast.IndexExpression:
		if origin, ok := a.containedOriginForAccess(expr); ok {
			if origin.MatchScoped {
				return origin.Name, origin.Token, true
			}
			return "", lexer.Token{}, false
		}
		if valueType, ok := a.expressionTypes[expr]; ok && typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginMatchScoped {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
		return a.matchScopedReferenceOriginInExpression(expr.Left)
	case *ast.SliceExpression:
		if valueType, ok := a.expressionTypes[expr]; ok && typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginMatchScoped {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
		return a.matchScopedReferenceOriginInExpression(expr.Left)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil {
				if name, token, ok := a.matchScopedReferenceOriginInExpression(field.Value); ok {
					return name, token, true
				}
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if name, token, ok := a.matchScopedReferenceOriginInExpression(element); ok {
				return name, token, true
			}
		}
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.matchScopedReferenceOriginInExpression(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.matchScopedReferenceOriginInExpression(expr.Value)
		}
	case *ast.ConversionExpression:
		return a.matchScopedReferenceOriginInExpression(expr.Value)
	case *ast.PrefixExpression:
		return a.matchScopedReferenceOriginInExpression(expr.Right)
	case *ast.InfixExpression:
		if name, token, ok := a.matchScopedReferenceOriginInExpression(expr.Left); ok {
			return name, token, true
		}
		return a.matchScopedReferenceOriginInExpression(expr.Right)
	case *ast.LambdaExpression:
		for _, capture := range expr.Captures {
			if capture.Name == nil {
				continue
			}
			if origin, ok := a.localRefContainers[capture.Name.Value]; ok && origin.MatchScoped {
				return origin.Name, origin.Token, true
			}
			if symbol, ok := a.symbols[capture.Name.Value]; ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginMatchScoped {
				return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, true
			}
		}
	}
	return "", lexer.Token{}, false
}

func (a *Analyzer) localReferenceOriginForTransfer(value ast.Expression) (localReferenceOrigin, bool) {
	switch value := value.(type) {
	case *ast.Identifier:
		if origin, ok := a.localRefContainers[value.Value]; ok {
			return cloneLocalReferenceOrigin(origin), true
		}
	case *ast.ConversionExpression:
		return a.localReferenceOriginForTransfer(value.Value)
	}
	return localReferenceOrigin{}, false
}

func (a *Analyzer) referencePlaceOrigin(expr ast.Expression) (Place, bool) {
	switch expr := expr.(type) {
	case *ast.RefExpression:
		place, ok := a.resolvePlace(expr.Value)
		if !ok {
			return Place{}, false
		}
		place.ReferenceHolder = ""
		for index := range place.AlternativeOrigins {
			place.AlternativeOrigins[index].ReferenceHolder = ""
		}
		return place, true
	case *ast.SliceExpression:
		place, ok := a.resolvePlace(expr)
		if !ok {
			return Place{}, false
		}
		place.ReferenceHolder = ""
		return place, true
	case *ast.Identifier:
		origin, ok := a.localRefContainers[expr.Value]
		if !ok || origin.Unknown {
			return Place{}, false
		}
		places := localOriginPlaces(origin)
		if len(places) == 0 {
			return Place{}, false
		}
		place := clonePlace(places[0])
		place.ReferenceHolder = expr.Value
		for _, alternative := range places[1:] {
			alternative = clonePlace(alternative)
			alternative.ReferenceHolder = expr.Value
			place.AlternativeOrigins = append(place.AlternativeOrigins, alternative)
		}
		return place, true
	case *ast.CallExpression:
		origin, ok := a.expressionReferenceOrigins[expr]
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, "")
	case *ast.MemberExpression:
		origin, ok := a.containedOriginForAccess(expr)
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, mustBorrowRootName(expr))
	case *ast.IndexExpression:
		origin, ok := a.containedOriginForAccess(expr)
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, mustBorrowRootName(expr))
	case *ast.ConversionExpression:
		return a.referencePlaceOrigin(expr.Value)
	default:
		return Place{}, false
	}
}

func (a *Analyzer) referencePlaceOriginIsAmbiguous(expr ast.Expression) bool {
	if origin, ok := a.containedOriginForAccess(expr); ok {
		return origin.Unknown
	}
	root, ok := borrowRootName(expr)
	if !ok {
		return false
	}
	origin, ok := a.localRefContainers[root]
	return ok && origin.Unknown
}

func placeFromReferenceOrigin(origin localReferenceOrigin, holder string) (Place, bool) {
	places := localOriginPlaces(origin)
	if len(places) == 0 {
		return Place{}, false
	}
	place := clonePlace(places[0])
	place.ReferenceHolder = holder
	for _, alternative := range places[1:] {
		alternative = clonePlace(alternative)
		alternative.ReferenceHolder = holder
		place.AlternativeOrigins = append(place.AlternativeOrigins, alternative)
	}
	return place, true
}

func mustBorrowRootName(expr ast.Expression) string {
	root, _ := borrowRootName(expr)
	return root
}

func (a *Analyzer) containedOriginForAccess(expr ast.Expression) (localReferenceOrigin, bool) {
	root, path, ok := a.aggregateAccessPath(expr)
	if !ok || path == "" {
		return localReferenceOrigin{}, false
	}
	container, ok := a.localRefContainers[root]
	if !ok || len(container.Contained) == 0 {
		return localReferenceOrigin{}, false
	}
	origins := make([]localReferenceOrigin, 0, 1)
	for candidate, origin := range container.Contained {
		if containedPathMatches(path, candidate) {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return localReferenceOrigin{}, false
	}
	return mergeLocalReferenceOrigins(origins, false), true
}

func (a *Analyzer) aggregateAccessPath(expr ast.Expression) (string, string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, "", true
	case *ast.MemberExpression:
		root, path, ok := a.aggregateAccessPath(expr.Object)
		if !ok || expr.Property == nil {
			return "", "", false
		}
		return root, path + "." + expr.Property.Value, true
	case *ast.IndexExpression:
		root, path, ok := a.aggregateAccessPath(expr.Left)
		if !ok {
			return "", "", false
		}
		index := "*"
		if value, constant := a.integerConstantValue(expr.Index); constant {
			index = value.String()
		}
		return root, path + "[" + index + "]", true
	default:
		return "", "", false
	}
}

func containedPathMatches(query, candidate string) bool {
	if query == candidate {
		return true
	}
	if !strings.Contains(query, "[*]") && !strings.Contains(candidate, "[*]") {
		return false
	}
	return normalizeContainedIndexPath(query) == normalizeContainedIndexPath(candidate)
}

func normalizeContainedIndexPath(path string) string {
	var normalized strings.Builder
	for index := 0; index < len(path); index++ {
		if path[index] != '[' {
			normalized.WriteByte(path[index])
			continue
		}
		end := strings.IndexByte(path[index:], ']')
		if end < 0 {
			normalized.WriteString(path[index:])
			break
		}
		normalized.WriteString("[*]")
		index += end
	}
	return normalized.String()
}

func (a *Analyzer) updateContainedOriginsForAssignment(target ast.Expression, value ast.Expression) {
	root, path, ok := a.aggregateAccessPath(target)
	if !ok || path == "" {
		return
	}
	symbol, ok := a.symbols[root]
	if !ok || !symbol.Local || typeCarriesReferenceOrigin(symbol.Type) {
		return
	}
	container := cloneLocalReferenceOrigin(a.localRefContainers[root])
	if container.Contained == nil {
		container.Contained = map[string]localReferenceOrigin{}
	}
	direct, hasDirect := a.directReferenceOrigin(value)
	children := a.containedReferenceOrigins(value)
	if strings.Contains(path, "[*]") {
		alternatives := make([]localReferenceOrigin, 0, len(container.Contained)+1)
		for candidate, origin := range container.Contained {
			if containedPathMatches(path, candidate) {
				alternatives = append(alternatives, origin)
			}
		}
		if hasDirect {
			alternatives = append(alternatives, direct)
		}
		if len(alternatives) > 0 {
			container.Contained[path] = mergeLocalReferenceOrigins(alternatives, false)
		}
		for childPath, child := range children {
			container.Contained[path+childPath] = cloneLocalReferenceOrigin(child)
		}
	} else {
		for candidate := range container.Contained {
			if candidate == path || strings.HasPrefix(candidate, path+".") || strings.HasPrefix(candidate, path+"[") {
				delete(container.Contained, candidate)
			}
		}
		if hasDirect {
			container.Contained[path] = direct
		}
		prefixContainedOrigins(path, children, container.Contained)
	}
	recomputeContainedOriginSummary(&container)
	if len(container.Contained) == 0 && !container.HasPlace && len(container.Places) == 0 {
		delete(a.localRefContainers, root)
		return
	}
	a.localRefContainers[root] = container
}

func localOriginWithPlaces(origin localReferenceOrigin, places []Place) localReferenceOrigin {
	origin.Place = Place{}
	origin.Places = nil
	origin.HasPlace = false
	origin.Ambiguous = false
	unique := uniquePlaces(places)
	if len(unique) > maxReferenceOriginAlternatives {
		origin.Unknown = true
		origin.Ambiguous = true
		return origin
	}
	if len(unique) == 1 {
		origin.Place = clonePlace(unique[0])
		origin.HasPlace = true
	} else if len(unique) > 1 {
		origin.Places = clonePlaces(unique)
		origin.Ambiguous = true
	}
	return origin
}

func localOriginPlaces(origin localReferenceOrigin) []Place {
	if origin.HasPlace {
		return []Place{clonePlace(origin.Place)}
	}
	return clonePlaces(origin.Places)
}

func uniquePlaces(places []Place) []Place {
	unique := make([]Place, 0, len(places))
	for _, place := range places {
		place.AlternativeOrigins = nil
		found := false
		for _, existing := range unique {
			if samePlaceIdentity(existing, place) {
				found = true
				break
			}
		}
		if !found {
			unique = append(unique, clonePlace(place))
		}
	}
	return unique
}

func (a *Analyzer) checkReturningLambdaCapturingLocalReference(functionName string, expr ast.Expression) bool {
	lambda, ok := expr.(*ast.LambdaExpression)
	if !ok {
		return false
	}
	for _, capture := range lambda.Captures {
		if capture.Name == nil {
			continue
		}
		symbol, ok := a.symbols[capture.Name.Value]
		if !ok || symbol.Type.Kind != ReferenceType || !symbol.Type.ReferenceOriginLocal {
			continue
		}
		originName := symbol.Type.ReferenceOriginName
		if originName == "" {
			originName = "local value"
		}
		if functionName == "lambda" {
			a.addErrorAtTokenWithPrevious(capture.Name.Token, symbol.Type.ReferenceOriginToken, "lambda cannot return lambda capturing reference to local variable %s", originName)
			return true
		}
		a.addErrorAtTokenWithPrevious(capture.Name.Token, symbol.Type.ReferenceOriginToken, "function %s cannot return lambda capturing reference to local variable %s", functionName, originName)
		return true
	}
	return false
}

func (a *Analyzer) localReferenceOriginInExpression(expr ast.Expression) (string, lexer.Token, bool) {
	switch expr := expr.(type) {
	case *ast.RefExpression:
		originName, originToken, originLocal, _, _ := a.referenceOriginForExpression(expr.Value)
		if originLocal {
			return originName, originToken, true
		}
	case *ast.SliceExpression:
		valueType, _ := a.inferSliceExpression(expr)
		if typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginLocal {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
	case *ast.Identifier:
		if origin, ok := a.localRefContainers[expr.Value]; ok {
			if origin.Local {
				return origin.Name, origin.Token, true
			}
		}
		symbol, ok := a.symbols[expr.Value]
		if ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginLocal {
			return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, true
		}
	case *ast.CallExpression:
		if origin, ok := a.expressionReferenceOrigins[expr]; ok && origin.Local {
			return origin.Name, origin.Token, true
		}
	case *ast.MemberExpression:
		if origin, ok := a.containedOriginForAccess(expr); ok {
			if origin.Local {
				return origin.Name, origin.Token, true
			}
			return "", lexer.Token{}, false
		}
		valueType, ok := a.inferMemberExpression(expr)
		if ok && typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginLocal {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
	case *ast.IndexExpression:
		if origin, ok := a.containedOriginForAccess(expr); ok {
			if origin.Local {
				return origin.Name, origin.Token, true
			}
			return "", lexer.Token{}, false
		}
		valueType, _ := a.inferIndexExpression(expr)
		if typeCarriesReferenceOrigin(valueType) && valueType.ReferenceOriginLocal {
			return valueType.ReferenceOriginName, valueType.ReferenceOriginToken, true
		}
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field == nil || field.Value == nil {
				continue
			}
			if name, token, ok := a.localReferenceOriginInExpression(field.Value); ok {
				return name, token, true
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if name, token, ok := a.localReferenceOriginInExpression(element); ok {
				return name, token, true
			}
		}
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.localReferenceOriginInExpression(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.localReferenceOriginInExpression(expr.Value)
		}
	case *ast.ConversionExpression:
		return a.localReferenceOriginInExpression(expr.Value)
	case *ast.PrefixExpression:
		return a.localReferenceOriginInExpression(expr.Right)
	case *ast.InfixExpression:
		if name, token, ok := a.localReferenceOriginInExpression(expr.Left); ok {
			return name, token, true
		}
		return a.localReferenceOriginInExpression(expr.Right)
	case *ast.LambdaExpression:
		for _, capture := range expr.Captures {
			if capture.Name == nil {
				continue
			}
			symbol, ok := a.symbols[capture.Name.Value]
			if ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginLocal {
				return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, true
			}
			if origin, ok := a.localRefContainers[capture.Name.Value]; ok {
				if origin.Local {
					return origin.Name, origin.Token, true
				}
			}
		}
	}
	return "", lexer.Token{}, false
}

func (a *Analyzer) analyzeResultReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if len(returnType.TypeArgs) != 2 {
		return
	}

	switch expr := stmt.Value.(type) {
	case *ast.OkExpression:
		expected := returnType.TypeArgs[0]
		if len(expr.Arguments) > 1 {
			a.addErrorAtToken(expr.Token, "Ok expects 1 argument, got %d", len(expr.Arguments))
			return
		}
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
			return
		}
		a.recordFunctionReturnOrigin(expected, expr.Value)
		if a.checkReturningReferenceToLocal(functionName, expected, valueType, expr.Value) {
			return
		}
		if a.checkExpressionEscapesLocalReference(functionName, expr.Value) {
			return
		}
		a.markResourceTransfer(expr.Value)
		a.markMoveSource(expr.Value)
	case *ast.ErrExpression:
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Err expects 1 argument, got %d", len(expr.Arguments))
			return
		}
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind == InvalidType {
			return
		}
		expected := returnType.TypeArgs[1]
		if !canInitialize(expected, valueType, expr.Value) {
			a.addErrorAtToken(expressionToken(expr.Value), "function %s must return Err(%s), got Err(%s)", functionName, typeDisplayName(expected), typeDisplayName(valueType))
			return
		}
		a.recordFunctionReturnOrigin(expected, expr.Value)
		a.markMoveSource(expr.Value)
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

func copyMoved(in map[string]lexer.Token) map[string]lexer.Token {
	out := make(map[string]lexer.Token, len(in))
	for name, token := range in {
		out[name] = token
	}
	return out
}

func copyMoveReasons(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for place, reason := range in {
		out[place] = reason
	}
	return out
}

func copyDefinitionTokens(in map[sourceTokenKey][]lexer.Token) map[sourceTokenKey][]lexer.Token {
	out := make(map[sourceTokenKey][]lexer.Token, len(in))
	for key, tokens := range in {
		out[key] = append([]lexer.Token(nil), tokens...)
	}
	return out
}

func copyBorrows(in map[string][]borrowRecord) map[string][]borrowRecord {
	out := make(map[string][]borrowRecord, len(in))
	for name, records := range in {
		copied := make([]borrowRecord, len(records))
		for index, record := range records {
			record.Place = clonePlace(record.Place)
			copied[index] = record
		}
		out[name] = copied
	}
	return out
}

func clearLoopBindingReferenceState(borrows map[string][]borrowRecord, origins map[string]localReferenceOrigin, name string) {
	delete(borrows, name)
	for root, records := range borrows {
		kept := records[:0]
		for _, record := range records {
			if record.Holder != name {
				kept = append(kept, record)
			}
		}
		if len(kept) == 0 {
			delete(borrows, root)
		} else {
			borrows[root] = kept
		}
	}
	delete(origins, name)
}

func (a *Analyzer) clearScopedBindingFromLoopEdges(name string) {
	for frameIndex := range a.loopBreakFrames {
		frame := &a.loopBreakFrames[frameIndex]
		for index := range frame.borrows {
			delete(frame.closedResources[index], name)
			clearLoopBindingReferenceState(frame.borrows[index], frame.localRefContainers[index], name)
		}
		for index := range frame.continueBorrows {
			delete(frame.continueClosedResources[index], name)
			clearLoopBindingReferenceState(frame.continueBorrows[index], frame.continueLocalRefContainers[index], name)
		}
	}
}

func copyLocalRefContainers(in map[string]localReferenceOrigin) map[string]localReferenceOrigin {
	out := make(map[string]localReferenceOrigin, len(in))
	for name, origin := range in {
		out[name] = cloneLocalReferenceOrigin(origin)
	}
	return out
}

func clonePlace(place Place) Place {
	place.Projections = append([]PlaceProjection(nil), place.Projections...)
	place.AlternativeOrigins = clonePlaces(place.AlternativeOrigins)
	return place
}

func clonePlaces(places []Place) []Place {
	cloned := make([]Place, len(places))
	for index, place := range places {
		place.Projections = append([]PlaceProjection(nil), place.Projections...)
		place.AlternativeOrigins = nil
		cloned[index] = place
	}
	return cloned
}

func copyArenaGenerations(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for name, generation := range in {
		out[name] = generation
	}
	return out
}

func (a *Analyzer) endBorrowsHeldBy(holder string) {
	for root, records := range a.borrows {
		out := records[:0]
		for _, record := range records {
			if record.Holder != holder {
				out = append(out, record)
			}
		}
		if len(out) == 0 {
			delete(a.borrows, root)
			continue
		}
		a.borrows[root] = out
	}
}

func (a *Analyzer) pushLoopBreakFrame() int {
	a.loopBreakFrames = append(a.loopBreakFrames, loopBreakFrame{})
	return len(a.loopBreakFrames) - 1
}

func (a *Analyzer) popLoopBreakFrame(frame int) loopBreakFrame {
	if frame < 0 || frame >= len(a.loopBreakFrames) {
		return loopBreakFrame{}
	}
	breaks := a.loopBreakFrames[frame]
	a.loopBreakFrames = a.loopBreakFrames[:frame]
	return breaks
}

func (a *Analyzer) recordLoopBreak() {
	if len(a.loopBreakFrames) == 0 {
		return
	}
	top := len(a.loopBreakFrames) - 1
	a.loopBreakFrames[top].assignments = append(a.loopBreakFrames[top].assignments, copyAssigned(a.assigned))
	a.loopBreakFrames[top].moved = append(a.loopBreakFrames[top].moved, copyMoved(a.moved))
	a.loopBreakFrames[top].moveReasons = append(a.loopBreakFrames[top].moveReasons, copyMoveReasons(a.moveReasons))
	a.loopBreakFrames[top].closedResources = append(a.loopBreakFrames[top].closedResources, copyMoved(a.closedResources))
	a.loopBreakFrames[top].borrows = append(a.loopBreakFrames[top].borrows, copyBorrows(a.borrows))
	a.loopBreakFrames[top].localRefContainers = append(a.loopBreakFrames[top].localRefContainers, copyLocalRefContainers(a.localRefContainers))
	a.loopBreakFrames[top].arenaGenerations = append(a.loopBreakFrames[top].arenaGenerations, copyArenaGenerations(a.arenaGenerations))
}

func (a *Analyzer) recordLoopContinue() {
	if len(a.loopBreakFrames) == 0 {
		return
	}
	top := len(a.loopBreakFrames) - 1
	a.loopBreakFrames[top].continueMoved = append(a.loopBreakFrames[top].continueMoved, copyMoved(a.moved))
	a.loopBreakFrames[top].continueReasons = append(a.loopBreakFrames[top].continueReasons, copyMoveReasons(a.moveReasons))
	a.loopBreakFrames[top].continueClosedResources = append(a.loopBreakFrames[top].continueClosedResources, copyMoved(a.closedResources))
	a.loopBreakFrames[top].continueBorrows = append(a.loopBreakFrames[top].continueBorrows, copyBorrows(a.borrows))
	a.loopBreakFrames[top].continueLocalRefContainers = append(a.loopBreakFrames[top].continueLocalRefContainers, copyLocalRefContainers(a.localRefContainers))
}

func mergeMoveStateInto(mergedMoved map[string]lexer.Token, mergedReasons map[string]string, moved map[string]lexer.Token, reasons map[string]string) {
	for place, token := range moved {
		if _, exists := mergedMoved[place]; exists {
			continue
		}
		mergedMoved[place] = token
		if reason := reasons[place]; reason != "" {
			mergedReasons[place] = reason
		}
	}
}

func mergeLoopMoveState(beforeMoved map[string]lexer.Token, beforeReasons map[string]string, loopMoved map[string]lexer.Token, loopReasons map[string]string, breaks loopBreakFrame) (map[string]lexer.Token, map[string]string) {
	// Ordinary loops may execute zero times. The post-loop fixed point is thus
	// the conservative union of entry, back-edge, and every explicit break exit.
	mergedMoved := map[string]lexer.Token{}
	mergedReasons := map[string]string{}
	mergeMoveStateInto(mergedMoved, mergedReasons, beforeMoved, beforeReasons)
	mergeMoveStateInto(mergedMoved, mergedReasons, loopMoved, loopReasons)
	for index, moved := range breaks.continueMoved {
		var reasons map[string]string
		if index < len(breaks.continueReasons) {
			reasons = breaks.continueReasons[index]
		}
		mergeMoveStateInto(mergedMoved, mergedReasons, moved, reasons)
	}
	for index, moved := range breaks.moved {
		var reasons map[string]string
		if index < len(breaks.moveReasons) {
			reasons = breaks.moveReasons[index]
		}
		mergeMoveStateInto(mergedMoved, mergedReasons, moved, reasons)
	}
	return mergedMoved, mergedReasons
}

func loopBackedgeMoveState(entryMoved map[string]lexer.Token, entryReasons map[string]string, loopMoved map[string]lexer.Token, loopReasons map[string]string, frame loopBreakFrame, bodyFallsThrough bool) (map[string]lexer.Token, map[string]string) {
	headerMoved := copyMoved(entryMoved)
	headerReasons := copyMoveReasons(entryReasons)
	if bodyFallsThrough {
		mergeMoveStateInto(headerMoved, headerReasons, loopMoved, loopReasons)
	}
	for index, moved := range frame.continueMoved {
		var reasons map[string]string
		if index < len(frame.continueReasons) {
			reasons = frame.continueReasons[index]
		}
		mergeMoveStateInto(headerMoved, headerReasons, moved, reasons)
	}
	return headerMoved, headerReasons
}

func loopBackedgeClosedResourceState(entry, loop map[string]lexer.Token, frame loopBreakFrame, bodyFallsThrough bool) map[string]lexer.Token {
	states := []map[string]lexer.Token{entry}
	if bodyFallsThrough {
		states = append(states, loop)
	}
	states = append(states, frame.continueClosedResources...)
	return intersectTokenStates(states...)
}

func mergeLoopClosedResourceState(entry, loop map[string]lexer.Token, frame loopBreakFrame, bodyFallsThrough, conditionAlwaysTrue bool) map[string]lexer.Token {
	if conditionAlwaysTrue {
		if len(frame.closedResources) == 0 {
			return copyMoved(entry)
		}
		return intersectTokenStates(frame.closedResources...)
	}
	states := []map[string]lexer.Token{entry}
	if bodyFallsThrough {
		states = append(states, loop)
	}
	states = append(states, frame.continueClosedResources...)
	states = append(states, frame.closedResources...)
	return intersectTokenStates(states...)
}

func loopBackedgeBorrowState(entry map[string][]borrowRecord, loop map[string][]borrowRecord, frame loopBreakFrame, bodyFallsThrough bool) map[string][]borrowRecord {
	states := []map[string][]borrowRecord{entry}
	if bodyFallsThrough {
		states = append(states, loop)
	}
	states = append(states, frame.continueBorrows...)
	merged := mergeBorrowStates(states...)
	for root, records := range merged {
		entryRecords := entry[root]
		for index := range records {
			records[index].LoopCarried = !containsBorrowRecord(entryRecords, records[index])
		}
		merged[root] = records
	}
	return merged
}

func mergeLoopBorrowState(entry map[string][]borrowRecord, loop map[string][]borrowRecord, frame loopBreakFrame) map[string][]borrowRecord {
	states := []map[string][]borrowRecord{entry, loop}
	states = append(states, frame.continueBorrows...)
	states = append(states, frame.borrows...)
	merged := mergeBorrowStates(states...)
	for root, records := range merged {
		for index := range records {
			records[index].LoopCarried = false
		}
		merged[root] = records
	}
	return merged
}

func mergeBorrowStates(states ...map[string][]borrowRecord) map[string][]borrowRecord {
	merged := map[string][]borrowRecord{}
	for _, state := range states {
		for root, records := range state {
			for _, record := range records {
				if containsBorrowRecord(merged[root], record) {
					continue
				}
				record.Place = clonePlace(record.Place)
				merged[root] = append(merged[root], record)
			}
		}
	}
	return merged
}

func loopBackedgeReferenceState(entry map[string]localReferenceOrigin, loop map[string]localReferenceOrigin, frame loopBreakFrame, bodyFallsThrough bool) map[string]localReferenceOrigin {
	states := []map[string]localReferenceOrigin{entry}
	if bodyFallsThrough {
		states = append(states, loop)
	}
	states = append(states, frame.continueLocalRefContainers...)
	return mergeReferenceOriginStates(states...)
}

func mergeLoopReferenceState(entry map[string]localReferenceOrigin, loop map[string]localReferenceOrigin, frame loopBreakFrame) map[string]localReferenceOrigin {
	states := []map[string]localReferenceOrigin{entry, loop}
	states = append(states, frame.continueLocalRefContainers...)
	states = append(states, frame.localRefContainers...)
	return mergeReferenceOriginStates(states...)
}

func mergeReferenceOriginStates(states ...map[string]localReferenceOrigin) map[string]localReferenceOrigin {
	merged := map[string]localReferenceOrigin{}
	names := map[string]bool{}
	for _, state := range states {
		for name := range state {
			names[name] = true
		}
	}
	for name := range names {
		origins := make([]localReferenceOrigin, 0, len(states))
		missing := false
		for _, state := range states {
			origin, ok := state[name]
			if !ok {
				missing = true
				continue
			}
			origins = append(origins, origin)
		}
		if len(origins) > 0 {
			merged[name] = mergeLocalReferenceOrigins(origins, missing)
		}
	}
	return merged
}

func mergeLocalReferenceOrigins(origins []localReferenceOrigin, missing bool) localReferenceOrigin {
	if len(origins) == 0 {
		return localReferenceOrigin{Unknown: missing, Ambiguous: missing}
	}
	joined := cloneLocalReferenceOrigin(origins[0])
	places := localOriginPlaces(joined)
	containedPaths := map[string]bool{}
	for path := range joined.Contained {
		containedPaths[path] = true
	}
	for _, origin := range origins[1:] {
		places = append(places, localOriginPlaces(origin)...)
		joined.Local = joined.Local || origin.Local
		joined.MatchScoped = joined.MatchScoped || origin.MatchScoped
		joined.Mutable = joined.Mutable || origin.Mutable
		joined.Unknown = joined.Unknown || origin.Unknown
		if joined.Name != origin.Name {
			joined.Name = ""
		}
		for path := range origin.Contained {
			containedPaths[path] = true
		}
	}
	if joined.Local {
		for _, origin := range origins {
			if origin.Local {
				joined.Name, joined.Token = origin.Name, origin.Token
				break
			}
		}
	} else if joined.MatchScoped {
		for _, origin := range origins {
			if origin.MatchScoped {
				joined.Name, joined.Token = origin.Name, origin.Token
				break
			}
		}
	}
	joined = localOriginWithPlaces(joined, places)
	joined.Unknown = joined.Unknown || missing
	joined.Ambiguous = joined.Ambiguous || joined.Unknown
	if len(containedPaths) > 0 {
		joined.Contained = make(map[string]localReferenceOrigin, len(containedPaths))
		for path := range containedPaths {
			children := make([]localReferenceOrigin, 0, len(origins))
			childMissing := missing
			for _, origin := range origins {
				child, ok := origin.Contained[path]
				if !ok {
					childMissing = true
					continue
				}
				children = append(children, child)
			}
			joined.Contained[path] = mergeLocalReferenceOrigins(children, childMissing)
		}
	}
	return joined
}

func sameReferenceOrigin(left, right localReferenceOrigin) bool {
	if left.Name != right.Name || left.Local != right.Local || left.MatchScoped != right.MatchScoped || left.Mutable != right.Mutable || left.Unknown != right.Unknown {
		return false
	}
	leftPlaces := uniquePlaces(localOriginPlaces(left))
	rightPlaces := uniquePlaces(localOriginPlaces(right))
	if len(leftPlaces) != len(rightPlaces) {
		return false
	}
	for _, leftPlace := range leftPlaces {
		found := false
		for _, rightPlace := range rightPlaces {
			if samePlaceIdentity(leftPlace, rightPlace) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(left.Contained) != len(right.Contained) {
		return false
	}
	for path, leftChild := range left.Contained {
		rightChild, ok := right.Contained[path]
		if !ok || !sameReferenceOrigin(leftChild, rightChild) {
			return false
		}
	}
	return true
}

func samePlaceIdentity(left, right Place) bool {
	if left.Root != right.Root || len(left.Projections) != len(right.Projections) {
		return false
	}
	for index := range left.Projections {
		leftProjection := left.Projections[index]
		rightProjection := right.Projections[index]
		if leftProjection.Kind != rightProjection.Kind || leftProjection.Name != rightProjection.Name ||
			leftProjection.ConstantIndex != rightProjection.ConstantIndex || leftProjection.DynamicIndex != rightProjection.DynamicIndex ||
			leftProjection.SliceStart != rightProjection.SliceStart || leftProjection.SliceEnd != rightProjection.SliceEnd ||
			leftProjection.SliceStartKnown != rightProjection.SliceStartKnown || leftProjection.SliceEndKnown != rightProjection.SliceEndKnown {
			return false
		}
	}
	return true
}

func (a *Analyzer) checkLoopBackedgeFixedPoint(
	condition ast.Expression,
	body *ast.BlockStatement,
	entry loopIterationAnalysisState,
	headerMoved map[string]lexer.Token,
	headerReasons map[string]string,
	headerClosedResources map[string]lexer.Token,
	headerBorrows map[string][]borrowRecord,
	headerLocalRefContainers map[string]localReferenceOrigin,
) {
	backedgePlaces := map[string]bool{}
	for place := range headerMoved {
		if _, existedAtEntry := entry.moved[place]; !existedAtEntry {
			backedgePlaces[place] = true
		}
	}
	if len(backedgePlaces) == 0 && tokenStateKeysEqual(entry.closedResources, headerClosedResources) && borrowStatesEqual(entry.borrows, headerBorrows) && referenceOriginStatesEqual(entry.localRefContainers, headerLocalRefContainers) {
		return
	}

	previousSymbols := a.symbols
	previousCompletionSymbols := a.completionSymbols
	previousDefinitionTokens := a.definitionTokens
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousScopeDepth := a.scopeDepth
	previousLoopBackedgePlaces := a.loopBackedgePlaces
	previousCallGraph := a.callGraph
	previousCallGraphPathReachable := a.callGraphPathReachable
	previousWarningCount := len(a.warnings)

	a.symbols = copySymbols(entry.symbols)
	a.completionSymbols = copySymbols(entry.completionSymbols)
	a.definitionTokens = copyDefinitionTokens(entry.definitionTokens)
	a.constInts = copyConstInts(entry.constInts)
	a.assigned = copyAssigned(entry.assigned)
	a.moved = copyMoved(headerMoved)
	a.moveReasons = copyMoveReasons(headerReasons)
	a.closedResources = copyMoved(headerClosedResources)
	a.borrows = copyBorrows(headerBorrows)
	a.localRefContainers = copyLocalRefContainers(headerLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(entry.arenaGenerations)
	a.scopeDepth = entry.scopeDepth
	a.loopBackedgePlaces = backedgePlaces
	a.callGraph = previousCallGraph.clone()
	a.callGraphPathReachable = previousCallGraphPathReachable
	frame := a.pushLoopBreakFrame()

	conditionErrorCount := len(a.errors)
	if condition != nil {
		a.inferExpression(condition)
	}
	if len(a.errors) == conditionErrorCount {
		a.analyzeBlockStatements(body)
	}
	a.popLoopBreakFrame(frame)

	a.symbols = previousSymbols
	a.completionSymbols = previousCompletionSymbols
	a.definitionTokens = previousDefinitionTokens
	a.constInts = previousConstInts
	a.assigned = previousAssigned
	a.moved = previousMoved
	a.moveReasons = previousMoveReasons
	a.closedResources = previousClosedResources
	a.borrows = previousBorrows
	a.localRefContainers = previousLocalRefContainers
	a.arenaGenerations = previousArenaGenerations
	a.scopeDepth = previousScopeDepth
	a.loopBackedgePlaces = previousLoopBackedgePlaces
	a.callGraph = previousCallGraph
	a.callGraphPathReachable = previousCallGraphPathReachable
	a.warnings = a.warnings[:previousWarningCount]
}

func tokenStateKeysEqual(left, right map[string]lexer.Token) bool {
	if len(left) != len(right) {
		return false
	}
	for name := range left {
		if _, ok := right[name]; !ok {
			return false
		}
	}
	return true
}

func borrowStatesEqual(left, right map[string][]borrowRecord) bool {
	if len(left) != len(right) {
		return false
	}
	for root, leftRecords := range left {
		rightRecords, ok := right[root]
		if !ok || len(leftRecords) != len(rightRecords) {
			return false
		}
		for _, record := range leftRecords {
			if !containsBorrowRecord(rightRecords, record) {
				return false
			}
		}
	}
	return true
}

func referenceOriginStatesEqual(left, right map[string]localReferenceOrigin) bool {
	if len(left) != len(right) {
		return false
	}
	for name, leftOrigin := range left {
		rightOrigin, ok := right[name]
		if !ok || leftOrigin.Ambiguous != rightOrigin.Ambiguous || !sameReferenceOrigin(leftOrigin, rightOrigin) {
			return false
		}
	}
	return true
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

func mergeLoopArenaGenerations(before, loop map[string]int, breaks []map[string]int) map[string]int {
	merged := copyArenaGenerations(before)
	mergeArenaGenerationMaxInto(merged, loop)
	for _, breakGenerations := range breaks {
		mergeArenaGenerationMaxInto(merged, breakGenerations)
	}
	return merged
}

func mergeArenaGenerationMaxInto(merged, next map[string]int) {
	for name, generation := range next {
		if current, ok := merged[name]; !ok || generation > current {
			merged[name] = generation
		}
	}
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

func (a *Analyzer) analyzeInterfaceDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		if iface, ok := stmt.(*ast.InterfaceDeclaration); ok {
			a.analyzeInterfaceDeclaration(iface)
		}
	})
	a.validateInterfaceInheritanceCycles(program)
}

type interfaceInheritanceEdge struct {
	Target string
	Token  lexer.Token
}

func (a *Analyzer) validateInterfaceInheritanceCycles(program *ast.Program) {
	if program == nil {
		return
	}

	edges := map[string][]interfaceInheritanceEdge{}
	order := []string{}
	for _, statement := range program.Statements {
		declaration, ok := statement.(*ast.InterfaceDeclaration)
		if !ok || declaration == nil || declaration.Name == nil {
			continue
		}
		name := declaration.Name.Value
		order = append(order, name)
		for _, parent := range declaration.Implements {
			if parent == nil {
				continue
			}
			parentType, exists := a.types[parent.Name]
			if !exists || parentType.Kind != InterfaceType {
				continue
			}
			edges[name] = append(edges[name], interfaceInheritanceEdge{Target: parentType.Name, Token: parent.Token})
		}
	}

	const (
		interfaceUnvisited uint8 = iota
		interfaceVisiting
		interfaceVisited
	)
	state := map[string]uint8{}
	stack := []string{}
	stackIndex := map[string]int{}

	var visit func(string) bool
	visit = func(name string) bool {
		state[name] = interfaceVisiting
		stackIndex[name] = len(stack)
		stack = append(stack, name)
		reachesCycle := false

		for _, edge := range edges[name] {
			switch state[edge.Target] {
			case interfaceUnvisited:
				if visit(edge.Target) {
					reachesCycle = true
				}
			case interfaceVisiting:
				start := stackIndex[edge.Target]
				cycle := append([]string(nil), stack[start:]...)
				cycle = append(cycle, edge.Target)
				for _, member := range cycle[:len(cycle)-1] {
					a.invalidInterfaceInheritance[member] = true
				}
				a.addErrorAtTokenWithMetadata(
					edge.Token,
					diagnostics.InterfaceInheritanceCycle,
					"remove one implements relationship from the cycle",
					"interface inheritance cycle: %s",
					strings.Join(cycle, " -> "),
				)
				reachesCycle = true
			case interfaceVisited:
				if a.invalidInterfaceInheritance[edge.Target] {
					reachesCycle = true
				}
			}
		}

		stack = stack[:len(stack)-1]
		delete(stackIndex, name)
		state[name] = interfaceVisited
		if reachesCycle {
			a.invalidInterfaceInheritance[name] = true
		}
		return reachesCycle
	}

	for _, name := range order {
		if state[name] == interfaceUnvisited {
			visit(name)
		}
	}
}

func (a *Analyzer) analyzeInterfaceDeclaration(stmt *ast.InterfaceDeclaration) {
	if stmt == nil || stmt.Name == nil {
		return
	}

	if len(stmt.GenericParameters) > 0 {
		a.validateGenericParameterConstraints(stmt.GenericParameters)
		a.withGenericTypeParameters(stmt.GenericParameters, func() {
			a.analyzeInterfaceDeclarationBody(stmt)
		})
		return
	}
	a.analyzeInterfaceDeclarationBody(stmt)
}

func (a *Analyzer) analyzeInterfaceDeclarationBody(stmt *ast.InterfaceDeclaration) {
	iface := Type{
		Name:              stmt.Name.Value,
		Module:            a.currentModule,
		Kind:              InterfaceType,
		Named:             true,
		Declared:          true,
		Underlying:        "interface",
		GenericParameters: genericParameterNameValues(stmt.GenericParameters),
	}

	iface.Implements = a.resolveImplementedInterfaces(stmt.Implements, stmt.Name.Value)

	seenMethods := map[string]lexer.Token{}
	for _, method := range stmt.Methods {
		if method == nil || method.Name == nil {
			continue
		}
		if previous, exists := seenMethods[method.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(method.Name.Token, "duplicate interface method %q in %s", method.Name.Value, stmt.Name.Value)
			continue
		}
		seenMethods[method.Name.Value] = method.Name.Token
		iface.InterfaceMethods = append(iface.InterfaceMethods, a.interfaceMethodRequirement(stmt.Name.Value, method))
	}

	seenProperties := map[string]lexer.Token{}
	for _, property := range stmt.Properties {
		if property == nil || property.Name == nil {
			continue
		}
		if previous, exists := seenProperties[property.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(property.Name.Token, "duplicate interface property %q in %s", property.Name.Value, stmt.Name.Value)
			continue
		}
		seenProperties[property.Name.Value] = property.Name.Token
		propertyType, ok := a.resolveType(property.Type)
		if !ok {
			continue
		}
		iface.InterfaceProperties = append(iface.InterfaceProperties, InterfaceProperty{
			Name:        property.Name.Value,
			Type:        propertyType,
			Token:       property.Name.Token,
			RequiresGet: property.RequiresGet,
			RequiresSet: property.RequiresSet,
		})
	}
	seenEvents := map[string]lexer.Token{}
	for _, event := range stmt.Events {
		if event == nil || event.Name == nil {
			continue
		}
		if previous, exists := seenEvents[event.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(event.Name.Token, "duplicate interface event %q in %s", event.Name.Value, stmt.Name.Value)
			continue
		}
		seenEvents[event.Name.Value] = event.Name.Token
		payload, ok := a.resolveType(event.Payload)
		if !ok {
			continue
		}
		iface.InterfaceEvents = append(iface.InterfaceEvents, InterfaceEvent{
			Name:    event.Name.Value,
			Payload: payload,
			Token:   event.Name.Token,
		})
	}

	a.mergeInheritedInterfaceRequirements(&iface)
	a.types[stmt.Name.Value] = iface
}

func (a *Analyzer) mergeInheritedInterfaceRequirements(iface *Type) {
	if iface == nil {
		return
	}

	methods := map[string]Function{}
	for _, method := range iface.InterfaceMethods {
		methods[method.Name] = method
	}
	properties := map[string]InterfaceProperty{}
	for _, property := range iface.InterfaceProperties {
		properties[property.Name] = property
	}
	events := map[string]InterfaceEvent{}
	for _, event := range iface.InterfaceEvents {
		events[event.Name] = event
	}

	for _, parent := range iface.Implements {
		parentType, ok := a.types[parent.Name]
		if !ok || parentType.Kind != InterfaceType {
			continue
		}
		for _, method := range parentType.InterfaceMethods {
			if existing, exists := methods[method.Name]; exists {
				if !sameInterfaceRequirementSignature(existing, method) || !sameConcreteType(existing.ReturnType, method.ReturnType) {
					a.addErrorAtToken(method.Token, "inherited interface method %s conflicts in %s", method.Name, iface.Name)
				}
				continue
			}
			methods[method.Name] = method
			iface.InterfaceMethods = append(iface.InterfaceMethods, method)
		}
		for _, property := range parentType.InterfaceProperties {
			if existing, exists := properties[property.Name]; exists {
				if !sameConcreteType(existing.Type, property.Type) || (property.RequiresGet && !existing.RequiresGet) || (property.RequiresSet && !existing.RequiresSet) {
					a.addErrorAtToken(property.Token, "inherited interface property %s conflicts in %s", property.Name, iface.Name)
				}
				continue
			}
			properties[property.Name] = property
			iface.InterfaceProperties = append(iface.InterfaceProperties, property)
		}
		for _, event := range parentType.InterfaceEvents {
			if existing, exists := events[event.Name]; exists {
				if !sameConcreteType(existing.Payload, event.Payload) {
					a.addErrorAtToken(event.Token, "inherited interface event %s conflicts in %s", event.Name, iface.Name)
				}
				continue
			}
			events[event.Name] = event
			iface.InterfaceEvents = append(iface.InterfaceEvents, event)
		}
	}
}

func (a *Analyzer) interfaceMethodRequirement(interfaceName string, fn *ast.FunctionDeclaration) Function {
	function := Function{
		Name:   fn.Name.Value,
		Module: a.currentModule,
		Token:  fn.Name.Token,
	}
	a.withImplTarget(interfaceName, func() {
		for _, param := range fn.Parameters {
			paramType, ok := a.resolveType(param.Type)
			if !ok {
				continue
			}
			function.Parameters = append(function.Parameters, FunctionParameter{
				Name:       param.Name.Value,
				Type:       paramType,
				Token:      param.Name.Token,
				Ref:        param.Ref,
				MutableRef: param.MutableRef,
			})
		}
		returnType, ok := a.resolveType(fn.ReturnType)
		if ok {
			function.ReturnType = returnType
		} else {
			function.ReturnType = Type{Kind: InvalidType}
		}
	})
	return function
}

func sameInterfaceRequirementSignature(left Function, right Function) bool {
	if len(left.Parameters) != len(right.Parameters) {
		return false
	}
	for i := range left.Parameters {
		if left.Parameters[i].Ref != right.Parameters[i].Ref || left.Parameters[i].MutableRef != right.Parameters[i].MutableRef {
			return false
		}
		if isSelfParameter(left.Parameters[i]) && isSelfParameter(right.Parameters[i]) {
			if left.Parameters[i].Type.Kind != ReferenceType || right.Parameters[i].Type.Kind != ReferenceType {
				return false
			}
			continue
		}
		if !sameConcreteType(left.Parameters[i].Type, right.Parameters[i].Type) {
			return false
		}
	}
	return true
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
		unit = UnitDefinition{Name: unitName, Category: OtherUnit, Dimension: dimensionFromBase(unitName, 1), DefaultNumeric: "decimal", Status: StatusActive, Token: stmt.Name.Token}
	}
	if stmt.Category != "" {
		unit.Category = UnitCategory(stmt.Category)
	}
	unit.DefaultNumeric = baseType.Name
	if unit.Status == "" {
		unit.Status = StatusActive
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
		typ := a.typeFromUnionDeclaration(stmt.Name.Value, stmt)
		typ.Implements = a.resolveImplementedInterfaces(stmt.Implements, stmt.Name.Value)
		a.types[stmt.Name.Value] = typ
		return
	}

	if stmt.StructType != nil {
		typ := a.typeFromStructDeclaration(stmt)
		typ.Implements = a.resolveImplementedInterfaces(stmt.Implements, stmt.Name.Value)
		a.types[stmt.Name.Value] = typ
		return
	}
	if stmt.RegisterType != nil {
		typ := a.typeFromRegisterDeclaration(stmt.Name.Value, stmt)
		typ.Implements = a.resolveImplementedInterfaces(stmt.Implements, stmt.Name.Value)
		a.types[stmt.Name.Value] = typ
		return
	}
	if len(stmt.Variants) > 0 {
		a.types[stmt.Name.Value] = a.typeFromVariantDeclaration(stmt.Name.Value, stmt.Variants, stmt.Attributes)
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

	if baseTypeOK {
		a.checkContractLiteralBounds(baseType, stmt.Contract)
	}

	if baseTypeOK {
		typ := a.typeFromDeclaration(stmt, baseType)
		typ.Implements = a.resolveImplementedInterfaces(stmt.Implements, stmt.Name.Value)
		a.types[stmt.Name.Value] = typ
	}
}

func (a *Analyzer) resolveImplementedInterfaces(refs []*ast.TypeReference, targetName string) []Type {
	if len(refs) == 0 {
		return nil
	}

	implemented := []Type{}
	seen := map[string]lexer.Token{}
	for _, ref := range refs {
		typ, ok := a.resolveType(ref)
		if !ok {
			continue
		}
		if typ.Kind != InterfaceType {
			a.addErrorAtToken(ref.Token, "implemented type %s on %s is not an interface", typeDisplayName(typ), targetName)
			continue
		}
		if previous, exists := seen[typ.Name]; exists {
			_ = previous
			a.addErrorAtToken(ref.Token, "duplicate implemented interface %s on %s", typeDisplayName(typ), targetName)
			continue
		}
		seen[typ.Name] = ref.Token
		implemented = append(implemented, typ)
	}
	return implemented
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
		a.types[qualifiedName] = a.typeFromVariantDeclaration(qualifiedName, stmt.Variants, stmt.Attributes)
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

	if baseTypeOK {
		a.checkContractLiteralBounds(baseType, stmt.Contract)
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
	noCopy := hasAttribute(stmt.Attributes, "noCopy")
	typ := Type{
		Name:                  name,
		Module:                a.currentModule,
		Kind:                  StructType,
		Named:                 true,
		Declared:              true,
		ExplicitlyNonCopyable: noCopy,
		Underlying:            "struct",
		GenericParameters:     genericParameterNameValues(stmt.GenericParameters),
	}
	if noCopy {
		typ.NoCopyPolicyOrigin = name
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
		if isEventType(fieldType) {
			event := eventFromField(name, field.Name.Value, fieldType, field.Name.Token)
			typ.Events = append(typ.Events, event)
		}
		if isBareSliceType(fieldType) {
			a.addErrorAtToken(field.Type.Token, "bare slice type %s must be used behind ref", typeDisplayName(fieldType))
			continue
		}
		if field.Contract != nil {
			a.checkContractLiteralBounds(fieldType, field.Contract)
			fieldType = a.applyContracts(fieldType, field.Contract)
		}

		typ.Fields = append(typ.Fields, StructField{
			Name:  field.Name.Value,
			Type:  fieldType,
			Token: field.Name.Token,
			Tags:  semaStructTags(field.Tags),
		})
		a.recordDefinition(field.Name.Token)
	}

	return typ
}

func (a *Analyzer) typeFromRegisterDeclaration(name string, stmt *ast.TypeDeclStatement) Type {
	noCopy := hasAttribute(stmt.Attributes, "noCopy")
	typ := Type{
		Name:                  name,
		Module:                a.currentModule,
		Kind:                  RegisterType,
		Named:                 true,
		Declared:              true,
		ExplicitlyNonCopyable: noCopy,
		Underlying:            "register",
		RegisterWidth:         stmt.RegisterType.Width,
		GenericParameters:     genericParameterNameValues(stmt.GenericParameters),
	}
	if noCopy {
		typ.NoCopyPolicyOrigin = name
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
		fieldWidth := field.Width
		fieldType := Type{}
		if field.Type != nil {
			if field.Name.Value == "_" {
				a.addErrorAtToken(field.Type.Token, "reserved register field _ must use bit or bit[N]")
				invalidFieldWidth = true
				continue
			}
			resolved, ok := a.resolveType(field.Type)
			if !ok {
				invalidFieldWidth = true
				continue
			}
			if resolved.Kind != EnumType || resolved.BitWidth <= 0 {
				a.addErrorAtToken(field.Type.Token, "register field %s.%s type must be bit or a bit-backed enum, got %s", name, field.Name.Value, typeDisplayName(resolved))
				invalidFieldWidth = true
				continue
			}
			fieldWidth = resolved.BitWidth
			fieldType = resolved
		}
		if fieldWidth <= 0 {
			a.addErrorAtToken(field.Token, "register field %s.%s width must be positive", name, field.Name.Value)
			invalidFieldWidth = true
			continue
		}
		used += fieldWidth

		if field.Name.Value != "_" {
			if previous, exists := seen[field.Name.Value]; exists {
				_ = previous
				a.addErrorAtToken(field.Name.Token, "duplicate register field %q in %s", field.Name.Value, name)
				continue
			}
			seen[field.Name.Value] = field.Name.Token
		}

		if field.Type == nil {
			fieldType = a.registerFieldType(field)
		}
		if field.Unit != "" {
			if _, ok := a.units[field.Unit]; !ok {
				a.addErrorAtToken(field.Token, "unknown unit %s on register field %s.%s", field.Unit, name, field.Name.Value)
			}
		}

		typ.RegisterFields = append(typ.RegisterFields, RegisterField{
			Name:  field.Name.Value,
			Width: fieldWidth,
			Unit:  field.Unit,
			Type:  fieldType,
			Token: field.Name.Token,
		})
		a.recordDefinition(field.Name.Token)
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
			typ.MinInteger = new(big.Int).SetUint64(min)
			typ.MaxInteger = new(big.Int).SetUint64(max)
		} else {
			typ.Named = true
			typ.Unit = field.Unit
			typ.Dimension = a.parseDimension(field.Unit)
		}
	}
	return typ
}

func (a *Analyzer) typeFromUnionDeclaration(name string, stmt *ast.TypeDeclStatement) Type {
	noCopy := hasAttribute(stmt.Attributes, "noCopy")
	typ := Type{
		Name:                  name,
		Module:                a.currentModule,
		Kind:                  UnionType,
		Named:                 true,
		Declared:              true,
		ExplicitlyNonCopyable: noCopy,
		Underlying:            "union",
		GenericParameters:     genericParameterNameValues(stmt.GenericParameters),
	}
	if noCopy {
		typ.NoCopyPolicyOrigin = name
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
		a.recordDefinition(variant.Name.Token)
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

func (a *Analyzer) typeFromVariantDeclaration(name string, variants []*ast.Identifier, attributes []*ast.Attribute) Type {
	enum := &ast.EnumDeclaration{
		Token:      lexer.Token{Type: lexer.ENUM, Lexeme: "enum"},
		Attributes: attributes,
		Name:       &ast.Identifier{Token: variants[0].Token, Value: name},
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
	if enum.BitUnderlying {
		width := enum.UnderlyingBitWidth
		if width <= 0 || width > 256 {
			a.addErrorAtToken(enum.Name.Token, "enum %s bit width must be between 1 and 256, got %d", name, width)
			return Type{Name: name, Kind: InvalidType, Named: true, Declared: true}
		}
		max := new(big.Int).Lsh(big.NewInt(1), uint(width))
		max.Sub(max, big.NewInt(1))
		underlying = Type{
			Name:       fmt.Sprintf("bit[%d]", width),
			Kind:       UintType,
			MinInteger: big.NewInt(0),
			MaxInteger: max,
		}
	} else if enum.UnderlyingType != nil {
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

	noCopy := hasAttribute(enum.Attributes, "noCopy")
	typ := Type{
		Name:                  name,
		Module:                a.currentModule,
		Kind:                  EnumType,
		Named:                 true,
		Declared:              true,
		ExplicitlyNonCopyable: noCopy,
		Underlying:            underlying.Name,
		MinInteger:            new(big.Int).Set(underlying.MinInteger),
		MaxInteger:            new(big.Int).Set(underlying.MaxInteger),
		EnumConsts:            map[string]EnumValue{},
	}
	if noCopy {
		typ.NoCopyPolicyOrigin = name
	}
	if enum.BitUnderlying {
		typ.BitWidth = enum.UnderlyingBitWidth
	}

	seen := map[string]lexer.Token{}
	previous := big.NewInt(-1)
	for i, value := range enum.Values {
		if previousToken, exists := seen[value.Name.Value]; exists {
			a.addErrorAtTokenWithPrevious(value.Token, previousToken, "duplicate enum value %q in enum %s", value.Name.Value, name)
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
		a.recordDefinition(value.Name.Token)
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
	a.withProgramModules(program, func(stmt ast.Statement) {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			return
		}
		a.registerImplStatement(impl)
	})
	a.resolveImplMethodReceiverMutability(program)
}

type implMethodDeclaration struct {
	Target Type
	Name   string
	Source *ast.FunctionDeclaration
}

// resolveImplMethodReceiverMutability computes receiver mutability after every
// impl method has been registered. Method declarations may be in any source
// order, and a mutating receiver requirement can propagate through calls.
func (a *Analyzer) resolveImplMethodReceiverMutability(program *ast.Program) {
	if program == nil {
		return
	}

	methods := []implMethodDeclaration{}
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || !a.validImplStatements[impl] || impl.Target == nil {
			continue
		}
		target, ok := a.types[impl.Target.Name]
		if !ok {
			continue
		}
		for _, member := range impl.Members {
			fn, ok := member.(*ast.FunctionDeclaration)
			if !ok || fn == nil || fn.Name == nil {
				continue
			}
			name := target.Name + "." + fn.Name.Value
			if len(a.functions[name]) == 0 {
				continue
			}
			methods = append(methods, implMethodDeclaration{Target: target, Name: name, Source: fn})
		}
	}

	for _, method := range methods {
		a.setImplMethodReceiverMutable(method.Name, method.Target.Name, false)
	}
	for changed := true; changed; {
		changed = false
		for _, method := range methods {
			if !a.functionBodyWritesTargetMember(method.Source.Body, method.Target, functionParameterNames(method.Source)) {
				continue
			}
			if a.implMethodReceiverMutable(method.Name, method.Target.Name) {
				continue
			}
			a.setImplMethodReceiverMutable(method.Name, method.Target.Name, true)
			changed = true
		}
	}
}

func (a *Analyzer) implMethodReceiverMutable(name string, targetName string) bool {
	for _, function := range a.functions[name] {
		if function.ImplTarget == targetName && function.ReceiverMutable {
			return true
		}
	}
	return false
}

func (a *Analyzer) setImplMethodReceiverMutable(name string, targetName string, mutable bool) {
	functions := a.functions[name]
	for index := range functions {
		if functions[index].ImplTarget == targetName {
			functions[index].ReceiverMutable = mutable
		}
	}
	a.functions[name] = functions
}

func (a *Analyzer) registerImplStatement(stmt *ast.ImplStatement) {
	if !a.validImplStatements[stmt] {
		return
	}
	target, ok := a.types[stmt.Target.Name]
	if !ok {
		return
	}

	if !target.Named && target.Kind != InvalidType && !isAllowedCoreBuiltinImpl(stmt.Target.Name, stmt.Target.Token) {
		return
	}
	if !a.validateImplGenericTarget(stmt, target) {
		return
	}
	genericParams := implGenericParametersForTarget(stmt, target)

	fields := map[string]lexer.Token{}
	for _, field := range target.Fields {
		fields[field.Name] = field.Token
	}
	methods := map[string]lexer.Token{}
	properties := map[string]lexer.Token{}
	for _, property := range target.Properties {
		properties[property.Name] = property.Token
	}
	events := map[string]lexer.Token{}
	for _, event := range target.Events {
		events[event.Name] = event.Token
	}

	targetChanged := false
	for _, member := range stmt.Members {
		if invalid, ok := member.(*ast.InvalidStatement); ok {
			if invalid.Message != "" {
				a.addErrorAtToken(invalid.Token, "%s", invalid.Message)
			}
			continue
		}
		if invalid, ok := member.(*ast.InvalidMember); ok {
			if invalid.Message != "" {
				a.addErrorAtToken(invalid.Token, "%s", invalid.Message)
			}
			continue
		}
		if _, ok := member.(*ast.UnitMetadataDeclaration); ok {
			continue
		}
		if let, ok := member.(*ast.LetStatement); ok {
			if !let.Static {
				a.addErrorAtToken(let.Token, "variable declarations inside impl must be static")
				continue
			}
			a.analyzeImplStaticLet(stmt.Target.Name, let)
			continue
		}

		if fn, ok := member.(*ast.FunctionDeclaration); ok {
			name := fn.Name.Value
			if _, exists := fields[name]; exists {
				a.addErrorAtToken(fn.Name.Token, "method %s conflicts with field %s in %s", name, name, stmt.Target.Name)
				continue
			}
			if _, exists := properties[name]; exists {
				a.addErrorAtToken(fn.Name.Token, "method %s conflicts with property %s in %s", name, name, stmt.Target.Name)
				continue
			}
			if _, exists := events[name]; exists {
				a.addErrorAtToken(fn.Name.Token, "method %s conflicts with event %s in %s", name, name, stmt.Target.Name)
				continue
			}
			if _, exists := a.types[stmt.Target.Name+"."+name]; exists {
				a.addErrorAtToken(fn.Name.Token, "method %s conflicts with nested type %s in %s", name, name, stmt.Target.Name)
				continue
			}
			methods[name] = fn.Name.Token
			if len(fn.GenericParameters) > 0 {
				a.addErrorAtToken(fn.Name.Token, "generic methods with additional type parameters are not supported yet")
				continue
			}
			a.withImplTarget(stmt.Target.Name, func() {
				a.withGenericTypeParameters(genericParams, func() {
					a.registerFunctionDeclarationNamed(fn, stmt.Target.Name+"."+fn.Name.Value)
					if a.isUnitConversionFunctionTarget(stmt.Target.Name, fn) && a.validateUnitConversionDimensions(stmt.Target.Name, fn) {
						a.registerFunctionDeclarationNamed(fn, fn.Name.Value)
					}
				})
			})
			continue
		}

		if event, ok := member.(*ast.EventDeclaration); ok {
			a.registerImplEventDeclaration(stmt.Target.Name, &target, event, fields, methods, properties, events)
			targetChanged = true
			continue
		}

		property, ok := member.(*ast.PropertyDeclaration)
		if !ok {
			continue
		}

		if _, exists := fields[property.Name.Value]; exists {
			a.addErrorAtToken(property.Name.Token, "property %s conflicts with field %s in %s", property.Name.Value, property.Name.Value, stmt.Target.Name)
			continue
		}
		if previous, exists := properties[property.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(property.Name.Token, "duplicate property %q in impl %s", property.Name.Value, stmt.Target.Name)
			continue
		}
		if _, exists := events[property.Name.Value]; exists {
			a.addErrorAtToken(property.Name.Token, "property %s conflicts with event %s in %s", property.Name.Value, property.Name.Value, stmt.Target.Name)
			continue
		}
		if _, exists := methods[property.Name.Value]; exists || len(a.functions[stmt.Target.Name+"."+property.Name.Value]) > 0 {
			a.addErrorAtToken(property.Name.Token, "property %s conflicts with method %s in %s", property.Name.Value, property.Name.Value, stmt.Target.Name)
			continue
		}
		if _, exists := a.types[stmt.Target.Name+"."+property.Name.Value]; exists {
			a.addErrorAtToken(property.Name.Token, "property %s conflicts with nested type %s in %s", property.Name.Value, property.Name.Value, stmt.Target.Name)
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
			Name:      property.Name.Value,
			Type:      propertyType,
			Token:     property.Name.Token,
			Fallible:  property.Setter != nil && property.Setter.Fallible,
			Error:     errorType,
			HasGetter: property.Getter != nil,
			HasSetter: property.Setter != nil,
		})
		a.recordDefinition(property.Name.Token)
		targetChanged = true
	}

	if targetChanged {
		a.types[target.Name] = target
	}
}

func (a *Analyzer) analyzeImplStaticLet(targetName string, stmt *ast.LetStatement) {
	if stmt == nil || stmt.Name == nil {
		return
	}
	qualifiedName := targetName + "." + stmt.Name.Value
	var declaredType Type
	var ok bool
	if stmt.Type != nil {
		a.withImplTarget(targetName, func() {
			declaredType, ok = a.resolveType(stmt.Type)
		})
	} else if stmt.Value != nil {
		declaredType, _ = a.inferExpression(stmt.Value)
		ok = declaredType.Kind != InvalidType
	}
	if !ok {
		return
	}
	if stmt.Value == nil && !stmt.Mutable {
		a.addErrorAtToken(stmt.Name.Token, "immutable variable %s requires initializer", stmt.Name.Value)
		return
	}
	if stmt.Value != nil && stmt.Type != nil {
		valueType, _ := a.inferExpressionWithExpected(stmt.Value, declaredType)
		if valueType.Kind != InvalidType && !canInitialize(declaredType, valueType, stmt.Value) {
			a.addErrorAtToken(expressionToken(stmt.Value), "cannot initialize %s with %s", typeDisplayName(declaredType), typeDisplayName(valueType))
			return
		}
	}
	if previous, exists := a.symbols[qualifiedName]; exists {
		a.addErrorAtToken(stmt.Name.Token, "static member %s already declared at %d:%d, previous declaration at %d:%d", qualifiedName, stmt.Name.Token.Line, stmt.Name.Token.Column, previous.Token.Line, previous.Token.Column)
		return
	}
	symbol := Symbol{Name: qualifiedName, Type: declaredType, Mutable: stmt.Mutable, Token: stmt.Name.Token, Storage: StorageOriginStatic, Local: false}
	a.symbols[qualifiedName] = symbol
	a.completionSymbols[qualifiedName] = symbol
	a.assigned[qualifiedName] = stmt.Value != nil
	a.recordDefinition(stmt.Name.Token)
}

func (a *Analyzer) registerImplEventDeclaration(targetName string, target *Type, event *ast.EventDeclaration, fields map[string]lexer.Token, methods map[string]lexer.Token, properties map[string]lexer.Token, events map[string]lexer.Token) {
	if event == nil || event.Name == nil {
		return
	}
	name := event.Name.Value
	if _, exists := fields[name]; exists {
		a.addErrorAtToken(event.Name.Token, "event %s conflicts with field %s in %s", name, name, targetName)
		return
	}
	if _, exists := methods[name]; exists || len(a.functions[targetName+"."+name]) > 0 {
		a.addErrorAtToken(event.Name.Token, "event %s conflicts with method %s in %s", name, name, targetName)
		return
	}
	if _, exists := properties[name]; exists {
		a.addErrorAtToken(event.Name.Token, "event %s conflicts with property %s in %s", name, name, targetName)
		return
	}
	if _, exists := a.types[targetName+"."+name]; exists {
		a.addErrorAtToken(event.Name.Token, "event %s conflicts with nested type %s in %s", name, name, targetName)
		return
	}
	if previous, exists := events[name]; exists {
		_ = previous
		a.addErrorAtToken(event.Name.Token, "duplicate event %q in impl %s", name, targetName)
		return
	}
	if event.Storage == nil {
		a.addErrorAtToken(event.Token, "event %s must specify storage with using", name)
		return
	}
	storageType, ok := lookupStructField(*target, event.Storage.Value)
	if !ok {
		a.addErrorAtToken(event.Storage.Token, "event %s uses unknown storage field %s", name, event.Storage.Value)
		return
	}
	if !isEventStorageType(storageType) {
		a.addErrorAtToken(event.Storage.Token, "event %s storage field %s must be EventStorage, got %s", name, event.Storage.Value, typeDisplayName(storageType))
		return
	}
	eventType := eventTypeFromStorage(storageType)
	target.Events = append(target.Events, Event{
		Name:          name,
		Type:          eventType,
		Payload:       storageType.TypeArgs[0],
		Capacity:      eventCapacity(storageType),
		Token:         event.Name.Token,
		Owner:         targetName,
		Storage:       event.Storage.Value,
		StorageBacked: true,
	})
	a.recordDefinition(event.Name.Token)
	events[name] = event.Name.Token
}

func (a *Analyzer) isUnitConversionFunctionTarget(targetName string, fn *ast.FunctionDeclaration) bool {
	if fn == nil || fn.Name == nil || fn.Name.Value != targetName {
		return false
	}
	target, ok := a.types[targetName]
	if !ok || target.Unit != targetName || !isNumericType(target) {
		return false
	}
	return true
}

func (a *Analyzer) validateUnitConversionDimensions(targetName string, fn *ast.FunctionDeclaration) bool {
	if fn == nil || len(fn.Parameters) != 1 || fn.Parameters[0] == nil || fn.Parameters[0].Type == nil {
		return true
	}
	target, targetOK := a.types[targetName]
	source, sourceOK := a.resolveType(fn.Parameters[0].Type)
	if !targetOK || !sourceOK || source.Unit == "" || source.Unit == targetName {
		return true
	}
	if target.Dimension.Equal(source.Dimension) {
		return true
	}

	a.addErrorAtTokenWithMetadata(
		fn.Parameters[0].Type.Token,
		diagnostics.IncompatibleUnitConversion,
		"Use a source unit with the same normalized dimension, or correct the Dimension metadata on the source or target unit.",
		"unit conversion %s from %s requires compatible dimensions",
		targetName,
		typeDisplayName(source),
	)
	return false
}

func (a *Analyzer) validateInterfaceConformance() {
	names := make([]string, 0, len(a.types))
	for name := range a.types {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		typ := a.types[name]
		if typ.Kind == InvalidType || len(typ.Implements) == 0 {
			continue
		}
		if typ.Kind == InterfaceType {
			continue
		}
		for _, iface := range typ.Implements {
			if a.invalidInterfaceInheritance[iface.Name] {
				continue
			}
			a.validateTypeImplementsInterface(typ, iface)
		}
	}
}

func (a *Analyzer) validateTypeImplementsInterface(typ Type, iface Type) {
	for _, required := range iface.InterfaceMethods {
		methods := a.functions[typ.Name+"."+required.Name]
		if len(methods) == 0 {
			a.addErrorAtToken(required.Token, "type %s implements %s but is missing method %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !hasCompatibleInterfaceMethod(typ, iface, methods, required) {
			a.addErrorAtToken(required.Token, "type %s method %s does not match interface %s", typ.Name, required.Name, iface.Name)
		}
	}

	for _, required := range iface.InterfaceProperties {
		property, ok := lookupProperty(typ, required.Name)
		if !ok {
			a.addErrorAtToken(required.Token, "type %s implements %s but is missing property %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !sameConcreteType(property.Type, required.Type) {
			a.addErrorAtToken(required.Token, "type %s property %s must be %s for interface %s, got %s", typ.Name, required.Name, typeDisplayName(required.Type), iface.Name, typeDisplayName(property.Type))
			continue
		}
		if required.RequiresGet && !property.HasGetter {
			a.addErrorAtToken(required.Token, "type %s property %s must provide get for interface %s", typ.Name, required.Name, iface.Name)
		}
		if required.RequiresSet && !property.HasSetter {
			a.addErrorAtToken(required.Token, "type %s property %s must provide set for interface %s", typ.Name, required.Name, iface.Name)
		}
	}

	for _, required := range iface.InterfaceEvents {
		event, ok := lookupEvent(typ, required.Name)
		if !ok {
			a.addErrorAtToken(required.Token, "type %s implements %s but is missing event %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !sameConcreteType(event.Payload, required.Payload) {
			a.addErrorAtToken(required.Token, "type %s event %s payload must be %s for interface %s, got %s", typ.Name, required.Name, typeDisplayName(required.Payload), iface.Name, typeDisplayName(event.Payload))
		}
	}
}

func hasCompatibleInterfaceMethod(typ Type, iface Type, methods []Function, required Function) bool {
	for _, method := range methods {
		if compatibleInterfaceMethodSignature(method, required) && sameConcreteType(method.ReturnType, required.ReturnType) {
			return true
		}
	}
	return false
}

func compatibleInterfaceMethodSignature(method Function, required Function) bool {
	methodParams := explicitInterfaceComparableParameters(method.Parameters)
	requiredParams := explicitInterfaceComparableParameters(required.Parameters)
	if len(methodParams) != len(requiredParams) {
		return false
	}
	for i := range methodParams {
		if methodParams[i].Ref != requiredParams[i].Ref || methodParams[i].MutableRef != requiredParams[i].MutableRef {
			return false
		}
		if !sameConcreteType(methodParams[i].Type, requiredParams[i].Type) {
			return false
		}
	}
	return true
}

func explicitInterfaceComparableParameters(parameters []FunctionParameter) []FunctionParameter {
	if len(parameters) > 0 && isSelfParameter(parameters[0]) {
		return parameters[1:]
	}
	return parameters
}

func isSelfParameter(param FunctionParameter) bool {
	return param.Name == "self"
}

func (a *Analyzer) analyzeImplBodies(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok {
			return
		}
		a.analyzeImplBody(impl)
	})
}

func (a *Analyzer) analyzeImplBody(stmt *ast.ImplStatement) {
	if !a.validImplStatements[stmt] {
		return
	}
	target, ok := a.types[stmt.Target.Name]
	if !ok || (!target.Named && !isAllowedCoreBuiltinImpl(stmt.Target.Name, stmt.Target.Token)) {
		return
	}
	genericParams := implGenericParametersForTarget(stmt, target)

	for _, member := range stmt.Members {
		switch member := member.(type) {
		case *ast.FunctionDeclaration:
			a.withImplTarget(stmt.Target.Name, func() {
				a.withGenericTypeParameters(genericParams, func() {
					a.analyzeFunctionBodyNamed(member, stmt.Target.Name+"."+member.Name.Value)
				})
			})
		case *ast.PropertyDeclaration:
			if a.summaryPass {
				continue
			}
			registeredProperty, ok := lookupPropertyByToken(target, member.Name.Value, member.Name.Token)
			if !ok {
				continue
			}

			a.withImplTarget(stmt.Target.Name, func() {
				a.withGenericTypeParameters(genericParams, func() {
					a.analyzePropertyBody(target, member, registeredProperty.Type)
				})
			})
		}
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
	a.analyzePropertyAccessorBody(target, property.Name.Value+".get", property.Getter, propertyType, false, nil, Type{})
	if !a.blockDefinitelyReturns(property.Getter) {
		a.addErrorAtToken(property.Name.Token, "getter %s must return %s", property.Name.Value, typeDisplayName(propertyType))
	}
}

func (a *Analyzer) analyzeSetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if !property.Setter.Fallible && blockReturnsErr(property.Setter.Body) {
		a.addErrorAtToken(property.Setter.Token, "non-fallible setter %s cannot return Err", property.Name.Value)
	}
	returnType := Type{Name: "void", Kind: VoidType}
	if property.Setter.Fallible {
		errorType := Type{Kind: InvalidType}
		if registered, ok := lookupProperty(target, property.Name.Value); ok && registered.Error != nil {
			errorType = *registered.Error
		} else if inferred, ok := a.inferPropertySetterErrorType(target, property, propertyType); ok {
			errorType = inferred
		}
		if errorType.Kind != InvalidType {
			returnType = Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{{Name: "void", Kind: VoidType}, errorType}}
		}
	}
	a.analyzePropertyAccessorBody(target, property.Name.Value+".set", property.Setter.Body, returnType, true, property.Setter.Parameter, propertyType)
}

func (a *Analyzer) analyzePropertyAccessorBody(target Type, name string, body *ast.BlockStatement, returnType Type, mutableSelf bool, setterParameter *ast.Identifier, setterType Type) {
	if body == nil || returnType.Kind == InvalidType {
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousFunctionName := a.currentFunctionName
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousScopeDepth := a.scopeDepth
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = map[string]lexer.Token{}
	a.moveReasons = map[string]string{}
	a.borrows = map[string][]borrowRecord{}
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.currentFunctionName = name
	a.currentFunctionReturn = returnType
	a.inFunctionBody = true
	a.scopeDepth = 0
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.currentFunctionName = previousFunctionName
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.scopeDepth = previousScopeDepth
	}()

	a.defineInstanceSymbols(target, mutableSelf, body.Token)
	if setterParameter != nil {
		a.symbols[setterParameter.Value] = Symbol{Name: setterParameter.Value, Type: setterType, Mutable: false, Token: setterParameter.Token, Storage: StorageOriginInline, Local: true, ScopeDepth: 0}
		a.assigned[setterParameter.Value] = true
		delete(a.constInts, setterParameter.Value)
	}

	a.analyzeBlockStatements(body)
}

func blockReturnsErr(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if statementReturnsErr(stmt) {
			return true
		}
	}
	return false
}

func statementReturnsErr(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStatement:
		if stmt.Value == nil {
			return false
		}
		_, ok := stmt.Value.(*ast.ErrExpression)
		return ok
	case *ast.IfStatement:
		return blockReturnsErr(stmt.Consequence) || blockReturnsErr(stmt.Alternative)
	case *ast.SwitchStatement:
		for _, clause := range stmt.Cases {
			if clause != nil && blockReturnsErr(clause.Body) {
				return true
			}
		}
		return stmt.Default != nil && blockReturnsErr(stmt.Default.Body)
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch != nil && blockReturnsErr(branch.Body) {
				return true
			}
		}
	case *ast.UnsafeStatement:
		return blockReturnsErr(stmt.Body)
	}
	return false
}

func (a *Analyzer) inferPropertySetterErrorType(target Type, property *ast.PropertyDeclaration, propertyType Type) (Type, bool) {
	if property.Setter == nil || property.Setter.Body == nil {
		return Type{}, false
	}
	for _, stmt := range property.Setter.Body.Statements {
		errExpr := firstErrReturnExpression(stmt)
		if errExpr == nil || errExpr.Value == nil {
			continue
		}
		valueType, ok := a.inferPropertyBodyExpression(target, property.Setter, propertyType, errExpr.Value)
		if ok && valueType.Kind != InvalidType {
			return valueType, true
		}
	}
	return Type{}, false
}

func firstErrReturnExpression(stmt ast.Statement) *ast.ErrExpression {
	switch stmt := stmt.(type) {
	case *ast.ReturnStatement:
		if stmt.Value == nil {
			return nil
		}
		errExpr, _ := stmt.Value.(*ast.ErrExpression)
		return errExpr
	case *ast.IfStatement:
		if errExpr := firstErrReturnInBlock(stmt.Consequence); errExpr != nil {
			return errExpr
		}
		return firstErrReturnInBlock(stmt.Alternative)
	case *ast.SwitchStatement:
		for _, clause := range stmt.Cases {
			if clause == nil {
				continue
			}
			if errExpr := firstErrReturnInBlock(clause.Body); errExpr != nil {
				return errExpr
			}
		}
		if stmt.Default != nil {
			return firstErrReturnInBlock(stmt.Default.Body)
		}
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			if errExpr := firstErrReturnInBlock(branch.Body); errExpr != nil {
				return errExpr
			}
		}
	case *ast.UnsafeStatement:
		return firstErrReturnInBlock(stmt.Body)
	}
	return nil
}

func firstErrReturnInBlock(block *ast.BlockStatement) *ast.ErrExpression {
	if block == nil {
		return nil
	}
	for _, stmt := range block.Statements {
		if errExpr := firstErrReturnExpression(stmt); errExpr != nil {
			return errExpr
		}
	}
	return nil
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
		if typ, ok := a.inferPropertyBodyCallAsConversion(target, setter, setterType, expr); ok {
			return typ, typ.Kind != InvalidType
		}
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
		case "+":
			if isNumericType(rightType) {
				return rightType, true
			}
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
		if fieldType, ok := lookupRegisterField(objectType, expr.Property.Value); ok {
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

func (a *Analyzer) inferPropertyBodyCallAsConversion(target Type, setter *ast.PropertySetter, setterType Type, expr *ast.CallExpression) (Type, bool) {
	name := callExpressionName(expr)
	if name == "" {
		return Type{}, false
	}
	typeName := a.resolveTypeName(name)
	if _, exists := a.types[typeName]; !exists {
		return Type{}, false
	}
	targetType, ok := a.resolveType(&ast.TypeReference{Token: expr.Token, Name: name, TypeArgs: expr.GenericArguments})
	if !ok {
		return Type{Kind: InvalidType}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "conversion to %s expects 1 argument, got %d", name, len(expr.Arguments))
		return Type{Kind: InvalidType}, true
	}
	valueType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Arguments[0])
	if !ok || valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, true
	}
	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, true
	}
	if targetType.Kind == EnumType && targetType.BitWidth > 0 && !a.validateBitEnumConversion(targetType, valueType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, true
	}
	return targetType, true
}

func (a *Analyzer) inferPropertyBodyInfixExpression(target Type, setter *ast.PropertySetter, setterType Type, expr *ast.InfixExpression) (Type, bool) {
	leftType, leftOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Left)
	rightType, rightOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Right)
	if !leftOK || !rightOK || leftType.Kind == InvalidType || rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}
	if expr.Operator == "x" {
		typ, _ := a.inferMatrixMultiplyExpression(expr, leftType, rightType)
		return typ, typ.Kind != InvalidType
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

	if stmt.Type != nil && stmt.Type.UnitOnly && stmt.Value != nil {
		valueType, _ := a.inferExpression(stmt.Value)
		preferred := preferredUnitOnlyNumeric(stmt.Value, valueType)
		declaredType, ok = a.resolveUnitOnlyType(stmt.Type, preferred)
	} else if stmt.Type != nil {
		declaredType, ok = a.resolveType(stmt.Type)
	} else if stmt.Value != nil {
		declaredType, _ = a.inferExpression(stmt.Value)
		ok = declaredType.Kind != InvalidType
	}
	if ok && stmt.Contract != nil {
		a.checkContractLiteralBounds(declaredType, stmt.Contract)
		declaredType = a.applyContracts(declaredType, stmt.Contract)
	}
	if ok && stmt.Value == nil && stmt.Mutable && stmt.Address == nil {
		resolution := DefaultValueOf(declaredType)
		stmt.Value = defaultExpression(resolution, declaredType, stmt.Name.Token)
		stmt.SynthesizedDefault = stmt.Value != nil
		if stmt.Value == nil {
			a.addErrorAtTokenWithMetadata(stmt.Name.Token, diagnostics.NoDefaultValue, "provide an explicit initializer", "mutable variable %s of type %s requires an initializer because the type has no default value", stmt.Name.Value, typeDisplayName(declaredType))
			ok = false
		}
	}

	defined := false
	if ok {
		if isBareSliceType(declaredType) {
			token := stmt.Name.Token
			if stmt.Type != nil {
				token = stmt.Type.Token
			} else if stmt.Value != nil {
				token = expressionToken(stmt.Value)
			}
			a.addErrorAtToken(token, "bare slice type %s must be used behind ref", typeDisplayName(declaredType))
			ok = false
		}
	}
	if !ok && stmt.Name != nil {
		// Keep a poisoned binding after an invalid initializer. Later references
		// then retain the root diagnostic instead of becoming unrelated
		// "undefined variable" errors.
		defined = a.defineSymbol(stmt.Name.Value, Type{Kind: InvalidType}, stmt.Mutable, stmt.Name.Token)
		if defined {
			a.assigned[stmt.Name.Value] = true
		}
	}
	if ok {
		defined = a.defineSymbol(stmt.Name.Value, declaredType, stmt.Mutable, stmt.Name.Token)
		if defined {
			if stmt.Static {
				symbol := a.symbols[stmt.Name.Value]
				symbol.Storage = StorageOriginStatic
				symbol.Local = false
				a.symbols[stmt.Name.Value] = symbol
			}
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
			referenceOrigin, hasReferenceOrigin := a.localReferenceOriginForTransfer(stmt.Value)
			if !a.validateLetOwnership(stmt) {
				return
			}
			if typeCarriesReferenceOrigin(declaredType) {
				a.updateReferenceSymbolOrigin(stmt.Name.Value, declaredType)
			}
			a.applyLetOwnership(stmt)
			a.bindBorrowHoldersFromExpression(stmt.Value, stmt.Name.Value)
			a.markLocalRefContainerFromValue(stmt.Name.Value, stmt.Value)
			if hasReferenceOrigin {
				a.localRefContainers[stmt.Name.Value] = referenceOrigin
			}
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

	if defined && typeCarriesReferenceOrigin(declaredType) && typeCarriesReferenceOrigin(exprType) {
		declaredType = referenceTypeWithOrigin(declaredType, exprType)
		a.updateReferenceSymbolOrigin(stmt.Name.Value, declaredType)
	}

	if a.checkIntegerExpressionRange(declaredType, stmt.Value) {
		return
	}

	if a.checkInitializerType(declaredType, exprType, stmt.Value) && defined {
		referenceOrigin, hasReferenceOrigin := a.localReferenceOriginForTransfer(stmt.Value)
		if !a.validateLetOwnership(stmt) {
			return
		}
		a.applyLetOwnership(stmt)
		a.bindBorrowHoldersFromExpression(stmt.Value, stmt.Name.Value)
		a.markLocalRefContainerFromValue(stmt.Name.Value, stmt.Value)
		if hasReferenceOrigin {
			a.localRefContainers[stmt.Name.Value] = referenceOrigin
		}
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
	symbol.Storage = StorageOriginFixedAddress
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
	if index, ok := stmt.Target.(*ast.IndexExpression); ok {
		a.analyzeIndexAssignmentStatement(stmt, index)
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
	a.bindDefinition(target.Token, symbol.Token)
	a.recordDeferCapture(target.Value, symbol, target.Token)

	if !symbol.Mutable {
		a.addErrorAtToken(target.Token, "cannot assign to immutable variable %s", target.Value)
		return
	}
	if property, ok := a.lookupCurrentImplProperty(target.Value); ok && property.Fallible && !allowFallible {
		a.addErrorAtToken(target.Token, "assigning fallible property %s requires try", target.Value)
		return
	}
	if a.checkBorrowedMutation(target.Value, target.Token) {
		return
	}

	if hasContracts(symbol.Type) && !allowFallible {
		a.addErrorAtToken(target.Token, "assigning variable %s requires try because %s has contracts", target.Value, typeDisplayName(symbol.Type))
		return
	}
	if !a.validateMoveAssignmentTarget(stmt) {
		return
	}

	exprType, _ := a.inferExpressionWithExpected(stmt.Value, symbol.Type)
	if exprType.Kind == InvalidType {
		return
	}
	if a.checkAssignmentEscapesMatchPayload(stmt.Target, stmt.Value) {
		return
	}

	assignmentType := exprType
	if stmt.Operator != "=" {
		var valid bool
		assignmentType, valid = a.inferCompoundAssignmentType(stmt.Operator, symbol.Type, exprType, stmt.Value)
		if !valid {
			return
		}
	}

	if a.checkIntegerAssignmentRange(symbol, stmt) {
		return
	}

	if a.checkInitializerType(symbol.Type, assignmentType, stmt.Value) {
		referenceOrigin, hasReferenceOrigin := a.localReferenceOriginForTransfer(stmt.Value)
		if !a.validateAssignmentOwnership(stmt) {
			return
		}
		a.applyAssignmentOwnership(stmt)
		a.updateAssignedConstInt(symbol.Name, stmt)
		a.assigned[symbol.Name] = true
		a.clearRootPlaceState(symbol.Name)
		if isCloseTrackedResourceType(symbol.Type) {
			delete(a.closedResources, symbol.Name)
		}
		a.endBorrowsHeldBy(symbol.Name)
		a.bindBorrowHoldersFromExpression(stmt.Value, symbol.Name)
		if typeCarriesReferenceOrigin(symbol.Type) && typeCarriesReferenceOrigin(exprType) {
			symbol.Type = referenceTypeWithOrigin(symbol.Type, exprType)
			a.symbols[symbol.Name] = symbol
		}
		delete(a.localRefContainers, symbol.Name)
		a.markLocalRefContainerFromValue(symbol.Name, stmt.Value)
		if hasReferenceOrigin {
			a.localRefContainers[symbol.Name] = referenceOrigin
		}
	}
}

func (a *Analyzer) validateLetOwnership(stmt *ast.LetStatement) bool {
	if stmt == nil {
		return false
	}
	return a.validateNamedOwnershipSource(stmt.Ownership, stmt.Value, stmt.Name.Token, true, stmt.Type == nil)
}

func (a *Analyzer) applyLetOwnership(stmt *ast.LetStatement) {
	if stmt == nil {
		return
	}
	if stmt.Ownership == ast.OwnershipMove {
		a.markExplicitMoveSource(stmt.Value)
		return
	}
	a.markMoveSource(stmt.Value)
}

func (a *Analyzer) validateAssignmentOwnership(stmt *ast.AssignmentStatement) bool {
	if stmt == nil {
		return false
	}
	return a.validateNamedOwnershipSource(stmt.Ownership, stmt.Value, stmt.Token, false, false)
}

func (a *Analyzer) validateMoveAssignmentTarget(stmt *ast.AssignmentStatement) bool {
	if stmt == nil || stmt.Ownership != ast.OwnershipMove {
		return true
	}
	source, ok := a.resolvePlace(stmt.Value)
	if !ok {
		return true
	}
	destination, ok := a.resolvePlace(stmt.Target)
	if !ok {
		return true
	}
	if !PlacesOverlap(destination, source) {
		return true
	}
	a.addErrorAtToken(expressionToken(stmt.Value), "cannot move value %s into itself", source.String())
	return false
}

func (a *Analyzer) applyAssignmentOwnership(stmt *ast.AssignmentStatement) {
	if stmt == nil {
		return
	}
	if stmt.Ownership == ast.OwnershipMove {
		a.markExplicitMoveSource(stmt.Value)
		return
	}
	a.markMoveSource(stmt.Value)
}

func (a *Analyzer) validateNamedOwnershipSource(mode ast.OwnershipMode, value ast.Expression, token lexer.Token, declaration bool, inferredDeclaration bool) bool {
	place, ok := a.resolvePlace(value)
	if !ok {
		if mode == ast.OwnershipMove {
			a.addErrorAtToken(token, "explicit move requires a reusable source place")
			return false
		}
		return true
	}
	if _, _, _, unavailable := a.unavailablePlace(place); unavailable {
		// inferExpression has already emitted the primary use-after-move error;
		// do not apply ownership again and overwrite the original move site.
		return false
	}
	if mode == ast.OwnershipMove {
		if !place.Addressable {
			a.addErrorAtToken(token, "explicit move requires an addressable source place")
			return false
		}
		if _, isIndex := value.(*ast.IndexExpression); isIndex {
			a.addErrorAtToken(token, "explicit indexed extraction is not implemented; move the containing value")
			return false
		}
		if len(place.Projections) > 0 && !place.PartialMoveSafe {
			a.addErrorAtToken(token, "partial move requires independently tracked local struct storage")
			return false
		}
		return !a.checkBorrowedMovePlace(place, expressionToken(value))
	}
	classification := CopyClassificationOf(place.Type)
	if classification == CopyTrivial || classification == CopySemantic {
		return true
	}
	moveSyntax := "<-"
	if declaration && inferredDeclaration {
		moveSyntax = ":<-"
	}
	help := "use `destination <- source` to transfer ownership"
	if declaration {
		help = "use `let destination :<- source` to transfer ownership"
	}
	switch classification {
	case CopyNonCopyable:
		a.addErrorAtTokenWithMetadata(
			expressionToken(value),
			diagnostics.ImplicitMoveDisallowed,
			help,
			"%s value %s cannot be copied because %s; use explicit move syntax %s",
			typeDisplayName(place.Type),
			place.String(),
			nonCopyableCause(place.Type),
			moveSyntax,
		)
		return false
	case CopyConditional:
		a.addErrorAtTokenWithMetadata(
			expressionToken(value),
			diagnostics.ImplicitMoveDisallowed,
			help,
			"cannot copy value %s because generic copyability has not been proven; use explicit move syntax %s",
			place.String(),
			moveSyntax,
		)
		return false
	}
	a.addErrorAtTokenWithMetadata(
		expressionToken(value),
		diagnostics.ImplicitMoveDisallowed,
		help,
		"cannot copy move-only value %s; use explicit move syntax %s",
		place.String(),
		moveSyntax,
	)
	return false
}

func directNonCopyableField(typ Type) (string, Type, bool) {
	for _, field := range typ.Fields {
		if CopyClassificationOf(field.Type) == CopyNonCopyable {
			return field.Name, field.Type, true
		}
	}
	return "", Type{}, false
}

func nonCopyableCause(typ Type) string {
	if typ.ExplicitlyNonCopyable {
		if typ.NoCopyPolicyOrigin != "" && typ.NoCopyPolicyOrigin != typ.Name {
			return fmt.Sprintf("underlying nominal type %s explicitly forbids implicit copy through @noCopy", typ.NoCopyPolicyOrigin)
		}
		return fmt.Sprintf("%s explicitly forbids implicit copy through @noCopy", typeDisplayName(typ))
	}
	if fieldName, fieldType, ok := directNonCopyableField(typ); ok {
		if fieldType.ExplicitlyNonCopyable {
			return fmt.Sprintf("field %s has type %s, which explicitly forbids implicit copy through @noCopy", fieldName, typeDisplayName(fieldType))
		}
		return fmt.Sprintf("field %s has non-copyable type %s", fieldName, typeDisplayName(fieldType))
	}
	if compilerKnownNonCopyable(typ) {
		return fmt.Sprintf("%s is compiler-known non-copyable", typeDisplayName(typ))
	}
	return "its type is non-copyable"
}

func compilerKnownNonCopyable(typ Type) bool {
	if typ.Kind != StructType {
		return false
	}
	switch typ.Name {
	case "Mutex", "Atomic", "ThreadLocal", "Event", "EventStorage", "ChannelOptions", "SenderID":
		return true
	default:
		return false
	}
}

func (a *Analyzer) markExplicitMoveSource(expr ast.Expression) bool {
	place, ok := a.resolvePlace(expr)
	if !ok {
		return false
	}
	if a.checkBorrowedMovePlace(place, expressionToken(expr)) {
		return false
	}
	a.markPlaceUnavailable(place, expressionToken(expr), "moved")
	if len(place.Projections) == 0 && place.Type.Kind != ReferenceType {
		a.endBorrowsHeldBy(place.Root)
	}
	return true
}

func (a *Analyzer) analyzeIndexAssignmentStatement(stmt *ast.AssignmentStatement, index *ast.IndexExpression) {
	targetType, _ := a.inferIndexExpression(index)
	if targetType.Kind == InvalidType {
		return
	}
	if !a.indexAssignmentTargetIsMutable(index.Left) {
		a.addErrorAtToken(expressionToken(index.Left), "cannot assign through immutable index target")
		return
	}
	if place, ok := a.resolvePlace(index); ok && a.checkBorrowedMutationPlace(place, expressionToken(index)) {
		return
	}
	exprType, _ := a.inferExpressionWithExpected(stmt.Value, targetType)
	if exprType.Kind == InvalidType {
		return
	}
	assignmentType := exprType
	if stmt.Operator != "=" {
		var valid bool
		assignmentType, valid = a.inferCompoundAssignmentType(stmt.Operator, targetType, exprType, stmt.Value)
		if !valid {
			return
		}
	}
	if a.checkAssignmentEscapesLocalReference(stmt.Target, stmt.Value) {
		return
	}
	if !a.validateMoveAssignmentTarget(stmt) {
		return
	}
	if a.checkInitializerType(targetType, assignmentType, stmt.Value) {
		if !a.validateAssignmentOwnership(stmt) {
			return
		}
		a.applyAssignmentOwnership(stmt)
		if stmt.Operator == "=" {
			a.updateContainedOriginsForAssignment(index, stmt.Value)
		}
	}
}

func (a *Analyzer) markMoveSource(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok || !requiresOwnershipTransfer(symbol.Type) {
			return false
		}
		if a.checkBorrowedMove(expr.Value, expr.Token) {
			return false
		}
		a.moved[expr.Value] = expr.Token
		a.moveReasons[expr.Value] = "moved"
		return true
	case *ast.StructLiteral:
		moved := false
		for _, field := range expr.Fields {
			if field != nil && a.markMoveSource(field.Value) {
				moved = true
			}
		}
		return moved
	case *ast.ArrayLiteral:
		moved := false
		for _, element := range expr.Elements {
			if a.markMoveSource(element) {
				moved = true
			}
		}
		return moved
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.markMoveSource(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.markMoveSource(expr.Value)
		}
	case *ast.SpreadExpression:
		return a.markMoveSource(expr.Value)
	case *ast.ConversionExpression:
		return a.markMoveSource(expr.Value)
	}
	return false
}

func (a *Analyzer) markResourceTransfer(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok || !isCloseTrackedResourceType(symbol.Type) {
			return false
		}
		if a.checkBorrowedMove(expr.Value, expr.Token) {
			return false
		}
		a.moved[expr.Value] = expr.Token
		a.moveReasons[expr.Value] = "returned"
		a.endBorrowsHeldBy(expr.Value)
		return true
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.markResourceTransfer(expr.Value)
		}
	case *ast.ConversionExpression:
		return a.markResourceTransfer(expr.Value)
	}
	return false
}

func (a *Analyzer) bindBorrowHoldersFromExpression(expr ast.Expression, holder string) {
	if holder == "" || expr == nil {
		return
	}
	switch expr := expr.(type) {
	case *ast.RefExpression:
		a.registerBorrow(holder, expr)
	case *ast.Identifier:
		a.transferBorrowHolderFromExpression(expr, holder)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil {
				a.bindBorrowHoldersFromExpression(field.Value, holder)
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			a.bindBorrowHoldersFromExpression(element, holder)
		}
	case *ast.OkExpression:
		if expr.Value != nil {
			a.bindBorrowHoldersFromExpression(expr.Value, holder)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			a.bindBorrowHoldersFromExpression(expr.Value, holder)
		}
	case *ast.SpreadExpression:
		a.bindBorrowHoldersFromExpression(expr.Value, holder)
	case *ast.ConversionExpression:
		a.bindBorrowHoldersFromExpression(expr.Value, holder)
	case *ast.CallExpression:
		if origin, ok := a.expressionReferenceOrigins[expr]; ok {
			a.registerBorrowFromReturnedOrigin(holder, origin, expr.Token)
		}
	}
}

func (a *Analyzer) registerBorrowFromReturnedOrigin(holder string, origin localReferenceOrigin, token lexer.Token) {
	if origin.Unknown {
		return
	}
	kind := sharedBorrow
	if origin.Mutable {
		kind = mutableBorrow
	}
	for _, place := range localOriginPlaces(origin) {
		for _, alternative := range placeOriginAlternatives(place) {
			a.borrows[alternative.Root] = append(a.borrows[alternative.Root], borrowRecord{
				Root: alternative.Root, Place: alternative, Holder: holder, Kind: kind, Token: token,
			})
		}
	}
	for _, child := range origin.Contained {
		a.registerBorrowFromReturnedOrigin(holder, child, token)
	}
}

func (a *Analyzer) transferBorrowHolderFromExpression(expr ast.Expression, holder string) {
	ident, ok := expr.(*ast.Identifier)
	if !ok || holder == "" || ident.Value == holder {
		return
	}
	symbol, ok := a.symbols[ident.Value]
	if !ok || symbol.Type.Kind != ReferenceType || !MoveOnly(symbol.Type) {
		return
	}
	a.transferBorrowHolder(ident.Value, holder)
}

func (a *Analyzer) registerBorrow(holder string, expr *ast.RefExpression) {
	if holder == "" || expr == nil {
		return
	}
	place, ok := a.resolvePlace(expr.Value)
	if !ok {
		return
	}
	kind := sharedBorrow
	if expr.Mutable {
		kind = mutableBorrow
	}
	for _, alternative := range placeOriginAlternatives(place) {
		root := alternative.Root
		a.borrows[root] = append(a.borrows[root], borrowRecord{
			Root:   root,
			Place:  alternative,
			Holder: holder,
			Kind:   kind,
			Token:  expr.Token,
		})
	}
}

func (a *Analyzer) transferBorrowHolder(from string, to string) {
	for root, records := range a.borrows {
		for i := range records {
			if records[i].Holder == from {
				records[i].Holder = to
			}
		}
		a.borrows[root] = records
	}
	if origin, ok := a.localRefContainers[from]; ok {
		a.localRefContainers[to] = cloneLocalReferenceOrigin(origin)
		delete(a.localRefContainers, from)
	}
}

func (a *Analyzer) updateReferenceSymbolOrigin(name string, typ Type) {
	if !typeCarriesReferenceOrigin(typ) {
		return
	}
	symbol, ok := a.symbols[name]
	if !ok {
		return
	}
	symbol.Type = typ
	a.symbols[name] = symbol
}

func referenceTypeWithOrigin(target Type, source Type) Type {
	target.ReferenceOriginName = source.ReferenceOriginName
	target.ReferenceOriginToken = source.ReferenceOriginToken
	target.ReferenceOriginLocal = source.ReferenceOriginLocal
	target.ReferenceOriginMatchScoped = source.ReferenceOriginMatchScoped
	target.ReferenceOriginStorage = source.ReferenceOriginStorage
	target.ReferenceOriginGeneration = source.ReferenceOriginGeneration
	return target
}

func (a *Analyzer) referenceTypeWithOriginFromExpression(target Type, expr ast.Expression) Type {
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr)
	target.ReferenceOriginName = originName
	target.ReferenceOriginToken = originToken
	target.ReferenceOriginLocal = originLocal
	target.ReferenceOriginStorage = originStorage
	target.ReferenceOriginGeneration = generation
	target.ReferenceOriginMatchScoped = a.referenceOriginMatchScopedForExpression(expr)
	return target
}

func (a *Analyzer) referenceOriginMatchScopedForExpression(expr ast.Expression) bool {
	root, ok := borrowRootName(expr)
	if !ok {
		return false
	}
	symbol, ok := a.symbols[root]
	return ok && typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginMatchScoped
}

func (a *Analyzer) referenceOriginForExpression(expr ast.Expression) (string, lexer.Token, bool, StorageOrigin, int) {
	root, ok := borrowRootName(expr)
	if !ok {
		return "", lexer.Token{}, false, StorageOriginUnknown, 0
	}
	symbol, ok := a.symbols[root]
	if !ok {
		return root, expressionToken(expr), true, StorageOriginUnknown, 0
	}
	if typeCarriesReferenceOrigin(symbol.Type) && symbol.Type.ReferenceOriginName != "" {
		return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginToken, symbol.Type.ReferenceOriginLocal, symbol.Type.ReferenceOriginStorage, symbol.Type.ReferenceOriginGeneration
	}
	return symbol.Name, symbol.Token, symbol.Local, symbol.Storage, a.arenaGenerations[symbol.Name]
}

func typeCarriesReferenceOrigin(typ Type) bool {
	return typ.Kind == ReferenceType || typ.Kind == SliceType
}

func (a *Analyzer) checkStaleArenaReference(symbol Symbol, token lexer.Token) bool {
	if !typeCarriesReferenceOrigin(symbol.Type) || symbol.Type.ReferenceOriginStorage != StorageOriginArena || symbol.Type.ReferenceOriginName == "" {
		return false
	}
	current := a.arenaGenerations[symbol.Type.ReferenceOriginName]
	if symbol.Type.ReferenceOriginGeneration == current {
		return false
	}
	a.addErrorAtTokenWithPrevious(token, symbol.Type.ReferenceOriginToken, "cannot use %s after arena %s was reset", symbol.Name, symbol.Type.ReferenceOriginName)
	return true
}

func (a *Analyzer) checkBorrowCreation(expr ast.Expression, mutable bool, token lexer.Token) bool {
	place, ok := a.resolvePlace(expr)
	if !ok {
		switch expr.(type) {
		case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression, *ast.SliceExpression:
			return false
		default:
			a.addErrorAtToken(token, "cannot borrow temporary expression; a reusable place is required")
			return true
		}
	}
	return a.checkBorrowCreationPlace(place, mutable, token)
}

func (a *Analyzer) checkBorrowCreationPlace(place Place, mutable bool, token lexer.Token) bool {
	if place.AmbiguousProvenance {
		a.addErrorAtToken(token, "cannot borrow through reference with unknown control-flow provenance")
		return true
	}
	if !place.Addressable {
		a.addErrorAtToken(token, "cannot borrow non-addressable place %s", place.String())
		return true
	}
	if mutable {
		if !place.Mutable {
			if len(place.Projections) == 0 {
				a.addErrorAtToken(token, "cannot create mutable reference to immutable variable %s", place.Root)
			} else {
				a.addErrorAtToken(token, "cannot create mutable reference to immutable place %s", place.String())
			}
			return true
		}
	}
	for _, candidate := range placeOriginAlternatives(place) {
		if candidate.Root == "" {
			continue
		}
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind == deferredUse {
				continue
			}
			if candidate.ReferenceHolder != "" && record.Holder == candidate.ReferenceHolder {
				continue
			}
			if !borrowPlacesOverlap(candidate, record) {
				continue
			}
			if !mutable && record.Kind == sharedBorrow {
				continue
			}
			if mutable {
				if record.LoopCarried {
					a.addErrorAtTokenWithPrevious(token, record.Token, "cannot create mutable reference to %s because an overlapping borrow may remain active from a previous loop iteration", place.String())
					return true
				}
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot create mutable reference to %s while it is already borrowed", place.String())
				return true
			}
			if record.LoopCarried {
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot create shared reference to %s because an overlapping mutable borrow may remain active from a previous loop iteration", place.String())
				return true
			}
			a.addErrorAtTokenWithPrevious(token, record.Token, "cannot create shared reference to %s while it is mutably borrowed", place.String())
			return true
		}
	}
	return false
}

func (a *Analyzer) checkBorrowedRead(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedReadPlace(place, token)
}

func (a *Analyzer) checkBorrowedReadPlace(place Place, token lexer.Token) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind != mutableBorrow || record.Holder == candidate.Root || record.Holder == candidate.ReferenceHolder || !borrowPlacesOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot read %s because it may remain mutably borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addErrorAtTokenWithPrevious(token, record.Token, "cannot read %s while it is mutably borrowed", place.String())
			return true
		}
	}
	return false
}

func (a *Analyzer) checkBorrowedMutation(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedMutationPlace(place, token)
}

func (a *Analyzer) checkBorrowedMutationPlace(place Place, token lexer.Token) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind == deferredUse {
				continue
			}
			if candidate.ReferenceHolder != "" && record.Holder == candidate.ReferenceHolder {
				continue
			}
			if !borrowPlacesOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot assign to %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			if record.Kind == mutableBorrow {
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot assign to %s while it is mutably borrowed", place.String())
				return true
			}
			a.addErrorAtTokenWithPrevious(token, record.Token, "cannot assign to %s while it is shared borrowed", place.String())
			return true
		}
	}
	return false
}

func (a *Analyzer) checkBorrowedMove(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedMovePlace(place, token)
}

func (a *Analyzer) checkBorrowedMovePlace(place Place, token lexer.Token) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		if a.checkDeferredUse(candidate.Root, token, "move") {
			return true
		}
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind == deferredUse {
				continue
			}
			if candidate.ReferenceHolder != "" && record.Holder == candidate.ReferenceHolder {
				continue
			}
			if !borrowPlacesOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addErrorAtTokenWithPrevious(token, record.Token, "cannot move %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addErrorAtTokenWithPrevious(token, record.Token, "cannot move %s while it is borrowed", place.String())
			return true
		}
	}
	return false
}

func (a *Analyzer) checkDeferredUse(name string, token lexer.Token, action string) bool {
	for _, record := range a.borrows[name] {
		if record.Kind != deferredUse {
			continue
		}
		a.addErrorAtTokenWithPrevious(token, record.Token, "cannot %s %s while it is required by defer", action, name)
		return true
	}
	return false
}

func (a *Analyzer) recordDeferCapture(name string, symbol Symbol, token lexer.Token) {
	if !a.inDeferBlock || a.deferCaptures == nil {
		return
	}
	outer, ok := a.deferOuterSymbols[name]
	if !ok || outer.Token != symbol.Token {
		return
	}
	if _, exists := a.deferCaptures[name]; !exists {
		a.deferCaptures[name] = token
	}
}

func borrowRootName(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		return borrowRootName(expr.Object)
	case *ast.IndexExpression:
		return borrowRootName(expr.Left)
	case *ast.SliceExpression:
		return borrowRootName(expr.Left)
	default:
		return "", false
	}
}

func (a *Analyzer) indexAssignmentTargetIsMutable(expr ast.Expression) bool {
	place, ok := a.resolvePlace(expr)
	return ok && place.Mutable
}

func (a *Analyzer) analyzeMemberAssignmentStatement(stmt *ast.AssignmentStatement, member *ast.MemberExpression, allowFallible bool) {
	targetType, ok := a.inferMemberExpression(member)
	if !ok {
		return
	}

	if symbol, ok := a.symbolForMemberObject(member.Object); ok {
		symbolType := dereferenceType(symbol.Type)
		if symbolType.Kind == RegisterType && !a.canWriteThroughSymbol(symbol) {
			if symbol.Addressed {
				a.addErrorAtToken(member.Property.Token, "cannot assign to field %s on read-only addressed register %s", member.Property.Value, symbol.Name)
			} else {
				a.addErrorAtToken(member.Property.Token, "cannot assign to field %s through read-only receiver %s", member.Property.Value, symbol.Name)
			}
			return
		}
	}
	property, propertyOK := a.lookupPropertyOnMember(member)
	if propertyOK && !property.HasSetter {
		a.addErrorAtToken(member.Property.Token, "property %s has no setter", member.Property.Value)
		return
	}
	place, placeOK := a.resolvePlace(member)
	if placeOK && !place.Mutable {
		a.addErrorAtToken(member.Property.Token, "cannot assign to immutable place %s", place.String())
		return
	}

	if propertyOK {
		if property.Fallible && !allowFallible {
			a.addErrorAtToken(member.Property.Token, "assigning fallible property %s requires try", member.Property.Value)
			return
		}
		if placeOK {
			rootPlace, ok := a.rootPlace(place.Root)
			if ok && a.checkBorrowedMutationPlace(rootPlace, member.Property.Token) {
				return
			}
		}
	} else if placeOK && a.checkBorrowedMutationPlace(place, member.Property.Token) {
		return
	}

	valueType, _ := a.inferExpressionWithExpected(stmt.Value, targetType)
	if valueType.Kind == InvalidType {
		return
	}
	if a.checkAssignmentEscapesLocalReference(stmt.Target, stmt.Value) {
		return
	}
	if !a.validateMoveAssignmentTarget(stmt) {
		return
	}

	if stmt.Operator == "=" && a.isRegisterBitField(member) && a.isZeroOrOneIntegerConstant(stmt.Value) {
		return
	}

	if a.checkIntegerExpressionRange(targetType, stmt.Value) {
		return
	}

	assignmentType := valueType
	if stmt.Operator != "=" {
		var valid bool
		assignmentType, valid = a.inferCompoundAssignmentType(stmt.Operator, targetType, valueType, stmt.Value)
		if !valid {
			return
		}
	}

	if !canInitialize(targetType, assignmentType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return
	}
	if !a.validateAssignmentOwnership(stmt) {
		return
	}
	a.applyAssignmentOwnership(stmt)
	if stmt.Operator == "=" && !propertyOK {
		a.updateContainedOriginsForAssignment(member, stmt.Value)
	}
	if placeOK {
		a.markPlaceAvailable(place)
	}
}

func (a *Analyzer) isRegisterBitField(member *ast.MemberExpression) bool {
	if member == nil || member.Property == nil {
		return false
	}
	objectType, _ := a.inferPlaceBase(member.Object)
	if objectType.Kind == InvalidType {
		return false
	}
	objectType = dereferenceType(objectType)
	if objectType.Kind != RegisterType {
		return false
	}
	for _, field := range objectType.RegisterFields {
		if field.Name == member.Property.Value {
			return field.Width == 1
		}
	}
	return false
}

func (a *Analyzer) isZeroOrOneIntegerConstant(expr ast.Expression) bool {
	value, ok := a.integerConstantValue(expr)
	return ok && (value.Sign() == 0 || value.Cmp(big.NewInt(1)) == 0)
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
	switch target := stmt.Target.(type) {
	case *ast.MemberExpression:
		property, ok := a.lookupPropertyOnMember(target)
		if !ok || property.Error == nil {
			return Type{}, false
		}
		return *property.Error, true
	case *ast.Identifier:
		property, ok := a.lookupCurrentImplProperty(target.Value)
		if ok {
			if property.Error == nil {
				return Type{}, false
			}
			return *property.Error, true
		}
		symbol, ok := a.symbols[target.Value]
		if ok && hasContracts(symbol.Type) {
			return a.types["ContractError"], true
		}
		return Type{}, false
	default:
		return Type{}, false
	}
}

func (a *Analyzer) resolveType(ref *ast.TypeReference) (Type, bool) {
	if ref == nil || ref.Invalid {
		return Type{Kind: InvalidType}, false
	}

	if ref.UnitOnly {
		return a.resolveUnitOnlyType(ref, "")
	}

	if ref.Ref {
		if ref.Slice && ref.ElementType != nil {
			element, ok := a.resolveType(ref.ElementType)
			if !ok {
				return Type{Kind: InvalidType}, false
			}
			slice := Type{
				Name:    typeDisplayName(element) + "[]",
				Kind:    SliceType,
				Element: &element,
			}
			name := "ref " + typeDisplayName(slice)
			if ref.MutableRef {
				name = "ref mut " + typeDisplayName(slice)
			}
			return Type{
				Name:             name,
				Kind:             ReferenceType,
				Element:          &slice,
				ReferenceMutable: ref.MutableRef,
			}, true
		}
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

	if ref.Name == "self" && a.currentImplTarget != "" {
		target, ok := a.types[a.currentImplTarget]
		if !ok {
			a.addErrorAtToken(ref.Token, "unknown type self")
			return Type{Kind: InvalidType}, false
		}
		if definition, exists := a.typeDefinitionTokens[a.currentImplTarget]; exists {
			a.bindDefinition(ref.Token, definition)
		}
		return target, true
	}

	if ref.ElementType != nil {
		element, ok := a.resolveType(ref.ElementType)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		if !ref.Slice {
			length, ok := a.resolveArrayLength(ref)
			if !ok {
				return Type{Kind: InvalidType}, false
			}
			return Type{
				Name:        fmt.Sprintf("%s[%d]", typeDisplayName(element), length),
				Kind:        ArrayType,
				Element:     &element,
				ArrayLength: length,
			}, true
		}
		return Type{
			Name:        typeDisplayName(element) + "[]",
			Kind:        ArrayType,
			Element:     &element,
			ArrayLength: dynamicArrayLength,
		}, true
	}

	if genericType, ok := a.genericTypes[ref.Name]; ok {
		if len(ref.TypeArgs) > 0 {
			a.addErrorAtToken(ref.Token, "generic parameter %s does not take type arguments", ref.Name)
			return Type{Kind: InvalidType}, false
		}
		if definition, exists := a.genericTypeDefinitions[ref.Name]; exists {
			a.bindDefinition(ref.Token, definition)
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
	constArgs := make([]int64, 0, len(ref.ConstArgs))
	for _, arg := range ref.ConstArgs {
		value, ok := a.integerConstantValue(arg)
		if !ok {
			a.addErrorAtToken(expressionToken(arg), "%s argument must be a compile-time integer", ref.Name)
			continue
		}
		if !value.IsInt64() {
			a.addErrorAtToken(expressionToken(arg), "%s argument cannot be represented by int64", ref.Name)
			continue
		}
		constArgs = append(constArgs, value.Int64())
	}

	name := a.resolveTypeName(ref.Name)

	typ, ok := a.types[name]
	if !ok {
		a.addErrorAtToken(ref.Token, "unknown type %s", ref.Name)
		return Type{Kind: InvalidType}, false
	}
	if definition, exists := a.typeDefinitionTokens[name]; exists {
		a.bindDefinition(ref.Token, definition)
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
	if len(typeArgs) != len(ref.TypeArgs) || len(constArgs) != len(ref.ConstArgs) {
		return Type{Kind: InvalidType}, false
	}

	typ.TypeArgs = typeArgs
	typ.ConstArgs = constArgs
	if ref.EventCapacitySet {
		typ.EventCapacity = ref.EventCapacity
		typ.EventCapacitySet = true
	}
	if !a.validateCompilerKnownGenericType(ref.Token, typ) {
		return Type{Kind: InvalidType}, false
	}
	if ref.Unit != "" {
		a.warnUnitStatus(ref.Token, ref.Unit)
		typ.Unit = ref.Unit
		typ.Dimension = a.parseDimension(ref.Unit)
	}
	if (typ.Kind == StructType || typ.Kind == UnionType) && len(typ.GenericParameters) > 0 {
		typ = a.instantiateGenericType(typ)
	}
	return typ, true
}

func (a *Analyzer) validateCompilerKnownGenericType(token lexer.Token, typ Type) bool {
	switch typ.Name {
	case "list":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 0, 1)
	case "map":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 2, 0, 1)
	case "set":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 0, 1)
	case "vector":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 1, 1)
	case "matrix":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 2, 2)
	case "tensor":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 1, -1)
	case "tensor_view":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 1, 1, 1)
	case "Shape", "Strides", "TensorLayout":
		if typ.Declared {
			return true
		}
		return a.validateCompilerKnownTypeArity(token, typ, 0, 1, 1)
	case "Atomic":
		if len(typ.TypeArgs) != 1 {
			return false
		}
		if !atomicElementTypeSupported(typ.TypeArgs[0]) {
			a.addErrorAtToken(token, "type %s is not supported by Atomic", typeDisplayName(typ.TypeArgs[0]))
			return false
		}
	case "Task",
		"Thread",
		"ThreadObserver",
		"ThreadLocal",
		"Mutex",
		"MutexGuard",
		"CompareExchangeResult",
		"Event",
		"EventStorage",
		"Channel",
		"Sender",
		"Receiver",
		"MessageTicket",
		"ChannelSendResult",
		"ChannelTryReceiveResult",
		"ChannelRevokeResult":
		if len(typ.TypeArgs) != 1 {
			return false
		}
		if (typ.Name == "Event" || typ.Name == "EventStorage") && typ.EventCapacitySet && typ.EventCapacity <= 0 {
			a.addErrorAtToken(token, "event capacity must be greater than zero")
			return false
		}
		if typ.Name == "EventStorage" && !typ.EventCapacitySet {
			a.addErrorAtToken(token, "EventStorage requires explicit capacity")
			return false
		}
		return true
	}
	return true
}

func (a *Analyzer) validateCompilerKnownTypeArity(token lexer.Token, typ Type, typeArgCount int, minConstArgs int, maxConstArgs int) bool {
	if len(typ.TypeArgs) != typeArgCount {
		a.addErrorAtToken(token, "%s requires %d type arguments, got %d", typ.Name, typeArgCount, len(typ.TypeArgs))
		return false
	}
	if len(typ.ConstArgs) < minConstArgs || (maxConstArgs >= 0 && len(typ.ConstArgs) > maxConstArgs) {
		expected := fmt.Sprintf("%d", minConstArgs)
		if maxConstArgs != minConstArgs {
			if maxConstArgs < 0 {
				expected = fmt.Sprintf("at least %d", minConstArgs)
			} else {
				expected = fmt.Sprintf("%d to %d", minConstArgs, maxConstArgs)
			}
		}
		a.addErrorAtToken(token, "%s requires %s compile-time integer arguments, got %d", typ.Name, expected, len(typ.ConstArgs))
		return false
	}
	for _, arg := range typ.ConstArgs {
		if arg < 0 {
			a.addErrorAtToken(token, "%s arguments must be non-negative", typ.Name)
			return false
		}
	}
	if (typ.Name == "list" || typ.Name == "map" || typ.Name == "set") && len(typ.ConstArgs) == 1 && typ.ConstArgs[0] <= 0 {
		a.addErrorAtToken(token, "%s capacity must be greater than zero", typ.Name)
		return false
	}
	return true
}

func atomicElementTypeSupported(typ Type) bool {
	switch typ.Kind {
	case BoolType, RawPtrType:
		return true
	case IntType, UintType:
		switch typ.Name {
		case "byte", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
			return true
		}
	}
	return false
}

func (a *Analyzer) resolveArrayLength(ref *ast.TypeReference) (int64, bool) {
	if ref.ArrayLengthExpression == nil {
		if ref.ArrayLength < 0 {
			a.addErrorAtToken(ref.Token, "array length must be non-negative")
			return 0, false
		}
		return ref.ArrayLength, true
	}

	value, ok := a.integerConstantValue(ref.ArrayLengthExpression)
	if !ok {
		a.addErrorAtToken(expressionToken(ref.ArrayLengthExpression), "array length must be a compile-time integer")
		return 0, false
	}
	if value.Sign() < 0 {
		a.addErrorAtToken(expressionToken(ref.ArrayLengthExpression), "array length must be non-negative")
		return 0, false
	}
	if !value.IsInt64() {
		a.addErrorAtToken(expressionToken(ref.ArrayLengthExpression), "array length cannot be represented by int64")
		return 0, false
	}
	return value.Int64(), true
}

func (a *Analyzer) resolveUnitOnlyType(ref *ast.TypeReference, preferredNumeric string) (Type, bool) {
	unit, ok := a.units[ref.Unit]
	if !ok {
		a.addErrorAtToken(ref.Token, "unknown unit %s", ref.Unit)
		return Type{Kind: InvalidType}, false
	}
	numeric := preferredNumeric
	if numeric == "" {
		numeric = unit.DefaultNumeric
	}
	if numeric == "" {
		numeric = "decimal"
	}
	baseType, ok := a.types[numeric]
	if !ok || !isNumericType(baseType) {
		a.addErrorAtToken(ref.Token, "unit %s has invalid default numeric type %s", ref.Unit, numeric)
		return Type{Kind: InvalidType}, false
	}
	a.warnUnitStatus(ref.Token, ref.Unit)
	typ := baseType
	typ.Unit = ref.Unit
	typ.Dimension = unit.Dimension
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
	typ, value := a.inferExpressionUnrecorded(expr)
	if expr != nil {
		a.expressionTypes[expr] = typ
		a.recordResolvedOperator(expr, typ)
	}
	return typ, value
}

func (a *Analyzer) inferExpressionUnrecorded(expr ast.Expression) (Type, expressionValue) {
	if expr == nil {
		// Editor parsing may leave an incomplete expression while the user is
		// typing. Treat it as invalid so semantic features can still respond.
		return Type{Kind: InvalidType}, expressionValue{}
	}

	switch expr := expr.(type) {
	case *ast.InvalidExpression:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.InvalidPattern:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.IntegerLiteral:
		switch expr.Suffix() {
		case "u":
			return Type{Name: "uint", Kind: UintType}, expressionValue{Display: expr.String()}
		case "g":
			return Type{Name: "float", Kind: FloatType}, expressionValue{Display: expr.String()}
		case "m":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		case "t":
			if !a.validUnicodeScalarLiteral(expr) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return Type{Name: "char", Kind: CharType}, expressionValue{Display: expr.String()}
		case "r":
			if !a.validUnicodeScalarLiteral(expr) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "int", Kind: IntType}, expressionValue{Display: expr.String()}
	case *ast.FloatLiteral:
		switch expr.Suffix() {
		case "g":
			return Type{Name: "float", Kind: FloatType}, expressionValue{Display: expr.String()}
		case "m":
			return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
	case *ast.StringLiteral, *ast.InterpolatedStringLiteral:
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	case *ast.CharLiteral:
		if !validCharLiteral(expr.Token.Lexeme) {
			a.addErrorAtToken(expr.Token, "character literal must contain exactly one character")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "char", Kind: CharType}, expressionValue{Display: expr.String()}
	case *ast.BooleanLiteral:
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok {
			if functions := a.accessibleFunctions(a.functions[expr.Value]); len(functions) > 0 {
				if len(functions) == 1 {
					a.bindDefinition(expr.Token, functions[0].Token)
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
			if a.currentImplTarget != "" {
				if target, ok := a.types[a.currentImplTarget]; ok {
					if property, ok := lookupProperty(target, expr.Value); ok {
						a.bindDefinition(expr.Token, property.Token)
						return property.Type, expressionValue{Display: expr.String()}
					}
				}
			}
			a.addErrorAtToken(expr.Token, "undefined variable %s", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.bindDefinition(expr.Token, symbol.Token)
		if assigned, ok := a.assigned[expr.Value]; ok && !assigned {
			a.addErrorAtToken(expr.Token, "variable %s is unassigned", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if place, placeOK := a.resolvePlace(expr); placeOK && a.checkPlaceAvailableForRead(place, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if a.checkStaleArenaReference(symbol, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if a.suppressPlaceRootRead == 0 && a.checkBorrowedRead(expr.Value, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.recordDeferCapture(expr.Value, symbol, expr.Token)
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
		return a.inferSpawnExpression(expr)
	case *ast.AwaitExpression:
		return a.inferAwaitExpression(expr)
	case *ast.MatchExpression:
		return a.inferMatchExpression(expr)
	case *ast.SpreadExpression:
		a.addErrorAtToken(expr.Token, "spread operator is not valid as a standalone expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case *ast.MemberExpression:
		typ, ok := a.inferMemberExpression(expr)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if place, ok := a.resolvePlace(expr); ok {
			if a.checkPlaceAvailableForRead(place, expr.Property.Token) || a.checkBorrowedReadPlace(place, expr.Property.Token) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
		return typ, expressionValue{Display: expr.String()}
	case *ast.ArrayLiteral:
		return a.inferArrayLiteral(expr)
	case *ast.IndexExpression:
		typ, value := a.inferIndexExpression(expr)
		if typ.Kind != InvalidType {
			if place, ok := a.resolvePlace(expr); ok && a.checkBorrowedReadPlace(place, expr.Token) {
				return Type{Kind: InvalidType}, value
			}
		}
		return typ, value
	case *ast.SliceExpression:
		typ, value := a.inferSliceExpression(expr)
		if typ.Kind != InvalidType {
			if place, ok := a.resolvePlace(expr); ok && a.checkBorrowedReadPlace(place, expr.Token) {
				return Type{Kind: InvalidType}, value
			}
		}
		return typ, value
	case *ast.RefExpression:
		return a.inferRefExpression(expr)
	case *ast.StructLiteral:
		return a.inferStructLiteral(expr)
	default:
		return Type{Kind: InvalidType}, expressionValue{}
	}
}

func (a *Analyzer) validUnicodeScalarLiteral(expr *ast.IntegerLiteral) bool {
	value, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	if !ok || value.Sign() < 0 || value.Cmp(big.NewInt(0x10FFFF)) > 0 || (value.Cmp(big.NewInt(0xD800)) >= 0 && value.Cmp(big.NewInt(0xDFFF)) <= 0) {
		a.addErrorAtToken(expr.Token, "value %s is not a valid Unicode scalar value", expr.Token.Lexeme)
		return false
	}
	return true
}

func (a *Analyzer) inferExpressionWithExpected(expr ast.Expression, expected Type) (Type, expressionValue) {
	if _, ok := expr.(*ast.CharLiteral); ok && expected.Kind == RuneType {
		return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
	}
	if lit, ok := expr.(*ast.ArrayLiteral); ok {
		return a.inferArrayLiteralWithExpected(lit, expected)
	}
	if typ, value, ok := a.inferExpectedUnionVariantExpression(expr, expected); ok {
		return typ, value
	}
	call, ok := expr.(*ast.CallExpression)
	if !ok || expected.Kind == InvalidType || expected.Kind == "" {
		typ, value := a.inferExpression(expr)
		return typeWithExpectedDimension(expr, typ, value, expected)
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
	typ, value := a.inferExpression(expr)
	return typeWithExpectedDimension(expr, typ, value, expected)
}

func (a *Analyzer) inferSpawnExpression(expr *ast.SpawnExpression) (Type, expressionValue) {
	if expr.Body != nil {
		a.addErrorAtToken(expr.Token, "spawn block syntax is deprecated; spawn requires a callable expression")
		a.withCancellableContext(func() {
			a.analyzeBlockStatements(expr.Body)
		})
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Value == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch expr.Value.(type) {
	case *ast.CallExpression, *ast.LambdaExpression:
	default:
		a.addErrorAtToken(expressionToken(expr.Value), "spawn requires a callable expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	execution, graphSpawn := spawnExecutionRelation(expr.Kind)
	spawnedType, _ := a.inferSpawnValue(expr.Value, execution, graphSpawn)
	if spawnedType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	returnType := spawnedType
	if spawnedType.Kind == FunctionType {
		if spawnedType.FunctionReturnType == nil {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		returnType = *spawnedType.FunctionReturnType
	}
	switch expr.Kind {
	case "", "task":
		return taskType(returnType), expressionValue{Display: expr.String()}
	case "thread":
		return threadType(returnType), expressionValue{Display: expr.String()}
	case "process":
		a.addErrorAtToken(expr.Token, "spawn process is not implemented yet")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "unknown spawn kind %s", expr.Kind)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func spawnExecutionRelation(kind string) (CallExecutionRelation, bool) {
	switch kind {
	case "", "task":
		return CallExecutionSpawnTask, true
	case "thread":
		return CallExecutionSpawnThread, true
	case "process":
		return CallExecutionSpawnProcess, false
	default:
		return "", false
	}
}

func (a *Analyzer) inferSpawnValue(expr ast.Expression, execution CallExecutionRelation, graphSpawn bool) (Type, expressionValue) {
	call, isCall := expr.(*ast.CallExpression)
	if !isCall {
		return a.inferExpressionInCancellableContext(expr)
	}
	previousCall := a.spawnCallExpression
	previousExecution := a.spawnCallExecution
	a.spawnCallExpression = call
	if graphSpawn {
		a.spawnCallExecution = execution
	} else {
		a.spawnCallExecution = ""
	}
	defer func() {
		a.spawnCallExpression = previousCall
		a.spawnCallExecution = previousExecution
	}()
	return a.inferExpressionInCancellableContext(expr)
}

func (a *Analyzer) inferExpressionInCancellableContext(expr ast.Expression) (Type, expressionValue) {
	var typ Type
	var value expressionValue
	a.withCancellableContext(func() {
		typ, value = a.inferExpression(expr)
	})
	return typ, value
}

func (a *Analyzer) withCancellableContext(fn func()) {
	a.cancellableDepth++
	defer func() {
		a.cancellableDepth--
	}()
	fn()
}

func (a *Analyzer) inferAwaitExpression(expr *ast.AwaitExpression) (Type, expressionValue) {
	if a.inDeferBlock {
		a.addErrorAtToken(expr.Token, "await is not allowed inside defer")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Value == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !isTaskType(valueType) {
		a.addErrorAtToken(expressionToken(expr.Value), "await requires Task[T], got %s", typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.markMoveSource(expr.Value)
	return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferExpectedUnionVariantExpression(expr ast.Expression, expected Type) (Type, expressionValue, bool) {
	if expected.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		variant, ok := lookupUnionVariant(expected, expr.Value)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s requires payload", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return expected, expressionValue{Display: expr.String()}, true
	case *ast.CallExpression:
		name := callExpressionName(expr)
		if name == "" {
			return Type{}, expressionValue{}, false
		}
		variant, ok := lookupUnionVariant(expected, name)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s requires named payload fields", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if variant.Payload == nil {
			if len(expr.Arguments) != 0 {
				a.addErrorAtToken(expr.Token, "union variant %s.%s expects 0 arguments, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			return expected, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		payloadType := *variant.Payload
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], payloadType)
		if valueType.Kind != InvalidType && !canInitialize(payloadType, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "union variant %s.%s payload must be %s, got %s", typeDisplayName(expected), variant.Name, typeDisplayName(payloadType), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return expected, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func typeWithExpectedDimension(expr ast.Expression, typ Type, value expressionValue, expected Type) (Type, expressionValue) {
	if _, ok := expr.(*ast.InfixExpression); !ok {
		return typ, value
	}
	if typ.Kind == DecimalType && expected.Kind == DecimalType && expected.Named && typ.Dimension.Equal(expected.Dimension) {
		return expected, value
	}
	return typ, value
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
	hasSpread := false
	for _, field := range expr.Fields {
		if field.Spread {
			hasSpread = true
			spreadType, _ := a.inferExpression(field.Value)
			if spreadType.Kind == InvalidType {
				continue
			}
			if !sameConcreteType(typ, spreadType) {
				a.addErrorAtToken(field.Token, "cannot spread %s into %s; spread source must have type %s", typeDisplayName(spreadType), typeDisplayName(typ), typeDisplayName(typ))
				continue
			}
			if !implicitlyCopyable(spreadType) {
				a.addErrorAtToken(field.Token, "cannot spread %s into %s; %s is not implicitly copyable", typeDisplayName(spreadType), typeDisplayName(typ), typeDisplayName(spreadType))
			}
			continue
		}
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
		if definition, exists := memberDefinitionToken(typ, field.Name.Value); exists {
			a.bindDefinition(field.Name.Token, definition)
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
	if !hasSpread {
		for _, field := range typ.Fields {
			if isEventType(field.Type) {
				continue
			}
			if _, supplied := seen[field.Name]; supplied {
				continue
			}
			resolution := DefaultValueOf(field.Type)
			value := defaultExpression(resolution, field.Type, expr.Token)
			if value == nil {
				a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.MissingNonDefaultableField, "initialize the field explicitly", "field %q in struct %s has no default value and must be initialized", field.Name, typ.Name)
				continue
			}
			expr.Fields = append(expr.Fields, &ast.StructLiteralField{Token: expr.Token, Name: &ast.Identifier{Token: field.Token, Value: field.Name}, Value: value})
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
		if field != nil && field.Spread {
			a.addErrorAtToken(field.Token, "spread is not supported in union payload literals")
			continue
		}
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

	a.defineLambdaCaptures(expr, previousSymbols, previousAssigned)

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

func (a *Analyzer) defineLambdaCaptures(expr *ast.LambdaExpression, outerSymbols map[string]Symbol, outerAssigned map[string]bool) {
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
		symbol, ok := outerSymbols[name]
		if !ok {
			a.addErrorAtToken(capture.Name.Token, "undefined capture %s", name)
			continue
		}
		if assigned, exists := outerAssigned[name]; exists && !assigned {
			a.addErrorAtToken(capture.Name.Token, "cannot capture unassigned variable %s", name)
			continue
		}
		symbol.Mutable = false
		a.symbols[name] = symbol
		a.assigned[name] = true
		delete(a.constInts, name)
		a.recordCaptureEscapeFact(name, capture.Name.Token, symbol)
	}
}

func (a *Analyzer) inferMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	if enumType, ok := a.inferEnumValueExpression(expr); ok {
		return enumType, true
	}
	if unionType, ok := a.inferUnionVariantExpression(expr); ok {
		return unionType, true
	}
	if staticType, ok := a.inferStaticMemberExpression(expr); ok {
		return staticType, true
	}

	objectType, _ := a.inferPlaceBase(expr.Object)
	if objectType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}
	objectType = dereferenceType(objectType)
	if definition, exists := memberDefinitionToken(objectType, expr.Property.Value); exists {
		a.bindDefinition(expr.Property.Token, definition)
	}

	if expr.Property.Value == "Ptr" || expr.Property.Value == "ptr" {
		return a.inferPointerMember(expr, objectType)
	}

	if member, ok := compilerKnownMember(objectType, expr.Property.Value, false); ok && member.Kind == CompilerKnownProperty {
		return member.Result, true
	}

	if memberType, ok := a.inferChannelMember(expr, objectType); ok {
		return memberType, true
	}

	if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
		if fieldType.Kind == ReferenceType {
			fieldType = a.referenceTypeWithOriginFromExpression(fieldType, expr.Object)
		}
		return fieldType, true
	}
	if event, ok := lookupEvent(objectType, expr.Property.Value); ok {
		return event.Type, true
	}
	if isMutexGuardType(objectType) {
		protected := objectType.TypeArgs[0]
		if fieldType, ok := lookupStructField(protected, expr.Property.Value); ok {
			if definition, exists := memberDefinitionToken(protected, expr.Property.Value); exists {
				a.bindDefinition(expr.Property.Token, definition)
			}
			return fieldType, true
		}
		if property, ok := lookupProperty(protected, expr.Property.Value); ok {
			a.bindDefinition(expr.Property.Token, property.Token)
			return property.Type, true
		}
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

func (a *Analyzer) inferPointerMember(expr *ast.MemberExpression, objectType Type) (Type, bool) {
	memberName := expr.Property.Value
	if !a.inUnsafe {
		a.addErrorAtToken(expr.Property.Token, "member %s requires unsafe", memberName)
		return Type{Kind: InvalidType}, false
	}
	if !isAddressablePointerSource(expr.Object) {
		a.addErrorAtToken(expr.Property.Token, "member %s requires an addressable value", memberName)
		return Type{Kind: InvalidType}, false
	}
	return compilerKnownRawPointerResult(objectType), true
}

func isAddressablePointerSource(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.Identifier,
		*ast.MemberExpression,
		*ast.IndexExpression,
		*ast.StringLiteral,
		*ast.InterpolatedStringLiteral:
		return true
	default:
		return false
	}
}

func rawPointerType(element Type) Type {
	return Type{
		Name:      "RawPtr",
		Kind:      RawPtrType,
		Intrinsic: true,
		TypeArgs:  []Type{element},
	}
}

func (a *Analyzer) inferChannelMember(expr *ast.MemberExpression, objectType Type) (Type, bool) {
	if !isChannelType(objectType) {
		return Type{}, false
	}
	messageType := objectType.TypeArgs[0]
	switch expr.Property.Value {
	case "tx":
		return senderType(messageType), true
	case "rx":
		return receiverType(messageType), true
	default:
		return Type{}, false
	}
}

func (a *Analyzer) inferStaticMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	path, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Type{}, false
	}
	typeName := a.resolveTypeName(path)
	typ, exists := a.types[typeName]
	if !exists {
		return Type{}, false
	}
	memberName := typeName + "." + expr.Property.Value
	symbol, exists := a.symbols[memberName]
	if !exists {
		if member, ok := compilerKnownMember(typ, expr.Property.Value, true); ok && member.Kind == CompilerKnownProperty {
			return member.Result, true
		}
		return Type{}, false
	}
	a.bindDefinition(expr.Property.Token, symbol.Token)
	return symbol.Type, true
}

func memberDefinitionToken(typ Type, name string) (lexer.Token, bool) {
	for _, field := range typ.Fields {
		if field.Name == name {
			return field.Token, true
		}
	}
	for _, field := range typ.RegisterFields {
		if field.Name == name {
			return field.Token, true
		}
	}
	for _, property := range typ.Properties {
		if property.Name == name {
			return property.Token, true
		}
	}
	for _, event := range typ.Events {
		if event.Name == name {
			return event.Token, true
		}
	}
	for _, method := range typ.InterfaceMethods {
		if method.Name == name {
			return method.Token, true
		}
	}
	for _, property := range typ.InterfaceProperties {
		if property.Name == name {
			return property.Token, true
		}
	}
	for _, event := range typ.InterfaceEvents {
		if event.Name == name {
			return event.Token, true
		}
	}
	return lexer.Token{}, false
}

func (a *Analyzer) inferArrayLiteral(expr *ast.ArrayLiteral) (Type, expressionValue) {
	elementTypes, ok := a.arrayLiteralElementTypes(expr, Type{Kind: InvalidType})
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if len(elementTypes) == 0 {
		a.addErrorAtToken(expr.Token, "cannot infer element type of empty array literal")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	firstType := elementTypes[0]
	if firstType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	for i, elementType := range elementTypes[1:] {
		if elementType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !sameConcreteType(firstType, elementType) {
			a.addErrorAtToken(expressionToken(expr.Elements[min(i+1, len(expr.Elements)-1)]), "array literal elements must have one identical type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}

	return Type{
		Name:        fmt.Sprintf("%s[%d]", typeDisplayName(firstType), len(elementTypes)),
		Kind:        ArrayType,
		Element:     &firstType,
		ArrayLength: int64(len(elementTypes)),
	}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferArrayLiteralWithExpected(expr *ast.ArrayLiteral, expected Type) (Type, expressionValue) {
	if expected.Kind != ArrayType || expected.Element == nil {
		return a.inferArrayLiteral(expr)
	}
	elementTypes, ok := a.arrayLiteralElementTypes(expr, *expected.Element)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if int64(len(elementTypes)) != expected.ArrayLength {
		if expected.ArrayLength != dynamicArrayLength {
			a.addErrorAtToken(expr.Token, "array literal has %d elements, expected %d", len(elementTypes), expected.ArrayLength)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}
	for i, elementType := range elementTypes {
		if elementType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if !canInitialize(*expected.Element, elementType, expr.Elements[min(i, len(expr.Elements)-1)]) {
			a.addErrorAtToken(expressionToken(expr.Elements[min(i, len(expr.Elements)-1)]), "array element %d must be %s, got %s", i+1, typeDisplayName(*expected.Element), typeDisplayName(elementType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
	}
	return expected, expressionValue{Display: expr.String()}
}

func (a *Analyzer) arrayLiteralElementTypes(expr *ast.ArrayLiteral, expected Type) ([]Type, bool) {
	out := []Type{}
	for _, element := range expr.Elements {
		if spread, ok := element.(*ast.SpreadExpression); ok {
			sourceType, _ := a.inferExpression(spread.Value)
			if sourceType.Kind == InvalidType {
				return nil, false
			}
			sourceType = dereferenceType(sourceType)
			if sourceType.Kind != ArrayType || sourceType.Element == nil {
				a.addErrorAtToken(spread.Token, "cannot spread %s into array literal; expansion count is not known at compile time", typeDisplayName(sourceType))
				return nil, false
			}
			if !implicitlyCopyable(*sourceType.Element) {
				a.addErrorAtToken(spread.Token, "cannot spread %s into array literal; %s is not implicitly copyable", typeDisplayName(sourceType), typeDisplayName(*sourceType.Element))
				return nil, false
			}
			for i := int64(0); i < sourceType.ArrayLength; i++ {
				out = append(out, *sourceType.Element)
			}
			continue
		}
		var elementType Type
		if expected.Kind != InvalidType && expected.Kind != "" {
			elementType, _ = a.inferExpressionWithExpected(element, expected)
		} else {
			elementType, _ = a.inferExpression(element)
		}
		if elementType.Kind == InvalidType {
			return nil, false
		}
		out = append(out, elementType)
	}
	return out, true
}

func (a *Analyzer) inferIndexExpression(expr *ast.IndexExpression) (Type, expressionValue) {
	if elementType, ok := a.inferStringPointerIndex(expr); ok {
		return elementType, expressionValue{Display: expr.String()}
	}
	leftType, _ := a.inferPlaceBase(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType = dereferenceType(leftType)
	indexType, _ := a.inferExpression(expr.Index)
	if indexType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !isIntegerType(indexType) {
		a.addErrorAtToken(expressionToken(expr.Index), "%s index must be integer, got %s", indexableKindName(leftType), typeDisplayName(indexType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch leftType.Kind {
	case ArrayType, SliceType:
		if leftType.Element == nil {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.checkConstantIndexBounds(expr, leftType)
		elementType := *leftType.Element
		if elementType.Kind == ReferenceType {
			elementType = a.referenceTypeWithOriginFromExpression(elementType, expr.Left)
		}
		return elementType, expressionValue{Display: expr.String()}
	case StringType:
		return Type{Name: "rune", Kind: RuneType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "type %s is not indexable", typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferStringPointerIndex(expr *ast.IndexExpression) (Type, bool) {
	member, ok := expr.Left.(*ast.MemberExpression)
	if !ok || member.Property == nil || member.Property.Value != "ptr" {
		return Type{}, false
	}
	objectType, _ := a.inferExpression(member.Object)
	if dereferenceType(objectType).Kind != StringType {
		return Type{}, false
	}
	if !a.inUnsafe {
		a.addErrorAtToken(member.Property.Token, "member ptr requires unsafe")
		return Type{Kind: InvalidType}, true
	}
	if !isAddressablePointerSource(member.Object) {
		a.addErrorAtToken(member.Property.Token, "member ptr requires an addressable value")
		return Type{Kind: InvalidType}, true
	}
	indexType, _ := a.inferExpression(expr.Index)
	if indexType.Kind == InvalidType {
		return Type{Kind: InvalidType}, true
	}
	if !isIntegerType(indexType) {
		a.addErrorAtToken(expressionToken(expr.Index), "string byte index must be integer, got %s", typeDisplayName(indexType))
		return Type{Kind: InvalidType}, true
	}
	return a.types["byte"], true
}

func (a *Analyzer) inferSliceExpression(expr *ast.SliceExpression) (Type, expressionValue) {
	leftType, _ := a.inferPlaceBase(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType = dereferenceType(leftType)
	if leftType.Kind != ArrayType && leftType.Kind != SliceType {
		a.addErrorAtToken(expr.Token, "type %s cannot be sliced", typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if leftType.Element == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.checkSliceBounds(expr, leftType)
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Left)
	return Type{
		Name:                       typeDisplayName(*leftType.Element) + "[]",
		Kind:                       SliceType,
		Element:                    leftType.Element,
		ReferenceOriginName:        originName,
		ReferenceOriginToken:       originToken,
		ReferenceOriginLocal:       originLocal,
		ReferenceOriginStorage:     originStorage,
		ReferenceOriginGeneration:  generation,
		ReferenceOriginMatchScoped: a.referenceOriginMatchScopedForExpression(expr.Left),
	}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferRefExpression(expr *ast.RefExpression) (Type, expressionValue) {
	if a.checkBorrowCreation(expr.Value, expr.Mutable, expr.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Value)
	return Type{
		Name:                       referenceTypeName(valueType, expr.Mutable),
		Kind:                       ReferenceType,
		Element:                    &valueType,
		ReferenceMutable:           expr.Mutable,
		ReferenceOriginName:        originName,
		ReferenceOriginToken:       originToken,
		ReferenceOriginLocal:       originLocal,
		ReferenceOriginStorage:     originStorage,
		ReferenceOriginGeneration:  generation,
		ReferenceOriginMatchScoped: a.referenceOriginMatchScopedForExpression(expr.Value),
	}, expressionValue{Display: expr.String()}
}

func referenceTypeName(typ Type, mutable bool) string {
	if mutable {
		return "ref mut " + typeDisplayName(typ)
	}
	return "ref " + typeDisplayName(typ)
}

func indexableKindName(typ Type) string {
	switch typ.Kind {
	case SliceType:
		return "slice"
	case ArrayType:
		return "array"
	default:
		return typeDisplayName(typ)
	}
}

func (a *Analyzer) checkConstantIndexBounds(expr *ast.IndexExpression, typ Type) {
	index, ok := a.integerExpressionInt64(expr.Index)
	if !ok || typ.Kind != ArrayType || typ.ArrayLength == dynamicArrayLength {
		return
	}
	if index < 0 || index >= typ.ArrayLength {
		a.addErrorAtToken(expressionToken(expr.Index), "array index %d is out of bounds for %s", index, typeDisplayName(typ))
	}
}

func (a *Analyzer) checkSliceBounds(expr *ast.SliceExpression, typ Type) {
	if expr.Start != nil {
		start, ok := a.integerExpressionInt64(expr.Start)
		if ok && start < 0 {
			a.addErrorAtToken(expressionToken(expr.Start), "slice lower bound must be non-negative")
		}
	}
	if expr.End != nil {
		end, ok := a.integerExpressionInt64(expr.End)
		if ok && end < 0 {
			a.addErrorAtToken(expressionToken(expr.End), "slice upper bound must be non-negative")
		}
	}
	if typ.Kind != ArrayType {
		return
	}
	if typ.ArrayLength == dynamicArrayLength {
		return
	}
	start, startOK := a.integerExpressionInt64(expr.Start)
	end, endOK := a.integerExpressionInt64(expr.End)
	if expr.Start == nil {
		start, startOK = 0, true
	}
	if expr.End == nil {
		end, endOK = typ.ArrayLength, true
	}
	if startOK && start > typ.ArrayLength {
		a.addErrorAtToken(expr.Token, "slice lower bound %d exceeds length %d", start, typ.ArrayLength)
	}
	if endOK {
		limit := typ.ArrayLength
		if !expr.Exclusive && expr.End != nil {
			limit = typ.ArrayLength - 1
		}
		if end > limit {
			if expr.Exclusive || expr.End == nil {
				a.addErrorAtToken(expr.Token, "exclusive slice upper bound %d exceeds length %d", end, typ.ArrayLength)
			} else {
				a.addErrorAtToken(expr.Token, "inclusive slice upper bound %d exceeds final index %d", end, typ.ArrayLength-1)
			}
		}
	}
	if startOK && endOK && expr.End != nil && start > end {
		op := ".."
		if expr.Exclusive {
			op = "..<"
		}
		a.addErrorAtToken(expr.Token, "descending slice range %d%s%d is invalid", start, op, end)
	}
}

func (a *Analyzer) integerExpressionInt64(expr ast.Expression) (int64, bool) {
	if expr == nil {
		return 0, false
	}
	value, ok := a.integerConstantValue(expr)
	if !ok || !value.IsInt64() {
		return 0, false
	}
	return value.Int64(), true
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
			a.bindDefinition(expr.Property.Token, variant.Token)
			a.addErrorAtToken(expr.Property.Token, "union variant %s.%s requires payload", typeDisplayName(typ), variant.Name)
			return Type{Kind: InvalidType}, true
		}
		a.bindDefinition(expr.Property.Token, variant.Token)
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
	value, ok := typ.EnumConsts[expr.Property.Value]
	if !ok {
		a.addErrorAtToken(expr.Property.Token, "unknown enum value: %s.%s has never been declared", typeName, expr.Property.Value)
		return Type{Kind: InvalidType}, true
	}
	a.bindDefinition(expr.Property.Token, value.Token)
	return typ, true
}

func (a *Analyzer) resolveTypeName(name string) string {
	if name == "self" && a.currentImplTarget != "" {
		return a.currentImplTarget
	}
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
	objectType, _ := a.inferPlaceBase(expr.Object)
	if objectType.Kind == InvalidType {
		return Property{}, false
	}
	return lookupProperty(objectType, expr.Property.Value)
}

func (a *Analyzer) lookupCurrentImplProperty(name string) (Property, bool) {
	if a.currentImplTarget == "" {
		return Property{}, false
	}
	target, ok := a.types[a.currentImplTarget]
	if !ok {
		return Property{}, false
	}
	return lookupProperty(target, name)
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
	typ = dereferenceType(typ)
	for _, field := range typ.Fields {
		if field.Name == name {
			return field.Type, true
		}
	}
	return Type{}, false
}

func lookupRegisterField(typ Type, name string) (Type, bool) {
	typ = dereferenceType(typ)
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

func dereferenceType(typ Type) Type {
	if typ.Kind == ReferenceType && typ.Element != nil {
		return *typ.Element
	}
	return typ
}

func lookupProperty(typ Type, name string) (Property, bool) {
	typ = dereferenceType(typ)
	for _, property := range typ.Properties {
		if property.Name == name {
			return property, true
		}
	}
	return Property{}, false
}

func lookupEvent(typ Type, name string) (Event, bool) {
	typ = dereferenceType(typ)
	for _, event := range typ.Events {
		if event.Name == name {
			return event, true
		}
	}
	return Event{}, false
}

func lookupPropertyByToken(typ Type, name string, token lexer.Token) (Property, bool) {
	for _, property := range typ.Properties {
		if property.Name == name && property.Token.Line == token.Line && property.Token.Column == token.Column {
			return property, true
		}
	}
	return Property{}, false
}

func isEventType(typ Type) bool {
	return typ.Name == "Event" && len(typ.TypeArgs) == 1
}

func isEventStorageType(typ Type) bool {
	return typ.Name == "EventStorage" && len(typ.TypeArgs) == 1
}

func isEventFamilyName(name string) bool {
	return name == "Event" || name == "EventStorage"
}

func isSubscriptionType(typ Type) bool {
	return typ.Name == "Subscription" && len(typ.TypeArgs) == 0
}

func eventCapacity(typ Type) int64 {
	if typ.EventCapacitySet {
		return typ.EventCapacity
	}
	return 4
}

func eventFromField(owner string, name string, typ Type, token lexer.Token) Event {
	return Event{
		Name:     name,
		Type:     typ,
		Payload:  typ.TypeArgs[0],
		Capacity: eventCapacity(typ),
		Token:    token,
		Owner:    owner,
	}
}

func eventTypeFromStorage(storage Type) Type {
	return Type{
		Name:             "Event",
		Kind:             StructType,
		Intrinsic:        true,
		TypeArgs:         []Type{storage.TypeArgs[0]},
		EventCapacity:    eventCapacity(storage),
		EventCapacitySet: storage.EventCapacitySet,
	}
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
	if targetType.Kind == EnumType && targetType.BitWidth > 0 && !a.validateBitEnumConversion(targetType, valueType, expr.Value) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) setCallReferenceOrigin(expr *ast.CallExpression, function Function, args []ast.Expression, isMethodCall bool) {
	delete(a.expressionReferenceOrigins, expr)
	if !function.HasReturnOrigin {
		return
	}
	var receiver ast.Expression
	if isMethodCall {
		if member, ok := expr.Callee.(*ast.MemberExpression); ok {
			receiver = member.Object
		}
	}
	origin := a.instantiateFunctionReturnOrigin(function.ReturnOrigin, args, receiver)
	a.expressionReferenceOrigins[expr] = origin
}

func (a *Analyzer) instantiateFunctionReturnOrigin(summary localReferenceOrigin, args []ast.Expression, receiver ast.Expression) localReferenceOrigin {
	summary = cloneLocalReferenceOrigin(summary)
	places := localOriginPlaces(summary)
	instantiatedPlaces := []Place{}
	actualOrigins := []localReferenceOrigin{}
	unknown := summary.Unknown
	for _, symbolic := range places {
		actual, ok := a.actualOriginForSummaryRoot(symbolic.Root, args, receiver)
		if !ok || actual.Unknown {
			unknown = true
			continue
		}
		actualOrigins = append(actualOrigins, actual)
		for _, base := range localOriginPlaces(actual) {
			for _, projection := range symbolic.Projections {
				base = appendPlaceProjection(base, projection)
			}
			instantiatedPlaces = append(instantiatedPlaces, base)
		}
	}
	result := localReferenceOrigin{Unknown: unknown, Ambiguous: unknown, Mutable: summary.Mutable}
	for _, actual := range actualOrigins {
		if actual.Local && !result.Local {
			result.Local, result.Name, result.Token = true, actual.Name, actual.Token
		}
		if actual.MatchScoped && !result.MatchScoped {
			result.MatchScoped = true
			if result.Name == "" {
				result.Name, result.Token = actual.Name, actual.Token
			}
		}
	}
	result = localOriginWithPlaces(result, instantiatedPlaces)
	result.Unknown = result.Unknown || unknown
	result.Ambiguous = result.Ambiguous || result.Unknown
	if len(summary.Contained) > 0 {
		result.Contained = make(map[string]localReferenceOrigin, len(summary.Contained))
		for path, child := range summary.Contained {
			result.Contained[path] = a.instantiateFunctionReturnOrigin(child, args, receiver)
		}
		if len(places) == 0 {
			recomputeContainedOriginSummary(&result)
			for _, child := range result.Contained {
				if child.Unknown {
					result.Unknown = true
					result.Ambiguous = true
				}
			}
		}
	}
	return result
}

func (a *Analyzer) actualOriginForSummaryRoot(root string, args []ast.Expression, receiver ast.Expression) (localReferenceOrigin, bool) {
	if strings.HasPrefix(root, "$param:") {
		index, err := strconv.Atoi(strings.TrimPrefix(root, "$param:"))
		if err != nil || index < 0 || index >= len(args) {
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		return a.directReferenceOrigin(args[index])
	}
	if root == "$receiver" {
		if receiver == nil {
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		if origin, ok := a.directReferenceOrigin(receiver); ok {
			return origin, true
		}
		place, ok := a.resolvePlace(receiver)
		if !ok {
			return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
		}
		name, token, local, _, _ := a.referenceOriginForExpression(receiver)
		return localOriginWithPlaces(localReferenceOrigin{Name: name, Token: token, Local: local}, placeOriginAlternatives(place)), true
	}
	if strings.HasPrefix(root, "$static:") {
		name := strings.TrimPrefix(root, "$static:")
		place, ok := a.rootPlace(name)
		if !ok {
			place = Place{Root: name, Addressable: true}
		}
		return localOriginWithPlaces(localReferenceOrigin{Name: name}, []Place{place}), true
	}
	return localReferenceOrigin{Unknown: true, Ambiguous: true}, false
}

func (a *Analyzer) inferCallExpression(expr *ast.CallExpression) (Type, expressionValue) {
	if typ, value, ok := a.inferCompilerKnownFunction(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferCompilerKnownConstructor(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferCompilerKnownMemberCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferChannelCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferEventCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferSubscriptionCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferMutexCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferAtomicCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferArenaCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferRawPointerCall(expr); ok {
		return typ, value
	}
	if typ, value, ok := a.inferRuneArrayToStringCall(expr); ok {
		return typ, value
	}

	name := callExpressionName(expr)
	if name == "" {
		return a.inferFunctionValueCall(expr, Type{Kind: InvalidType})
	}

	functions, ok := a.functions[name]
	methodReceiver := methodReceiverInfo{}
	isMethodCall := false
	if !ok || len(functions) == 0 {
		if implName, implOK := a.implScopedFunctionName(name); implOK {
			if implFunctions := a.functions[implName]; len(implFunctions) > 0 {
				name = implName
				functions = implFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		if methodName, methodOK := a.methodCallName(expr); methodOK {
			if methodFunctions := a.functions[methodName]; len(methodFunctions) > 0 {
				methodReceiver, isMethodCall = a.methodCallReceiver(expr)
				if !isMethodCall {
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
				}
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
	a.bindDefinitions(callCalleeDefinitionToken(expr), functionDeclarationTokens(functions))

	sourceArgTypes, sourceArgs, ok := a.callArgumentTypes(expr.Arguments)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	arityMatches := []Function{}
	for _, function := range functions {
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		if len(function.Parameters) == len(argTypes) {
			arityMatches = append(arityMatches, function)
		}
	}

	if len(arityMatches) == 0 {
		a.addErrorAtToken(expr.Token, "function %s expects %s arguments, got %d", name, formatFunctionArities(functions), len(sourceArgTypes))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	matches := []overloadMatch{}
	var receiverError string
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
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			hadGenericInference = true
			function = instantiated
		}
		matchesArguments := true
		rank := 0
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		if isMethodCall && functionUsesReceiver(function) && !a.canPassImplicitMethodReceiver(function, methodReceiver) {
			receiverError = a.implicitMethodReceiverError(function.Name, methodReceiver)
			matchesArguments = false
		}
		for i := range argTypes {
			if !matchesArguments {
				break
			}
			var arg ast.Expression
			arg = sourceArgs[i]
			argType := a.contextualCallArgumentType(arg, argTypes[i], function.Parameters[i].Type)
			if !canInitialize(function.Parameters[i].Type, argType, arg) {
				matchesArguments = false
				break
			}
			rank += overloadArgumentRank(function.Parameters[i].Type, argType)
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
		a.setDefinitions(callCalleeDefinitionToken(expr), best[0].Function.Token)
		dispatch := CallDispatchDirect
		if isMethodCall {
			dispatch = CallDispatchStaticMethod
		} else if best[0].Function.Extern {
			dispatch = CallDispatchForeign
		}
		a.resolvedCalls[expr] = ResolvedCall{Function: best[0].Function, Kind: resolvedCallKind(dispatch)}
		execution, recordCall := a.callGraphExecutionForCall(expr)
		if !a.summaryPass && a.callGraphPathReachable && recordCall {
			a.callGraph.addCall(a.currentCallable, best[0].Function, callCalleeDefinitionToken(expr), dispatch, execution)
		}
		a.setCallReferenceOrigin(expr, best[0].Function, sourceArgs, isMethodCall)
		a.markMovedCallArguments(best[0].Function, sourceArgs, isMethodCall)
		a.markClosedResourceCall(best[0].Function, methodReceiver, isMethodCall, expr.Token)
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}
	}

	if len(best) > 1 {
		ambiguous := make([]lexer.Token, 0, len(best))
		for _, match := range best {
			ambiguous = append(ambiguous, match.Function.Token)
		}
		a.bindDefinitions(callCalleeDefinitionToken(expr), ambiguous)
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
	if receiverError != "" {
		a.addErrorAtToken(expr.Token, "%s", receiverError)
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
			argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
			instantiated, ok := a.inferGenericFunctionInstance(function, argTypes)
			if !ok {
				continue
			}
			function = instantiated
			displayName = genericFunctionDisplayName(name, function)
		}
		argTypes := a.callArgumentTypesForFunction(function, sourceArgTypes, methodReceiver, isMethodCall)
		for i := range argTypes {
			arg := sourceArgs[i]
			param := function.Parameters[i]
			argType := a.contextualCallArgumentType(arg, argTypes[i], param.Type)
			if !canInitialize(param.Type, argType, arg) {
				a.addErrorAtToken(expressionToken(arg), "argument %d to %s must be %s, got %s", i+1, displayName, typeDisplayName(param.Type), typeDisplayName(argType))
			}
		}
		break
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) callGraphExecutionForCall(expr *ast.CallExpression) (CallExecutionRelation, bool) {
	if expr == a.spawnCallExpression {
		if a.spawnCallExecution == "" {
			return "", false
		}
		return a.spawnCallExecution, true
	}
	return CallExecutionSynchronous, true
}

func callCalleeDefinitionToken(expr *ast.CallExpression) lexer.Token {
	if expr == nil {
		return lexer.Token{}
	}
	switch callee := expr.Callee.(type) {
	case *ast.Identifier:
		if callee != nil {
			return callee.Token
		}
	case *ast.MemberExpression:
		if callee != nil && callee.Property != nil {
			return callee.Property.Token
		}
	}
	if expr.Function != nil {
		return expr.Function.Token
	}
	return expr.Token
}

func functionDeclarationTokens(functions []Function) []lexer.Token {
	tokens := make([]lexer.Token, 0, len(functions))
	for _, function := range functions {
		tokens = append(tokens, function.Token)
	}
	return tokens
}

func (a *Analyzer) contextualCallArgumentType(arg ast.Expression, actual Type, expected Type) Type {
	if _, ok := arg.(*ast.CharLiteral); ok && expected.Kind == RuneType {
		runeType := Type{Name: "rune", Kind: RuneType}
		a.expressionTypes[arg] = runeType
		return runeType
	}
	return actual
}

func (a *Analyzer) inferCompilerKnownFunction(expr *ast.CallExpression) (Type, expressionValue, bool) {
	if callExpressionName(expr) != "len" {
		return Type{}, expressionValue{}, false
	}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "len infers its element type from its argument")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "len expects 1 argument, got %d", len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	argumentType, _ := a.inferExpression(expr.Arguments[0])
	if argumentType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if argumentType.Kind == StringType || argumentType.Kind == ArrayType {
		return a.types["int"], expressionValue{Display: expr.String()}, true
	}
	if argumentType.Kind == ReferenceType && argumentType.Element != nil {
		referencedType := *argumentType.Element
		if referencedType.Kind == ArrayType || referencedType.Kind == SliceType {
			return a.types["int"], expressionValue{Display: expr.String()}, true
		}
	}

	a.addErrorAtToken(expressionToken(expr.Arguments[0]), "len requires string, an array, or a slice reference, got %s", typeDisplayName(argumentType))
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferCompilerKnownMemberCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	memberExpr, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || memberExpr == nil || memberExpr.Property == nil {
		return Type{}, expressionValue{}, false
	}

	if path, static := typePathFromExpression(memberExpr.Object); static {
		typ, exists := a.types[a.resolveTypeName(path)]
		if exists {
			member, memberExists := compilerKnownMember(typ, memberExpr.Property.Value, true)
			if !memberExists || (member.Kind != CompilerKnownMethod && member.Kind != CompilerKnownAssociatedFunction) {
				return Type{}, expressionValue{}, false
			}
			switch member.Name {
			case "SizeOf":
				if !a.checkCompilerKnownCallArity(expr, typeDisplayName(typ)+".SizeOf", 0, 0) {
					return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
				}
				return member.Result, expressionValue{Display: expr.String()}, true
			case "FromByteArray", "FromRuneArray":
				return a.inferCompilerKnownStringConstructor(expr, member)
			case "FromBuffer", "WithCapacity", "Growable":
				return a.inferArenaConstructorCall(expr, member)
			}
			return Type{}, expressionValue{}, false
		}
		if root := compilerKnownReceiverRoot(memberExpr.Object); root != nil {
			if _, isValue := a.symbols[root.Value]; !isValue {
				return Type{}, expressionValue{}, false
			}
		}
	}

	receiverType, _ := a.inferExpression(memberExpr.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	lookupType := dereferenceType(receiverType)
	member, exists := compilerKnownMember(lookupType, memberExpr.Property.Value, false)
	if !exists || member.Kind != CompilerKnownMethod {
		return Type{}, expressionValue{}, false
	}
	if lookupType.Named && len(a.functions[lookupType.Name+"."+member.Name]) > 0 {
		return Type{}, expressionValue{}, false
	}
	if lookupType.Kind == RawPtrType || lookupType.Name == "Arena" {
		return Type{}, expressionValue{}, false
	}
	switch member.Name {
	case "SizeOf", "ToString", "ToByteArray", "ToCharArray", "ToRuneArray":
		if !a.checkCompilerKnownCallArity(expr, typeDisplayName(lookupType)+"."+member.Name, 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return member.Result, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func compilerKnownReceiverRoot(expr ast.Expression) *ast.Identifier {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr
	case *ast.MemberExpression:
		return compilerKnownReceiverRoot(expr.Object)
	case *ast.IndexExpression:
		return compilerKnownReceiverRoot(expr.Left)
	default:
		return nil
	}
}

func (a *Analyzer) inferCompilerKnownStringConstructor(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "string."+member.Name, 1, 1) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	argumentType, _ := a.inferExpression(expr.Arguments[0])
	sequence := dereferenceType(argumentType)
	want := "byte"
	if member.Name == "FromRuneArray" {
		want = "rune"
	}
	if (sequence.Kind != ArrayType && sequence.Kind != SliceType) || sequence.Element == nil || sequence.Element.Name != want {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string.%s requires a %s array or slice, got %s", member.Name, want, typeDisplayName(argumentType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return member.Result, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferArenaConstructorCall(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "Arena."+member.Name, 1, 1) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	argumentType, _ := a.inferExpression(expr.Arguments[0])
	if argumentType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if member.Name == "FromBuffer" {
		if argumentType.Kind != ReferenceType || !argumentType.ReferenceMutable || argumentType.Element == nil || argumentType.Element.Kind != SliceType || argumentType.Element.Element == nil || argumentType.Element.Element.Name != "byte" {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.FromBuffer requires ref mut byte[], got %s", typeDisplayName(argumentType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordArenaEffect(ArenaEffectCreateBorrowed, "", callCalleeDefinitionToken(expr), false)
		return a.types["Arena"], expressionValue{Display: expr.String()}, true
	}
	if !canInitialize(a.types["uint"], argumentType, expr.Arguments[0]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.%s capacity must be uint, got %s", member.Name, typeDisplayName(argumentType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	effect := ArenaEffectCreateOwned
	if member.Name == "Growable" {
		effect = ArenaEffectCreateGrowable
	}
	a.recordArenaEffect(effect, "", callCalleeDefinitionToken(expr), true)
	return arenaResultType(a.types["Arena"], a.types["AllocationError"]), expressionValue{Display: expr.String()}, true
}

func arenaResultType(value Type, err Type) Type {
	return Type{Name: "Result[" + typeDisplayName(value) + ", " + typeDisplayName(err) + "]", Kind: ResultType, TypeArgs: []Type{value, err}}
}

// inferRuneArrayToStringCall recognizes the allocation-backed text
// materialization available on rune arrays and rune slice views.
func (a *Analyzer) inferRuneArrayToStringCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil || member.Property.Value != "ToString" || a.expressionNamesType(member.Object) {
		return Type{}, expressionValue{}, false
	}

	receiverType, _ := a.inferExpression(member.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	receiverType = dereferenceType(receiverType)
	if (receiverType.Kind != ArrayType && receiverType.Kind != SliceType) || receiverType.Element == nil || receiverType.Element.Kind != RuneType {
		return Type{}, expressionValue{}, false
	}
	if len(expr.Arguments) != 0 {
		a.addErrorAtToken(expr.Token, "rune array ToString expects 0 arguments, got %d", len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return a.types["string"], expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferRawPointerCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !a.rawPointerCallMayHaveValueReceiver(member.Object) {
		return Type{}, expressionValue{}, false
	}
	receiverType, _ := a.inferExpression(member.Object)
	if receiverType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	receiverType = dereferenceType(receiverType)
	if receiverType.Kind != RawPtrType {
		return Type{}, expressionValue{}, false
	}

	switch member.Property.Value {
	case "Read":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Read requires unsafe because it reads through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.Read", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		if element.Kind == InvalidType || element.Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Read requires a concrete non-void element type")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "Write":
		if !a.inUnsafe {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Write requires unsafe because it writes through a raw pointer")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkCompilerKnownCallArity(expr, "RawPtr.Write", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		element := compilerKnownRawPointerElement(receiverType)
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if element.Kind == InvalidType || element.Kind == VoidType || !canInitialize(element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "RawPtr.Write value must be %s, got %s", typeDisplayName(element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["void"], expressionValue{Display: expr.String()}, true
	case "Offset":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.Offset") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.Offset expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(receiverType.TypeArgs) == 1 && receiverType.TypeArgs[0].Kind == VoidType {
			a.addErrorAtToken(member.Property.Token, "RawPtr.Offset requires a typed element pointer, got RawPtr[void]")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.Offset") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "AddBytes":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.AddBytes") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.AddBytes expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !isRawBytePointer(receiverType) {
			a.addErrorAtToken(member.Property.Token, "RawPtr.AddBytes requires RawPtr[byte], got %s", typeDisplayName(receiverType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.rawPointerArgumentIsInt(expr.Arguments[0], "RawPtr.AddBytes") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return receiverType, expressionValue{Display: expr.String()}, true
	case "Difference":
		if !a.rawPointerOperationRequiresUnsafe(member.Property.Token, "RawPtr.Difference") {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "RawPtr.Difference expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		otherType, _ := a.inferExpression(expr.Arguments[0])
		if otherType.Kind == InvalidType {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !sameConcreteType(receiverType, otherType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "RawPtr.Difference argument must be %s, got %s", typeDisplayName(receiverType), typeDisplayName(otherType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["int"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) rawPointerCallMayHaveValueReceiver(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		_, exists := a.symbols[expr.Value]
		return exists
	case *ast.MemberExpression:
		return a.rawPointerCallMayHaveValueReceiver(expr.Object)
	case *ast.IndexExpression:
		return true
	case *ast.CallExpression, *ast.ConversionExpression, *ast.RefExpression, *ast.AwaitExpression, *ast.TryExpression:
		return true
	default:
		return false
	}
}

func (a *Analyzer) rawPointerOperationRequiresUnsafe(token lexer.Token, operation string) bool {
	if a.inUnsafe {
		return true
	}
	a.addErrorAtToken(token, "%s requires unsafe because it performs raw-pointer arithmetic", operation)
	return false
}

func (a *Analyzer) rawPointerArgumentIsInt(expr ast.Expression, operation string) bool {
	argType, _ := a.inferExpression(expr)
	if argType.Kind == InvalidType {
		return false
	}
	if argType.Kind != IntType || argType.Name != "int" {
		a.addErrorAtToken(expressionToken(expr), "%s argument must be int, got %s", operation, typeDisplayName(argType))
		return false
	}
	return true
}

func isRawBytePointer(typ Type) bool {
	return typ.Kind == RawPtrType && len(typ.TypeArgs) == 1 && typ.TypeArgs[0].Name == "byte"
}

func (a *Analyzer) inferArenaCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	symbol, ok := a.symbols[receiver.Value]
	if !ok || symbol.Type.Name != "Arena" {
		return Type{}, expressionValue{}, false
	}
	switch member.Property.Value {
	case "New":
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.New requires exactly 1 type argument, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.New expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.New requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, resolved := a.resolveType(expr.GenericArguments[0])
		if !resolved {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !TriviallyDestructible(elementType) || !IsDefaultable(elementType) {
			a.addErrorAtToken(expr.GenericArguments[0].Token, "Arena.New requires a defaultable, trivially destructible type, got %s", typeDisplayName(elementType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordArenaEffect(ArenaEffectAllocate, receiver.Value, member.Property.Token, true)
		refType := Type{Name: "ref mut " + typeDisplayName(elementType), Kind: ReferenceType, Element: &elementType, ReferenceMutable: true, ReferenceOriginName: receiver.Value, ReferenceOriginToken: receiver.Token, ReferenceOriginLocal: symbol.Local, ReferenceOriginStorage: StorageOriginArena, ReferenceOriginGeneration: a.arenaGenerations[receiver.Value]}
		return arenaResultType(refType, a.types["AllocationError"]), expressionValue{Display: expr.String()}, true
	case "Alloc":
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.Alloc requires exactly 1 type argument, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.Alloc expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Alloc requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		countType, _ := a.inferExpressionWithExpected(expr.Arguments[0], a.types["uint"])
		if countType.Kind != InvalidType && !canInitialize(a.types["uint"], countType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.Alloc count must be uint, got %s", typeDisplayName(countType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := a.resolveType(expr.GenericArguments[0])
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		sliceType := Type{
			Name:    typeDisplayName(elementType) + "[]",
			Kind:    SliceType,
			Element: &elementType,
		}
		refSliceType := Type{
			Name:                      "ref mut " + typeDisplayName(sliceType),
			Kind:                      ReferenceType,
			Element:                   &sliceType,
			ReferenceMutable:          true,
			ReferenceOriginName:       receiver.Value,
			ReferenceOriginToken:      receiver.Token,
			ReferenceOriginLocal:      symbol.Local,
			ReferenceOriginStorage:    StorageOriginArena,
			ReferenceOriginGeneration: a.arenaGenerations[receiver.Value],
		}
		errType := a.types["AllocationError"]
		a.recordArenaEffect(ArenaEffectAllocate, receiver.Value, member.Property.Token, true)
		return Type{
			Name:     "Result[" + typeDisplayName(refSliceType) + ", AllocationError]",
			Kind:     ResultType,
			TypeArgs: []Type{refSliceType, errType},
		}, expressionValue{Display: expr.String()}, true
	case "Reset":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Reset does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Reset expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Reset requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.arenaGenerations[receiver.Value]++
		a.recordArenaEffect(ArenaEffectReset, receiver.Value, member.Property.Token, false)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Release":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Release does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Release expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Release requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.arenaGenerations[receiver.Value]++
		a.moved[receiver.Value] = expr.Token
		a.moveReasons[receiver.Value] = "released"
		a.recordArenaEffect(ArenaEffectRelease, receiver.Value, member.Property.Token, false)
		return a.types["void"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferCompilerKnownConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	name := callExpressionName(expr)
	if name == "Channel" {
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Channel requires exactly 1 message type, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Channel expects 1 capacity argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageType, ok := a.resolveType(expr.GenericArguments[0])
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		capacityType, _ := a.inferExpression(expr.Arguments[0])
		if capacityType.Kind != InvalidType && !isIntegerType(capacityType) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Channel capacity must be integer, got %s", typeDisplayName(capacityType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return channelType(messageType), expressionValue{Display: expr.String()}, true
	}
	if name != "Mutex" && name != "Atomic" {
		return Type{}, expressionValue{}, false
	}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "%s constructor infers its type argument from the initializer", name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "%s expects 1 argument, got %d", name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	valueType, _ := a.inferExpression(expr.Arguments[0])
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if name == "Atomic" && !atomicElementTypeSupported(valueType) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "type %s is not supported by Atomic", typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return Type{Name: name, Kind: StructType, TypeArgs: []Type{valueType}}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferMutexCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !isMutexType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	switch member.Property.Value {
	case "lock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.lock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.lock expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return mutexGuardType(receiverType.TypeArgs[0]), expressionValue{Display: expr.String()}, true
	case "tryLock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.tryLock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.tryLock expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		guard := mutexGuardType(receiverType.TypeArgs[0])
		return Type{Name: "Option", Kind: UnionType, TypeArgs: []Type{guard}}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

type eventReceiverInfo struct {
	Type  Type
	Name  string
	Owner string
}

func (a *Analyzer) inferEventCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	if member.Property.Value != "Publish" && member.Property.Value != "Subscribe" {
		return Type{}, expressionValue{}, false
	}
	receiver, ok := a.eventReceiverInfo(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	payload := receiver.Type.TypeArgs[0]
	switch member.Property.Value {
	case "Publish":
		if !a.checkCompilerKnownCallArity(expr, "Event.Publish", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if receiver.Owner == "" || receiver.Owner != a.currentImplTarget {
			owner := receiver.Owner
			if owner == "" {
				owner = "the owning type"
			}
			a.addErrorAtToken(member.Property.Token, "event %s may only be published by %s", receiver.Name, owner)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		argType, _ := a.inferExpressionWithExpected(expr.Arguments[0], payload)
		if argType.Kind != InvalidType && !canInitialize(payload, argType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Event.Publish payload must be %s, got %s", typeDisplayName(payload), typeDisplayName(argType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Subscribe":
		if !a.checkCompilerKnownCallArity(expr, "Event.Subscribe", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		handlerType, _ := a.inferExpression(expr.Arguments[0])
		expected := eventHandlerType(payload)
		if handlerType.Kind != InvalidType && !sameFunctionType(handlerType, expected) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Event.Subscribe handler must be %s, got %s", typeDisplayName(expected), typeDisplayName(handlerType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.types["EventSubscribeResult"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) eventReceiverInfo(expr ast.Expression) (eventReceiverInfo, bool) {
	eventType, _ := a.inferExpression(expr)
	if !isEventType(eventType) {
		return eventReceiverInfo{}, false
	}
	info := eventReceiverInfo{Type: eventType, Name: "event"}
	switch receiver := expr.(type) {
	case *ast.Identifier:
		info.Name = receiver.Value
		if a.currentImplTarget != "" {
			if target, ok := a.types[a.currentImplTarget]; ok {
				if event, ok := lookupEvent(target, receiver.Value); ok {
					info.Owner = event.Owner
				}
			}
		}
	case *ast.MemberExpression:
		info.Name = receiver.Property.Value
		objectType, _ := a.inferExpression(receiver.Object)
		objectType = dereferenceType(objectType)
		if event, ok := lookupEvent(objectType, receiver.Property.Value); ok {
			info.Owner = event.Owner
		}
	}
	return info, true
}

func eventHandlerType(payload Type) Type {
	return Type{
		Name:                   functionTypeName([]Type{payload}, Type{Name: "void", Kind: VoidType}),
		Kind:                   FunctionType,
		FunctionParameterTypes: []Type{payload},
		FunctionReturnType:     &Type{Name: "void", Kind: VoidType},
	}
}

func (a *Analyzer) inferSubscriptionCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || member.Property.Value != "Close" {
		return Type{}, expressionValue{}, false
	}
	if ident, ok := member.Object.(*ast.Identifier); ok {
		symbol, exists := a.symbols[ident.Value]
		if !exists || !isSubscriptionType(symbol.Type) {
			return Type{}, expressionValue{}, false
		}
	} else {
		receiverType, _ := a.inferExpression(member.Object)
		if !isSubscriptionType(receiverType) {
			return Type{}, expressionValue{}, false
		}
	}
	receiverType, _ := a.inferExpression(member.Object)
	if !isSubscriptionType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	if !a.checkCompilerKnownCallArity(expr, "Subscription.Close", 0, 0) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	a.markMoveSource(member.Object)
	return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
}

func (a *Analyzer) inferChannelCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if isSenderType(receiverType) {
		return a.inferSenderCall(expr, member, receiverType)
	}
	if isReceiverType(receiverType) {
		return a.inferReceiverCall(expr, member, receiverType)
	}
	return Type{}, expressionValue{}, false
}

func (a *Analyzer) inferSenderCall(expr *ast.CallExpression, member *ast.MemberExpression, receiverType Type) (Type, expressionValue, bool) {
	messageType := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "Share":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Share", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return senderType(messageType), expressionValue{Display: expr.String()}, true
	case "Send":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Send", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.Send message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 2 {
			lifetimeType, _ := a.inferExpression(expr.Arguments[1])
			if lifetimeType.Kind != InvalidType && !isNumericType(lifetimeType) {
				a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Sender.Send message lifetime must be duration-compatible numeric value, got %s", typeDisplayName(lifetimeType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelSendResult", messageType), expressionValue{Display: expr.String()}, true
	case "SendRevocable":
		if !a.checkCompilerKnownCallArity(expr, "Sender.SendRevocable", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.SendRevocable message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 2 {
			lifetimeType, _ := a.inferExpression(expr.Arguments[1])
			if lifetimeType.Kind != InvalidType && !isNumericType(lifetimeType) {
				a.addErrorAtToken(expressionToken(expr.Arguments[1]), "Sender.SendRevocable message lifetime must be duration-compatible numeric value, got %s", typeDisplayName(lifetimeType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		a.markMoveSource(expr.Arguments[0])
		return messageTicketType(messageType), expressionValue{Display: expr.String()}, true
	case "TrySend":
		if !a.checkCompilerKnownCallArity(expr, "Sender.TrySend", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		messageArgType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageType)
		if messageArgType.Kind != InvalidType && !canInitialize(messageType, messageArgType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.TrySend message must be %s, got %s", typeDisplayName(messageType), typeDisplayName(messageArgType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelSendResult", messageType), expressionValue{Display: expr.String()}, true
	case "Revoke":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Revoke", 1, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		ticketType, _ := a.inferExpressionWithExpected(expr.Arguments[0], messageTicketType(messageType))
		if ticketType.Kind != InvalidType && !sameConcreteType(ticketType, messageTicketType(messageType)) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Sender.Revoke ticket must be MessageTicket[%s], got %s", typeDisplayName(messageType), typeDisplayName(ticketType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(expr.Arguments[0])
		return a.intrinsicGenericType("ChannelRevokeResult", messageType), expressionValue{Display: expr.String()}, true
	case "Close":
		if !a.checkCompilerKnownCallArity(expr, "Sender.Close", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(member.Object)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferReceiverCall(expr *ast.CallExpression, member *ast.MemberExpression, receiverType Type) (Type, expressionValue, bool) {
	messageType := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "Receive":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Receive", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.intrinsicGenericType("Option", messageType), expressionValue{Display: expr.String()}, true
	case "TryReceive":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.TryReceive", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return a.intrinsicGenericType("ChannelTryReceiveResult", messageType), expressionValue{Display: expr.String()}, true
	case "Discard":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Discard", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Close":
		if !a.checkCompilerKnownCallArity(expr, "Receiver.Close", 0, 0) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.markMoveSource(member.Object)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) inferAtomicCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !isAtomicType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	element := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "load":
		if !a.checkAtomicCallArity(expr, "Atomic.load", 0, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "store":
		if !a.checkAtomicCallArity(expr, "Atomic.store", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "swap":
		if !a.checkAtomicCallArity(expr, "Atomic.swap", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "fetchAdd", "fetchSub", "fetchAnd", "fetchOr", "fetchXor":
		if !a.checkAtomicCallArity(expr, "Atomic."+member.Property.Value, 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !atomicFetchOperationSupported(member.Property.Value, element) {
			a.addErrorAtToken(member.Property.Token, "%s is not supported for Atomic[%s]", member.Property.Value, typeDisplayName(element))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "compareExchange":
		if !a.checkAtomicCallArity(expr, "Atomic.compareExchange", 2, 4) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		a.checkAtomicValueArgument(expr, element, 1)
		return Type{Name: "CompareExchangeResult", Kind: StructType, TypeArgs: []Type{element}}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

func (a *Analyzer) checkAtomicCallArity(expr *ast.CallExpression, name string, minArgs int, maxArgs int) bool {
	return a.checkCompilerKnownCallArity(expr, name, minArgs, maxArgs)
}

func (a *Analyzer) checkCompilerKnownCallArity(expr *ast.CallExpression, name string, minArgs int, maxArgs int) bool {
	if len(expr.GenericArguments) != 0 {
		a.addErrorAtToken(expr.Token, "%s does not take type arguments", name)
		return false
	}
	if len(expr.Arguments) < minArgs || len(expr.Arguments) > maxArgs {
		if minArgs == maxArgs {
			a.addErrorAtToken(expr.Token, "%s expects %d arguments, got %d", name, minArgs, len(expr.Arguments))
		} else {
			a.addErrorAtToken(expr.Token, "%s expects %d or %d arguments, got %d", name, minArgs, maxArgs, len(expr.Arguments))
		}
		return false
	}
	return true
}

func (a *Analyzer) checkAtomicValueArgument(expr *ast.CallExpression, expected Type, index int) {
	if index >= len(expr.Arguments) {
		return
	}
	argType, _ := a.inferExpression(expr.Arguments[index])
	if argType.Kind != InvalidType && !canInitialize(expected, argType, expr.Arguments[index]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[index]), "atomic argument %d must be %s, got %s", index+1, typeDisplayName(expected), typeDisplayName(argType))
	}
}

func (a *Analyzer) compilerKnownReceiverType(expr ast.Expression) (Type, bool) {
	ident, ok := expr.(*ast.Identifier)
	if !ok {
		return Type{}, false
	}
	symbol, ok := a.symbols[ident.Value]
	if !ok {
		return Type{}, false
	}
	return symbol.Type, true
}

func (a *Analyzer) callArgumentTypes(args []ast.Expression) ([]Type, []ast.Expression, bool) {
	types := []Type{}
	expressions := []ast.Expression{}
	for _, arg := range args {
		if spread, ok := arg.(*ast.SpreadExpression); ok {
			argType, _ := a.inferExpression(spread.Value)
			if argType.Kind == InvalidType {
				return nil, nil, false
			}
			argType = dereferenceType(argType)
			if argType.Kind != ArrayType || argType.Element == nil {
				a.addErrorAtToken(spread.Token, "cannot spread %s into fixed-arity call; expansion count is not known at compile time", typeDisplayName(argType))
				return nil, nil, false
			}
			if argType.ArrayLength == dynamicArrayLength {
				a.addErrorAtToken(spread.Token, "cannot spread %s into fixed-arity call; expansion count is not known at compile time", typeDisplayName(argType))
				return nil, nil, false
			}
			if !implicitlyCopyable(*argType.Element) {
				a.addErrorAtToken(spread.Token, "cannot spread %s into function arguments; %s is not implicitly copyable", typeDisplayName(argType), typeDisplayName(*argType.Element))
				return nil, nil, false
			}
			for i := int64(0); i < argType.ArrayLength; i++ {
				types = append(types, *argType.Element)
				expressions = append(expressions, spread.Value)
			}
			continue
		}
		argType, _ := a.inferExpression(arg)
		if argType.Kind == InvalidType {
			return nil, nil, false
		}
		types = append(types, argType)
		expressions = append(expressions, arg)
	}
	return types, expressions, true
}

func (a *Analyzer) markMovedCallArguments(function Function, sourceArgs []ast.Expression, isMethodCall bool) {
	sourceIndex := 0
	for _, param := range function.Parameters {
		if sourceIndex >= len(sourceArgs) {
			return
		}
		arg := sourceArgs[sourceIndex]
		sourceIndex++
		if param.Ref {
			continue
		}
		if a.markMoveSource(arg) {
			if ident, ok := arg.(*ast.Identifier); ok {
				a.endBorrowsHeldBy(ident.Value)
			}
		}
	}
}

func (a *Analyzer) methodCallName(expr *ast.CallExpression) (string, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return "", false
	}
	if a.expressionNamesType(member.Object) {
		return "", false
	}
	objectType, _ := a.inferExpression(member.Object)
	if objectType.Kind == InvalidType || objectType.Name == "" {
		return "", false
	}
	objectType = dereferenceType(objectType)
	return objectType.Name + "." + member.Property.Value, true
}

type methodReceiverInfo struct {
	Type   Type
	Symbol *Symbol
}

func (a *Analyzer) callArgumentTypesForFunction(_ Function, sourceArgTypes []Type, _ methodReceiverInfo, _ bool) []Type {
	return sourceArgTypes
}

func functionUsesReceiver(function Function) bool {
	return function.ImplTarget != ""
}

func (a *Analyzer) methodCallReceiver(expr *ast.CallExpression) (methodReceiverInfo, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return methodReceiverInfo{}, false
	}
	if a.expressionNamesType(member.Object) {
		return methodReceiverInfo{}, false
	}
	receiverType, _ := a.inferExpression(member.Object)
	info := methodReceiverInfo{Type: receiverType}
	if symbol, ok := a.symbolForMemberObject(member.Object); ok {
		info.Symbol = &symbol
	}
	return info, receiverType.Kind != InvalidType
}

func (a *Analyzer) expressionNamesType(expr ast.Expression) bool {
	if ident, ok := expr.(*ast.Identifier); ok {
		if _, exists := a.symbols[ident.Value]; exists {
			return false
		}
	}
	typeName, ok := typePathFromExpression(expr)
	if !ok {
		return false
	}
	_, exists := a.types[a.resolveTypeName(typeName)]
	return exists
}

func (a *Analyzer) canPassImplicitMethodReceiver(function Function, receiver methodReceiverInfo) bool {
	if function.ImplTarget == "" {
		return true
	}
	if receiver.Type.Kind == InvalidType || typeDisplayName(dereferenceType(receiver.Type)) != function.ImplTarget {
		return false
	}
	if function.ReceiverMutable {
		if receiver.Type.Kind == ReferenceType {
			return receiver.Type.ReferenceMutable
		}
		if receiver.Symbol != nil {
			return a.canWriteThroughSymbol(*receiver.Symbol)
		}
		return false
	}
	return true
}

func (a *Analyzer) implicitMethodReceiverError(methodName string, receiver methodReceiverInfo) string {
	displayName := visibilityBaseName(methodName)
	if receiver.Symbol != nil {
		if receiver.Symbol.Addressed {
			return fmt.Sprintf("method %s requires writable receiver storage", displayName)
		}
		return fmt.Sprintf("method %s requires mutable receiver", displayName)
	}
	return ""
}

func (a *Analyzer) canWriteThroughSymbol(symbol Symbol) bool {
	if symbol.Type.Kind == ReferenceType {
		return symbol.Type.ReferenceMutable
	}
	return symbol.Mutable
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
	a.bindDefinition(member.Property.Token, variant.Token)

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
		if implName, implOK := a.implScopedFunctionName(name); implOK {
			if implFunctions := a.functions[implName]; len(implFunctions) > 0 {
				name = implName
				functions = implFunctions
				ok = true
			}
		}
	}
	if !ok || len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}
	functions = a.accessibleFunctions(functions)
	if len(functions) == 0 {
		return Type{}, expressionValue{}, false
	}

	argTypes, args, argsOK := a.callArgumentTypes(expr.Arguments)
	if !argsOK {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	matches := []overloadMatch{}
	for _, function := range functions {
		if len(function.Parameters) != len(argTypes) || len(function.GenericParameters) == 0 {
			continue
		}
		instantiated, ok := a.inferGenericFunctionInstanceWithExpected(function, argTypes, expected)
		if !ok {
			continue
		}

		matchesArguments := true
		rank := 0
		for i, arg := range args {
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
		a.setDefinitions(callCalleeDefinitionToken(expr), best[0].Function.Token)
		a.setCallReferenceOrigin(expr, best[0].Function, args, false)
		return best[0].Function.ReturnType, expressionValue{Display: expr.String()}, true
	}
	if len(best) > 1 {
		ambiguous := make([]lexer.Token, 0, len(best))
		for _, match := range best {
			ambiguous = append(ambiguous, match.Function.Token)
		}
		a.bindDefinitions(callCalleeDefinitionToken(expr), ambiguous)
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
		Declaration:    typeDeclarationIdentity(typ),
		Arguments:      canonicalTypeArgumentsKey(typ.TypeArgs),
		ConstArguments: canonicalConstArgumentsKey(typ.ConstArgs),
	}
}

func canonicalConstArgumentsKey(args []int64) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, strconv.FormatInt(arg, 10))
	}
	return strings.Join(parts, ",")
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
		if len(typ.ConstArgs) > 0 {
			parts := make([]string, 0, len(typ.ConstArgs))
			for _, arg := range typ.ConstArgs {
				parts = append(parts, fmt.Sprintf("%d", arg))
			}
			identity += "[" + strings.Join(parts, ";") + "]"
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

func (a *Analyzer) implScopedFunctionName(name string) (string, bool) {
	if a.currentImplTarget == "" || name == "" || strings.Contains(name, ".") {
		return "", false
	}
	return a.currentImplTarget + "." + name, true
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
	if targetType.Kind == EnumType && targetType.BitWidth > 0 && !a.validateBitEnumConversion(targetType, valueType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) validateBitEnumConversion(target Type, sourceType Type, source ast.Expression) bool {
	if value, ok := a.integerConstantValue(source); ok {
		return !a.checkIntegerValueRange(target, value, expressionToken(source))
	}
	if sourceType.MinInteger != nil && sourceType.MaxInteger != nil &&
		target.MinInteger != nil && target.MaxInteger != nil &&
		sourceType.MinInteger.Cmp(target.MinInteger) >= 0 &&
		sourceType.MaxInteger.Cmp(target.MaxInteger) <= 0 {
		return true
	}
	a.addErrorAtToken(
		expressionToken(source),
		"conversion to %d-bit enum %s requires a value proven to be in range 0..%s",
		target.BitWidth,
		target.Name,
		target.MaxInteger.String(),
	)
	return false
}

func (a *Analyzer) inferTryExpression(expr *ast.TryExpression) (Type, expressionValue) {
	valueType, _ := a.inferExpression(expr.Expression)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if operator, ok := a.ResolvedOperatorOf(expr.Expression); ok && operator.RuntimeCheck {
		return a.inferArithmeticTryExpression(expr, operator)
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
		plan, valid := a.analyzeTryHandlers(expr, valueType)
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryHandledResult, SuccessType: valueType.TypeArgs[0], ErrorType: valueType.TypeArgs[1],
		}
		if valid {
			a.resolvedTryPlans[expr] = plan
		}
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
	a.resolvedTries[expr] = ResolvedTry{
		Kind: ResolvedTryResultPropagation, SuccessType: valueType.TypeArgs[0], ErrorType: valueErrorType,
		EnclosingResultType: a.currentFunctionReturn,
	}

	return valueType.TypeArgs[0], expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferArithmeticTryExpression(expr *ast.TryExpression, operator ResolvedOperator) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	arithmeticError := a.types["ArithmeticError"]
	if len(expr.Handlers) != 0 {
		resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{operator.ResultType, arithmeticError}}
		plan, valid := a.analyzeTryHandlers(expr, resultType)
		a.resolvedTries[expr] = ResolvedTry{
			Kind: ResolvedTryHandledArithmetic, SuccessType: operator.ResultType, ErrorType: arithmeticError,
		}
		if valid {
			a.resolvedTryPlans[expr] = plan
		}
		return operator.ResultType, result
	}
	if a.inDeferBlock {
		a.addErrorAtToken(expr.Token, "try cannot propagate from inside defer")
		return operator.ResultType, result
	}
	if !a.inFunctionBody {
		a.addErrorAtToken(expr.Token, "cannot use arithmetic try outside function")
		return operator.ResultType, result
	}
	if a.currentFunctionReturn.Kind != ResultType || len(a.currentFunctionReturn.TypeArgs) != 2 {
		a.addErrorAtToken(expr.Token, "arithmetic try requires enclosing return type Result[U, ArithmeticError], got %s", typeDisplayName(a.currentFunctionReturn))
		return operator.ResultType, result
	}
	functionError := a.currentFunctionReturn.TypeArgs[1]
	if !sameConcreteType(functionError, arithmeticError) {
		a.addErrorAtToken(expr.Token, "arithmetic try cannot propagate ArithmeticError from function returning %s; map the error explicitly", typeDisplayName(a.currentFunctionReturn))
		return operator.ResultType, result
	}
	a.resolvedTries[expr] = ResolvedTry{
		Kind: ResolvedTryArithmeticPropagation, SuccessType: operator.ResultType,
		ErrorType: arithmeticError, EnclosingResultType: a.currentFunctionReturn,
	}
	return operator.ResultType, result
}

func (a *Analyzer) analyzeTryHandlers(expr *ast.TryExpression, resultType Type) (ResolvedTryPlan, bool) {
	successType := resultType.TypeArgs[0]
	errorType := resultType.TypeArgs[1]
	plan := ResolvedTryPlan{SuccessType: successType, ErrorType: errorType}
	errorsBefore := len(a.errors)
	errorCatchAllSeen := false
	okSeen := false
	matchedVariants := map[string]lexer.Token{}

	for sourceIndex, handler := range expr.Handlers {
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
			plan.HasExplicitOk = true
			patternKind := TryHandlerOkBinding
			if bindingName == "" {
				patternKind = TryHandlerOkDiscard
			}
			flow := a.analyzeTryHandlerBody(handler, successType, bindingType, bindingName)
			plan.Handlers = append(plan.Handlers, ResolvedTryHandler{PatternKind: patternKind, BindingName: bindingName, BindingType: bindingType, Flow: flow, ResultType: successType, SourceIndex: sourceIndex})
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

		patternKind := TryHandlerErrVariant
		if bindingName != "" {
			patternKind = TryHandlerErrCatchAll
		}
		flow := a.analyzeTryHandlerBody(handler, successType, bindingType, bindingName)
		plan.Handlers = append(plan.Handlers, ResolvedTryHandler{PatternKind: patternKind, Variant: variantName, BindingName: bindingName, BindingType: bindingType, Flow: flow, ResultType: successType, SourceIndex: sourceIndex})
	}

	if errorCatchAllSeen {
		plan.Exhaustive = true
		return plan, len(a.errors) == errorsBefore
	}

	if errorType.Kind == EnumType {
		if len(matchedVariants) < len(errorType.EnumValues) {
			a.addErrorAtToken(expr.Token, "non-exhaustive try handlers for %s", typeDisplayName(errorType))
		} else {
			plan.Exhaustive = true
		}
		return plan, len(a.errors) == errorsBefore
	}

	a.addErrorAtToken(expr.Token, "non-exhaustive try handlers for %s", typeDisplayName(errorType))
	return plan, false
}

func (a *Analyzer) analyzeTryHandlerPattern(handler *ast.TryHandler, successType Type, errorType Type) (kind string, bindingName string, variantName string, bindingType Type, ok bool) {
	switch pattern := handler.Pattern.(type) {
	case *ast.InvalidPattern:
		return "", "", "", Type{}, false
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

func (a *Analyzer) analyzeTryHandlerBody(handler *ast.TryHandler, successType Type, errorType Type, bindingName string) ResolvedTryHandlerFlow {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	if bindingName != "" {
		bindingIdentifier := tryHandlerBindingIdentifier(handler)
		bindingToken := handler.Token
		if bindingIdentifier != nil {
			bindingToken = bindingIdentifier.Token
			a.recordDefinition(bindingToken)
			a.recordBinding(bindingToken, BindingLocal, bindingName, errorType, false)
		}
		a.symbols[bindingName] = Symbol{Name: bindingName, Type: errorType, Mutable: false, Token: bindingToken, Storage: StorageOriginInline}
		delete(a.constInts, bindingName)
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
	}()

	if handler.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, handler.ReturnBody)
		return TryHandlerReturns
	}

	if handler.BlockBody != nil {
		a.analyzeBlockStatements(handler.BlockBody)
		if successType.Kind == VoidType {
			if blockCanFallThrough(handler.BlockBody) {
				return TryHandlerProducesValue
			}
			if blockDefinitelyReturns(handler.BlockBody) {
				return TryHandlerReturns
			}
			return TryHandlerTerminates
		}
		if blockCanFallThrough(handler.BlockBody) {
			a.addErrorAtToken(handler.Token, "try handler must return, propagate, terminate or produce %s", typeDisplayName(successType))
			return TryHandlerInvalidFlow
		}
		if blockDefinitelyReturns(handler.BlockBody) {
			return TryHandlerReturns
		}
		return TryHandlerTerminates
	}

	if handler.Body == nil {
		a.addErrorAtToken(handler.Token, "try handler must return, propagate, terminate or produce %s", typeDisplayName(successType))
		return TryHandlerInvalidFlow
	}

	bodyType, _ := a.inferExpression(handler.Body)
	if bodyType.Kind == InvalidType {
		return TryHandlerInvalidFlow
	}
	if !canInitialize(successType, bodyType, handler.Body) {
		a.addErrorAtToken(expressionToken(handler.Body), "try handler must produce %s, got %s", typeDisplayName(successType), typeDisplayName(bodyType))
	}
	return TryHandlerProducesValue
}

func tryHandlerBindingIdentifier(handler *ast.TryHandler) *ast.Identifier {
	if handler == nil {
		return nil
	}
	switch pattern := handler.Pattern.(type) {
	case *ast.OkExpression:
		identifier, _ := pattern.Value.(*ast.Identifier)
		return identifier
	case *ast.ErrExpression:
		identifier, _ := pattern.Value.(*ast.Identifier)
		return identifier
	default:
		return nil
	}
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
	BindingName          string
	BindingType          Type
	Kind                 string
	Variant              string
	PayloadVariant       string
	PayloadPlace         Place
	PayloadMoves         bool
	PayloadBorrow        bool
	PayloadBorrowMutable bool
	PayloadToken         lexer.Token
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
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)
	branches := []branchAnalysis{}
	patternError := false

	for _, arm := range expr.Arms {
		info, ok := a.analyzeMatchPattern(arm.Pattern, subjectType)
		if !ok {
			patternError = true
			continue
		}
		if info.BindingName != "" && info.PayloadVariant != "" {
			if subjectPlace, placeOK := a.resolvePlace(expr.Subject); placeOK {
				payloadType := info.BindingType
				if info.PayloadBorrow && payloadType.Element != nil {
					payloadType = *payloadType.Element
				}
				info.PayloadPlace = unionPayloadPlace(subjectPlace, info.PayloadVariant, payloadType, info.PayloadToken)
				if info.PayloadBorrow {
					info.BindingType = a.referenceTypeWithOriginFromExpression(info.BindingType, expr.Subject)
					info.BindingType.ReferenceOriginMatchScoped = true
					info.BindingType.ReferenceOriginToken = info.PayloadToken
				}
			} else if info.PayloadBorrow {
				a.addErrorAtToken(info.PayloadToken, "cannot borrow union payload from temporary match subject")
				info.BindingType = Type{Kind: InvalidType}
			}
			if subjectType.Kind == UnionType && !info.PayloadBorrow {
				info.PayloadMoves = requiresOwnershipTransfer(info.BindingType)
			}
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
	if !exhaustive {
		branches = append(branches, branchAnalysis{assigned: beforeAssigned, moved: beforeMoved, moveReasons: beforeMoveReasons, closedResources: beforeClosedResources, borrows: beforeBorrows, localRefContainers: beforeLocalRefContainers, arenaGenerations: beforeArenaGenerations, continues: true})
	}
	a.assigned = mergeContinuingAssigned(beforeAssigned, branches...)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, branches...)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, branches...)
	a.borrows = mergeContinuingBorrows(beforeBorrows, branches...)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, branches...)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, branches...)

	if valueContext {
		if !hasResultType {
			a.addErrorAtToken(expr.Token, "match expression must produce a value")
			return Type{Kind: InvalidType}
		}
		if typeCarriesReferenceOrigin(resultType) && resultType.ReferenceOriginMatchScoped {
			a.addErrorAtTokenWithPrevious(expr.Token, resultType.ReferenceOriginToken, "match expression cannot produce a branch-scoped union payload reference")
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
	case *ast.InvalidPattern:
		return matchPatternInfo{}, false
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
		return a.analyzeResultPayloadPattern("Ok", pattern.Value, pattern.Token, subjectType.TypeArgs[0], false)
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
		return a.analyzeResultPayloadPattern("Err", pattern.Value, pattern.Token, subjectType.TypeArgs[1], true)
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
	bindingExpression := arguments[0]
	binding, borrowed, mutable, ok := unionPayloadPatternBinding(bindingExpression)
	if !ok {
		a.addErrorAtToken(expressionToken(bindingExpression), "union payload pattern must bind an identifier, ref identifier, or ref mut identifier")
		return matchPatternInfo{}, false
	}

	info := matchPatternInfo{Kind: "variant", Variant: variant.Name, PayloadVariant: variant.Name}
	if binding.Value == "_" {
		return info, true
	}
	info.BindingName = binding.Value
	info.PayloadToken = binding.Token
	info.PayloadBorrow = borrowed
	info.PayloadBorrowMutable = mutable
	var payloadType Type
	if variant.Payload != nil {
		payloadType = *variant.Payload
	} else {
		payloadType = Type{
			Name:   typeDisplayName(subjectType) + "." + variant.Name,
			Kind:   StructType,
			Fields: variant.PayloadFields,
		}
	}
	info.BindingType = payloadType
	if borrowed {
		info.BindingType = Type{
			Name:             referenceTypeName(payloadType, mutable),
			Kind:             ReferenceType,
			Element:          &payloadType,
			ReferenceMutable: mutable,
		}
	}
	return info, true
}

func (a *Analyzer) analyzeResultPayloadPattern(kind string, expr ast.Expression, token lexer.Token, payloadType Type, rejectDiscard bool) (matchPatternInfo, bool) {
	info := matchPatternInfo{Kind: kind, PayloadVariant: kind}
	if expr == nil {
		return info, true
	}
	binding, borrowed, mutable, ok := unionPayloadPatternBinding(expr)
	if !ok {
		a.addErrorAtToken(expressionToken(expr), "%s payload pattern must bind an identifier, ref identifier, or ref mut identifier", kind)
		return matchPatternInfo{}, false
	}
	if binding.Value == "_" {
		if rejectDiscard {
			a.addErrorAtToken(binding.Token, "Err payload must be named; use discard name inside the handler")
			return matchPatternInfo{}, false
		}
		return info, true
	}
	info.BindingName = binding.Value
	info.BindingType = payloadType
	info.PayloadToken = binding.Token
	info.PayloadBorrow = borrowed
	info.PayloadBorrowMutable = mutable
	if borrowed {
		info.BindingType = Type{
			Name: referenceTypeName(payloadType, mutable), Kind: ReferenceType,
			Element: &payloadType, ReferenceMutable: mutable,
		}
	}
	return info, true
}

func unionPayloadPatternBinding(expr ast.Expression) (binding *ast.Identifier, borrowed bool, mutable bool, ok bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr, false, false, true
	case *ast.RefExpression:
		binding, ok := expr.Value.(*ast.Identifier)
		return binding, true, expr.Mutable, ok
	default:
		return nil, false, false, false
	}
}

func (a *Analyzer) analyzeMatchArmBody(arm *ast.MatchArm, info matchPatternInfo) (Type, branchAnalysis) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	if info.PayloadMoves && info.PayloadPlace.Root != "" {
		if !info.PayloadPlace.PartialMoveSafe {
			a.addErrorAtToken(info.PayloadToken, "union payload move requires independently tracked local union storage")
		} else if _, _, _, unavailable := a.unavailablePlace(info.PayloadPlace); !unavailable && !a.checkBorrowedMovePlace(info.PayloadPlace, info.PayloadToken) {
			a.markPlaceUnavailable(info.PayloadPlace, info.PayloadToken, "moved")
		}
	}
	if info.BindingName != "" {
		if a.defineSymbol(info.BindingName, info.BindingType, false, arm.Token) {
			a.assigned[info.BindingName] = true
			a.clearRootPlaceState(info.BindingName)
			delete(a.constInts, info.BindingName)
			if info.PayloadBorrow && info.PayloadPlace.Root != "" && info.BindingType.Kind != InvalidType {
				if !a.checkBorrowCreationPlace(info.PayloadPlace, info.PayloadBorrowMutable, info.PayloadToken) {
					kind := sharedBorrow
					if info.PayloadBorrowMutable {
						kind = mutableBorrow
					}
					a.borrows[info.PayloadPlace.Root] = append(a.borrows[info.PayloadPlace.Root], borrowRecord{
						Root: info.PayloadPlace.Root, Place: info.PayloadPlace, Holder: info.BindingName,
						Kind: kind, Token: info.PayloadToken,
					})
					a.localRefContainers[info.BindingName] = localReferenceOrigin{
						Name: info.BindingType.ReferenceOriginName, Token: info.BindingType.ReferenceOriginToken,
						Local: info.BindingType.ReferenceOriginLocal, MatchScoped: true,
						Place: clonePlace(info.PayloadPlace), HasPlace: true,
					}
				}
			}
		}
	}
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
	}()

	if arm.Guard != nil {
		guardType, _ := a.inferExpression(arm.Guard)
		if guardType.Kind != InvalidType && guardType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(arm.Guard), "match guard must be bool, got %s", typeDisplayName(guardType))
		}
	}

	if arm.ReturnBody != nil {
		a.analyzeReturnStatement(a.currentFunctionName, a.currentFunctionReturn, arm.ReturnBody)
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: false})
	}
	if arm.BlockBody != nil {
		a.analyzeBlockStatements(arm.BlockBody)
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{
			assigned:           copyAssigned(a.assigned),
			moved:              copyMoved(a.moved),
			moveReasons:        copyMoveReasons(a.moveReasons),
			closedResources:    copyMoved(a.closedResources),
			borrows:            copyBorrows(a.borrows),
			localRefContainers: copyLocalRefContainers(a.localRefContainers),
			arenaGenerations:   copyArenaGenerations(a.arenaGenerations),
			continues:          blockCanFallThrough(arm.BlockBody),
		})
	}
	if arm.Body == nil {
		return Type{Kind: InvalidType}, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: true})
	}
	bodyType, _ := a.inferExpression(arm.Body)
	a.markMoveSource(arm.Body)
	return bodyType, a.finalizeMatchBranch(info, branchAnalysis{assigned: copyAssigned(a.assigned), moved: copyMoved(a.moved), moveReasons: copyMoveReasons(a.moveReasons), closedResources: copyMoved(a.closedResources), borrows: copyBorrows(a.borrows), localRefContainers: copyLocalRefContainers(a.localRefContainers), arenaGenerations: copyArenaGenerations(a.arenaGenerations), continues: true})
}

func (a *Analyzer) finalizeMatchBranch(info matchPatternInfo, branch branchAnalysis) branchAnalysis {
	if info.BindingName == "" {
		return branch
	}
	delete(branch.assigned, info.BindingName)
	delete(branch.closedResources, info.BindingName)
	for place := range branch.moved {
		if place == info.BindingName || strings.HasPrefix(place, info.BindingName+".") || strings.HasPrefix(place, info.BindingName+"[") {
			delete(branch.moved, place)
			delete(branch.moveReasons, place)
		}
	}
	removeBorrowHolder(branch.borrows, info.BindingName)
	delete(branch.localRefContainers, info.BindingName)
	return branch
}

func removeBorrowHolder(borrows map[string][]borrowRecord, holder string) {
	for root, records := range borrows {
		kept := records[:0]
		for _, record := range records {
			if record.Holder != holder {
				kept = append(kept, record)
			}
		}
		if len(kept) == 0 {
			delete(borrows, root)
		} else {
			borrows[root] = kept
		}
	}
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
	if expr.Operator == "x" {
		return a.inferMatrixMultiplyExpression(expr, leftType, rightType)
	}

	if isComparisonOperator(expr.Operator) {
		if _, ok := expr.Right.(*ast.CharLiteral); ok && leftType.Kind == RuneType {
			rightType = Type{Name: "rune", Kind: RuneType}
			a.expressionTypes[expr.Right] = rightType
		}
		if _, ok := expr.Left.(*ast.CharLiteral); ok && rightType.Kind == RuneType {
			leftType = Type{Name: "rune", Kind: RuneType}
			a.expressionTypes[expr.Left] = leftType
		}
		if isUntypedNumericExpression(expr.Right) && isNumericType(leftType) && canInitialize(leftType, rightType, expr.Right) {
			rightType = leftType
			a.expressionTypes[expr.Right] = rightType
		}
		if isUntypedNumericExpression(expr.Left) && isNumericType(rightType) && canInitialize(rightType, leftType, expr.Left) {
			leftType = rightType
			a.expressionTypes[expr.Left] = leftType
		}
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
			a.addErrorAtTokenWithMetadata(
				expr.Token,
				diagnostics.OperatorNonComparable,
				"Compare values of the same equality-comparable type, or use an explicit content or identity operation for views and resources.",
				"cannot compare %s and %s",
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isOrderedComparisonOperator(expr.Operator) {
		if !canCompareOrdered(leftType, rightType) {
			a.addErrorAtTokenWithMetadata(
				expr.Token,
				diagnostics.OperatorNonOrderable,
				"Use compatible numeric operands, or compare two char, rune, or string values.",
				"operator %s cannot order %s and %s",
				expr.Operator,
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	if isBitwiseOperator(expr.Operator) {
		if !isIntegerType(leftType) || !isIntegerType(rightType) {
			a.addErrorAtToken(expr.Token, "operator %s requires integer operands", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if expr.Operator == "<<" || expr.Operator == ">>" {
			if !a.validateShiftExpression(expr, leftType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return leftType, expressionValue{Display: expr.String()}
		}
		if !sameConcreteType(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot apply operator %s to %s and %s", expr.Operator, typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if expr.Operator == "+" && (isTextConcatKind(leftType) || isTextConcatKind(rightType)) {
		return a.inferPlainArithmeticExpression(expr, leftType, rightType)
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		return a.inferDecimalInfixExpression(expr, leftType, rightType)
	}

	if isNumericType(leftType) && isNumericType(rightType) && (!leftType.Dimension.IsZero() || !rightType.Dimension.IsZero()) {
		return a.inferNumericUnitInfixExpression(expr, leftType, rightType)
	}

	if isArithmeticOperator(expr.Operator) {
		return a.inferPlainArithmeticExpression(expr, leftType, rightType)
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferPlainArithmeticExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if expr.Operator == "+" && (isTextConcatKind(leftType) || isTextConcatKind(rightType)) {
		if !isDirectTextConcatOperand(leftType) || !isDirectTextConcatOperand(rightType) {
			a.addInvalidConcatOperandError(expr.Token, leftType, rightType)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	}

	if !isNumericType(leftType) || !isNumericType(rightType) {
		a.addErrorAtToken(expr.Token, "operator %s requires numeric operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if sameConcreteType(leftType, rightType) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if compatiblePlainNumericAlias(leftType, rightType) {
		if leftType.Named {
			if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return rightType, expressionValue{Display: expr.String()}
		}
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Right) && canInitialize(leftType, rightType, expr.Right) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Left) && canInitialize(rightType, leftType, expr.Left) {
		if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, expressionValue{Display: expr.String()}
	}

	a.addErrorAtToken(expr.Token, "cannot apply operator %s to %s and %s", expr.Operator, typeDisplayName(leftType), typeDisplayName(rightType))
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) validateCompileTimeIntegerArithmetic(expr *ast.InfixExpression, resultType Type) bool {
	if expr == nil || !isBuiltinIntegerOperatorType(resultType) {
		return true
	}
	left, leftKnown := a.integerConstantValue(expr.Left)
	right, rightKnown := a.integerConstantValue(expr.Right)
	if !leftKnown || !rightKnown {
		return true
	}
	representation, _, ok := a.integerRepresentation(resultType)
	if !ok || representation.MinInteger == nil || representation.MaxInteger == nil {
		return true
	}
	if (expr.Operator == "/" || expr.Operator == "%") && right.Sign() == 0 {
		id := diagnostics.OperatorDivisionByZero
		operation := "division"
		help := "Use a non-zero divisor or guard the operation before evaluating it."
		if expr.Operator == "%" {
			id = diagnostics.OperatorRemainderByZero
			operation = "remainder"
			help = "Use a non-zero remainder divisor or guard the operation before evaluating it."
		}
		a.addErrorAtTokenWithMetadata(expr.Token, id, help, "constant integer %s by zero", operation)
		return false
	}
	value := new(big.Int)
	switch expr.Operator {
	case "+":
		value.Add(left, right)
	case "-":
		value.Sub(left, right)
	case "*":
		value.Mul(left, right)
	case "/":
		value.Quo(left, right)
	case "%":
		value.Rem(left, right)
	default:
		return true
	}
	divisionOverflow := (expr.Operator == "/" || expr.Operator == "%") &&
		representation.Kind == IntType && left.Cmp(representation.MinInteger) == 0 && right.Cmp(big.NewInt(-1)) == 0
	if !divisionOverflow && value.Cmp(representation.MinInteger) >= 0 && value.Cmp(representation.MaxInteger) <= 0 {
		return true
	}
	a.addErrorAtTokenWithMetadata(
		expr.Token,
		diagnostics.OperatorIntegerOverflow,
		fmt.Sprintf("Use a wider integer type or restructure the constant expression to remain within %s..%s.", representation.MinInteger, representation.MaxInteger),
		"constant integer operation %s overflows %s",
		expr.String(),
		typeDisplayName(resultType),
	)
	return false
}

func isTextConcatKind(typ Type) bool {
	switch typ.Kind {
	case StringType, CharType, RuneType:
		return true
	default:
		return false
	}
}

func isDirectTextConcatOperand(typ Type) bool {
	return !typ.Named && isTextConcatKind(typ)
}

func (a *Analyzer) addInvalidConcatOperandError(token lexer.Token, left Type, right Type) {
	a.addErrorAtTokenWithMetadata(
		token,
		diagnostics.OperatorInvalidConcatOperand,
		"Convert the non-text operand explicitly with .ToString(), or use interpolation when it has a formatting contract.",
		"cannot concatenate %s and %s directly; string concatenation accepts string, char, and rune",
		typeDisplayName(left),
		typeDisplayName(right),
	)
}

func (a *Analyzer) inferCompoundAssignmentType(operator string, target Type, value Type, expr ast.Expression) (Type, bool) {
	if operator == "+=" && (isTextConcatKind(target) || isTextConcatKind(value)) {
		if target.Kind == StringType && !target.Named && isDirectTextConcatOperand(value) {
			return Type{Name: "string", Kind: StringType}, true
		}
		a.addInvalidConcatOperandError(expressionToken(expr), target, value)
		return Type{Kind: InvalidType}, false
	}

	if !canInitialize(target, value, expr) {
		a.addErrorAtToken(
			expressionToken(expr),
			"cannot %s %s to %s",
			assignmentVerb(operator),
			typeDisplayName(value),
			typeDisplayName(target),
		)
		return Type{Kind: InvalidType}, false
	}
	return target, true
}

func (a *Analyzer) inferMatrixMultiplyExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	invalid := func(format string, args ...any) (Type, expressionValue) {
		a.addErrorAtToken(expr.Token, format, args...)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if leftType.Name != "matrix" || len(leftType.TypeArgs) != 1 || len(leftType.ConstArgs) != 2 {
		return invalid("left operand of x must be matrix, got %s", typeDisplayName(leftType))
	}
	if (rightType.Name != "matrix" && rightType.Name != "vector") || len(rightType.TypeArgs) != 1 {
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
	if !sameConcreteType(leftType.TypeArgs[0], rightType.TypeArgs[0]) {
		return invalid("matrix multiplication element types differ: %s and %s", typeDisplayName(leftType.TypeArgs[0]), typeDisplayName(rightType.TypeArgs[0]))
	}

	inner := leftType.ConstArgs[1]
	switch rightType.Name {
	case "matrix":
		if len(rightType.ConstArgs) != 2 {
			return invalid("right operand of x must have two matrix dimensions, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := leftType
		result.ConstArgs = []int64{leftType.ConstArgs[0], rightType.ConstArgs[1]}
		return result, expressionValue{Display: expr.String()}
	case "vector":
		if len(rightType.ConstArgs) != 1 {
			return invalid("right operand of x must have one vector dimension, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix-vector multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := rightType
		result.ConstArgs = []int64{leftType.ConstArgs[0]}
		return result, expressionValue{Display: expr.String()}
	default:
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
}

func compatiblePlainNumericAlias(left Type, right Type) bool {
	if !isNumericType(left) || !isNumericType(right) || left.Kind != right.Kind {
		return false
	}
	if isNominal(left) || isNominal(right) {
		return false
	}
	if !left.Named && !right.Named {
		return false
	}
	leftBase := left.Underlying
	if leftBase == "" {
		leftBase = left.Name
	}
	rightBase := right.Underlying
	if rightBase == "" {
		rightBase = right.Name
	}
	return leftBase == rightBase
}

func (a *Analyzer) inferNumericUnitInfixExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if isComparisonOperator(expr.Operator) {
		if isEqualityOperator(expr.Operator) && !canCompareEquality(leftType, rightType) {
			a.addErrorAtToken(expr.Token, "cannot compare %s and %s", typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "+", "-":
		if sameConcreteType(leftType, rightType) {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) {
			if isNominal(leftType) || isNominal(rightType) {
				a.addErrorAtToken(expr.Token, "cannot %s %s to %s", infixVerb(expr.Operator), typeDisplayName(rightType), typeDisplayName(leftType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return a.typeForDimension(DecimalType, leftType.Dimension), expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(expr.Token, "cannot %s %s to %s", infixVerb(expr.Operator), typeDisplayName(rightType), typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case "*":
		if leftType.Dimension.IsZero() {
			return rightType, expressionValue{Display: expr.String()}
		}
		if rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) && leftType.Dimension.HasCurrencyBase() {
			a.addErrorAtToken(expr.Token, "cannot multiply %s by %s", typeDisplayName(leftType), typeDisplayName(rightType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
	case "/":
		if rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferMembershipExpression(expr *ast.InfixExpression, leftType Type) (Type, expressionValue) {
	rangeExpr, ok := expr.Right.(*ast.RangeExpression)
	if ok {
		return a.inferRangeMembershipExpression(expr, rangeExpr, leftType)
	}

	rightType, _ := a.inferExpression(expr.Right)
	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	elementType, ok := membershipElementType(rightType)
	if !ok {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			"Use a contextual range, a fixed array, or a slice. For other containers, call an explicit membership API such as Contains.",
			"operator in supports ranges, fixed arrays, and slices in Sec 0.1; got %s",
			typeDisplayName(rightType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !EqualityComparable(elementType) {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			"Use a collection whose element type supports equality, or perform an explicit content or identity search.",
			"cannot test membership in %s because element type %s is not equality-comparable",
			typeDisplayName(rightType),
			typeDisplayName(elementType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	leftType = a.contextualMembershipValueType(expr.Left, leftType, elementType)
	if !canCompareEquality(leftType, elementType) {
		a.addErrorAtTokenWithMetadata(
			expr.Token,
			diagnostics.OperatorInvalidMembership,
			fmt.Sprintf("Search with a value compatible with the collection element type %s.", typeDisplayName(elementType)),
			"cannot test %s for membership in %s with element type %s",
			typeDisplayName(leftType),
			typeDisplayName(rightType),
			typeDisplayName(elementType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferRangeMembershipExpression(expr *ast.InfixExpression, rangeExpr *ast.RangeExpression, leftType Type) (Type, expressionValue) {
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

func membershipElementType(collection Type) (Type, bool) {
	collection = dereferenceType(collection)
	if collection.Element == nil {
		return Type{}, false
	}

	switch collection.Kind {
	case ArrayType:
		if collection.ArrayLength == dynamicArrayLength {
			return Type{}, false
		}
		return *collection.Element, true
	case SliceType:
		return *collection.Element, true
	default:
		return Type{}, false
	}
}

func (a *Analyzer) contextualMembershipValueType(expr ast.Expression, actual Type, element Type) Type {
	if _, ok := expr.(*ast.CharLiteral); ok && element.Kind == RuneType {
		actual = Type{Name: "rune", Kind: RuneType}
		a.expressionTypes[expr] = actual
		return actual
	}
	if isUntypedNumericExpression(expr) && isNumericType(element) && canInitialize(element, actual, expr) {
		a.expressionTypes[expr] = element
		return element
	}
	return actual
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
	if !EqualityComparable(left) || !EqualityComparable(right) {
		return false
	}
	if isNumericType(left) && isNumericType(right) {
		if left.Named || right.Named || !left.Dimension.IsZero() || !right.Dimension.IsZero() {
			return sameConcreteType(left, right)
		}
		return true
	}
	if left.Kind == ArrayType || right.Kind == ArrayType {
		return sameComparableArrayType(left, right)
	}
	if left.Kind != right.Kind {
		return false
	}
	return sameConcreteType(left, right)
}

func sameComparableArrayType(left Type, right Type) bool {
	if left.Kind != ArrayType || right.Kind != ArrayType || left.ArrayLength != right.ArrayLength || left.Element == nil || right.Element == nil {
		return false
	}
	if left.Element.Kind == ArrayType || right.Element.Kind == ArrayType {
		return sameComparableArrayType(*left.Element, *right.Element)
	}
	return sameConcreteType(*left.Element, *right.Element)
}

func canCompareOrdered(left Type, right Type) bool {
	if left.Kind == InvalidType || right.Kind == InvalidType {
		return true
	}

	switch left.Kind {
	case CharType, RuneType, StringType:
		return left.Kind == right.Kind && sameConcreteType(left, right)
	}

	if !isNumericType(left) || !isNumericType(right) {
		return false
	}
	if isNominal(left) || isNominal(right) {
		return sameConcreteType(left, right)
	}
	if !left.Dimension.IsZero() || !right.Dimension.IsZero() {
		return left.Dimension.Equal(right.Dimension)
	}
	return true
}

func (a *Analyzer) validateShiftExpression(expr *ast.InfixExpression, leftType Type) bool {
	count, known := a.integerConstantValue(expr.Right)
	if !known {
		return true
	}

	representation, width, ok := a.integerRepresentation(leftType)
	if !ok {
		return true
	}
	if count.Sign() < 0 || !count.IsUint64() || count.Uint64() >= uint64(width) {
		a.addErrorAtTokenWithMetadata(
			expressionToken(expr.Right),
			diagnostics.OperatorInvalidShiftCount,
			fmt.Sprintf("Use a shift count from 0 through %d for %s.", width-1, typeDisplayName(leftType)),
			"shift count %s is outside the valid range 0..%d for %s",
			count.String(),
			width-1,
			typeDisplayName(leftType),
		)
		return false
	}

	if expr.Operator != "<<" || representation.Kind != IntType {
		return true
	}
	left, known := a.integerConstantValue(expr.Left)
	if !known || representation.MinInteger == nil || representation.MaxInteger == nil {
		return true
	}
	shifted := new(big.Int).Lsh(new(big.Int).Set(left), uint(count.Uint64()))
	if shifted.Cmp(representation.MinInteger) >= 0 && shifted.Cmp(representation.MaxInteger) <= 0 {
		return true
	}

	a.addErrorAtTokenWithMetadata(
		expr.Token,
		diagnostics.OperatorShiftOverflow,
		"Use a wider signed type, reduce the shift count, or use an unsigned type when fixed-width bit truncation is intended.",
		"signed left shift %s produces %s, which is outside %s range %s..%s",
		expr.String(),
		shifted.String(),
		typeDisplayName(leftType),
		representation.MinInteger.String(),
		representation.MaxInteger.String(),
	)
	return false
}

func (a *Analyzer) integerRepresentation(typ Type) (Type, int64, bool) {
	if !isIntegerType(typ) {
		return Type{}, 0, false
	}

	current := typ
	seen := map[string]bool{}
	for current.Underlying != "" && !seen[current.Underlying] {
		seen[current.Underlying] = true
		underlying, ok := a.types[current.Underlying]
		if !ok || !isIntegerType(underlying) {
			break
		}
		current = underlying
	}

	width := numericTypeSizeBytes(current) * 8
	if width <= 0 {
		return Type{}, 0, false
	}
	return current, width, true
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
			if !rightType.Dimension.IsZero() {
				if expr.Operator == "*" {
					return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
				}
				return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			return leftType, expressionValue{Display: expr.String()}
		}
	}

	if (leftType.Kind == IntType || leftType.Kind == UintType) && rightType.Kind == DecimalType {
		switch expr.Operator {
		case "*":
			if !leftType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			return rightType, expressionValue{Display: expr.String()}
		case "/":
			if !leftType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
			if !rightType.Dimension.IsZero() {
				return a.typeForDimension(DecimalType, Dimension{}.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
			}
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
			if isNominal(leftType) || isNominal(rightType) {
				a.addErrorAtToken(
					expr.Token,
					"cannot %s %s to %s",
					infixVerb(expr.Operator),
					typeDisplayName(rightType),
					typeDisplayName(leftType),
				)
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
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
		if leftType.Dimension.IsZero() && rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
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
	case "+":
		if isNumericType(rightType) {
			return rightType, expressionValue{Display: rightValue.Display}
		}
	case "-":
		if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
			if isBuiltinIntegerOperatorType(rightType) {
				if value, known := a.integerConstantValue(expr.Right); known {
					representation, _, ok := a.integerRepresentation(rightType)
					if ok && representation.MinInteger != nil && value.Cmp(representation.MinInteger) == 0 {
						a.addErrorAtTokenWithMetadata(
							expr.Token,
							diagnostics.OperatorIntegerOverflow,
							"Use a wider signed integer type; the minimum value has no positive counterpart in the same type.",
							"constant negation %s overflows %s",
							expr.String(),
							typeDisplayName(rightType),
						)
						return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
					}
				}
			}
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
	case "~":
		if !isIntegerType(rightType) {
			a.addErrorAtToken(expr.Token, "operator ~ requires integer operand")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) addError(format string, args ...any) {
	err := Error{Severity: diagnostics.SeverityError, Message: fmt.Sprintf(format, args...)}
	a.appendError(err)
}

func (a *Analyzer) addErrorAtToken(token lexer.Token, format string, args ...any) {
	err := Error{
		Severity: diagnostics.SeverityError,
		Message:  fmt.Sprintf(format, args...),
		File:     token.File,
		Line:     token.Line,
		Column:   token.Column,
	}
	a.appendError(err)
}

func (a *Analyzer) addErrorAtTokenWithID(token lexer.Token, id string, format string, args ...any) {
	err := Error{
		ID:       id,
		Severity: diagnostics.SeverityError,
		Message:  fmt.Sprintf(format, args...),
		File:     token.File,
		Line:     token.Line,
		Column:   token.Column,
	}
	a.appendError(err)
}

func (a *Analyzer) addErrorAtTokenWithMetadata(token lexer.Token, id string, help string, format string, args ...any) {
	err := Error{
		ID:       id,
		Severity: diagnostics.SeverityError,
		Help:     help,
		Message:  fmt.Sprintf(format, args...),
		File:     token.File,
		Line:     token.Line,
		Column:   token.Column,
	}
	a.appendError(err)
}

func (a *Analyzer) addErrorAtTokenWithPrevious(token lexer.Token, previous lexer.Token, format string, args ...any) {
	err := Error{
		Severity:       diagnostics.SeverityError,
		Message:        fmt.Sprintf(format, args...),
		File:           token.File,
		Line:           token.Line,
		Column:         token.Column,
		PreviousFile:   previous.File,
		PreviousLine:   previous.Line,
		PreviousColumn: previous.Column,
	}
	a.appendError(err)
}

func (a *Analyzer) addErrorAtTokenWithPreviousID(token lexer.Token, previous lexer.Token, id string, format string, args ...any) {
	err := Error{
		ID:             id,
		Severity:       diagnostics.SeverityError,
		Message:        fmt.Sprintf(format, args...),
		File:           token.File,
		Line:           token.Line,
		Column:         token.Column,
		PreviousFile:   previous.File,
		PreviousLine:   previous.Line,
		PreviousColumn: previous.Column,
	}
	a.appendError(err)
}

func (a *Analyzer) appendError(err Error) {
	if a.summaryPass {
		return
	}
	if err.Severity == "" {
		err.Severity = diagnostics.SeverityError
	}
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s\x00%d\x00%d", err.Message, err.File, err.Line, err.Column, err.PreviousFile, err.PreviousLine, err.PreviousColumn)
	if a.errorKeys == nil {
		a.errorKeys = map[string]bool{}
	}
	if a.errorKeys[key] {
		return
	}
	a.errorKeys[key] = true
	a.errors = append(a.errors, err)
}

func (a *Analyzer) addWarningAtToken(token lexer.Token, format string, args ...any) {
	a.appendWarning(Error{
		Severity: diagnostics.SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		File:     token.File,
		Line:     token.Line,
		Column:   token.Column,
	})
}

func (a *Analyzer) addWarningAtTokenWithMetadata(token lexer.Token, id string, help string, format string, args ...any) {
	severity := diagnostics.DefaultSeverity(id)
	if severity == "" {
		severity = diagnostics.SeverityWarning
	}
	a.appendWarning(Error{
		ID:       id,
		Severity: severity,
		Help:     help,
		Message:  fmt.Sprintf(format, args...),
		File:     token.File,
		Line:     token.Line,
		Column:   token.Column,
	})
}

func (a *Analyzer) appendWarning(warning Error) {
	if a.summaryPass {
		return
	}
	if warning.Severity == "" {
		warning.Severity = diagnostics.SeverityWarning
	}
	a.warnings = append(a.warnings, warning)
}

func (a *Analyzer) warnUnitStatus(token lexer.Token, unitName string) {
	unit, ok := a.units[unitName]
	if !ok {
		return
	}
	switch unit.Status {
	case StatusDeprecated:
		a.addWarningAtToken(token, "unit %s is deprecated", unitName)
	case StatusObsolete:
		a.addWarningAtToken(token, "unit %s is obsolete", unitName)
	}
}

func (a *Analyzer) defineSymbol(name string, typ Type, mutable bool, token lexer.Token) bool {
	if previous, exists := a.symbols[name]; exists {
		if !previous.ImplicitMember {
			a.appendError(Error{
				ID:             diagnostics.DuplicateLocalVariable,
				Severity:       diagnostics.SeverityError,
				Message:        fmt.Sprintf("variable %q already declared", name),
				File:           token.File,
				Line:           token.Line,
				Column:         token.Column,
				PreviousFile:   previous.Token.File,
				PreviousLine:   previous.Token.Line,
				PreviousColumn: previous.Token.Column,
			})
			return false
		}
	}

	symbol := Symbol{Name: name, Type: typ, Mutable: mutable, Token: token, Storage: StorageOriginInline, Local: a.inFunctionBody, ScopeDepth: a.scopeDepth}
	a.symbols[name] = symbol
	a.completionSymbols[name] = symbol
	a.recordDefinition(token)
	if a.inFunctionBody {
		a.recordBinding(token, BindingLocal, name, typ, mutable)
	}
	a.clearRootPlaceState(name)
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

	if target.Kind == ArrayType || value.Kind == ArrayType || target.Kind == SliceType || value.Kind == SliceType {
		if target.Kind == ArrayType && value.Kind == ArrayType && target.Element != nil && value.Element != nil &&
			(target.ArrayLength == dynamicArrayLength || value.ArrayLength == dynamicArrayLength) {
			return canInitialize(*target.Element, *value.Element, expr)
		}
		return sameConcreteType(target, value)
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

	if (isNominal(target) || isNominal(value)) && target.Name != value.Name {
		return canUntypedNumericInitializeNominal(target, value, expr)
	}

	if target.Kind == value.Kind {
		if isNumericType(target) && isNumericType(value) {
			if isNumericLiteral(expr) {
				return true
			}
			return sameConcreteType(target, value)
		}
		return true
	}

	if target.Kind == UintType && value.Kind == IntType {
		return isNumericLiteral(expr)
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

func preferredUnitOnlyNumeric(expr ast.Expression, valueType Type) string {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		switch expr.Suffix() {
		case "i":
			return "int"
		case "u":
			return "uint"
		case "g":
			return "float"
		case "m":
			return "decimal"
		default:
			return ""
		}
	case *ast.FloatLiteral:
		switch expr.Suffix() {
		case "g":
			return "float"
		case "m":
			return "decimal"
		default:
			return ""
		}
	}
	if isNumericType(valueType) {
		return valueType.Name
	}
	return ""
}

func canUntypedNumericInitializeNominal(target Type, value Type, expr ast.Expression) bool {
	if !isUntypedNumericExpression(expr) {
		return false
	}
	switch target.Underlying {
	case "int", "int8", "int16", "int32", "int64", "int128", "int256":
		return value.Kind == IntType || value.Kind == UintType
	case "uint", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256":
		return value.Kind == IntType || value.Kind == UintType
	case "float", "float32", "float64":
		return value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType
	case "decimal", "decimal128":
		return value.Kind == IntType || value.Kind == UintType || value.Kind == DecimalType || value.Kind == FloatType
	default:
		return false
	}
}

func canExplicitConvert(target Type, value Type) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return false
	}

	if target.Kind == EnumType && isIntegerType(value) {
		return true
	}

	if isIntegerType(target) && (isIntegerType(value) || value.Kind == DecimalType) {
		return true
	}

	if isIntegerType(target) && value.Kind == EnumType {
		return true
	}

	if target.Kind == RuneType && isIntegerType(value) {
		return true
	}

	if target.Kind == BoolType && isNumericType(value) {
		return true
	}

	if target.Kind == RawPtrType && (value.Kind == UintType || value.Kind == RawPtrType || value.Kind == ReferenceType) {
		return true
	}
	if target.Kind == UintType && (value.Kind == RawPtrType || value.Kind == ReferenceType) {
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

	if target.Kind == FloatType && isNumericType(value) {
		return true
	}

	if target.Kind == DecimalType && isNumericType(value) {
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

func taskType(result Type) Type {
	return Type{Name: "Task", Kind: StructType, TypeArgs: []Type{result}}
}

func threadType(result Type) Type {
	return Type{Name: "Thread", Kind: StructType, TypeArgs: []Type{result}}
}

func mutexGuardType(inner Type) Type {
	return Type{Name: "MutexGuard", Kind: StructType, TypeArgs: []Type{inner}}
}

func (a *Analyzer) intrinsicGenericType(name string, args ...Type) Type {
	typ, ok := a.types[name]
	if !ok {
		return Type{Name: name, Kind: InvalidType, Intrinsic: true, TypeArgs: args}
	}
	typ.TypeArgs = append([]Type(nil), args...)
	if typ.Kind == StructType || typ.Kind == UnionType {
		typ = a.instantiateGenericType(typ)
	}
	typ.Intrinsic = true
	return typ
}

func channelType(message Type) Type {
	return Type{Name: "Channel", Kind: StructType, Intrinsic: true, TypeArgs: []Type{message}}
}

func senderType(message Type) Type {
	return Type{Name: "Sender", Kind: StructType, Intrinsic: true, TypeArgs: []Type{message}}
}

func receiverType(message Type) Type {
	return Type{Name: "Receiver", Kind: StructType, Intrinsic: true, TypeArgs: []Type{message}}
}

func messageTicketType(message Type) Type {
	return Type{Name: "MessageTicket", Kind: StructType, Intrinsic: true, TypeArgs: []Type{message}}
}

func isTaskType(typ Type) bool {
	return typ.Name == "Task" && len(typ.TypeArgs) == 1
}

func isThreadType(typ Type) bool {
	return typ.Name == "Thread" && len(typ.TypeArgs) == 1
}

func isChannelType(typ Type) bool {
	return typ.Name == "Channel" && len(typ.TypeArgs) == 1
}

func isSenderType(typ Type) bool {
	return typ.Name == "Sender" && len(typ.TypeArgs) == 1
}

func isReceiverType(typ Type) bool {
	return typ.Name == "Receiver" && len(typ.TypeArgs) == 1
}

func isMutexType(typ Type) bool {
	return typ.Name == "Mutex" && len(typ.TypeArgs) == 1
}

func isMutexGuardType(typ Type) bool {
	return typ.Name == "MutexGuard" && len(typ.TypeArgs) == 1
}

func isAtomicType(typ Type) bool {
	return typ.Name == "Atomic" && len(typ.TypeArgs) == 1
}

func atomicFetchOperationSupported(operation string, element Type) bool {
	switch operation {
	case "fetchAnd", "fetchOr", "fetchXor":
		return element.Kind == BoolType || isIntegerType(element)
	case "fetchAdd", "fetchSub":
		return isIntegerType(element)
	default:
		return false
	}
}

func sameConcreteType(left Type, right Type) bool {
	if left.Kind == FunctionType || right.Kind == FunctionType {
		return sameFunctionType(left, right)
	}
	if left.Unit != "" || right.Unit != "" {
		return left.Kind == right.Kind &&
			left.Name == right.Name &&
			left.Unit == right.Unit &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	if !left.Dimension.IsZero() || !right.Dimension.IsZero() {
		return left.Kind == right.Kind &&
			left.Name == right.Name &&
			left.Dimension.Equal(right.Dimension) &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
	}
	if left.Name != "" || right.Name != "" {
		return left.Name == right.Name &&
			(!isEventFamilyName(left.Name) || eventCapacity(left) == eventCapacity(right)) &&
			sameTypeArguments(left.TypeArgs, right.TypeArgs) &&
			sameConstArguments(left.ConstArgs, right.ConstArgs)
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

func sameConstArguments(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func implicitlyCopyable(typ Type) bool {
	switch CopyClassificationOf(typ) {
	case CopyTrivial, CopySemantic:
		return true
	default:
		return false
	}
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

func isArithmeticOperator(operator string) bool {
	switch operator {
	case "+", "-", "*", "/", "%":
		return true
	default:
		return false
	}
}

func isBitwiseOperator(operator string) bool {
	switch operator {
	case "&", "|", "^", "<<", ">>":
		return true
	default:
		return false
	}
}

func validCharLiteral(lexeme string) bool {
	if len(lexeme) < 3 || lexeme[0] != '\'' || lexeme[len(lexeme)-1] != '\'' {
		return false
	}

	body := lexeme[1 : len(lexeme)-1]
	if body[0] != '\\' {
		return utf8.ValidString(body) && utf8.RuneCountInString(body) == 1
	}
	if len(body) < 2 {
		return false
	}

	switch body[1] {
	case '\\', '\'', '"', 'n', 'r', 't', '0':
		return len(body) == 2
	case 'x':
		if len(body) != 4 {
			return false
		}
		_, err := strconv.ParseUint(body[2:], 16, 8)
		return err == nil
	case 'u':
		if len(body) < 5 || body[2] != '{' || body[len(body)-1] != '}' {
			return false
		}
		digits := body[3 : len(body)-1]
		if len(digits) == 0 || len(digits) > 6 {
			return false
		}
		value, err := strconv.ParseUint(digits, 16, 32)
		if err != nil {
			return false
		}
		r := rune(value)
		return utf8.ValidRune(r) && (r < 0xD800 || r > 0xDFFF)
	default:
		return false
	}
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
		if typ.ArrayLength == dynamicArrayLength {
			return typeDisplayName(*typ.Element) + "[]"
		}
		return fmt.Sprintf("%s[%d]", typeDisplayName(*typ.Element), typ.ArrayLength)
	}
	if typ.Kind == SliceType && typ.Element != nil {
		return typeDisplayName(*typ.Element) + "[]"
	}
	if typ.Kind == FunctionType {
		return functionTypeName(typ.FunctionParameterTypes, functionReturnType(typ))
	}
	if typ.Name != "" && (len(typ.TypeArgs) > 0 || len(typ.ConstArgs) > 0) {
		out := typ.Name + "["
		for i, arg := range typ.TypeArgs {
			if i > 0 {
				out += ", "
			}
			out += typeDisplayName(arg)
		}
		for i, arg := range typ.ConstArgs {
			if len(typ.TypeArgs) > 0 || i > 0 {
				out += ", "
			}
			out += fmt.Sprintf("%d", arg)
		}
		if typ.EventCapacitySet {
			out += fmt.Sprintf(", %d", typ.EventCapacity)
		}
		out += "]"
		return out
	}
	if typ.Name != "" && typ.Unit != "" && !typ.Named && typ.Name != typ.Unit {
		return typ.Name + "<" + typ.Unit + ">"
	}
	if typ.Name != "" {
		return typ.Name
	}
	return string(typ.Kind)
}

func isBareSliceType(typ Type) bool {
	return typ.Kind == SliceType
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
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ImportStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.TypeDeclStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.EnumDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ImplStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.FunctionDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.StructStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.LetStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.LetGroupStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.AssignmentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.TryAssignmentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DeferStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DiscardStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.DetachStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.CancelStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ExpressionStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ReturnStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.IfStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ForStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.WhileStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.SwitchStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.SelectStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.FallthroughStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.BreakStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.ContinueStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.UnsafeStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.AsmStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.MatchStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.CommentStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.InvalidStatement:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	case *ast.InvalidDeclaration:
		if stmt == nil {
			return lexer.Token{}
		}
		return stmt.Token
	default:
		return lexer.Token{}
	}
}

func expressionToken(expr ast.Expression) lexer.Token {
	switch expr := expr.(type) {
	case *ast.InvalidExpression:
		return expr.Token
	case *ast.InvalidPattern:
		return expr.Token
	case *ast.Identifier:
		return expr.Token
	case *ast.IntegerLiteral:
		return expr.Token
	case *ast.FloatLiteral:
		return expr.Token
	case *ast.StringLiteral:
		return expr.Token
	case *ast.CharLiteral:
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
	case *ast.SpawnExpression:
		return expr.Token
	case *ast.AwaitExpression:
		return expr.Token
	case *ast.MatchExpression:
		return expr.Token
	case *ast.SpreadExpression:
		return expr.Token
	case *ast.RangeExpression:
		return expr.Token
	case *ast.MemberExpression:
		return expr.Token
	case *ast.ArrayLiteral:
		return expr.Token
	case *ast.IndexExpression:
		return expr.Token
	case *ast.SliceExpression:
		return expr.Token
	case *ast.RefExpression:
		return expr.Token
	case *ast.StructLiteral:
		return expr.Token
	case *ast.LambdaExpression:
		return expr.Token
	default:
		return lexer.Token{}
	}
}
