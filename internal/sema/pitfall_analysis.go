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
	PitfallInclusiveLengthIndex       PitfallRuleID = "pitfall.bounds.inclusive-length-index"
	PitfallDirectIndexAtLength        PitfallRuleID = "pitfall.bounds.direct-index-at-length"
	PitfallBooleanLiteralComparison   PitfallRuleID = "pitfall.boolean.redundant-literal-comparison"
	PitfallExplicitSelfMethodArgument PitfallRuleID = "pitfall.api.explicit-self-method-argument"
	PitfallIneffectiveLengthGuard     PitfallRuleID = "pitfall.bounds.ineffective-length-guard"
	PitfallUpperNeighborIndex         PitfallRuleID = "pitfall.bounds.upper-neighbor-index"
	PitfallLowerNeighborIndex         PitfallRuleID = "pitfall.bounds.lower-neighbor-index"
	PitfallFinalElementNeedsNonEmpty  PitfallRuleID = "pitfall.bounds.final-element-needs-nonempty"
	PitfallSkippedFirstElement        PitfallRuleID = "pitfall.bounds.skipped-zero"
	PitfallTautologicalInterval       PitfallRuleID = "pitfall.range.tautological-interval"
	PitfallImpossibleInterval         PitfallRuleID = "pitfall.range.impossible-interval"
	PitfallRangeMembershipIdiom       PitfallRuleID = "pitfall.range.membership-idiom"
	PitfallWrongGuardSubject          PitfallRuleID = "pitfall.control-flow.wrong-guard-subject"
	PitfallCheckWithoutTransfer       PitfallRuleID = "pitfall.control-flow.check-without-transfer"
	PitfallIndexedStructuralMutation  PitfallRuleID = "pitfall.iteration.structural-mutation-in-indexed-loop"
	PitfallCapacityAsLength           PitfallRuleID = "pitfall.bounds.capacity-as-length"
	PitfallOmittedLastElement         PitfallRuleID = "pitfall.range.omitted-last-element"
	PitfallFragileInclusiveLength     PitfallRuleID = "pitfall.range.fragile-inclusive-length"
	PitfallWrongBoundSource           PitfallRuleID = "pitfall.collection.wrong-bound-source"
	PitfallMeaninglessComparison      PitfallRuleID = "pitfall.range.meaningless-comparison"
	PitfallForeignExtentUnit          PitfallRuleID = "pitfall.ffi.extent-unit-mismatch"
	PitfallForeignExtentOrigin        PitfallRuleID = "pitfall.ffi.pointer-extent-origin-mismatch"
	PitfallWrongStateSubject          PitfallRuleID = "pitfall.state.wrong-checked-subject"
)

type PitfallFamily string

const (
	PitfallBoundsAndRanges     PitfallFamily = "bounds-and-ranges"
	PitfallBooleanIntent       PitfallFamily = "boolean-intent"
	PitfallAPIUsage            PitfallFamily = "api-usage"
	PitfallControlFlow         PitfallFamily = "control-flow"
	PitfallIterationMutation   PitfallFamily = "iteration-and-mutation"
	PitfallCollectionRelations PitfallFamily = "collection-relations"
	PitfallForeignContracts    PitfallFamily = "ffi-contracts"
	PitfallOptionResultFlow    PitfallFamily = "option-result-error-flow"
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
	Idiom       PitfallCanonicalIdiom
	Safety      PitfallFixSafety
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
	Rule PitfallRuleID
	// DiagnosticID identifies the coalesced mandatory owner, independently of analysis rule identity.
	DiagnosticID    string
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
	// Incomplete preserves partial coverage even when another unit produced a finding.
	Incomplete bool
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
	{
		ID: PitfallExplicitSelfMethodArgument, Family: PitfallAPIUsage,
		RequiredFacts: []string{"resolved-calls", "receiver-semantics", "argument-bindings"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallIneffectiveLengthGuard, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "control-flow", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallUpperNeighborIndex, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "constant-values", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallLowerNeighborIndex, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "constant-values", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallFinalElementNeedsNonEmpty, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "constant-values", "control-flow", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallSkippedFirstElement, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "constant-values", "control-flow"},
		MinimumDepth:  AnalysisStandard, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallTautologicalInterval, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "expression-types", "constant-values", "operator-semantics"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallImpossibleInterval, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "expression-types", "constant-values", "operator-semantics"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallRangeMembershipIdiom, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "expression-types", "constant-values", "operator-semantics", "range-domain"},
		MinimumDepth:  AnalysisDeep, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallWrongGuardSubject, Family: PitfallControlFlow,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "control-flow", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallCheckWithoutTransfer, Family: PitfallControlFlow,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "control-flow", "bounds"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallOmittedLastElement, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "constant-values", "control-flow"},
		MinimumDepth:  AnalysisStandard, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallFragileInclusiveLength, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "constant-values", "control-flow"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallCapacityAsLength, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "control-flow", "operation-contracts"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallMeaninglessComparison, Family: PitfallBoundsAndRanges,
		RequiredFacts: []string{"expression-types", "range-contracts", "constant-values", "operator-semantics"},
		MinimumDepth:  AnalysisDeep, DefaultConfidence: PitfallConfidenceProven,
	},
	{
		ID: PitfallWrongBoundSource, Family: PitfallCollectionRelations,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "control-flow", "operation-contracts"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{
		ID: PitfallIndexedStructuralMutation, Family: PitfallIterationMutation,
		RequiredFacts: []string{"resolved-bindings", "compiler-known-members", "range-domain", "control-flow", "operation-contracts"},
		MinimumDepth:  AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh,
	},
	{ID: PitfallWrongStateSubject, Family: PitfallOptionResultFlow, RequiredFacts: []string{"resolved-bindings", "state-requirements", "control-flow", "option-result-projections"}, MinimumDepth: AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh},
	{ID: PitfallForeignExtentOrigin, Family: PitfallForeignContracts, RequiredFacts: []string{"foreign-buffer-extent-contracts", "place-provenance", "compiler-known-members"}, MinimumDepth: AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh},
	{ID: PitfallForeignExtentUnit, Family: PitfallForeignContracts, RequiredFacts: []string{"foreign-buffer-extent-contracts", "extent-quantities", "resolved-element-layout"}, MinimumDepth: AnalysisInteractive, DefaultConfidence: PitfallConfidenceHigh},
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
	results             []PitfallFinding
	evaluations         []PitfallRuleEvaluation
	coverage            PitfallCoverage
	foreignExtentInputs []ResolvedForeignBufferExtent
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
	result.coverage = p.coverage
	result.foreignExtentInputs = p.ForeignExtentInputs()
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
	boundsOnly                bool
	result                    *PitfallAnalysis
	counts                    map[PitfallRuleID]*PitfallRuleEvaluation
	handledBooleanComparisons map[*ast.InfixExpression]bool
	// handledIntervalChains marks inner links of an inspected `&&`/`||` chain.
	handledIntervalChains    map[*ast.InfixExpression]bool
	activeNonEmptyProofs     map[string]lexer.Token
	activeIndexGuards        []pitfallIndexGuard
	activeCapacityEqualities map[string]lexer.Token
	activeLengthConditions   []ast.Expression // enclosing true if conditions, for Len relations
	activePreceding          []ast.Statement  // statements before the walked one in its block
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

// add publishes a supported finding after requiring explicit automatic-fix safety
// and preferring supported canonical idiom suggestions.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Analysis states", "Canonical idiom guidance".
func (b *pitfallBuilder) add(finding PitfallFinding) {
	requirePitfallFixSafety(&finding)
	preferPitfallCanonicalIdioms(&finding)
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
	if b.boundsOnly {
		b.analyzer.attachLengthBoundsDiagnostic(&finding)
	} else {
		b.analyzer.coalescePitfallDiagnostic(&finding, finding.Subject.Source)
	}
	b.result.results = append(b.result.results, finding)
}

// walkStatement consumes canonical body facts in required-bounds or optional
// insight mode; required bounds never run optional operation recognizers.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis",
// "Proven invalidity is not a warning", "Reachability".
func (b *pitfallBuilder) walkStatement(statement ast.Statement) {
	if parameterUsageNodeIsNil(statement) {
		return
	}
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		b.walkBlock(statement.Body)
	case *ast.TestDeclaration:
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
		b.walkIfStatement(statement)
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
		if !b.boundsOnly {
			b.inspectNeighborIndexes(statement)
			b.inspectIndexedStructuralMutation(statement)
		}
		b.walkExpression(statement.Iterable)
		b.walkExpression(statement.Step)
		var guards []pitfallIndexGuard
		if guard, ok := b.loopIndexGuard(statement); ok && b.guardSurvivesStatement(guard, statement) {
			guards = append(guards, guard)
		}
		b.withLoopIndexGuards(statement, guards, func() { b.walkBlock(statement.Body) })
	case *ast.WhileStatement:
		b.walkExpression(statement.Condition)
		if b.boundsOnly {
			if flow, ok := b.analyzer.ResolvedWhileFlowOf(statement); ok && flow.ConditionKnown && !flow.ConditionValue {
				return
			}
		}
		b.withLoopIndexGuards(statement, b.conditionIndexGuards(statement.Condition), func() { b.walkBlock(statement.Body) })
	case *ast.DeferStatement:
		b.walkIndependentGuardBody(statement.Body)
	case *ast.UnsafeStatement:
		b.walkBlock(statement.Body)
	case *ast.MatchStatement:
		b.walkMatch(statement.Match)
	}
}

// walkBlock preserves owning reachability for bounds proofs while retaining
// contextual guard and intent witnesses for optional pitfall findings.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Evidence model",
// "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	if b.boundsOnly {
		for _, statement := range block.Statements {
			b.walkStatement(statement)
			if !b.analyzer.statementCanFallThrough(statement) {
				break
			}
		}
		return
	}
	b.inspectIneffectiveRejectionGuards(block)
	b.inspectCheckWithoutTransfer(block)
	inheritedProofs := b.activeNonEmptyProofs
	inheritedPreceding := b.activePreceding
	inheritedGuards := b.activeIndexGuards
	for index, statement := range block.Statements {
		b.activeNonEmptyProofs = nil
		// The proof that holds on entry to this statement: inherited by the
		// block's first statement, or established by a directly preceding
		// empty-collection exit guard.
		var entryProofs map[string]lexer.Token
		if index == 0 {
			entryProofs = clonePitfallNonEmptyProofs(inheritedProofs)
		} else if collection, proof, ok := b.emptyCollectionExitGuard(block.Statements[index-1]); ok {
			entryProofs = map[string]lexer.Token{collection: proof}
		}
		if pitfallStraightLineStatement(statement) {
			b.activeNonEmptyProofs = entryProofs
		}
		if loop, ok := statement.(*ast.ForStatement); ok {
			b.inspectSkippedFirstElement(loop, block.Statements[:index])
			b.inspectCapacityAsLength(loop, block.Statements[:index])
			b.inspectWrongBoundSource(loop, block.Statements[:index])
			b.inspectLengthEndpointIntent(loop, block.Statements, index, entryProofs)
		}
		b.activePreceding = block.Statements[:index]
		b.walkStatement(statement)
		b.activeIndexGuards = b.survivingIndexGuards(b.activeIndexGuards, statement)
		b.activeIndexGuards = append(b.activeIndexGuards, b.continuationIndexGuards(statement)...)
		if !b.analyzer.statementCanFallThrough(statement) {
			break
		}
	}
	b.activeIndexGuards = inheritedGuards
	b.activeNonEmptyProofs = inheritedProofs
	b.activePreceding = inheritedPreceding
}

// clonePitfallNonEmptyProofs keeps branch-local proof state isolated across
// recursive block traversal.
func clonePitfallNonEmptyProofs(proofs map[string]lexer.Token) map[string]lexer.Token {
	if len(proofs) == 0 {
		return nil
	}
	result := make(map[string]lexer.Token, len(proofs))
	for collection, token := range proofs {
		result[collection] = token
	}
	return result
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

// walkExpression follows evaluated operations, sharing owning bounds facts
// with optional consumers and honoring constant short-circuit paths for proofs.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Diagnostic ownership and coalescing";
// rules/foundations/operators.md — "Logical operators".
func (b *pitfallBuilder) walkExpression(expression ast.Expression) {
	if parameterUsageNodeIsNil(expression) {
		return
	}
	if index, ok := expression.(*ast.IndexExpression); ok {
		b.inspectDirectIndexAtLength(index)
		if !b.boundsOnly {
			b.inspectFinalElementAccess(index)
			b.inspectDirectCapacityIndex(index)
		}
	}
	if !b.boundsOnly {
		b.inspectStateCorrelation(expression)
		if conversion, ok := expression.(*ast.ConversionExpression); ok {
			b.inspectBooleanConversion(conversion, conversion.Value, conversion.Type != nil && conversion.Type.Name == "bool")
		}
		if call, ok := expression.(*ast.CallExpression); ok {
			callee, isIdentifier := call.Callee.(*ast.Identifier)
			b.inspectBooleanConversion(call, firstPitfallArgument(call.Arguments), isIdentifier && callee.Value == "bool" && len(call.Arguments) == 1)
			b.inspectExplicitSelfMethodArgument(call)
			b.consumeForeignBufferExtents(call)
		}
		if comparison, ok := expression.(*ast.InfixExpression); ok && !b.handledBooleanComparisons[comparison] {
			b.inspectBooleanLiteralComparison(comparison, comparison)
		}
		if condition, ok := expression.(*ast.InfixExpression); ok {
			b.inspectIntervalCondition(condition)
		}
		if comparison, ok := expression.(*ast.InfixExpression); ok {
			b.inspectMeaninglessComparison(comparison)
		}
	}
	switch expression := expression.(type) {
	case *ast.PrefixExpression:
		b.walkExpression(expression.Right)
	case *ast.InfixExpression:
		b.walkExpression(expression.Left)
		if b.boundsOnly {
			if value, known := b.analyzer.constantBooleanValue(expression.Left); known && (expression.Operator == "&&" && !value || expression.Operator == "||" && value) {
				return
			}
		}
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
		b.walkIndependentGuardBody(expression.Body)
	case *ast.SpawnExpression:
		b.walkExpression(expression.Value)
		b.walkIndependentGuardBody(expression.Body)
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

// inspectExplicitSelfMethodArgument recognizes self.Method(self), where the
// receiver already supplies self implicitly. It is deliberately limited to a
// Sema-resolved instance method so ordinary functions, static members, and
// calls on another receiver retain their ordinary argument semantics.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Explicit self passed to its own method"
//   - rules/declarations/functions.md — implicit instance receiver
func (b *pitfallBuilder) inspectExplicitSelfMethodArgument(call *ast.CallExpression) {
	if call == nil {
		return
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || !isSelfIdentifier(member.Object) {
		return
	}
	resolved, ok := b.analyzer.ResolvedCallTarget(call)
	if !ok || resolved.Function.Static {
		return
	}
	for _, argument := range call.Arguments {
		if !isSelfIdentifier(argument) {
			continue
		}
		b.add(PitfallFinding{
			Rule:           PitfallExplicitSelfMethodArgument,
			Family:         PitfallAPIUsage,
			Classification: PitfallLikelyMistake,
			Confidence:     PitfallConfidenceHigh,
			Subject:        PitfallSubject{Expression: call.String(), Source: expressionToken(argument)},
			EvidenceFor: []PitfallEvidence{
				{Strength: PitfallEvidenceProof, Fact: "the call resolves to an instance method", Source: call.Token},
				{Strength: PitfallEvidenceProof, Fact: "self is already supplied as the implicit method receiver", Source: expressionToken(member.Object)},
				{Strength: PitfallEvidenceStrong, Fact: "self is also passed explicitly as an argument", Source: expressionToken(argument)},
			},
			OwningRule: "method-receiver-and-argument-semantics",
		})
		return
	}
}

func isSelfIdentifier(expression ast.Expression) bool {
	identifier, ok := expression.(*ast.Identifier)
	return ok && identifier != nil && identifier.Value == "self"
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

	b.addForDiagnosticRoot(PitfallFinding{
		Rule:           PitfallBooleanLiteralComparison,
		Family:         PitfallBooleanIntent,
		Classification: classification,
		Confidence:     confidence,
		Subject:        PitfallSubject{Expression: subject.String(), Source: expressionToken(subject)},
		EvidenceFor:    evidence,
		OwningRule:     "equality-type-compatibility",
		Actions: []PitfallSuggestedAction{{
			Kind:        actionKind,
			Safety:      booleanFixSafety(intent),
			Title:       "use the boolean value directly",
			Replacement: intent.replacement,
			Source:      expressionToken(subject),
		}},
	}, comparison)
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

// inspectDirectIndexAtLength produces one canonical bounds proof or consumes
// that same proof as insight; repeated getters cannot establish collection identity.
// Rules: rules/analysis/pitfall_analysis.md — "Direct index at length",
// "Semantic correlation, not spelling heuristics", "Diagnostic ownership and coalescing".
func (b *pitfallBuilder) inspectDirectIndexAtLength(index *ast.IndexExpression) {
	if !b.boundsOnly {
		b.consumeLengthBoundsFacts(index.Token)
		return
	}
	before := len(b.result.results)
	defer func() {
		b.analyzer.boundsFindings[sourceTokenLocation(index.Token)] = append([]PitfallFinding(nil), b.result.results[before:]...)
	}()
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

// pitfallBlockDefinitelyExits recognizes an unconditional transfer away from
// the current loop-body remainder, including continue to the next iteration.
// Nested conditionals count only when both arms transfer.
//
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate
// in pitfall reasoning"; rules/control-flow/flowcontrol_while.md — §14 "continue".
func pitfallBlockDefinitelyExits(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.Statements {
		switch statement := statement.(type) {
		case *ast.BreakStatement, *ast.ContinueStatement, *ast.ReturnStatement:
			return true
		case *ast.IfStatement:
			if statement.Alternative != nil && pitfallBlockDefinitelyExits(statement.Consequence) && pitfallBlockDefinitelyExits(statement.Alternative) {
				return true
			}
		}
	}
	return false
}

// lengthReceiver requires compiler-owned Len and stable stored collection
// reads for owning proofs; a property getter spelling cannot prove identity.
// Rules: rules/analysis/pitfall_analysis.md — "Semantic recognition, not syntax matching",
// "Canonical facts consumed by pitfall analysis", "Direct index at length".
func (b *pitfallBuilder) lengthReceiver(expression ast.Expression) (string, lexer.Token, bool) {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return "", lexer.Token{}, false
	}
	if b.boundsOnly && !b.pitfallPureOperand(member.Object) {
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
