package main

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Rules: rules/analysis/stack_analysis.md §§6,58,75; cli.md §§5.1–5.4,6.
func TestAnalyseStackCLI(t *testing.T) {
	const fixture = "../../testdata/tooling/analyse_cli/stack_inspection.sec"
	decode := func(args ...string) analysisReportDocument {
		t.Helper()
		out, stderr, code := runCLIForDiagnostics(t, append([]string{"analyse"}, args...)...)
		var report analysisReportDocument
		if err := json.Unmarshal([]byte(out), &report); err != nil || code != 0 || stderr != "" {
			t.Fatalf("stack CLI: %d %v %s %s", code, err, out, stderr)
		}
		return report
	}
	stack := decode("stack", "--json", fixture)
	if len(stack.Analyses) != 1 || stack.Analyses[0].Name != "stack" || stack.Analyses[0].Status != "executed" || !strings.Contains(stack.Analyses[0].Note, "does not establish") {
		t.Fatal(stack.Analyses)
	}
	if len(stack.Stack) != 5 {
		t.Fatalf("expected five source callables: %+v", stack.Stack)
	}
	for _, entry := range stack.Stack {
		if entry.SemanticOwnFrame.Kind != "Unknown" || entry.SemanticOwnFrame.Bytes != nil || entry.SemanticMaximum.Kind != "Unknown" || entry.SemanticMaximum.Bytes != nil || entry.FiniteSemanticProof || entry.MachineAvailable {
			t.Fatalf("fabricated stack proof: %+v", entry)
		}
	}
	all := decode("--all", "--json", fixture)
	if !reflect.DeepEqual(all.Stack, stack.Stack) {
		t.Fatal("all and stack-only use different facts")
	}
	implicit := decode("--json", fixture)
	if !reflect.DeepEqual(implicit.Stack, stack.Stack) {
		t.Fatal("default analysis omitted/differed from stack composition")
	}
	focus := decode("stack", "--json", "-f", "StackRoot", fixture)
	if len(focus.Stack) != 2 {
		t.Fatal(focus.Stack)
	}
	for _, entry := range focus.Stack {
		if entry.Name != "StackRoot" && entry.Name != "StackLeaf" {
			t.Fatal("focus leaked a callable", entry.Name)
		}
	}
	out, stderr, code := runCLIForDiagnostics(t, "analyse", "stack", "--verbose", fixture)
	if code != 0 || stderr != "" || !strings.Contains(out, "semantic own=Unknown") || !strings.Contains(out, "machine frame=not available") {
		t.Fatalf("verbose: %d %s %s", code, out, stderr)
	}
	out, stderr, code = runCLIForDiagnostics(t, "analyse", "stack", fixture)
	if code != 0 || out != "" || stderr != "" {
		t.Fatalf("optional unknown insight polluted default output: %d %s %s", code, out, stderr)
	}
	out, stderr, code = runCLIForDiagnostics(t, "analyse", "stack", "--json", analyseRejectedFixture)
	var rejected analysisReportDocument
	if err := json.Unmarshal([]byte(out), &rejected); err != nil || code == 0 || rejected.Status != "rejected" || len(rejected.Diagnostics) == 0 || len(rejected.Stack) != 0 {
		t.Fatalf("mandatory rejection was hidden by inspection: %d %v %s %s", code, err, out, stderr)
	}
}

// Canonical supplied frames must retain exactness and arbitrary precision;
// changing frame evidence cannot mutate source facts or invent machine data.
// Rules: stack_analysis.md §§6,58,75.
func TestAnalyseStackCanonicalFrames(t *testing.T) {
	_, _, graph := analyseFocusFixture(t, "stack_inspection")
	var store sema.StackSummaryStore
	value := new(big.Int).Lsh(big.NewInt(1), 90)
	for _, node := range graph.Nodes() {
		bound, err := sema.NewExactStackBound(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordSemantic(sema.SemanticStackSummary{Callable: node.ID, OwnFrame: bound}); err != nil {
			t.Fatal(err)
		}
	}
	before := store.SemanticSummaries()
	report := analyseReport{names: map[sema.CallableID]string{}}
	report.addStack(graph, graph.Nodes(), &store, "")
	byName := map[string]analysisStackCallable{}
	for _, entry := range report.stack {
		byName[entry.Name] = entry
	}
	want := new(big.Int).Mul(value, big.NewInt(2)).String()
	root := byName["StackRoot"]
	if root.SemanticMaximum.Bytes == nil || *root.SemanticMaximum.Bytes != want || !root.FiniteSemanticProof || root.MachineAvailable {
		t.Fatalf("canonical frame composition: %+v", root)
	}
	for _, name := range []string{"StackRecur", "StackOpaque"} {
		entry := byName[name]
		if entry.SemanticMaximum.Kind != "Unknown" || entry.FiniteSemanticProof || entry.SemanticMaximum.Bytes != nil {
			t.Fatal("unsupported boundary became finite", entry)
		}
	}
	if !reflect.DeepEqual(before, store.SemanticSummaries()) {
		t.Fatal("CLI mutated frame evidence")
	}
	other := analyseReport{}
	other.addStack(graph, graph.Nodes(), &store, "different-plan")
	for _, entry := range other.stack {
		if entry.SemanticOwnFrame.Kind != "Unknown" || entry.SemanticOwnFrame.Bytes != nil {
			t.Fatal("reused frames across plans", entry)
		}
	}
	small, _ := sema.NewExactStackBound(big.NewInt(3))
	_ = store.RecordSemantic(sema.SemanticStackSummary{Callable: graph.Nodes()[0].ID, OwnFrame: small})
	if *root.SemanticMaximum.Bytes != want {
		t.Fatal("published report aliased a changed frame store")
	}
}
