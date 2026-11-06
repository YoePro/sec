package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sort"
)

// analyzeBlockStatements analyzes a lexical block and reports the first
// statement that cannot execute after a preceding terminating statement.
//
// Rules: rules/tooling/diagnostics.md — §21(3)–(5) (S3001);
// rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code";
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,39 (lexical cleanup identity).
func (a *Analyzer) analyzeBlockStatements(block *ast.BlockStatement) {
	a.analyzeBlockStatementsWithRecovery(block, nil)
}

// analyzeBlockStatementsWithRecovery checks a block and optionally analyzes its
// final recovery expression before releasing block-local holders and cleanups.
// The full statement frame remains visible to reference liveness analysis.
// Rules: rules/errors/errorhandling.md — §21.1; rules/memory/borrowing.md — §5.5;
// rules/memory/destruction.md — §6.1.
func (a *Analyzer) analyzeBlockStatementsWithRecovery(block *ast.BlockStatement, recovery func()) {
	if block == nil {
		return
	}
	defer a.enterCleanupScope(block)()

	symbolsBefore := map[string]bool{}
	for name := range a.symbols.all() {
		symbolsBefore[name] = true
	}
	a.scopeDepth++
	refinementCount := len(a.activeConditionFacts)
	defer func() {
		a.activeConditionFacts = a.activeConditionFacts[:refinementCount]
		newNames := make([]string, 0)
		for name := range a.symbols.all() {
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
	a.statementFrames = append(a.statementFrames, statementFrame{block: block, depth: a.scopeDepth})
	frame := len(a.statementFrames) - 1
	defer func() { a.statementFrames = a.statementFrames[:frame] }()
	statements := block.Statements
	if recovery != nil {
		statements = statements[:len(statements)-1]
	}
	for index, stmt := range statements {
		if unreachable {
			a.addErrorAtTokenWithMetadata(statementToken(stmt), diagnostics.UnreachableStatement,
				"A preceding statement ends this block on every path. Remove this statement or change the control flow.",
				"unreachable statement")
			break
		}
		a.statementFrames[frame].index = index
		a.releaseDeadReferenceHolders(statementToken(stmt))
		a.analyzeStatement(stmt)
		if a.statementTerminatesBlock(stmt) {
			unreachable = true
		}
	}
	if recovery != nil {
		recovery()
	}
}
