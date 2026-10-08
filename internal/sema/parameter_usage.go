package sema

import (
	"reflect"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

type ParameterAccessDemand string

const (
	ParameterAccessUnused  ParameterAccessDemand = "unused"
	ParameterAccessRead    ParameterAccessDemand = "read"
	ParameterAccessWrite   ParameterAccessDemand = "write"
	ParameterAccessUnknown ParameterAccessDemand = "unknown"
)

type ParameterMutationDemand string

const (
	ParameterNoMutation             ParameterMutationDemand = "no-mutation"
	ParameterElementOrFieldMutation ParameterMutationDemand = "element-or-field-mutation"
	ParameterStructuralMutation     ParameterMutationDemand = "structural-mutation"
	ParameterUnknownMutation        ParameterMutationDemand = "unknown"
)

type ParameterOwnershipDemand string

const (
	ParameterBorrowSufficient    ParameterOwnershipDemand = "borrow-sufficient"
	ParameterOwnershipRequired   ParameterOwnershipDemand = "ownership-required"
	ParameterConsumptionRequired ParameterOwnershipDemand = "consumption-required"
	ParameterUnknownOwnership    ParameterOwnershipDemand = "unknown"
)

type ParameterLifetimeDemand string

const (
	ParameterLifetimeCallOnly         ParameterLifetimeDemand = "call-only"
	ParameterLifetimeReturned         ParameterLifetimeDemand = "returned"
	ParameterLifetimeRetained         ParameterLifetimeDemand = "retained"
	ParameterLifetimeCrossTask        ParameterLifetimeDemand = "cross-task"
	ParameterLifetimeCrossThread      ParameterLifetimeDemand = "cross-thread"
	ParameterLifetimeForeignRetention ParameterLifetimeDemand = "foreign-retention"
	ParameterLifetimeUnknown          ParameterLifetimeDemand = "unknown"
)

type ParameterIdentityDemand string

const (
	ParameterValueOnly              ParameterIdentityDemand = "value-only"
	ParameterAddressRequired        ParameterIdentityDemand = "address-required"
	ParameterStableIdentityRequired ParameterIdentityDemand = "stable-identity-required"
	ParameterUnknownIdentity        ParameterIdentityDemand = "unknown"
)

type ParameterShapeDemand string

const (
	ParameterShapeWholeValue         ParameterShapeDemand = "whole-value"
	ParameterShapeSequence           ParameterShapeDemand = "sequence"
	ParameterShapeContiguousSequence ParameterShapeDemand = "contiguous-sequence"
	ParameterShapeRandomAccess       ParameterShapeDemand = "random-access-sequence"
	ParameterShapeExactExtent        ParameterShapeDemand = "exact-extent"
	ParameterShapeMinimumExtent      ParameterShapeDemand = "minimum-extent"
	ParameterShapeKnownRange         ParameterShapeDemand = "known-range"
	ParameterShapeUnknown            ParameterShapeDemand = "unknown"
)

type ParameterStorageDemand string

const (
	ParameterStorageNone          ParameterStorageDemand = "no-special-storage"
	ParameterStorageContiguous    ParameterStorageDemand = "contiguous"
	ParameterStorageStableAddress ParameterStorageDemand = "stable-address"
	ParameterStorageAligned       ParameterStorageDemand = "aligned"
	ParameterStoragePinned        ParameterStorageDemand = "pinned"
	ParameterStorageMemorySpace   ParameterStorageDemand = "specific-memory-space"
	ParameterStorageUnknown       ParameterStorageDemand = "unknown"
)

type ParameterRepresentationDemand string

const (
	ParameterRepresentationNone    ParameterRepresentationDemand = "none"
	ParameterRepresentationExact   ParameterRepresentationDemand = "exact"
	ParameterRepresentationUnknown ParameterRepresentationDemand = "unknown"
)

type ParameterDemandPrecision string

const (
	ParameterDemandExact   ParameterDemandPrecision = "exact"
	ParameterDemandPartial ParameterDemandPrecision = "partial"
	ParameterDemandUnknown ParameterDemandPrecision = "unknown"
)

type ParameterDemand struct {
	Access         ParameterAccessDemand
	Mutation       ParameterMutationDemand
	Ownership      ParameterOwnershipDemand
	Lifetime       ParameterLifetimeDemand
	Identity       ParameterIdentityDemand
	Shapes         []ParameterShapeDemand
	MinimumExtent  int64
	Storage        []ParameterStorageDemand
	Representation ParameterRepresentationDemand
	Precision      ParameterDemandPrecision
}

type ParameterUseKind string

const (
	ParameterUseRead      ParameterUseKind = "read"
	ParameterUseWrite     ParameterUseKind = "write"
	ParameterUseMove      ParameterUseKind = "move"
	ParameterUseReference ParameterUseKind = "reference"
	ParameterUseIteration ParameterUseKind = "iteration"
	ParameterUseCall      ParameterUseKind = "call"
)

type ParameterUse struct {
	Kind   ParameterUseKind
	Source lexer.Token
	Place  Place
}

type ParameterUsageParameterSummary struct {
	Binding      BindingID
	Index        int
	Name         string
	Declaration  lexer.Token
	DeclaredType Type
	DeclaredRef  bool
	DeclaredMut  bool
	Consuming    bool
	Receiver     bool
	Demand       ParameterDemand
	Uses         []ParameterUse
}

type ParameterUsageCallableSummary struct {
	Callable    CallableID
	Name        string
	Declaration lexer.Token
	Parameters  []ParameterUsageParameterSummary
	Receiver    *ParameterUsageParameterSummary
	Precision   ParameterDemandPrecision
}

type ParameterUsageAnalysis struct {
	summaries      map[CallableID]*ParameterUsageCallableSummary
	summaryOrder   []CallableID
	iterations     int
	converged      bool
	budget         ParameterUsageBudget
	importCoverage ParameterImportCoverage
}

func newParameterUsageAnalysis() *ParameterUsageAnalysis {
	return &ParameterUsageAnalysis{summaries: map[CallableID]*ParameterUsageCallableSummary{}, converged: true, budget: parameterUsageBudget(AnalysisStandard)}
}

func (p *ParameterUsageAnalysis) clone() *ParameterUsageAnalysis {
	result := newParameterUsageAnalysis()
	if p == nil {
		return result
	}
	result.summaryOrder = append([]CallableID(nil), p.summaryOrder...)
	result.iterations = p.iterations
	result.converged = p.converged
	result.budget = p.budget
	result.importCoverage = p.importCoverage
	for id, summary := range p.summaries {
		copySummary := cloneParameterUsageCallableSummary(*summary)
		result.summaries[id] = &copySummary
	}
	return result
}

func (p *ParameterUsageAnalysis) InterproceduralStatus() (iterations int, converged bool) {
	if p == nil {
		return 0, true
	}
	return p.iterations, p.converged
}

func (p *ParameterUsageAnalysis) Summaries() []ParameterUsageCallableSummary {
	if p == nil {
		return nil
	}
	result := make([]ParameterUsageCallableSummary, 0, len(p.summaryOrder))
	for _, id := range p.summaryOrder {
		if summary := p.summaries[id]; summary != nil {
			result = append(result, cloneParameterUsageCallableSummary(*summary))
		}
	}
	return result
}

func (p *ParameterUsageAnalysis) Summary(id CallableID) (ParameterUsageCallableSummary, bool) {
	if p == nil || p.summaries[id] == nil {
		return ParameterUsageCallableSummary{}, false
	}
	return cloneParameterUsageCallableSummary(*p.summaries[id]), true
}

func (p *ParameterUsageAnalysis) SummariesForDeclaration(token lexer.Token) []ParameterUsageCallableSummary {
	result := []ParameterUsageCallableSummary{}
	for _, summary := range p.Summaries() {
		if sameSourceToken(summary.Declaration, token) {
			result = append(result, summary)
		}
	}
	return result
}

type parameterUsageBuilder struct {
	analyzer           *Analyzer
	result             *ParameterUsageAnalysis
	summary            *ParameterUsageCallableSummary
	byBinding          map[BindingID]*ParameterUsageParameterSummary
	byName             map[string]*ParameterUsageParameterSummary
	callSites          []parameterUsageCallSite
	functionValueCalls map[parameterUsageInvocation]CallSite
}

type parameterUsageCallSite struct {
	caller    CallableID
	targets   []CallableID
	open      bool
	contract  *OpenCallableContract
	source    lexer.Token
	arguments []parameterUsageCallArgument
}

type parameterUsageCallArgument struct {
	callerParameter *ParameterUsageParameterSummary
	callerPlace     Place
	calleeIndex     int
	receiver        bool
}

// buildParameterUsageAnalysis gathers local callable demand before joining
// direct, closure and function-value boundaries in one finite fixed point.
// Rules: rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses",
// "Calls propagate demand", "Function-value calls", "Recursive functions".
func buildParameterUsageAnalysis(program *ast.Program, analyzer *Analyzer) *ParameterUsageAnalysis {
	builder := &parameterUsageBuilder{analyzer: analyzer, result: newParameterUsageAnalysis()}
	if program == nil || analyzer == nil {
		return builder.result
	}
	builder.result.budget = analyzer.parameterBudget
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			builder.analyzeFunction(statement, "")
		case *ast.ImplStatement:
			if statement == nil || statement.Target == nil || !analyzer.validImplStatements[statement] {
				continue
			}
			for _, member := range statement.Members {
				if function, ok := member.(*ast.FunctionDeclaration); ok {
					builder.analyzeFunction(function, statement.Target.Name)
				}
			}
		}
	}
	builder.analyzeLambdaParameters()
	builder.installImportedDemands()
	builder.propagateCalls()
	for _, id := range builder.result.summaryOrder {
		builder.finishSummary(builder.result.summaries[id])
	}
	return builder.result
}

func parameterDemandHasShape(demand ParameterDemand, shape ParameterShapeDemand) bool {
	for _, candidate := range demand.Shapes {
		if candidate == shape {
			return true
		}
	}
	return false
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
		return 16, true
	case EnumType, RegisterType:
		if typ.BitWidth > 0 {
			return maxParameterSize(1, (typ.BitWidth+7)/8), true
		}
		if typ.RegisterWidth > 0 {
			return maxParameterSize(1, (typ.RegisterWidth+7)/8), true
		}
		return 4, true
	case StringType, ReferenceType, RawPtrType, SliceType, FunctionType:
		return 16, true
	case ArrayType:
		length, ok := legacyArrayLength(typ)
		if typ.Element == nil || !ok || length < 0 {
			return 0, false
		}
		elementSize, ok := estimatedTypeSizeBytes(*typ.Element, visiting)
		if !ok || length != 0 && elementSize > int64(^uint64(0)>>1)/length {
			return 0, false
		}
		return elementSize * length, true
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
		for _, argument := range typ.TypeArgs {
			if size, ok := estimatedTypeSizeBytes(argument, visiting); ok && size > maxPayload {
				maxPayload = size
			}
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				if size, ok := estimatedTypeSizeBytes(*variant.Payload, visiting); ok && size > maxPayload {
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

func arrayElementDisplayName(typ Type) string {
	if typ.Kind == ArrayType && typ.Element != nil {
		return typeDisplayName(*typ.Element)
	}
	return typeDisplayName(typ)
}

func maxParameterSize(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

// analyzeFunction derives local demand for resolved parameters and the implicit
// instance receiver. Static members have no receiver demand to export.
// Rules: rules/analysis/parameter_usage_analysis.md — "Receiver demand",
// "Function summaries"; rules/declarations/static.md — static members.
func (b *parameterUsageBuilder) analyzeFunction(declaration *ast.FunctionDeclaration, implTarget string) {
	if declaration == nil || declaration.Name == nil || declaration.Body == nil {
		return
	}
	function, ok := b.functionForDeclaration(declaration, implTarget)
	if !ok {
		return
	}
	id := callableID(function)
	summary := &ParameterUsageCallableSummary{
		Callable: id, Name: declaration.Name.Value, Declaration: function.Token, Precision: ParameterDemandExact,
	}
	b.summary = summary
	b.byBinding = map[BindingID]*ParameterUsageParameterSummary{}
	b.byName = map[string]*ParameterUsageParameterSummary{}
	for index, parameter := range declaration.Parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		fact := b.parameterFact(parameter)
		item := ParameterUsageParameterSummary{
			Binding: fact.ID, Index: index, Name: parameter.Name.Value,
			Declaration: parameter.Name.Token, DeclaredType: semanticSnapshotType(fact.Type),
			DeclaredRef: parameter.Ref, DeclaredMut: parameter.MutableRef, Consuming: parameter.Consuming,
			Demand: defaultParameterDemand(),
		}
		summary.Parameters = append(summary.Parameters, item)
	}
	for index := range summary.Parameters {
		stored := &summary.Parameters[index]
		b.byName[stored.Name] = stored
		if stored.Binding != 0 {
			b.byBinding[stored.Binding] = stored
		}
	}
	if implTarget != "" && !function.Static {
		receiverType := semanticSnapshotType(b.analyzer.types[implTarget])
		summary.Receiver = &ParameterUsageParameterSummary{
			Index: -1, Name: "self", DeclaredType: receiverType, Receiver: true, Demand: defaultParameterDemand(),
		}
		b.byName["self"] = summary.Receiver
	}
	b.walkBlock(declaration.Body)
	b.applyEscapeSummary(id)
	b.finishSummary(summary)
	b.result.summaries[id] = summary
	b.result.summaryOrder = append(b.result.summaryOrder, id)
}

func defaultParameterDemand() ParameterDemand {
	return ParameterDemand{
		Access: ParameterAccessUnused, Mutation: ParameterNoMutation,
		Ownership: ParameterBorrowSufficient, Lifetime: ParameterLifetimeCallOnly,
		Identity: ParameterValueOnly, Storage: []ParameterStorageDemand{ParameterStorageNone},
		Representation: ParameterRepresentationNone, Precision: ParameterDemandExact,
	}
}

func (b *parameterUsageBuilder) functionForDeclaration(declaration *ast.FunctionDeclaration, implTarget string) (Function, bool) {
	registeredName := declaration.Name.Value
	if implTarget != "" {
		registeredName = implTarget + "." + registeredName
	}
	for _, functions := range b.analyzer.functions {
		for _, function := range functions {
			if function.Name == registeredName && function.ImplTarget == implTarget && sameSourceToken(function.Token, declaration.Name.Token) {
				return function, true
			}
		}
	}
	return Function{}, false
}

func (b *parameterUsageBuilder) parameterFact(parameter *ast.Parameter) ResolvedBinding {
	key := sourceTokenLocation(parameter.Token)
	fact := b.analyzer.bindingFacts[key]
	fact.ID = b.analyzer.bindingIDs[key]
	if fact.Type.Kind == InvalidType || fact.Type.Name == "" {
		if parameter.Type != nil {
			if typ, ok := b.analyzer.types[parameter.Type.Name]; ok {
				fact.Type = typ
			}
		}
	}
	return fact
}

func (b *parameterUsageBuilder) finishSummary(summary *ParameterUsageCallableSummary) {
	summary.Precision = ParameterDemandExact
	for index := range summary.Parameters {
		if summary.Parameters[index].Demand.Precision != ParameterDemandExact {
			summary.Precision = ParameterDemandPartial
		}
		sortParameterDemand(&summary.Parameters[index].Demand)
	}
	if summary.Receiver != nil {
		if summary.Receiver.Demand.Precision != ParameterDemandExact {
			summary.Precision = ParameterDemandPartial
		}
		sortParameterDemand(&summary.Receiver.Demand)
	}
}

func (b *parameterUsageBuilder) applyEscapeSummary(id CallableID) {
	escape, ok := b.analyzer.escapeAnalysis.Summary(id)
	if !ok {
		return
	}
	for _, parameter := range escape.Parameters {
		if parameter.Index < 0 || parameter.Index >= len(b.summary.Parameters) {
			continue
		}
		b.applyEscapeDispositions(&b.summary.Parameters[parameter.Index], parameter.Dispositions)
	}
	if b.summary.Receiver != nil {
		b.applyEscapeDispositions(b.summary.Receiver, escape.Receiver)
	}
}

// applyEscapeDispositions imports lifetime and transfer evidence without
// treating a copied projected return as a surviving borrow dependency.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Returning a parameter by value"
//   - rules/analysis/parameter_usage_analysis.md — "Returning a borrow/view"
//   - rules/analysis/parameter_usage_analysis.md — "Retaining/storing a parameter"
func (b *parameterUsageBuilder) applyEscapeDispositions(parameter *ParameterUsageParameterSummary, dispositions []EscapeParameterDisposition) {
	for _, disposition := range dispositions {
		switch disposition {
		case EscapeParameterReturned:
			if typeCarriesReferenceOrigin(parameter.DeclaredType) || parameterHasUseKind(parameter, ParameterUseReference) {
				parameter.Demand.Lifetime = ParameterLifetimeReturned
				parameter.Demand.Identity = strongerIdentity(parameter.Demand.Identity, ParameterAddressRequired)
			} else if parameterHasWholePlaceUse(parameter) {
				parameter.Demand.Ownership = strongerOwnership(parameter.Demand.Ownership, ParameterOwnershipRequired)
			}
		case EscapeParameterRetained, EscapeParameterStoredInEscapingCarrier:
			parameter.Demand.Lifetime = ParameterLifetimeRetained
		case EscapeParameterOwnershipTransferred:
			parameter.Demand.Ownership = ParameterConsumptionRequired
		case EscapeParameterTransferredToTask:
			parameter.Demand.Lifetime = ParameterLifetimeCrossTask
		case EscapeParameterTransferredToThread:
			parameter.Demand.Lifetime = ParameterLifetimeCrossThread
		case EscapeParameterPassedToForeign:
			parameter.Demand.Lifetime = ParameterLifetimeForeignRetention
			parameter.Demand.Identity = strongerIdentity(parameter.Demand.Identity, ParameterStableIdentityRequired)
		case EscapeParameterUnknownRetention:
			parameter.Demand.Lifetime = ParameterLifetimeUnknown
			parameter.Demand.Precision = ParameterDemandPartial
		}
	}
}

// parameterHasUseKind distinguishes a returned borrow dependency from a
// copied scalar/field result that carries no lifetime relation to its source.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Returning a parameter by value"
//   - rules/analysis/parameter_usage_analysis.md — "Returning a borrow/view"
func parameterHasUseKind(parameter *ParameterUsageParameterSummary, kind ParameterUseKind) bool {
	for _, use := range parameter.Uses {
		if use.Kind == kind {
			return true
		}
	}
	return false
}

func parameterHasWholePlaceUse(parameter *ParameterUsageParameterSummary) bool {
	for _, use := range parameter.Uses {
		if len(use.Place.Projections) == 0 {
			return true
		}
	}
	return false
}

// walkStatement joins demand from reachable statement operations, preserving
// returned fixed-array shape requirements independently of ordinary reads.
// Rules: rules/analysis/parameter_usage_analysis.md — "Operation-to-demand transfer",
// "Exact extent and length observation are distinct", "Unreachable paths".
func (b *parameterUsageBuilder) walkStatement(statement ast.Statement) {
	if parameterUsageNodeIsNil(statement) {
		return
	}
	switch statement := statement.(type) {
	case *ast.LetStatement:
		b.walkExpression(statement.Value)
		b.walkExpression(statement.Address)
		if statement.Ownership == ast.OwnershipMove {
			b.markExpression(statement.Value, ParameterUseMove, false, ParameterConsumptionRequired, ParameterValueOnly)
		}
	case *ast.LetGroupStatement:
		for _, item := range statement.Lets {
			b.walkStatement(item)
		}
	case *ast.AssignmentStatement:
		b.markExpression(statement.Target, ParameterUseWrite, true, ParameterBorrowSufficient, ParameterValueOnly)
		if statement.Operator != "=" && statement.Operator != ":=" && statement.Operator != "<-" {
			b.markExpression(statement.Target, ParameterUseRead, false, ParameterBorrowSufficient, ParameterValueOnly)
		}
		b.walkExpression(statement.Value)
		if statement.Ownership == ast.OwnershipMove {
			b.markExpression(statement.Value, ParameterUseMove, false, ParameterConsumptionRequired, ParameterValueOnly)
		}
	case *ast.TryAssignmentStatement:
		b.walkStatement(statement.Assignment)
		b.walkTryHandlers(statement.Handlers)
	case *ast.ExpressionStatement:
		b.walkExpression(statement.Expression)
	case *ast.DiscardStatement:
		b.walkExpression(statement.Value)
	case *ast.AssertStatement:
		b.walkExpression(statement.Condition)
	case *ast.DetachStatement:
		b.walkExpression(statement.Value)
	case *ast.ReturnStatement:
		b.walkExpression(statement.Value)
		b.recordReturnedExactExtent(statement.Value)
	case *ast.IfStatement:
		b.walkIfStatement(statement)
	case *ast.SwitchStatement:
		b.walkExpression(statement.Subject)
		for _, item := range statement.Cases {
			b.walkSwitchCase(item)
		}
		b.walkSwitchCase(statement.Default)
	case *ast.SelectStatement:
		for _, branch := range statement.Branches {
			if branch == nil {
				continue
			}
			b.walkExpression(branch.Value)
			b.walkBlock(branch.Body)
		}
	case *ast.ForStatement:
		b.markExpression(statement.Iterable, ParameterUseIteration, false, ParameterBorrowSufficient, ParameterValueOnly)
		b.addShape(statement.Iterable, ParameterShapeSequence, 0)
		b.walkExpressionChildren(statement.Iterable)
		b.walkExpression(statement.Step)
		b.walkBlock(statement.Body)
	case *ast.WhileStatement:
		b.walkExpression(statement.Condition)
		b.walkBlock(statement.Body)
	case *ast.DeferStatement:
		b.walkBlock(statement.Body)
	case *ast.UnsafeStatement:
		b.walkBlock(statement.Body)
	case *ast.MatchStatement:
		b.walkMatch(statement.Match)
	}
}

func (b *parameterUsageBuilder) walkSwitchCase(item *ast.SwitchCase) {
	if item == nil {
		return
	}
	for _, candidate := range item.Items {
		switch candidate := candidate.(type) {
		case *ast.SwitchValueCase:
			b.walkExpression(candidate.Value)
		case *ast.SwitchRangeCase:
			b.walkExpression(candidate.Range)
		case *ast.SwitchRelationalCase:
			b.walkExpression(candidate.Value)
		}
	}
	b.walkBlock(item.Body)
}

// walkExpression records resolved parameter access and shape demand, consuming
// the compiler-owned pointer member fact before ordinary member projections.
// Rules: rules/analysis/parameter_usage_analysis.md — "Operation-to-demand transfer",
// "Raw pointer/address formation", "Contiguous-sequence demand".
func (b *parameterUsageBuilder) walkExpression(expression ast.Expression) {
	if parameterUsageNodeIsNil(expression) {
		return
	}
	if b.walkCompilerKnownPointerAccess(expression) {
		return
	}
	if b.markExpression(expression, ParameterUseRead, false, ParameterBorrowSufficient, ParameterValueOnly) {
		b.addShapeForAccess(expression)
		b.walkExpressionChildren(expression)
		return
	}
	b.walkExpressionChildren(expression)
}

func parameterUsageNodeIsNil(node any) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return value.Kind() == reflect.Ptr && value.IsNil()
}

func (b *parameterUsageBuilder) walkExpressionChildren(expression ast.Expression) {
	switch expression := expression.(type) {
	case *ast.PrefixExpression:
		b.walkExpression(expression.Right)
	case *ast.InfixExpression:
		b.walkExpression(expression.Left)
		b.walkExpression(expression.Right)
	case *ast.RangeExpression:
		b.walkExpression(expression.Start)
		b.walkExpression(expression.End)
	case *ast.ConversionExpression:
		b.walkExpression(expression.Value)
	case *ast.MemberExpression:
		// The full member Place was recorded by markExpression.
	case *ast.IndexExpression:
		b.walkExpression(expression.Index)
	case *ast.SliceExpression:
		b.walkExpression(expression.Start)
		b.walkExpression(expression.End)
	case *ast.RefExpression:
		b.markExpression(expression.Value, ParameterUseReference, expression.Mutable, ParameterBorrowSufficient, ParameterAddressRequired)
	case *ast.ArrayLiteral:
		for _, item := range expression.Elements {
			b.walkExpression(item)
		}
	case *ast.SpreadExpression:
		b.walkExpression(expression.Value)
	case *ast.StructLiteral:
		for _, field := range expression.Fields {
			b.walkExpression(field.Value)
		}
	case *ast.CallExpression:
		b.walkCall(expression)
	case *ast.RuntimeCallExpression:
		for _, argument := range expression.Arguments {
			b.markUnknownCallArgument(argument)
		}
	case *ast.OkExpression:
		b.walkExpression(expression.Value)
		for _, argument := range expression.Arguments {
			b.walkExpression(argument)
		}
	case *ast.ErrExpression:
		b.walkExpression(expression.Value)
		for _, argument := range expression.Arguments {
			b.walkExpression(argument)
		}
	case *ast.TryExpression:
		b.walkExpression(expression.Expression)
		b.walkTryHandlers(expression.Handlers)
	case *ast.MatchExpression:
		b.walkMatch(expression)
	case *ast.LambdaExpression:
		b.recordCaptureCreationDemand(expression)
	case *ast.SpawnExpression:
		b.walkExpression(expression.Value)
		b.walkBlock(expression.Body)
	case *ast.AwaitExpression:
		b.walkExpression(expression.Value)
	}
}

// walkCall derives local operation demand and records resolved call boundaries
// for interprocedural fixed-point propagation. Compiler-known collection
// operations are consumed through their canonical registry contracts first.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Calls propagate demand"
//   - rules/analysis/parameter_usage_analysis.md — "Function-value calls"
//   - rules/analysis/parameter_usage_analysis.md — "Structural collection operations", "Inputs from other analyses"
func (b *parameterUsageBuilder) walkCall(call *ast.CallExpression) {
	if b.walkCompilerKnownStructuralCollectionCall(call) {
		return
	}
	resolved, ok := b.analyzer.ResolvedCallTarget(call)
	if ok && isCompilerKnownFunctionName(resolved.Function.Name) {
		for _, argument := range call.Arguments {
			b.walkExpression(argument)
			b.addShape(argument, ParameterShapeSequence, 0)
		}
		return
	}
	if b.walkFunctionValueCall(call) {
		return
	}
	if !ok || resolved.Kind == ResolvedForeignCall {
		// Invocation reads a function-value parameter even when its target
		// contract is unavailable; that is separate from unknown argument demand.
		if _, identifierCall := call.Callee.(*ast.Identifier); identifierCall {
			b.markExpression(call.Callee, ParameterUseCall, false, ParameterBorrowSufficient, ParameterValueOnly)
		}
		if member, memberCall := call.Callee.(*ast.MemberExpression); memberCall {
			b.markUnknownCallArgument(member.Object)
		}
		for _, argument := range call.Arguments {
			b.markUnknownCallArgument(argument)
		}
		return
	}

	site := parameterUsageCallSite{
		caller: b.summary.Callable, targets: []CallableID{callableID(resolved.Function)}, source: expressionToken(call),
	}
	if member, memberCall := call.Callee.(*ast.MemberExpression); memberCall {
		mutable := resolved.Function.ReceiverMutable
		identity := ParameterValueOnly
		if mutable {
			identity = ParameterAddressRequired
		}
		b.markExpression(member.Object, ParameterUseCall, mutable, ParameterBorrowSufficient, identity)
		if parameter, place, rooted := b.parameterPlace(member.Object); rooted {
			site.arguments = append(site.arguments, parameterUsageCallArgument{
				callerParameter: parameter, callerPlace: cloneEscapePlace(place), receiver: true,
			})
		}
	}
	for index, argument := range call.Arguments {
		argumentSource := parameterUsageTransferSource(argument)
		if index >= len(resolved.Function.Parameters) {
			b.markUnknownCallArgument(argumentSource)
			continue
		}
		parameter := resolved.Function.Parameters[index]
		if parameter.Ref || parameter.Type.Kind == ReferenceType {
			mutable := parameter.MutableRef || parameter.Type.ReferenceMutable
			b.markExpression(argumentSource, ParameterUseCall, mutable, ParameterBorrowSufficient, ParameterAddressRequired)
			b.walkExpressionChildren(argumentSource)
		} else {
			ownership := ParameterBorrowSufficient
			if parameter.Consuming {
				ownership = ParameterConsumptionRequired
			}
			b.walkExpression(argumentSource)
			b.markExpression(argumentSource, ParameterUseCall, false, ownership, ParameterValueOnly)
		}
		if callerParameter, place, rooted := b.parameterPlace(argumentSource); rooted {
			site.arguments = append(site.arguments, parameterUsageCallArgument{
				callerParameter: callerParameter, callerPlace: cloneEscapePlace(place), calleeIndex: index,
			})
		}
	}
	if len(site.arguments) > 0 {
		b.callSites = append(b.callSites, site)
	}
}

// parameterUsageTransferSource exposes the canonical Place below explicit <-
// so consuming call demand is attributed to the caller parameter rather than
// lost on the syntactic ownership marker.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Move use"
//   - rules/analysis/parameter_usage_analysis.md — "Calls propagate demand"
func parameterUsageTransferSource(expression ast.Expression) ast.Expression {
	prefix, ok := expression.(*ast.PrefixExpression)
	if ok && prefix.Operator == "<-" && prefix.Right != nil {
		return prefix.Right
	}
	return expression
}

func cloneParameterDemand(demand ParameterDemand) ParameterDemand {
	demand.Shapes = append([]ParameterShapeDemand(nil), demand.Shapes...)
	demand.Storage = append([]ParameterStorageDemand(nil), demand.Storage...)
	return demand
}

func parameterDemandsEqual(left, right ParameterDemand) bool {
	if left.Access != right.Access || left.Mutation != right.Mutation || left.Ownership != right.Ownership ||
		left.Lifetime != right.Lifetime || left.Identity != right.Identity || left.MinimumExtent != right.MinimumExtent ||
		left.Representation != right.Representation || left.Precision != right.Precision ||
		len(left.Shapes) != len(right.Shapes) || len(left.Storage) != len(right.Storage) {
		return false
	}
	for index := range left.Shapes {
		if left.Shapes[index] != right.Shapes[index] {
			return false
		}
	}
	for index := range left.Storage {
		if left.Storage[index] != right.Storage[index] {
			return false
		}
	}
	return true
}

func setAccessDemand(demand *ParameterDemand, value ParameterAccessDemand) bool {
	if demand.Access == value {
		return false
	}
	demand.Access = value
	return true
}

func setMutationDemand(demand *ParameterDemand, value ParameterMutationDemand) bool {
	if demand.Mutation == value {
		return false
	}
	demand.Mutation = value
	return true
}

func setOwnershipDemand(demand *ParameterDemand, value ParameterOwnershipDemand) bool {
	if demand.Ownership == value {
		return false
	}
	demand.Ownership = value
	return true
}

func setLifetimeDemand(demand *ParameterDemand, value ParameterLifetimeDemand) bool {
	if demand.Lifetime == value {
		return false
	}
	demand.Lifetime = value
	return true
}

func setIdentityDemand(demand *ParameterDemand, value ParameterIdentityDemand) bool {
	if demand.Identity == value {
		return false
	}
	demand.Identity = value
	return true
}

func setRepresentationDemand(demand *ParameterDemand, value ParameterRepresentationDemand) bool {
	if demand.Representation == value {
		return false
	}
	demand.Representation = value
	return true
}

func setDemandPrecision(demand *ParameterDemand, value ParameterDemandPrecision) bool {
	if demand.Precision == value {
		return false
	}
	demand.Precision = value
	return true
}

func (b *parameterUsageBuilder) markUnknownCallArgument(expression ast.Expression) {
	b.walkExpression(expression)
	if parameter, _, ok := b.parameterPlace(expression); ok {
		parameter.Demand.Ownership = ParameterUnknownOwnership
		parameter.Demand.Lifetime = ParameterLifetimeUnknown
		parameter.Demand.Identity = ParameterUnknownIdentity
		parameter.Demand.Precision = ParameterDemandPartial
	}
}

func (b *parameterUsageBuilder) walkTryHandlers(handlers []*ast.TryHandler) {
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		b.walkExpression(handler.Pattern)
		b.walkExpression(handler.Body)
		b.walkStatement(handler.ReturnBody)
		b.walkBlock(handler.BlockBody)
	}
}

func (b *parameterUsageBuilder) walkMatch(expression *ast.MatchExpression) {
	if expression == nil {
		return
	}
	b.walkExpression(expression.Subject)
	for _, arm := range expression.Arms {
		if arm == nil {
			continue
		}
		b.walkExpression(arm.Guard)
		b.walkExpression(arm.Body)
		b.walkStatement(arm.ReturnBody)
		b.walkBlock(arm.BlockBody)
	}
}

func (b *parameterUsageBuilder) markExpression(expression ast.Expression, kind ParameterUseKind, write bool, ownership ParameterOwnershipDemand, identity ParameterIdentityDemand) bool {
	parameter, place, ok := b.parameterPlace(expression)
	if !ok {
		return false
	}
	if write {
		parameter.Demand.Access = ParameterAccessWrite
		parameter.Demand.Mutation = strongerMutation(parameter.Demand.Mutation, ParameterElementOrFieldMutation)
	} else if parameter.Demand.Access == ParameterAccessUnused {
		parameter.Demand.Access = ParameterAccessRead
	}
	parameter.Demand.Ownership = strongerOwnership(parameter.Demand.Ownership, ownership)
	parameter.Demand.Identity = strongerIdentity(parameter.Demand.Identity, identity)
	parameter.Uses = append(parameter.Uses, ParameterUse{Kind: kind, Source: expressionToken(expression), Place: cloneEscapePlace(place)})
	return true
}

func resolvedToken(analyzer *Analyzer, id BindingID) lexer.Token {
	for key, candidate := range analyzer.bindingIDs {
		if candidate == id {
			return lexer.Token{File: key.File, Line: key.Line, Column: key.Column}
		}
	}
	return lexer.Token{}
}

func (b *parameterUsageBuilder) addShapeForAccess(expression ast.Expression) {
	switch expression := expression.(type) {
	case *ast.Identifier:
		parameter, _, rooted := b.parameterPlace(expression)
		if rooted && !parameter.DeclaredRef && parameter.DeclaredType.Kind != ReferenceType {
			b.addShape(expression, ParameterShapeWholeValue, 0)
		}
	case *ast.IndexExpression:
		minimum := int64(0)
		if value, ok := constantIntegerValue(expression.Index); ok && value.IsInt64() && value.Int64() >= 0 {
			minimum = value.Int64() + 1
		}
		b.addShape(expression, ParameterShapeRandomAccess, minimum)
	case *ast.SliceExpression:
		b.addShape(expression, ParameterShapeSequence, 0)
		b.addShape(expression, ParameterShapeKnownRange, 0)
	}
}

func (b *parameterUsageBuilder) addShape(expression ast.Expression, shape ParameterShapeDemand, minimum int64) {
	parameter, _, ok := b.parameterPlace(expression)
	if !ok {
		return
	}
	parameter.Demand.Shapes = appendUniqueShape(parameter.Demand.Shapes, shape)
	if minimum > parameter.Demand.MinimumExtent {
		parameter.Demand.MinimumExtent = minimum
		parameter.Demand.Shapes = appendUniqueShape(parameter.Demand.Shapes, ParameterShapeMinimumExtent)
	}
}

func strongerMutation(left, right ParameterMutationDemand) ParameterMutationDemand {
	rank := map[ParameterMutationDemand]int{ParameterNoMutation: 0, ParameterElementOrFieldMutation: 1, ParameterStructuralMutation: 2, ParameterUnknownMutation: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerAccess(left, right ParameterAccessDemand) ParameterAccessDemand {
	rank := map[ParameterAccessDemand]int{ParameterAccessUnused: 0, ParameterAccessRead: 1, ParameterAccessWrite: 2, ParameterAccessUnknown: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerOwnership(left, right ParameterOwnershipDemand) ParameterOwnershipDemand {
	rank := map[ParameterOwnershipDemand]int{ParameterBorrowSufficient: 0, ParameterOwnershipRequired: 1, ParameterConsumptionRequired: 2, ParameterUnknownOwnership: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerIdentity(left, right ParameterIdentityDemand) ParameterIdentityDemand {
	rank := map[ParameterIdentityDemand]int{ParameterValueOnly: 0, ParameterAddressRequired: 1, ParameterStableIdentityRequired: 2, ParameterUnknownIdentity: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerLifetime(left, right ParameterLifetimeDemand) ParameterLifetimeDemand {
	rank := map[ParameterLifetimeDemand]int{
		ParameterLifetimeCallOnly: 0, ParameterLifetimeReturned: 1, ParameterLifetimeRetained: 2,
		ParameterLifetimeCrossTask: 3, ParameterLifetimeCrossThread: 4,
		ParameterLifetimeForeignRetention: 5, ParameterLifetimeUnknown: 6,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerRepresentation(left, right ParameterRepresentationDemand) ParameterRepresentationDemand {
	rank := map[ParameterRepresentationDemand]int{
		ParameterRepresentationNone: 0, ParameterRepresentationExact: 1, ParameterRepresentationUnknown: 2,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func strongerPrecision(left, right ParameterDemandPrecision) ParameterDemandPrecision {
	rank := map[ParameterDemandPrecision]int{ParameterDemandExact: 0, ParameterDemandPartial: 1, ParameterDemandUnknown: 2}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func appendUniqueShape(items []ParameterShapeDemand, item ParameterShapeDemand) []ParameterShapeDemand {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func appendUniqueParameterStorage(items []ParameterStorageDemand, item ParameterStorageDemand) []ParameterStorageDemand {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func removeParameterStorage(items []ParameterStorageDemand, remove ParameterStorageDemand) []ParameterStorageDemand {
	result := items[:0]
	for _, item := range items {
		if item != remove {
			result = append(result, item)
		}
	}
	return result
}

func hasSpecialParameterStorage(items []ParameterStorageDemand) bool {
	for _, item := range items {
		if item != ParameterStorageNone {
			return true
		}
	}
	return false
}

func sortParameterDemand(demand *ParameterDemand) {
	sort.Slice(demand.Shapes, func(i, j int) bool { return demand.Shapes[i] < demand.Shapes[j] })
	sort.Slice(demand.Storage, func(i, j int) bool { return demand.Storage[i] < demand.Storage[j] })
}

func cloneParameterUsageCallableSummary(summary ParameterUsageCallableSummary) ParameterUsageCallableSummary {
	summary.Parameters = append([]ParameterUsageParameterSummary(nil), summary.Parameters...)
	for index := range summary.Parameters {
		summary.Parameters[index] = cloneParameterUsageParameterSummary(summary.Parameters[index])
	}
	if summary.Receiver != nil {
		copyReceiver := cloneParameterUsageParameterSummary(*summary.Receiver)
		summary.Receiver = &copyReceiver
	}
	return summary
}

func cloneParameterUsageParameterSummary(summary ParameterUsageParameterSummary) ParameterUsageParameterSummary {
	summary.DeclaredType = semanticSnapshotType(summary.DeclaredType)
	summary.Demand.Shapes = append([]ParameterShapeDemand(nil), summary.Demand.Shapes...)
	summary.Demand.Storage = append([]ParameterStorageDemand(nil), summary.Demand.Storage...)
	summary.Uses = append([]ParameterUse(nil), summary.Uses...)
	for index := range summary.Uses {
		summary.Uses[index].Place = cloneEscapePlace(summary.Uses[index].Place)
	}
	return summary
}

// walkBlock records demand only until the current block can no longer
// continue. The completed Sema flow facts remain authoritative for constructs
// whose reachability cannot be recovered safely from syntax alone.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/analysis/parameter_usage_analysis.md — "Control-flow joins"
func (b *parameterUsageBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		b.walkStatement(statement)
		if !b.statementCanFallThrough(statement) {
			return
		}
	}
}

// statementCanFallThrough consumes compiler-owned if-flow decisions before
// falling back to the shared Sema control-flow query. This preserves dynamic
// branches while excluding paths eliminated by availability and other
// canonical compile-time reasoning.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/control-flow/flowcontrol_if.md — §19 "Terminating branches"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func (b *parameterUsageBuilder) statementCanFallThrough(statement ast.Statement) bool {
	if branch, ok := statement.(*ast.IfStatement); ok {
		if flow, resolved := b.analyzer.ResolvedIfFlowOf(branch); resolved {
			trueContinues := flow.TruePathExecution != ResolvedIfPathNever && flow.TruePathContinues
			falseContinues := flow.FalsePathExecution != ResolvedIfPathNever && flow.FalsePathContinues
			return trueContinues || falseContinues
		}
	}
	return b.analyzer.statementCanFallThrough(statement)
}

// walkIfStatement records the condition demand and excludes only branches
// that completed semantic analysis has proven impossible. Missing flow facts
// retain the conservative pre-existing behavior and visit both branches.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func (b *parameterUsageBuilder) walkIfStatement(statement *ast.IfStatement) {
	if statement == nil {
		return
	}
	b.walkExpression(statement.Condition)

	flow, resolved := b.analyzer.ResolvedIfFlowOf(statement)
	if !resolved {
		b.walkBlock(statement.Consequence)
		b.walkBlock(statement.Alternative)
		return
	}
	if flow.TruePathExecution != ResolvedIfPathNever {
		b.walkBlock(statement.Consequence)
	}
	if flow.FalsePathExecution != ResolvedIfPathNever {
		b.walkBlock(statement.Alternative)
	}
}
