package sema

import (
	"sort"
	"strconv"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// executionRequirementKind is the current-execution context an operation
// needs (cancellation.md §§ 19–23).
type executionRequirementKind string

const (
	// requiresLogicalTask: Task.Current() and Task.Yield() need a real
	// logical task; a physical thread does not satisfy it (§ 23).
	requiresLogicalTask executionRequirementKind = "logical-task"
	// requiresCancellableExecution: `cancel` needs a task or an explicit
	// cancellable thread callable (§§ 14, 19).
	requiresCancellableExecution executionRequirementKind = "cancellable-execution"
)

type executionRequirementSite struct {
	Kind      executionRequirementKind
	Operation string
	Source    lexer.Token
}

// recordExecutionRequirement notes that the current callable directly
// performs an operation needing a current-execution context. The requirement
// is compiler-inferred and propagates through synchronous calls (§§ 19–20).
func (a *Analyzer) recordExecutionRequirement(kind executionRequirementKind, operation string, source lexer.Token) {
	if a.summaryPass || a.currentCallable == "" {
		return
	}
	if a.executionRequirements == nil {
		a.executionRequirements = map[CallableID][]executionRequirementSite{}
	}
	for _, existing := range a.executionRequirements[a.currentCallable] {
		if existing.Kind == kind && existing.Source == source {
			return
		}
	}
	a.executionRequirements[a.currentCallable] = append(a.executionRequirements[a.currentCallable], executionRequirementSite{Kind: kind, Operation: operation, Source: source})
}

// validateExecutionContextRequirements checks every execution root against
// the requirements reachable through its synchronous and deferred calls:
// ordinary program entry is neither a logical task nor a cancellable thread
// (scheduling.md § 22), a spawned thread is cancellable but not a logical task
// (§§ 14(3), 23(2)), a test body runs like program entry (testing.md
// § 9.7), and a spawned task satisfies both. Spawn edges start new
// roots and are not followed. A requirement reached from main is reported at
// main's first call on the path, or at the operation itself.
//
// Rules:
//   - rules/concurrency/cancellation.md — §§ 14, 19–23
//   - rules/concurrency/scheduling.md — § 22 "No implicit root task in main"
func (a *Analyzer) validateExecutionContextRequirements() {
	if a.callGraph == nil || len(a.executionRequirements) == 0 {
		return
	}
	for _, root := range a.callGraph.Roots() {
		forbidden := map[executionRequirementKind]string{}
		switch root.Kind {
		case CallRootProgramEntry:
			forbidden[requiresLogicalTask] = "ordinary program entry is not a logical task"
			forbidden[requiresCancellableExecution] = "ordinary program entry is not a cancellable task or thread"
		case CallRootThreadEntry:
			forbidden[requiresLogicalTask] = "a spawned thread is a physical thread, not a logical task"
		default:
			continue
		}
		a.reportExecutionRequirementsFrom(root, forbidden)
	}
	// Test roots exist only in plan-bound graphs; in Sema's graph every test
	// body node is its own entry (testing.md § 9.7).
	for _, node := range a.callGraph.Nodes() {
		if node.Kind != CallableBodyTest {
			continue
		}
		a.reportExecutionRequirementsFrom(CallRoot{Kind: CallRootTestEntry, Node: node.ID, Source: node.Declaration}, map[executionRequirementKind]string{
			requiresLogicalTask:          "a test body is not a logical task",
			requiresCancellableExecution: "a test body is not a cancellable task or thread",
		})
	}
}

// reportExecutionRequirementsFrom follows synchronous/deferred calls from one
// execution root and reports unmet requirements at its first call, retaining
// the source operation as a navigable witness for transitive requirements.
// Rules: rules/concurrency/cancellation.md — §§19–23;
// rules/concurrency/scheduling.md — §§7(2),22,50,57;
// rules/tooling/diagnostics.md — §§8–12.
func (a *Analyzer) reportExecutionRequirementsFrom(root CallRoot, forbidden map[executionRequirementKind]string) {
	// Keep one traversal state per first call from the root. Joining solely by
	// callee loses other invalid calls when several paths reach the same body.
	// This remains finite for recursive graphs: at most nodes × root call sites.
	type path struct {
		callable CallableID
		entry    CallSiteID
	}
	initial := path{callable: root.Node}
	firstEdge := map[path]*CallSite{initial: nil}
	queue := []path{initial}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, site := range a.callGraph.Outgoing(current.callable) {
			if site.Execution != CallExecutionSynchronous && site.Execution != CallExecutionDeferred {
				continue
			}
			edge := firstEdge[current]
			if current == initial {
				copied := site
				edge = &copied
			}
			for _, target := range site.Targets {
				if target == root.Node {
					continue // The root's own requirements are checked directly.
				}
				next := path{callable: target, entry: edge.ID}
				if _, seen := firstEdge[next]; seen {
					continue
				}
				firstEdge[next] = edge
				queue = append(queue, next)
			}
		}
	}
	paths := make([]path, 0, len(firstEdge))
	for reached := range firstEdge {
		paths = append(paths, reached)
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].callable != paths[j].callable {
			return paths[i].callable < paths[j].callable
		}
		return paths[i].entry < paths[j].entry
	})
	reported := map[string]bool{}
	for _, reached := range paths {
		callable := reached.callable
		for _, requirement := range a.executionRequirements[callable] {
			reason, invalid := forbidden[requirement.Kind]
			if !invalid {
				continue
			}
			id := ""
			if requirement.Kind == requiresLogicalTask {
				id = diagnostics.LogicalTaskContextRequired
			}
			edge := firstEdge[reached]
			if edge == nil {
				key := string(requirement.Kind) + "@" + tokenLocation(requirement.Source)
				if !reported[key] {
					reported[key] = true
					a.addErrorAtTokenWithMetadata(requirement.Source, id, a.executionRequirementHelp(requirement.Kind),
						"%s requires a current %s; %s", requirement.Operation, executionRequirementNoun(requirement.Kind), reason)
				}
				continue
			}
			key := string(requirement.Kind) + "@" + tokenLocation(edge.Source)
			if reported[key] {
				continue
			}
			reported[key] = true
			callee := a.callableDisplayName(callable)
			if requirement.Kind != requiresLogicalTask {
				a.addErrorAtTokenWithMetadata(edge.Source, id, a.executionRequirementHelp(requirement.Kind),
					"call requires a current %s because %s uses %s at %d:%d; %s", executionRequirementNoun(requirement.Kind), callee, requirement.Operation, requirement.Source.Line, requirement.Source.Column, reason)
				continue
			}
			before := len(a.errors)
			a.addErrorAtTokenWithMetadataAndPrevious(edge.Source, requirement.Source, id, a.executionRequirementHelp(requirement.Kind),
				"call requires a current %s because %s uses %s at %d:%d; %s", executionRequirementNoun(requirement.Kind), callee, requirement.Operation, requirement.Source.Line, requirement.Source.Column, reason)
			if len(a.errors) > before {
				a.errors[len(a.errors)-1].RelatedLabel = "current-execution requirement introduced here"
			}
		}
	}
}

func executionRequirementNoun(kind executionRequirementKind) string {
	if kind == requiresLogicalTask {
		return "logical task context"
	}
	return "cancellable task or thread execution context"
}

func (a *Analyzer) executionRequirementHelp(kind executionRequirementKind) string {
	if kind == requiresLogicalTask {
		return "Run the work as a task, for example `let work := spawn Work()` followed by `await work`; main, test bodies and spawned threads have no TaskContext."
	}
	return "cancel ends the current task or explicit thread; run the work with `spawn` instead of calling it directly from main or a test."
}

func (a *Analyzer) callableDisplayName(id CallableID) string {
	if node, ok := a.callGraph.Node(id); ok && node.Name != "" {
		return node.Name
	}
	return string(id)
}

func tokenLocation(token lexer.Token) string {
	return token.File + ":" + strconv.Itoa(token.Line) + ":" + strconv.Itoa(token.Column)
}
