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
	DeclaredType Type
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
	summaries    map[CallableID]*ParameterUsageCallableSummary
	summaryOrder []CallableID
	iterations   int
	converged    bool
}

func newParameterUsageAnalysis() *ParameterUsageAnalysis {
	return &ParameterUsageAnalysis{summaries: map[CallableID]*ParameterUsageCallableSummary{}, converged: true}
}

func (p *ParameterUsageAnalysis) clone() *ParameterUsageAnalysis {
	result := newParameterUsageAnalysis()
	if p == nil {
		return result
	}
	result.summaryOrder = append([]CallableID(nil), p.summaryOrder...)
	result.iterations = p.iterations
	result.converged = p.converged
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
	analyzer  *Analyzer
	result    *ParameterUsageAnalysis
	summary   *ParameterUsageCallableSummary
	byBinding map[BindingID]*ParameterUsageParameterSummary
	byName    map[string]*ParameterUsageParameterSummary
	callSites []parameterUsageCallSite
}

type parameterUsageCallSite struct {
	caller    CallableID
	target    CallableID
	source    lexer.Token
	arguments []parameterUsageCallArgument
}

type parameterUsageCallArgument struct {
	callerParameter *ParameterUsageParameterSummary
	callerPlace     Place
	calleeIndex     int
	receiver        bool
}

func buildParameterUsageAnalysis(program *ast.Program, analyzer *Analyzer) *ParameterUsageAnalysis {
	builder := &parameterUsageBuilder{analyzer: analyzer, result: newParameterUsageAnalysis()}
	if program == nil || analyzer == nil {
		return builder.result
	}
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
	builder.propagateDirectCalls()
	for _, id := range builder.result.summaryOrder {
		builder.finishSummary(builder.result.summaries[id])
	}
	return builder.result
}

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
			DeclaredType: escapeSnapshotType(fact.Type), Demand: defaultParameterDemand(),
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
	if implTarget != "" {
		receiverType := escapeSnapshotType(b.analyzer.types[implTarget])
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

func (b *parameterUsageBuilder) applyEscapeDispositions(parameter *ParameterUsageParameterSummary, dispositions []EscapeParameterDisposition) {
	for _, disposition := range dispositions {
		switch disposition {
		case EscapeParameterReturned:
			parameter.Demand.Lifetime = ParameterLifetimeReturned
			parameter.Demand.Identity = strongerIdentity(parameter.Demand.Identity, ParameterAddressRequired)
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

func (b *parameterUsageBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		b.walkStatement(statement)
	}
}

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
	case *ast.DetachStatement:
		b.walkExpression(statement.Value)
	case *ast.ReturnStatement:
		b.walkExpression(statement.Value)
	case *ast.IfStatement:
		b.walkExpression(statement.Condition)
		b.walkBlock(statement.Consequence)
		b.walkBlock(statement.Alternative)
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

func (b *parameterUsageBuilder) walkExpression(expression ast.Expression) {
	if parameterUsageNodeIsNil(expression) {
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
		b.walkBlock(expression.Body)
	case *ast.SpawnExpression:
		b.walkExpression(expression.Value)
		b.walkBlock(expression.Body)
	case *ast.AwaitExpression:
		b.walkExpression(expression.Value)
	}
}

func (b *parameterUsageBuilder) walkCall(call *ast.CallExpression) {
	resolved, ok := b.analyzer.ResolvedCallTarget(call)
	if ok && isCompilerKnownFunctionName(resolved.Function.Name) {
		for _, argument := range call.Arguments {
			b.walkExpression(argument)
			b.addShape(argument, ParameterShapeSequence, 0)
		}
		return
	}
	if !ok || resolved.Kind == ResolvedForeignCall {
		if member, memberCall := call.Callee.(*ast.MemberExpression); memberCall {
			b.markUnknownCallArgument(member.Object)
		}
		for _, argument := range call.Arguments {
			b.markUnknownCallArgument(argument)
		}
		return
	}

	site := parameterUsageCallSite{
		caller: b.summary.Callable, target: callableID(resolved.Function), source: expressionToken(call),
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
		if index >= len(resolved.Function.Parameters) {
			b.markUnknownCallArgument(argument)
			continue
		}
		parameter := resolved.Function.Parameters[index]
		if parameter.Ref || parameter.Type.Kind == ReferenceType {
			mutable := parameter.MutableRef || parameter.Type.ReferenceMutable
			b.markExpression(argument, ParameterUseCall, mutable, ParameterBorrowSufficient, ParameterAddressRequired)
			b.walkExpressionChildren(argument)
		} else {
			ownership := ParameterBorrowSufficient
			if parameter.Consuming {
				ownership = ParameterConsumptionRequired
			}
			b.walkExpression(argument)
			b.markExpression(argument, ParameterUseCall, false, ownership, ParameterValueOnly)
		}
		if callerParameter, place, rooted := b.parameterPlace(argument); rooted {
			site.arguments = append(site.arguments, parameterUsageCallArgument{
				callerParameter: callerParameter, callerPlace: cloneEscapePlace(place), calleeIndex: index,
			})
		}
	}
	if len(site.arguments) > 0 {
		b.callSites = append(b.callSites, site)
	}
}

const parameterUsageProjectionLimit = 8

func (b *parameterUsageBuilder) propagateDirectCalls() {
	if len(b.callSites) == 0 {
		return
	}
	sites := b.orderedCallSites()
	limit := len(sites)*24 + len(b.result.summaries) + 1
	if configured := b.analyzer.analysisBudget.MaxSummaryIterations; configured > 0 && configured < limit {
		limit = configured
	}
	b.result.converged = false
	for iteration := 1; iteration <= limit; iteration++ {
		changed := false
		for index := range sites {
			if b.propagateCallSite(&sites[index]) {
				changed = true
			}
		}
		b.result.iterations = iteration
		if !changed {
			b.result.converged = true
			return
		}
	}
	for index := range sites {
		for _, argument := range sites[index].arguments {
			widenParameterDemand(&argument.callerParameter.Demand)
		}
	}
}

func (b *parameterUsageBuilder) orderedCallSites() []parameterUsageCallSite {
	sites := append([]parameterUsageCallSite(nil), b.callSites...)
	componentByCallable := map[CallableID]int{}
	if b.analyzer.callGraph != nil {
		for index, component := range b.analyzer.callGraph.sameStackComponents() {
			for id := range component {
				componentByCallable[id] = index
			}
		}
	}
	sort.SliceStable(sites, func(i, j int) bool {
		leftComponent, leftOK := componentByCallable[sites[i].caller]
		rightComponent, rightOK := componentByCallable[sites[j].caller]
		if leftOK != rightOK {
			return leftOK
		}
		if leftComponent != rightComponent {
			return leftComponent < rightComponent
		}
		left := sourceTokenLocation(sites[i].source)
		right := sourceTokenLocation(sites[j].source)
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	return sites
}

func (b *parameterUsageBuilder) propagateCallSite(site *parameterUsageCallSite) bool {
	target := b.result.summaries[site.target]
	changed := false
	for _, argument := range site.arguments {
		var callee *ParameterUsageParameterSummary
		if target != nil {
			if argument.receiver {
				callee = target.Receiver
			} else if argument.calleeIndex >= 0 && argument.calleeIndex < len(target.Parameters) {
				callee = &target.Parameters[argument.calleeIndex]
			}
		}
		if callee == nil {
			changed = widenParameterDemand(&argument.callerParameter.Demand) || changed
			continue
		}
		changed = joinParameterDemand(&argument.callerParameter.Demand, callee.Demand) || changed
		for _, use := range callee.Uses {
			place, widened := instantiateParameterUsePlace(argument.callerPlace, use.Place)
			if widened {
				changed = setDemandPrecision(&argument.callerParameter.Demand, ParameterDemandPartial) || changed
			}
			propagated := ParameterUse{Kind: ParameterUseCall, Source: site.source, Place: place}
			if appendUniqueParameterUse(argument.callerParameter, propagated) {
				changed = true
			}
		}
	}
	return changed
}

func instantiateParameterUsePlace(base Place, callee Place) (Place, bool) {
	result := cloneEscapePlace(base)
	widened := false
	for _, projection := range callee.Projections {
		if len(result.Projections) >= parameterUsageProjectionLimit {
			widened = true
			break
		}
		result = appendPlaceProjection(result, projection)
	}
	return result, widened
}

func appendUniqueParameterUse(parameter *ParameterUsageParameterSummary, candidate ParameterUse) bool {
	for _, existing := range parameter.Uses {
		if existing.Kind == candidate.Kind && sameSourceToken(existing.Source, candidate.Source) && existing.Place.String() == candidate.Place.String() {
			return false
		}
	}
	parameter.Uses = append(parameter.Uses, candidate)
	return true
}

func joinParameterDemand(target *ParameterDemand, source ParameterDemand) bool {
	changed := false
	changed = setAccessDemand(target, strongerAccess(target.Access, source.Access)) || changed
	changed = setMutationDemand(target, strongerMutation(target.Mutation, source.Mutation)) || changed
	changed = setOwnershipDemand(target, strongerOwnership(target.Ownership, source.Ownership)) || changed
	changed = setLifetimeDemand(target, strongerLifetime(target.Lifetime, source.Lifetime)) || changed
	changed = setIdentityDemand(target, strongerIdentity(target.Identity, source.Identity)) || changed
	for _, shape := range source.Shapes {
		before := len(target.Shapes)
		target.Shapes = appendUniqueShape(target.Shapes, shape)
		changed = len(target.Shapes) != before || changed
	}
	if source.MinimumExtent > target.MinimumExtent {
		target.MinimumExtent = source.MinimumExtent
		changed = true
	}
	for _, storage := range source.Storage {
		if storage == ParameterStorageNone && hasSpecialParameterStorage(target.Storage) {
			continue
		}
		if storage != ParameterStorageNone {
			target.Storage = removeParameterStorage(target.Storage, ParameterStorageNone)
		}
		before := len(target.Storage)
		target.Storage = appendUniqueParameterStorage(target.Storage, storage)
		changed = len(target.Storage) != before || changed
	}
	changed = setRepresentationDemand(target, strongerRepresentation(target.Representation, source.Representation)) || changed
	changed = setDemandPrecision(target, strongerPrecision(target.Precision, source.Precision)) || changed
	return changed
}

func widenParameterDemand(demand *ParameterDemand) bool {
	before := cloneParameterDemand(*demand)
	demand.Access = ParameterAccessUnknown
	demand.Mutation = ParameterUnknownMutation
	demand.Ownership = ParameterUnknownOwnership
	demand.Lifetime = ParameterLifetimeUnknown
	demand.Identity = ParameterUnknownIdentity
	demand.Shapes = appendUniqueShape(demand.Shapes, ParameterShapeUnknown)
	demand.Storage = removeParameterStorage(demand.Storage, ParameterStorageNone)
	demand.Storage = appendUniqueParameterStorage(demand.Storage, ParameterStorageUnknown)
	demand.Representation = ParameterRepresentationUnknown
	demand.Precision = ParameterDemandUnknown
	return !parameterDemandsEqual(before, *demand)
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
		b.walkExpression(arm.Pattern)
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

func (b *parameterUsageBuilder) parameterPlace(expression ast.Expression) (*ParameterUsageParameterSummary, Place, bool) {
	switch expression := expression.(type) {
	case *ast.Identifier:
		if resolved, ok := b.analyzer.ResolvedBindingOf(expression); ok {
			if parameter := b.byBinding[resolved.ID]; parameter != nil {
				return parameter, Place{Root: parameter.Name, RootToken: resolvedToken(b.analyzer, resolved.ID), Type: escapeSnapshotType(resolved.Type)}, true
			}
			if resolved.Kind == BindingParameter {
				if parameter := b.byName[resolved.Name]; parameter != nil {
					return parameter, Place{Root: parameter.Name, RootToken: expression.Token, Type: escapeSnapshotType(resolved.Type)}, true
				}
			}
		}
		if parameter := b.byName[expression.Value]; parameter != nil {
			return parameter, Place{Root: parameter.Name, RootToken: expression.Token, Type: parameter.DeclaredType}, true
		}
		return nil, Place{}, false
	case *ast.MemberExpression:
		parameter, place, ok := b.parameterPlace(expression.Object)
		if !ok || expression.Property == nil {
			return nil, Place{}, false
		}
		place = appendPlaceProjection(place, PlaceProjection{Kind: PlaceField, Name: expression.Property.Value, Token: expression.Property.Token})
		return parameter, place, true
	case *ast.IndexExpression:
		parameter, place, ok := b.parameterPlace(expression.Left)
		if !ok {
			return nil, Place{}, false
		}
		projection := PlaceProjection{Kind: PlaceIndex, DynamicIndex: true, Token: expressionToken(expression.Index)}
		if value, constant := constantIntegerValue(expression.Index); constant && value.IsInt64() {
			projection.ConstantIndex = value.Int64()
			projection.DynamicIndex = false
		}
		return parameter, appendPlaceProjection(place, projection), true
	case *ast.SliceExpression:
		parameter, place, ok := b.parameterPlace(expression.Left)
		if !ok {
			return nil, Place{}, false
		}
		projection := PlaceProjection{Kind: PlaceSlice, SliceStartKnown: expression.Start == nil, Token: expression.Token}
		if value, constant := constantIntegerValue(expression.Start); constant && value.IsInt64() {
			projection.SliceStart, projection.SliceStartKnown = value.Int64(), true
		}
		if value, constant := constantIntegerValue(expression.End); constant && value.IsInt64() {
			projection.SliceEnd, projection.SliceEndKnown = value.Int64(), true
			if !expression.Exclusive {
				projection.SliceEnd++
			}
		}
		return parameter, appendPlaceProjection(place, projection), true
	case *ast.RefExpression:
		return b.parameterPlace(expression.Value)
	default:
		return nil, Place{}, false
	}
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
	summary.DeclaredType = escapeSnapshotType(summary.DeclaredType)
	summary.Demand.Shapes = append([]ParameterShapeDemand(nil), summary.Demand.Shapes...)
	summary.Demand.Storage = append([]ParameterStorageDemand(nil), summary.Demand.Storage...)
	summary.Uses = append([]ParameterUse(nil), summary.Uses...)
	for index := range summary.Uses {
		summary.Uses[index].Place = cloneEscapePlace(summary.Uses[index].Place)
	}
	return summary
}
