package sema

import (
	"sort"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// PitfallRuleID is a stable analysis identity. It is deliberately separate
// from user-facing diagnostic IDs because proven errors remain owned by the
// normative analysis that establishes invalidity.
type PitfallRuleID string

const (
	PitfallInclusiveLengthIndex     PitfallRuleID = "pitfall.bounds.inclusive-length-index"
	PitfallDirectIndexAtLength      PitfallRuleID = "pitfall.bounds.direct-index-at-length"
	PitfallBooleanLiteralComparison PitfallRuleID = "pitfall.boolean.redundant-literal-comparison"
)

type PitfallFamily string

const (
	PitfallBoundsAndRanges PitfallFamily = "bounds-and-ranges"
	PitfallBooleanIntent   PitfallFamily = "boolean-intent"
)

type PitfallClassification string

const (
	PitfallProvenInvalid    PitfallClassification = "proven-invalid"
	PitfallLikelyMistake    PitfallClassification = "likely-mistake"
	PitfallSuspiciousIntent PitfallClassification = "suspicious-intent"
)

type PitfallConfidence string

const (
	PitfallConfidenceProven PitfallConfidence = "proven"
	PitfallConfidenceHigh   PitfallConfidence = "high"
	PitfallConfidenceMedium PitfallConfidence = "medium"
)

type PitfallAnalysisState string

const (
	PitfallStateFinding      PitfallAnalysisState = "finding"
	PitfallStateNoFinding    PitfallAnalysisState = "no-finding"
	PitfallStateSuppressed   PitfallAnalysisState = "suppressed"
	PitfallStateNotEvaluated PitfallAnalysisState = "not-evaluated"
	PitfallStatePending      PitfallAnalysisState = "pending"
)

type PitfallEvidenceStrength string

const (
	PitfallEvidenceProof         PitfallEvidenceStrength = "proof"
	PitfallEvidenceStrong        PitfallEvidenceStrength = "strong"
	PitfallEvidenceSupporting    PitfallEvidenceStrength = "supporting"
	PitfallEvidenceContradicting PitfallEvidenceStrength = "contradicting"
	PitfallEvidenceSuppressing   PitfallEvidenceStrength = "suppressing"
)

type PitfallActionKind string

const (
	PitfallProvenFix     PitfallActionKind = "proven-fix"
	PitfallSuggestedEdit PitfallActionKind = "suggested-edit"
)

type PitfallEvidence struct {
	Strength PitfallEvidenceStrength
	Fact     string
	Source   lexer.Token
}

type PitfallSuppression struct {
	Reason   string
	Evidence []PitfallEvidence
}

type PitfallSuggestedAction struct {
	Kind        PitfallActionKind
	Title       string
	Replacement string
	Source      lexer.Token
}

type PitfallSubject struct {
	Expression string
	Source     lexer.Token
}

type PitfallFinding struct {
	Rule            PitfallRuleID
	Family          PitfallFamily
	Classification  PitfallClassification
	Confidence      PitfallConfidence
	Subject         PitfallSubject
	EvidenceFor     []PitfallEvidence
	EvidenceAgainst []PitfallEvidence
	Suppression     *PitfallSuppression
	OwningRule      string
	Actions         []PitfallSuggestedAction
	State           PitfallAnalysisState
}

type PitfallRuleDefinition struct {
	ID                PitfallRuleID
	Family            PitfallFamily
	RequiredFacts     []string
	MinimumDepth      AnalysisDepth
	DefaultConfidence PitfallConfidence
}

type PitfallRuleEvaluation struct {
	Rule            PitfallRuleID
	State           PitfallAnalysisState
	FindingCount    int
	SuppressedCount int
}

var pitfallRuleRegistry = []PitfallRuleDefinition{
	{
		ID: PitfallInclusiveLengthIndex, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "control-flow"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallDirectIndexAtLength, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallBooleanLiteralComparison, Family: PitfallBooleanIntent,
		RequiredFacts: []string{"expression-types", "constant-values", "operator-semantics"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
}

// PitfallRules returns a defensive, deterministic snapshot of the canonical
// rule registry.
func PitfallRules() []PitfallRuleDefinition {
	result := make([]PitfallRuleDefinition, len(pitfallRuleRegistry))
	for index, rule := range pitfallRuleRegistry {
		result[index] = rule
		result[index].RequiredFacts = append([]string(nil), rule.RequiredFacts...)
	}
	return result
}

// PitfallAnalysis is immutable when returned by Analyzer.
type PitfallAnalysis struct {
	results     []PitfallFinding
	evaluations []PitfallRuleEvaluation
}

func newPitfallAnalysis() *PitfallAnalysis { return &PitfallAnalysis{} }

func (p *PitfallAnalysis) clone() *PitfallAnalysis {
	result := newPitfallAnalysis()
	if p == nil {
		return result
	}
	result.results = make([]PitfallFinding, len(p.results))
	for index, finding := range p.results {
		result.results[index] = clonePitfallFinding(finding)
	}
	result.evaluations = append([]PitfallRuleEvaluation(nil), p.evaluations...)
	return result
}

// Results includes reported and suppressed occurrences. NoFinding,
// NotEvaluated, and Pending are represented by Evaluations.
func (p *PitfallAnalysis) Results() []PitfallFinding {
	return p.clone().results
}

func (p *PitfallAnalysis) Findings() []PitfallFinding {
	result := []PitfallFinding{}
	if p == nil {
		return result
	}
	for _, finding := range p.results {
		if finding.State == PitfallStateFinding {
			result = append(result, clonePitfallFinding(finding))
		}
	}
	return result
}

func (p *PitfallAnalysis) Evaluations() []PitfallRuleEvaluation {
	if p == nil {
		return nil
	}
	return append([]PitfallRuleEvaluation(nil), p.evaluations...)
}

func clonePitfallFinding(finding PitfallFinding) PitfallFinding {
	finding.EvidenceFor = append([]PitfallEvidence(nil), finding.EvidenceFor...)
	finding.EvidenceAgainst = append([]PitfallEvidence(nil), finding.EvidenceAgainst...)
	finding.Actions = append([]PitfallSuggestedAction(nil), finding.Actions...)
	if finding.Suppression != nil {
		suppression := *finding.Suppression
		suppression.Evidence = append([]PitfallEvidence(nil), finding.Suppression.Evidence...)
		finding.Suppression = &suppression
	}
	return finding
}

type pitfallBuilder struct {
	analyzer                  *Analyzer
	result                    *PitfallAnalysis
	counts                    map[PitfallRuleID]*PitfallRuleEvaluation
	handledBooleanComparisons map[*ast.InfixExpression]bool
}

func buildPitfallAnalysis(program *ast.Program, analyzer *Analyzer) *PitfallAnalysis {
	builder := &pitfallBuilder{
		analyzer:                  analyzer,
		result:                    newPitfallAnalysis(),
		counts:                    map[PitfallRuleID]*PitfallRuleEvaluation{},
		handledBooleanComparisons: map[*ast.InfixExpression]bool{},
	}
	for _, rule := range pitfallRuleRegistry {
		evaluation := &PitfallRuleEvaluation{Rule: rule.ID, State: PitfallStateNoFinding}
		if !analysisDepthAtLeast(analyzer.analysisDepth, rule.MinimumDepth) {
			evaluation.State = PitfallStateNotEvaluated
		}
		builder.counts[rule.ID] = evaluation
	}
	if program != nil {
		for _, statement := range program.Statements {
			builder.walkStatement(statement)
		}
	}
	builder.finish()
	return builder.result
}

func analysisDepthAtLeast(actual AnalysisDepth, minimum AnalysisDepth) bool {
	rank := map[AnalysisDepth]int{AnalysisInteractive: 0, AnalysisStandard: 1, AnalysisDeep: 2}
	return rank[actual] >= rank[minimum]
}

func (b *pitfallBuilder) finish() {
	sort.SliceStable(b.result.results, func(i, j int) bool {
		left, right := b.result.results[i], b.result.results[j]
		if left.Subject.Source.File != right.Subject.Source.File {
			return left.Subject.Source.File < right.Subject.Source.File
		}
		if left.Subject.Source.Line != right.Subject.Source.Line {
			return left.Subject.Source.Line < right.Subject.Source.Line
		}
		if left.Subject.Source.Column != right.Subject.Source.Column {
			return left.Subject.Source.Column < right.Subject.Source.Column
		}
		return left.Rule < right.Rule
	})
	for _, rule := range pitfallRuleRegistry {
		if evaluation := b.counts[rule.ID]; evaluation != nil {
			if evaluation.FindingCount > 0 {
				evaluation.State = PitfallStateFinding
			} else if evaluation.SuppressedCount > 0 {
				evaluation.State = PitfallStateSuppressed
			}
			b.result.evaluations = append(b.result.evaluations, *evaluation)
		}
	}
}

func (b *pitfallBuilder) add(finding PitfallFinding) {
	evaluation := b.counts[finding.Rule]
	if evaluation == nil || evaluation.State == PitfallStateNotEvaluated {
		return
	}
	if finding.State == PitfallStateSuppressed {
		evaluation.SuppressedCount++
	} else {
		finding.State = PitfallStateFinding
		evaluation.FindingCount++
	}
	b.result.results = append(b.result.results, finding)
}

func (b *pitfallBuilder) walkStatement(statement ast.Statement) {
	if parameterUsageNodeIsNil(statement) {
		return
	}
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		b.walkBlock(statement.Body)
	case *ast.ImplStatement:
		for _, member := range statement.Members {
			switch member := member.(type) {
			case *ast.FunctionDeclaration:
				b.walkBlock(member.Body)
			case *ast.PropertyDeclaration:
				b.walkBlock(member.Getter)
				if member.Setter != nil {
					b.walkBlock(member.Setter.Body)
				}
			}
		}
	case *ast.LetStatement:
		b.walkExpression(statement.Value)
	case *ast.LetGroupStatement:
		for _, item := range statement.Lets {
			b.walkStatement(item)
		}
	case *ast.AssignmentStatement:
		b.walkExpression(statement.Target)
		b.walkExpression(statement.Value)
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
			if branch != nil {
				b.walkExpression(branch.Value)
				b.walkBlock(branch.Body)
			}
		}
	case *ast.ForStatement:
		b.inspectInclusiveLengthLoop(statement)
		b.walkExpression(statement.Iterable)
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

func (b *pitfallBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		b.walkStatement(statement)
	}
}

func (b *pitfallBuilder) walkSwitchCase(item *ast.SwitchCase) {
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

func (b *pitfallBuilder) walkExpression(expression ast.Expression) {
	if parameterUsageNodeIsNil(expression) {
		return
	}
	if index, ok := expression.(*ast.IndexExpression); ok {
		b.inspectDirectIndexAtLength(index)
	}
	if conversion, ok := expression.(*ast.ConversionExpression); ok {
		b.inspectBooleanConversion(conversion, conversion.Value, conversion.Type != nil && conversion.Type.Name == "bool")
	}
	if call, ok := expression.(*ast.CallExpression); ok {
		callee, isIdentifier := call.Callee.(*ast.Identifier)
		b.inspectBooleanConversion(call, firstPitfallArgument(call.Arguments), isIdentifier && callee.Value == "bool" && len(call.Arguments) == 1)
	}
	if comparison, ok := expression.(*ast.InfixExpression); ok && !b.handledBooleanComparisons[comparison] {
		b.inspectBooleanLiteralComparison(comparison, comparison)
	}
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
		b.walkExpression(expression.Object)
	case *ast.IndexExpression:
		b.walkExpression(expression.Left)
		b.walkExpression(expression.Index)
	case *ast.SliceExpression:
		b.walkExpression(expression.Left)
		b.walkExpression(expression.Start)
		b.walkExpression(expression.End)
	case *ast.RefExpression:
		b.walkExpression(expression.Value)
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
		b.walkExpression(expression.Callee)
		for _, argument := range expression.Arguments {
			b.walkExpression(argument)
		}
	case *ast.RuntimeCallExpression:
		for _, argument := range expression.Arguments {
			b.walkExpression(argument)
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

type booleanComparisonIntent struct {
	booleanExpression ast.Expression
	literalToken      lexer.Token
	literalValue      bool
	numericLiteral    bool
	replacement       string
}

func booleanLiteralComparisonIntent(comparison *ast.InfixExpression, leftType, rightType Type) (booleanComparisonIntent, bool) {
	if comparison == nil || (comparison.Operator != "==" && comparison.Operator != "!=") {
		return booleanComparisonIntent{}, false
	}

	booleanExpression, literalExpression, numericLiteral, literalValue, ok := booleanComparisonOperands(
		comparison.Left, leftType, comparison.Right, rightType,
	)
	if !ok {
		booleanExpression, literalExpression, numericLiteral, literalValue, ok = booleanComparisonOperands(
			comparison.Right, rightType, comparison.Left, leftType,
		)
	}
	if !ok {
		return booleanComparisonIntent{}, false
	}

	resultMatchesOperand := (comparison.Operator == "==") == literalValue
	replacement := booleanExpression.String()
	if !resultMatchesOperand {
		replacement = negateBooleanExpression(booleanExpression)
	}
	return booleanComparisonIntent{
		booleanExpression: booleanExpression,
		literalToken:      expressionToken(literalExpression),
		literalValue:      literalValue,
		numericLiteral:    numericLiteral,
		replacement:       replacement,
	}, true
}

func booleanComparisonOperands(booleanExpression ast.Expression, booleanType Type, literalExpression ast.Expression, literalType Type) (ast.Expression, ast.Expression, bool, bool, bool) {
	if booleanType.Kind != BoolType {
		return nil, nil, false, false, false
	}
	if literal, ok := literalExpression.(*ast.BooleanLiteral); ok {
		// A literal-to-literal comparison has no useful subject to simplify.
		if _, subjectIsLiteral := booleanExpression.(*ast.BooleanLiteral); subjectIsLiteral {
			return nil, nil, false, false, false
		}
		return booleanExpression, literalExpression, false, literal.Value, true
	}
	if literalType.Kind == BoolType {
		return nil, nil, false, false, false
	}
	value, ok := constantIntegerValue(literalExpression)
	if !ok || !value.IsInt64() || (value.Int64() != 0 && value.Int64() != 1) {
		return nil, nil, false, false, false
	}
	return booleanExpression, literalExpression, true, value.Sign() != 0, true
}

func negateBooleanExpression(expression ast.Expression) string {
	switch expression.(type) {
	case *ast.Identifier, *ast.MemberExpression:
		return "!" + expression.String()
	default:
		return "!(" + expression.String() + ")"
	}
}

func firstPitfallArgument(arguments []ast.Expression) ast.Expression {
	if len(arguments) == 0 {
		return nil
	}
	return arguments[0]
}

func (b *pitfallBuilder) inspectBooleanConversion(subject ast.Expression, value ast.Expression, isBoolConversion bool) {
	if !isBoolConversion {
		return
	}
	comparison, ok := value.(*ast.InfixExpression)
	if !ok {
		return
	}
	if b.inspectBooleanLiteralComparison(comparison, subject) {
		b.handledBooleanComparisons[comparison] = true
	}
}

func (b *pitfallBuilder) inspectBooleanLiteralComparison(comparison *ast.InfixExpression, subject ast.Expression) bool {
	leftType, leftOK := b.analyzer.expressionTypes[comparison.Left]
	rightType, rightOK := b.analyzer.expressionTypes[comparison.Right]
	if !leftOK || !rightOK {
		return false
	}
	intent, ok := booleanLiteralComparisonIntent(comparison, leftType, rightType)
	if !ok {
		return false
	}

	classification := PitfallLikelyMistake
	confidence := PitfallConfidenceProven
	actionKind := PitfallProvenFix
	truthSpelling := strconv.FormatBool(intent.literalValue)
	evidence := []PitfallEvidence{
		{Strength: PitfallEvidenceProof, Fact: "the compared expression has type bool", Source: expressionToken(intent.booleanExpression)},
		{Strength: PitfallEvidenceProof, Fact: "the comparison operand represents " + truthSpelling, Source: intent.literalToken},
	}
	if intent.numericLiteral {
		classification = PitfallProvenInvalid
		actionKind = PitfallSuggestedEdit
		evidence = append(evidence, PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "bool and integer equality operands are not type-compatible",
			Source:   comparison.Token,
		})
	}
	if subject != comparison {
		evidence = append(evidence, PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "converting a boolean comparison result to bool cannot change it",
			Source:   expressionToken(subject),
		})
	}

	b.add(PitfallFinding{
		Rule:           PitfallBooleanLiteralComparison,
		Family:         PitfallBooleanIntent,
		Classification: classification,
		Confidence:     confidence,
		Subject:        PitfallSubject{Expression: subject.String(), Source: expressionToken(subject)},
		EvidenceFor:    evidence,
		OwningRule:     "equality-type-compatibility",
		Actions: []PitfallSuggestedAction{{
			Kind:        actionKind,
			Title:       "use the boolean value directly",
			Replacement: intent.replacement,
			Source:      expressionToken(subject),
		}},
	})
	return true
}

func (b *pitfallBuilder) walkTryHandlers(handlers []*ast.TryHandler) {
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		b.walkExpression(handler.Pattern)
		b.walkExpression(handler.Body)
		if handler.ReturnBody != nil {
			b.walkStatement(handler.ReturnBody)
		}
		b.walkBlock(handler.BlockBody)
	}
}

func (b *pitfallBuilder) walkMatch(expression *ast.MatchExpression) {
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
		if arm.ReturnBody != nil {
			b.walkStatement(arm.ReturnBody)
		}
		b.walkBlock(arm.BlockBody)
	}
}

func (b *pitfallBuilder) inspectDirectIndexAtLength(index *ast.IndexExpression) {
	lengthCollection, lengthToken, ok := b.lengthReceiver(index.Index)
	if !ok {
		return
	}
	indexedCollection, ok := b.expressionIdentity(index.Left)
	if !ok || indexedCollection != lengthCollection {
		return
	}
	b.add(PitfallFinding{
		Rule: PitfallDirectIndexAtLength, Family: PitfallBoundsAndRanges,
		Classification: PitfallProvenInvalid, Confidence: PitfallConfidenceProven,
		Subject: PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the index is the Len of the same collection", Source: lengthToken},
			{Strength: PitfallEvidenceProof, Fact: "Len is one past the final zero-based index", Source: index.Token},
		},
		OwningRule: "bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "use an index strictly below Len", Source: index.Token,
		}},
	})
}

func (b *pitfallBuilder) inspectInclusiveLengthLoop(loop *ast.ForStatement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || loop.Body == nil || rangeExpression.Exclusive || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	collection, lengthToken, ok := b.lengthReceiver(rangeExpression.End)
	if !ok {
		return
	}
	binding := loop.Bindings[0]
	guarded := false
	for _, statement := range loop.Body.Statements {
		if b.endpointBreakGuard(statement, binding.Token, collection) {
			guarded = true
		}
		for _, index := range indexesInStatement(statement) {
			indexedCollection, sameCollection := b.expressionIdentity(index.Left)
			if !sameCollection || indexedCollection != collection || !b.expressionUsesBinding(index.Index, binding.Token) {
				continue
			}
			finding := PitfallFinding{
				Rule: PitfallInclusiveLengthIndex, Family: PitfallBoundsAndRanges,
				Classification: PitfallProvenInvalid, Confidence: PitfallConfidenceProven,
				Subject: PitfallSubject{Expression: index.String(), Source: index.Token},
				EvidenceFor: []PitfallEvidence{
					{Strength: PitfallEvidenceProof, Fact: "the inclusive range reaches the collection Len", Source: rangeExpression.Token},
					{Strength: PitfallEvidenceProof, Fact: "the same loop binding indexes the same collection", Source: index.Token},
				},
				OwningRule: "bounds",
				Actions: []PitfallSuggestedAction{{
					Kind: PitfallSuggestedEdit, Title: "use the canonical half-open range", Replacement: "..<", Source: rangeExpression.Token,
				}},
			}
			if guarded {
				evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "a preceding endpoint guard exits before the indexed access", Source: lengthToken}
				finding.State = PitfallStateSuppressed
				finding.EvidenceAgainst = []PitfallEvidence{evidence}
				finding.Suppression = &PitfallSuppression{Reason: "the indexed access is unreachable when the loop binding equals Len", Evidence: []PitfallEvidence{evidence}}
			}
			b.add(finding)
		}
	}
}

func (b *pitfallBuilder) endpointBreakGuard(statement ast.Statement, binding lexer.Token, collection string) bool {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok || !pitfallBlockDefinitelyExits(conditional.Consequence) {
		return false
	}
	comparison, ok := conditional.Condition.(*ast.InfixExpression)
	if !ok || comparison.Operator != "==" {
		return false
	}
	return b.bindingAndLengthComparison(comparison.Left, comparison.Right, binding, collection) ||
		b.bindingAndLengthComparison(comparison.Right, comparison.Left, binding, collection)
}

func pitfallBlockDefinitelyExits(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.Statements {
		switch statement := statement.(type) {
		case *ast.BreakStatement, *ast.ReturnStatement:
			return true
		case *ast.IfStatement:
			if statement.Alternative != nil && pitfallBlockDefinitelyExits(statement.Consequence) && pitfallBlockDefinitelyExits(statement.Alternative) {
				return true
			}
		}
	}
	return false
}

func (b *pitfallBuilder) bindingAndLengthComparison(bindingExpression ast.Expression, lengthExpression ast.Expression, binding lexer.Token, collection string) bool {
	if !b.expressionUsesBinding(bindingExpression, binding) {
		return false
	}
	lengthCollection, _, ok := b.lengthReceiver(lengthExpression)
	return ok && lengthCollection == collection
}

func (b *pitfallBuilder) lengthReceiver(expression ast.Expression) (string, lexer.Token, bool) {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return "", lexer.Token{}, false
	}
	known, resolved := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !resolved || known.Kind != CompilerKnownProperty || known.Name != "Len" {
		return "", lexer.Token{}, false
	}
	identity, ok := b.expressionIdentity(member.Object)
	return identity, member.Property.Token, ok
}

func (b *pitfallBuilder) expressionIdentity(expression ast.Expression) (string, bool) {
	switch expression := expression.(type) {
	case *ast.Identifier:
		definitions := b.analyzer.definitionTokens[sourceTokenLocation(expression.Token)]
		if len(definitions) != 1 {
			return "", false
		}
		definition := definitions[0]
		return strings.Join([]string{definition.File, strconv.Itoa(definition.Line), strconv.Itoa(definition.Column)}, ":"), true
	case *ast.MemberExpression:
		base, ok := b.expressionIdentity(expression.Object)
		if !ok || expression.Property == nil {
			return "", false
		}
		return base + "." + expression.Property.Value, true
	default:
		return "", false
	}
}

func (b *pitfallBuilder) expressionUsesBinding(expression ast.Expression, binding lexer.Token) bool {
	identifier, ok := expression.(*ast.Identifier)
	if !ok {
		return false
	}
	definitions := b.analyzer.definitionTokens[sourceTokenLocation(identifier.Token)]
	return len(definitions) == 1 && sourceTokenLocation(definitions[0]) == sourceTokenLocation(binding)
}

func indexesInStatement(statement ast.Statement) []*ast.IndexExpression {
	result := []*ast.IndexExpression{}
	visitStatementExpressions(statement, func(expression ast.Expression) {
		if index, ok := expression.(*ast.IndexExpression); ok {
			result = append(result, index)
		}
	})
	return result
}

func visitStatementExpressions(statement ast.Statement, visit func(ast.Expression)) {
	if statement == nil {
		return
	}
	var walkExpression func(ast.Expression)
	var walkBlock func(*ast.BlockStatement)
	walkExpression = func(expression ast.Expression) {
		if parameterUsageNodeIsNil(expression) {
			return
		}
		visit(expression)
		switch expression := expression.(type) {
		case *ast.PrefixExpression:
			walkExpression(expression.Right)
		case *ast.InfixExpression:
			walkExpression(expression.Left)
			walkExpression(expression.Right)
		case *ast.RangeExpression:
			walkExpression(expression.Start)
			walkExpression(expression.End)
		case *ast.ConversionExpression:
			walkExpression(expression.Value)
		case *ast.MemberExpression:
			walkExpression(expression.Object)
		case *ast.IndexExpression:
			walkExpression(expression.Left)
			walkExpression(expression.Index)
		case *ast.SliceExpression:
			walkExpression(expression.Left)
			walkExpression(expression.Start)
			walkExpression(expression.End)
		case *ast.RefExpression:
			walkExpression(expression.Value)
		case *ast.ArrayLiteral:
			for _, item := range expression.Elements {
				walkExpression(item)
			}
		case *ast.SpreadExpression:
			walkExpression(expression.Value)
		case *ast.StructLiteral:
			for _, field := range expression.Fields {
				walkExpression(field.Value)
			}
		case *ast.CallExpression:
			walkExpression(expression.Callee)
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.RuntimeCallExpression:
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.OkExpression:
			walkExpression(expression.Value)
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.ErrExpression:
			walkExpression(expression.Value)
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.TryExpression:
			walkExpression(expression.Expression)
		case *ast.MatchExpression:
			walkExpression(expression.Subject)
			for _, arm := range expression.Arms {
				if arm != nil {
					walkExpression(arm.Guard)
					walkExpression(arm.Body)
					walkBlock(arm.BlockBody)
				}
			}
		case *ast.LambdaExpression:
			walkBlock(expression.Body)
		case *ast.SpawnExpression:
			walkExpression(expression.Value)
			walkBlock(expression.Body)
		case *ast.AwaitExpression:
			walkExpression(expression.Value)
		}
	}
	walkBlock = func(block *ast.BlockStatement) {
		if block == nil {
			return
		}
		for _, nested := range block.Statements {
			visitStatementExpressions(nested, visit)
		}
	}
	switch statement := statement.(type) {
	case *ast.LetStatement:
		walkExpression(statement.Value)
	case *ast.LetGroupStatement:
		for _, item := range statement.Lets {
			visitStatementExpressions(item, visit)
		}
	case *ast.AssignmentStatement:
		walkExpression(statement.Target)
		walkExpression(statement.Value)
	case *ast.TryAssignmentStatement:
		visitStatementExpressions(statement.Assignment, visit)
	case *ast.ExpressionStatement:
		walkExpression(statement.Expression)
	case *ast.DiscardStatement:
		walkExpression(statement.Value)
	case *ast.AssertStatement:
		walkExpression(statement.Condition)
	case *ast.DetachStatement:
		walkExpression(statement.Value)
	case *ast.ReturnStatement:
		walkExpression(statement.Value)
	case *ast.IfStatement:
		walkExpression(statement.Condition)
		walkBlock(statement.Consequence)
		walkBlock(statement.Alternative)
	case *ast.SwitchStatement:
		walkExpression(statement.Subject)
		for _, item := range statement.Cases {
			if item == nil {
				continue
			}
			for _, candidate := range item.Items {
				switch candidate := candidate.(type) {
				case *ast.SwitchValueCase:
					walkExpression(candidate.Value)
				case *ast.SwitchRangeCase:
					walkExpression(candidate.Range)
				case *ast.SwitchRelationalCase:
					walkExpression(candidate.Value)
				}
			}
			walkBlock(item.Body)
		}
		if statement.Default != nil {
			walkBlock(statement.Default.Body)
		}
	case *ast.SelectStatement:
		for _, branch := range statement.Branches {
			if branch != nil {
				walkExpression(branch.Value)
				walkBlock(branch.Body)
			}
		}
	case *ast.ForStatement:
		walkExpression(statement.Iterable)
		walkExpression(statement.Step)
		walkBlock(statement.Body)
	case *ast.WhileStatement:
		walkExpression(statement.Condition)
		walkBlock(statement.Body)
	case *ast.DeferStatement:
		walkBlock(statement.Body)
	case *ast.UnsafeStatement:
		walkBlock(statement.Body)
	case *ast.MatchStatement:
		walkExpression(statement.Match)
	}
}
