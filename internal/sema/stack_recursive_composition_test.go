package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// recursiveStackInputs obtains canonical SCCs from source and supplies
// independently verified own-frame facts at separate measurement levels.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution".
func recursiveStackInputs(t *testing.T) (*CallGraph, *StackSummaryStore, map[string]CallableID) {
	t.Helper()
	source, err := os.ReadFile("../../testdata/sema/stack_recursive_composition_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errs, nil)
	graph := a.CallGraph()
	frames := &StackSummaryStore{}
	ids := map[string]CallableID{}
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
		size := int64(50)
		switch node.Name {
		case "A", "MultiA":
			size = 100
		case "B", "MultiB":
			size = 200
		case "C":
			size = 300
		case "MultiC":
			size = 500
		case "Tail":
			size = 1000
		case "Self":
			size = 7
		case "Indirect":
			size = 11
		}
		semantic, _ := NewExactStackBound(big.NewInt(size))
		machine, _ := NewExactStackBound(big.NewInt(size * 2))
		_ = frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: semantic})
		_ = frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: machine})
	}
	return graph, frames, ids
}

// recursiveStackCoverage lists every canonical internal invocation. The test
// producer separately establishes finite depth; the edge list alone is no proof.
// Rules: rules/analysis/stack_analysis.md — "Recursive progress must cover relevant transitions".
func recursiveStackCoverage(graph *CallGraph, entry CallableID) []CallSiteID {
	members := map[CallableID]bool{}
	for _, node := range graph.SameStackSCC(entry) {
		members[node.ID] = true
	}
	var sites []CallSiteID
	for member := range members {
		for _, site := range graph.Outgoing(member) {
			for _, target := range site.Targets {
				if sameStackExecution(site.Execution) && members[target] {
					sites = append(sites, site.ID)
					break
				}
			}
		}
	}
	return sites
}

// recursiveStackProof consumes a verified upper limit on all live component
// frames. These source cycles pass stop=true on recursive transitions, so any
// upper limit >=2 is sound for all entry values in the test scope.
// Rules: rules/analysis/stack_analysis.md — "Finite recursion and finite global bounds".
func recursiveStackProof(t *testing.T, graph *CallGraph, entry CallableID, level StackMeasurementLevel, plan string, count *big.Int) *StackRecursionDepthProof {
	t.Helper()
	depth, _ := NewUpperRecursionDepth(count)
	proof, err := NewVerifiedStackRecursionDepthProof(graph, entry, level, plan, depth, recursiveStackCoverage(graph, entry), lexer.Token{File: "depth-proof.sec", Line: 1, Column: 1})
	if err != nil {
		t.Fatal(err)
	}
	return proof
}

// Recursive composition includes every live SCC frame, differing cycle costs,
// outgoing contributions and ordinary callers, at independent semantic/machine
// levels. Computed walk bounds remain UpperBound even with an exact depth.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution",
// "UpperBound", "Call-path composition", "Semantic and machine frame authority".
func TestStackRecursiveComposition(t *testing.T) {
	graph, frames, ids := recursiveStackInputs(t)
	before := frames.SemanticSummaries()
	proofs := []*StackRecursionDepthProof{}
	for _, scope := range []struct {
		level StackMeasurementLevel
		plan  string
	}{{StackMeasurementSemantic, ""}, {StackMeasurementMachine, "plan"}} {
		for _, name := range []string{"A", "MultiA", "Self", "Indirect", "Callback", "ForeignCycle"} {
			proofs = append(proofs, recursiveStackProof(t, graph, ids[name], scope.level, scope.plan, big.NewInt(3)))
		}
	}
	contracts := StackBoundaryContracts{Recursion: proofs}
	semantic := ComposeSemanticStackSummariesWithBoundaryContracts(graph, frames, "", contracts)
	machine, err := ComposeMachineStackSummariesWithBoundaryContracts(graph, frames, "plan", contracts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"A": 600, "B": 600, "C": 600, "Entry": 650, "MultiA": 2100, "MultiB": 1800, "MultiC": 2500, "Self": 21, "Indirect": 33}
	for name, size := range want {
		found := false
		for _, summary := range semantic {
			if summary.Callable != ids[name] {
				continue
			}
			found = true
			count, finite := summary.TransitiveMaximum.Bytes()
			if !finite || count.Cmp(big.NewInt(size)) != 0 || summary.TransitiveMaximum.Kind() != StackBoundUpperBound {
				t.Fatalf("%s: %s, want UpperBound(%d)", name, summary.TransitiveMaximum, size)
			}
			if len(summary.MaximumCause) == 0 || len(summary.Evidence.Contributors) == 0 {
				t.Fatal("missing recursive evidence", name)
			}
		}
		if !found {
			t.Fatal("missing recursive result", name)
		}
		for _, summary := range machine {
			if summary.Callable == ids[name] {
				count, finite := summary.TransitiveMaximum.Bytes()
				if !finite || count.Cmp(big.NewInt(2*size)) != 0 {
					t.Fatal("machine frames replaced by semantic frames", name, summary.TransitiveMaximum)
				}
			}
		}
	}
	for _, summary := range semantic {
		if summary.Callable == ids["Callback"] || summary.Callable == ids["ForeignCycle"] {
			if summary.TransitiveMaximum.Kind() != StackBoundUnknown || len(summary.Evidence.UnknownCauses) == 0 {
				t.Fatal("opaque boundary gained finite proof", summary)
			}
		}
	}
	if !reflect.DeepEqual(before, frames.SemanticSummaries()) {
		t.Fatal("composition mutated frame inputs")
	}
	// Reordering canonical graph registration changes neither costs nor causes.
	reversed := graph.clone()
	for i, j := 0, len(reversed.nodeOrder)-1; i < j; i, j = i+1, j-1 {
		reversed.nodeOrder[i], reversed.nodeOrder[j] = reversed.nodeOrder[j], reversed.nodeOrder[i]
	}
	var reordered []*StackRecursionDepthProof
	for _, name := range []string{"ForeignCycle", "Callback", "Indirect", "Self", "MultiA", "A"} {
		reordered = append(reordered, recursiveStackProof(t, reversed, ids[name], StackMeasurementSemantic, "", big.NewInt(3)))
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummariesWithBoundaryContracts(reversed, frames, "", StackBoundaryContracts{Recursion: reordered})) {
		t.Fatal("graph/proof registration order changed recursive results")
	}
}

// Coverage, graph lifetime and measurement identity prevent a partial or stale
// progress fact from authorizing finite composition over another cycle/scope.
// Rules: rules/analysis/stack_analysis.md — "Recursive progress must cover relevant transitions",
// "Summary invalidation", "CompilationPlan dependence".
func TestStackRecursiveProofCoverage(t *testing.T) {
	graph, frames, ids := recursiveStackInputs(t)
	depth, _ := NewUpperRecursionDepth(big.NewInt(3))
	covered := recursiveStackCoverage(graph, ids["MultiA"])
	if len(covered) < 3 {
		t.Fatal("fixture does not contain multiple cycles")
	}
	for _, invalid := range [][]CallSiteID{nil, covered[:1], append(append([]CallSiteID(nil), covered[1:]...), covered[1]), append(append([]CallSiteID(nil), covered...), "unrelated")} {
		if _, err := NewVerifiedStackRecursionDepthProof(graph, ids["MultiA"], StackMeasurementSemantic, "", depth, invalid, lexer.Token{}); err == nil {
			t.Fatal("partial or duplicate transition proof accepted")
		}
	}
	zero, _ := NewUpperRecursionDepth(new(big.Int))
	for _, invalid := range []RecursionDepthBound{zero, UnknownRecursionDepth(), UnboundedRecursionDepth()} {
		if _, err := NewVerifiedStackRecursionDepthProof(graph, ids["A"], StackMeasurementSemantic, "", invalid, recursiveStackCoverage(graph, ids["A"]), lexer.Token{}); err == nil {
			t.Fatal("nonpositive/nonfinite depth accepted")
		}
	}
	if _, err := NewVerifiedStackRecursionDepthProof(graph, ids["Tail"], StackMeasurementSemantic, "", depth, nil, lexer.Token{}); err == nil {
		t.Fatal("acyclic node accepted as recursive SCC")
	}
	if _, err := NewVerifiedStackRecursionDepthProof(graph, ids["A"], StackMeasurementMachine, "", depth, recursiveStackCoverage(graph, ids["A"]), lexer.Token{}); err == nil {
		t.Fatal("machine proof without plan accepted")
	}
	proof := recursiveStackProof(t, graph, ids["A"], StackMeasurementSemantic, "", big.NewInt(3))
	for _, summary := range frames.SemanticSummaries() {
		summary.CompilationPlanID = "wrong-plan"
		_ = frames.RecordSemantic(summary)
	}
	assertUnknown := func(graph *CallGraph, plan string, proofs []*StackRecursionDepthProof) {
		t.Helper()
		for _, summary := range ComposeSemanticStackSummariesWithBoundaryContracts(graph, frames, plan, StackBoundaryContracts{Recursion: proofs}) {
			if summary.Callable == ids["A"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
				t.Fatal("invalid scope gained finite recursive proof")
			}
		}
	}
	assertUnknown(graph, "", nil)
	assertUnknown(graph.clone(), "", []*StackRecursionDepthProof{proof})
	assertUnknown(graph, "wrong-plan", []*StackRecursionDepthProof{proof})
	assertUnknown(graph, "", []*StackRecursionDepthProof{proof, proof})
	// Another level never supplies missing machine depth evidence.
	machine, err := ComposeMachineStackSummariesWithBoundaryContracts(graph, frames, "plan", StackBoundaryContracts{Recursion: []*StackRecursionDepthProof{proof}})
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range machine {
		if summary.Callable == ids["A"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("semantic depth reused as machine proof")
		}
	}
	// A changed declaration invalidates this exact pointer's old proof even
	// when all recursion edges remain finite and structurally unchanged.
	node := graph.nodes[ids["B"]]
	node.Declaration.Line++
	graph.nodes[ids["B"]] = node
	assertUnknown(graph, "", []*StackRecursionDepthProof{proof})
}

// Huge finite counts remain finite arbitrary-precision facts without recursion
// unrolling. Exhausted arithmetic budgets and absent frame facts preserve Unknown.
// Rules: rules/analysis/stack_analysis.md — "Required performance and termination tests",
// "Unknown", "Partial information", "Stack-bound classification".
func TestStackRecursiveLargeCountsAndFallback(t *testing.T) {
	graph, frames, ids := recursiveStackInputs(t)
	large := new(big.Int).Lsh(big.NewInt(1), 80)
	proof := recursiveStackProof(t, graph, ids["Self"], StackMeasurementSemantic, "", large)
	check := func(proof *StackRecursionDepthProof, target CallableID) SemanticStackSummary {
		t.Helper()
		for _, summary := range ComposeSemanticStackSummariesWithBoundaryContracts(graph, frames, "", StackBoundaryContracts{Recursion: []*StackRecursionDepthProof{proof}}) {
			if summary.Callable == target {
				return summary
			}
		}
		t.Fatal("missing result")
		return SemanticStackSummary{}
	}
	result := check(proof, ids["Self"])
	got, finite := result.TransitiveMaximum.Bytes()
	if !finite || got.Cmp(new(big.Int).Mul(large, big.NewInt(7))) != 0 {
		t.Fatal("huge recursive count lost precision", result.TransitiveMaximum)
	}
	// Exact maximum depth does not establish path-correlated exact stack demand.
	exact, _ := NewExactRecursionDepth(big.NewInt(2))
	proof, err := NewVerifiedStackRecursionDepthProof(graph, ids["Self"], StackMeasurementSemantic, "", exact, recursiveStackCoverage(graph, ids["Self"]), lexer.Token{})
	if err != nil {
		t.Fatal(err)
	}
	if check(proof, ids["Self"]).TransitiveMaximum.Kind() != StackBoundUpperBound {
		t.Fatal("exact depth fabricated exact stack demand")
	}
	zero, _ := NewExactStackBound(new(big.Int))
	_ = frames.RecordSemantic(SemanticStackSummary{Callable: ids["Self"], OwnFrame: zero})
	got, finite = check(proof, ids["Self"]).TransitiveMaximum.Bytes()
	if !finite || got.Sign() != 0 {
		t.Fatal("zero-cost finite recursion became unknown")
	}
	proof = recursiveStackProof(t, graph, ids["A"], StackMeasurementSemantic, "", new(big.Int).Lsh(big.NewInt(1), 50000))
	result = check(proof, ids["A"])
	if result.TransitiveMaximum.Kind() != StackBoundUnknown || !strings.Contains(result.MaximumCause[0].Detail, "resource limit") || len(result.Evidence.Contributors) < 3 {
		t.Fatal("arithmetic budget lost uncertainty or known frames")
	}
	proof = recursiveStackProof(t, graph, ids["A"], StackMeasurementSemantic, "", big.NewInt(3))
	delete(frames.semantic, stackSummaryKey{ids["C"], ""})
	result = check(proof, ids["A"])
	if result.TransitiveMaximum.Kind() != StackBoundUnknown || len(result.Evidence.UnknownCauses) == 0 {
		t.Fatal("missing SCC frame gained finite bound")
	}
}
