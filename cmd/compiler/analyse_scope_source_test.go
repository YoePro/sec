package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// analyseFocusFixture reads source-backed canonical graph/range facts without
// rebuilding a name or token-range resolver in the CLI tests.
// Rules: rules/tooling/cli.md — §5.3; rules/analysis/call_graph.md — Callable node.
func analyseFocusFixture(t *testing.T, name string) (string, string, *sema.CallGraph) {
	t.Helper()
	file := "../../testdata/tooling/analyse_cli/" + name + ".sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return file, string(data), a.CallGraph()
}

// Finding ownership follows actual nested/same-line ranges, never a preceding
// declaration. Deferred and closure expressions retain the enclosing named fn.
// Rules: rules/tooling/cli.md — §5.3; MD-017 correction — §§3,11(4–5).
func TestAnalyseFocusSourceOwnership(t *testing.T) {
	file, source, graph := analyseFocusFixture(t, "focus_ranges")
	scope, err := resolveAnalyseScope(graph, "Outer", map[string]bool{file: true})
	if err != nil {
		t.Fatal(err)
	}
	tokenAt := func(marker string) lexer.Token {
		t.Helper()
		offset := strings.Index(source, marker)
		if offset < 0 {
			t.Fatal(marker)
		}
		prefix := source[:offset]
		last := strings.LastIndex(prefix, "\n")
		return lexer.Token{File: file, Line: strings.Count(prefix, "\n") + 1, Column: len(prefix) - last}
	}
	for _, test := range []struct{ marker, want string }{
		{"Nested()", "Nested"}, {"let value", "Outer"}, {"let Between", ""},
		{"First()", "First"}, {"Second()", "Second"}, {"let deferred", "WithDeferred"},
		{"return 1", "WithDeferred"}, {"value: int", "Foreign"},
		{"let initialized", "@init:Widget"}, {"let freed", "@free:Widget"}, {"return 2", "Number.get"},
	} {
		id, found := scope.owner(tokenAt(test.marker))
		node, _ := graph.Node(id)
		if found != (test.want != "") || node.Name != test.want {
			t.Errorf("%s -> %+v (%v), want %s", test.marker, node, found, test.want)
		}
	}
	out, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", "-f", "Outer", file)
	var doc analysisReportDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err, out)
	}
	if code != 0 || len(doc.Findings) != 1 || doc.Findings[0].Callable != string(scope.root.ID) {
		t.Fatalf("nested source stole CLI finding: %d %s %+v", code, stderr, doc)
	}
	if !scope.includes(tokenAt("let value")) || scope.includes(tokenAt("Nested()")) {
		t.Fatal("nested declaration stole enclosing findings or became reachable")
	}
	foreignFile := tokenAt("let value")
	foreignFile.File = "other.sec"
	if _, found := scope.owner(foreignFile); found {
		t.Fatal("source ownership leaked across files")
	}
	// Unknown source extents cannot consume the following module contents.
	node := sema.CallableNode{Declaration: tokenAt("First()")}
	if analyseCallableContains(node, tokenAt("Second()")) {
		t.Fatal("unknown range leaked into following function")
	}
}

// Short and long focus selectors agree through recursive SCCs, concrete generic
// specializations and closed indirect targets; dormant source findings stay out.
// Rules: MD-017 correction — §11(4–5); rules/tooling/cli.md — §5.3.
func TestAnalyseFocusGraphForms(t *testing.T) {
	file, _, _ := analyseFocusFixture(t, "focus_graph")
	for _, root := range []string{"SelfCycle", "MutualA", "GenericRoot", "Closed"} {
		short, stderr, code := runCLIForDiagnostics(t, "analyse", "-f", root, file)
		if code != 0 {
			t.Fatalf("%s: %d %s %s", root, code, short, stderr)
		}
		long, _, code := runCLIForDiagnostics(t, "analyse", "--function", root, file)
		if code != 0 || short != long || !strings.Contains(short, "Leaf") || strings.Contains(short, "Dormant") {
			t.Fatalf("%s focus: %d %s / %s", root, code, short, long)
		}
		out, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", "-f", root, file)
		var doc analysisReportDocument
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("%s: %s", err, out)
		}
		if code != 0 || doc.Focus == nil || len(doc.Findings) != 1 || !strings.Contains(doc.Findings[0].Path, "Leaf") {
			t.Fatalf("%s: %d %s %+v", root, code, stderr, doc)
		}
	}
}

// An open indirect dependency remains visible in default and JSON output;
// focus does not fabricate a closed graph or invent a concrete callee.
// Rules: rules/tooling/cli.md — §§5.3(3),5.4(7),6(3); MD-017 — §11(5).
func TestAnalyseFocusOpenTargets(t *testing.T) {
	file, _, graph := analyseFocusFixture(t, "focus_graph")
	scope, err := resolveAnalyseScope(graph, "Open", map[string]bool{file: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.reachable) != 1 {
		t.Fatal("open contract expanded to unrelated concrete bodies", scope.reachable)
	}
	for _, flag := range []string{"-f", "--function"} {
		out, stderr, code := runCLIForDiagnostics(t, "analyse", flag, "Open", file)
		if code != 0 || !strings.Contains(out, "open callable target") || !strings.Contains(out, "effects remain unknown") {
			t.Fatalf("%s: %d %s %s", flag, code, out, stderr)
		}
	}
	out, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", "-f", "Open", file)
	var doc analysisReportDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err, out)
	}
	if code != 0 || doc.Status != "incomplete" || len(doc.Limitations) != 1 || len(doc.Focus.Reachable) != 1 || !strings.Contains(doc.Limitations[0], "declared contract") {
		t.Fatalf("%d %s %+v", code, stderr, doc)
	}
	mixed, err := resolveAnalyseScope(graph, "Mixed", map[string]bool{file: true})
	if err != nil || len(mixed.reachable) != 2 {
		t.Fatalf("known-plus-open targets lost: %v %+v", err, mixed)
	}
	mixedOut, _, mixedCode := runCLIForDiagnostics(t, "analyse", "--json", "-f", "Mixed", file)
	var mixedDoc analysisReportDocument
	if err := json.Unmarshal([]byte(mixedOut), &mixedDoc); err != nil {
		t.Fatal(err, mixedOut)
	}
	if mixedCode != 0 || mixedDoc.Status != "incomplete" || len(mixedDoc.Findings) != 1 || !strings.Contains(mixedDoc.Findings[0].Path, "Leaf") {
		t.Fatalf("known findings or open limitation lost: %d %+v", mixedCode, mixedDoc)
	}
	out, _, code = runCLIForDiagnostics(t, "analyse", "closures", "--verbose", "-f", "Open", file)
	if code != 0 || !strings.Contains(out, "open-contract targets=[]") {
		t.Fatalf("lost canonical open target facts: %d %s", code, out)
	}
}

// Two concrete signatures share generic source ranges. Source findings can be
// attributed to the reachable instance; displayed callable facts still use the
// exact reachable IDs rather than admitting every source-sharing specialization.
// Rules: rules/tooling/cli.md — §5.3; MD-017 — §11(4).
func TestAnalyseFocusGenericIdentity(t *testing.T) {
	file, _, graph := analyseFocusFixture(t, "focus_graph")
	scope, err := resolveAnalyseScope(graph, "GenericRoot", map[string]bool{file: true})
	if err != nil {
		t.Fatal(err)
	}
	instances := 0
	for _, node := range graph.Nodes() {
		if node.Name != "Generic" {
			continue
		}
		if _, reachable := scope.reachable[node.ID]; reachable {
			instances++
			owner, found := scope.owner(node.Declaration)
			if !found || owner != node.ID {
				t.Fatalf("generic source owner %s, want %s", owner, node.ID)
			}
		}
	}
	if instances != 1 {
		t.Fatalf("reachable Generic instances=%d", instances)
	}

	out, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "-f", "GenericRoot", file)
	if code != 0 || !strings.Contains(out, "3 callables, 2 call sites") {
		t.Fatalf("source-sharing instances leaked into displayed graph: %d %s %s", code, out, stderr)
	}
}

// Focusing presentation never skips mandatory foreign-contract validation in
// another callable; an unknown external guarantee must still reject the input.
// Rules: rules/tooling/cli.md — §§5.3(3),5.6; MD-017 — §11(5).
func TestAnalyseFocusMandatoryForeignContract(t *testing.T) {
	file := "../../testdata/tooling/analyse_cli/focus_foreign_invalid.sec"
	for _, flag := range []string{"-f", "--function"} {
		out, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", flag, "Focus", file)
		var doc analysisReportDocument
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatal(err, out, stderr)
		}
		if code != 3 || doc.Status != "rejected" || len(doc.Diagnostics) == 0 {
			t.Fatalf("foreign failure hidden by focus: %d %s %+v", code, stderr, doc)
		}
		if !strings.Contains(out, "noPanic") {
			t.Fatal("missing mandatory guarantee diagnostic", out)
		}
	}
}
