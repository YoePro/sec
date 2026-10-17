package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// analysisFamilyAvailability states whether an analysis family can be
// selected individually by `sec analyse <name>`.
type analysisFamilyAvailability string

const (
	// analysisAvailable families have a canonical producer the command can
	// run and report on its own.
	analysisAvailable analysisFamilyAvailability = "available"
	// analysisNotSelectable families run only as mandatory diagnostics of
	// every compilation or are not implemented; selecting one alone fails
	// with an explanation instead of reporting a misleading clean result.
	analysisNotSelectable analysisFamilyAvailability = "not-selectable"
)

// analysisFamily is one entry of the single analysis registry shared by the
// parser, help, the human report and the JSON report.
//
// Rules:
//   - rules/tooling/cli.md — § 5 (analysis names)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — §§ 2(3)–(5), 10(1)
type analysisFamily struct {
	Name         string
	Title        string
	Description  string
	Availability analysisFamilyAvailability
	// Limitation explains why a not-selectable family cannot run alone.
	Limitation string
	// DiagnosticFamilies and DiagnosticIDs name the registered diagnostics
	// the analysis owns, used to attribute warnings to it.
	DiagnosticFamilies []string
	DiagnosticIDs      []string
}

func analysisFamilies() []analysisFamily {
	return []analysisFamily{
		{Name: "pitfalls", Title: "pitfall analysis", Description: "likely mistakes such as final-element access without a non-empty proof, with suggested fixes",
			Availability: analysisAvailable, DiagnosticFamilies: []string{"suspicious"}},
		{Name: "parameter-usage", Title: "parameter usage analysis", Description: "how each parameter is used and where a narrower parameter form (ref, ref mut) is recommended",
			Availability: analysisAvailable, DiagnosticIDs: []string{diagnostics.LargeValueParameter}},
		{Name: "stack", Title: "stack analysis", Description: "stack budget proofs for entry points, interrupts and tasks",
			Availability: analysisAvailable, DiagnosticFamilies: []string{"stack"}},
		{Name: "isr", Title: "ISR analysis", Description: "interrupt-handler safety: no allocation, no panic, no blocking",
			Availability: analysisNotSelectable, Limitation: "its checks run as mandatory diagnostics of every compilation and cannot yet be selected alone; run `sec analyse` or `sec check`"},
		{Name: "deadlocks", Title: "deadlock analysis", Description: "lock-order and same-resource reacquisition hazards",
			Availability: analysisNotSelectable, Limitation: "only same-resource reacquisition is checked, as a mandatory diagnostic of every compilation; the individually selectable deadlock analysis is not implemented yet"},
		{Name: "data-races", Title: "data race analysis", Description: "unsynchronized shared mutable state across tasks and interrupts",
			Availability: analysisNotSelectable, Limitation: "the data race analysis is not implemented yet"},
	}
}

func findAnalysisFamily(name string) (analysisFamily, bool) {
	for _, family := range analysisFamilies() {
		if family.Name == name {
			return family, true
		}
	}
	return analysisFamily{}, false
}

func analysisFamilyNames() []string {
	names := []string{}
	for _, family := range analysisFamilies() {
		names = append(names, family.Name)
	}
	return names
}

func analyseHelpText() string {
	var out strings.Builder
	out.WriteString(`Runs the compiler's analyses over the selected sources (the current directory
when no source is named) and reports what needs attention. A clean result
prints nothing and exits 0.

Analyses (select one by name; the default and --all run every analysis):
`)
	for _, family := range analysisFamilies() {
		fmt.Fprintf(&out, "  %-16s %s\n", family.Name, family.Description)
		if family.Availability != analysisAvailable {
			fmt.Fprintf(&out, "  %-16s (%s)\n", "", family.Limitation)
		}
	}
	out.WriteString(`
Options:
  --all                 Run every analysis (the default).
  -f, --function NAME   Analyse NAME and every callable it can reach. NAME is a
                        unique function name (myFunc), a method (Type.Method) or
                        a canonical callable ID; an ambiguous overload is rejected
                        with the candidate IDs.
  -d, --depth LEVEL     Analysis budget: interactive, standard or deep (default).
                        A smaller budget can leave results unproven; it never
                        turns an unproven result into a proven one.
  --info                Add a short summary: analyses run, scope, depth, counts
                        and limitations.
  --verbose             Add every fact, call path and piece of evidence (includes
                        --info).
  --json                Print one versioned machine-readable report
                        (sec.analysis-report) instead of the human layout.
  --target OS-ARCH      Analyse for another target.

--info and --verbose change how much is shown, not how much is analysed, and
they are not the INFO diagnostic severity. To analyse a directory whose name is
also an analysis name, write it as a path (./stack).

Examples:
  sec analyse
  sec analyse --all --depth deep
  sec analyse isr
  sec analyse data-races
  sec analyse parameter-usage
  sec analyse pitfalls
  sec analyse deadlocks
  sec analyse stack
  sec analyse -f myFunc
  sec analyse deadlocks --function myFunc
  sec analyse -d standard
  sec analyse --info
  sec analyse --verbose
  sec analyse --json
`)
	return out.String()
}

// analysePresentation is the requested output form; it never changes what
// is analysed.
type analysePresentation string

const (
	analysePresentDefault analysePresentation = "default"
	analysePresentInfo    analysePresentation = "info"
	analysePresentVerbose analysePresentation = "verbose"
	analysePresentJSON    analysePresentation = "json"
)

// looksLikeAnalysisName reports whether a positional word is meant as an
// analysis name rather than a source path: lowercase words and hyphens with
// no path separator, extension or glob, and no existing file of that name.
func looksLikeAnalysisName(arg string) bool {
	if arg == "" || strings.ContainsAny(arg, "/\\.*?[") {
		return false
	}
	for _, r := range arg {
		if (r < 'a' || r > 'z') && r != '-' {
			return false
		}
	}
	if _, err := os.Stat(arg); err == nil {
		return false
	}
	return true
}

// analyseFinding is one reportable analysis result shared by the default,
// verbose and JSON presentations.
type analyseFinding struct {
	Family   string
	Class    analyseReportClass
	Rule     string
	Token    lexer.Token
	Message  string
	Callable sema.CallableID
	Actions  []string
}

// analyseScope is the resolved function focus of one invocation: the root
// callable and every callable reachable from it, with the parent of each
// reachable callable for call-path reporting.
//
// Rules:
//   - rules/tooling/cli.md — § 5.3 (function-focused analysis)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — § 3
type analyseScope struct {
	selector  string
	root      sema.CallableNode
	reachable map[sema.CallableID]sema.CallableID // callable -> parent on a shortest path ("" for the root)
	byFile    map[string][]sema.CallableNode      // declarations per file in source order
}

// resolveAnalyseScope resolves a callable selector to exactly one canonical
// callable among the user-selected sources: a canonical callable ID, a
// qualified Type.Method name or a unique short name. Ambiguity is rejected
// with the candidates; nothing is chosen by guesswork.
func resolveAnalyseScope(graph *sema.CallGraph, selector string, selected map[string]bool) (*analyseScope, error) {
	candidates := []sema.CallableNode{}
	for _, node := range graph.Nodes() {
		if node.Kind != sema.CallableBodyNamedFunction {
			continue
		}
		if string(node.ID) == selector || analyseCallableName(node) == selector || (node.Name == selector && selected[node.Declaration.File]) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no callable named %q in the selected sources; use a function name, Type.Method or a canonical callable ID (see `sec help analyse`)", selector)
	}
	if len(candidates) > 1 {
		ids := []string{}
		for _, candidate := range candidates {
			ids = append(ids, fmt.Sprintf("%s (%s)", candidate.ID, analysePosition(candidate.Declaration)))
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("%q names %d callables; select one canonical callable ID:\n  %s", selector, len(candidates), strings.Join(ids, "\n  "))
	}
	scope := &analyseScope{selector: selector, root: candidates[0], reachable: map[sema.CallableID]sema.CallableID{candidates[0].ID: ""}, byFile: map[string][]sema.CallableNode{}}
	queue := []sema.CallableID{candidates[0].ID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, site := range graph.Outgoing(current) {
			for _, target := range site.Targets {
				if _, seen := scope.reachable[target]; !seen {
					scope.reachable[target] = current
					queue = append(queue, target)
				}
			}
		}
	}
	for _, node := range graph.Nodes() {
		scope.byFile[node.Declaration.File] = append(scope.byFile[node.Declaration.File], node)
	}
	for file := range scope.byFile {
		nodes := scope.byFile[file]
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Declaration.Line != nodes[j].Declaration.Line {
				return nodes[i].Declaration.Line < nodes[j].Declaration.Line
			}
			return nodes[i].Declaration.Column < nodes[j].Declaration.Column
		})
	}
	return scope, nil
}

// owner attributes a source token to the named function whose declaration
// most closely precedes it in the same file. Closures and defers are part of
// their enclosing function for attribution.
func (s *analyseScope) owner(token lexer.Token) (sema.CallableID, bool) {
	var found sema.CallableID
	ok := false
	for _, node := range s.byFile[token.File] {
		if node.Kind != sema.CallableBodyNamedFunction {
			continue
		}
		if node.Declaration.Line > token.Line || node.Declaration.Line == token.Line && node.Declaration.Column > token.Column {
			break
		}
		found, ok = node.ID, true
	}
	return found, ok
}

// includes reports whether a token lies in a callable reachable from the
// focus root.
func (s *analyseScope) includes(token lexer.Token) bool {
	owner, ok := s.owner(token)
	if !ok {
		return false
	}
	_, reachable := s.reachable[owner]
	return reachable
}

// path is the call path from the root to callable.
func (s *analyseScope) path(callable sema.CallableID, names map[sema.CallableID]string) string {
	chain := []string{}
	for current := callable; current != ""; current = s.reachable[current] {
		name := names[current]
		if name == "" {
			name = string(current)
		}
		chain = append([]string{name}, chain...)
		if current == s.root.ID {
			break
		}
	}
	return strings.Join(chain, " -> ")
}

// analyseFamilyOfWarning attributes a registered diagnostic to an analysis
// family through the diagnostic registry; unregistered diagnostics belong to
// no individual family.
func analyseFamilyOfWarning(warning sema.Error) string {
	definition, ok := diagnostics.Lookup(warning.ID)
	if !ok {
		return ""
	}
	for _, family := range analysisFamilies() {
		for _, owned := range family.DiagnosticFamilies {
			if definition.Family == owned {
				return family.Name
			}
		}
		for _, owned := range family.DiagnosticIDs {
			if definition.ID == owned {
				return family.Name
			}
		}
	}
	return ""
}

// analysisReportDocument is version 1 of the machine-readable
// `sec.analysis-report`. Fields are never reinterpreted within a version.
//
// Rules:
//   - rules/tooling/cli.md — § 6 (machine-readable analysis report)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — § 6
type analysisReportDocument struct {
	Format      string                   `json:"format"`
	Version     int                      `json:"version"`
	Status      string                   `json:"status"`
	Target      string                   `json:"target"`
	Depth       string                   `json:"depth"`
	Analyses    []analysisReportAnalysis `json:"analyses"`
	Focus       *analysisReportFocus     `json:"focus"`
	Diagnostics []emittedOccurrence      `json:"diagnostics"`
	Findings    []analysisReportFinding  `json:"findings"`
	Limitations []string                 `json:"limitations"`
	Summary     analysisReportSummary    `json:"summary"`
}

type analysisReportAnalysis struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Complete bool   `json:"complete"`
	Note     string `json:"note,omitempty"`
}

type analysisReportFocus struct {
	Selector  string   `json:"selector"`
	Callable  string   `json:"callable"`
	Name      string   `json:"name"`
	Reachable []string `json:"reachable"`
}

type analysisReportFinding struct {
	Analysis string         `json:"analysis"`
	Class    string         `json:"class"`
	Rule     string         `json:"rule"`
	Location occurrenceSpan `json:"location"`
	Message  string         `json:"message"`
	Callable string         `json:"callable,omitempty"`
	Path     string         `json:"path,omitempty"`
	Actions  []string       `json:"actions"`
}

type analysisReportSummary struct {
	Errors      int `json:"errors"`
	Unproven    int `json:"unproven"`
	Warnings    int `json:"warnings"`
	Advisory    int `json:"advisory"`
	Information int `json:"information"`
}

const (
	analysisReportFormat        = "sec.analysis-report"
	analysisReportFormatVersion = 1
)

// writeAnalysisJSON writes one analysis report document.
func writeAnalysisJSON(out io.Writer, document analysisReportDocument) {
	if document.Analyses == nil {
		document.Analyses = []analysisReportAnalysis{}
	}
	if document.Diagnostics == nil {
		document.Diagnostics = []emittedOccurrence{}
	}
	if document.Findings == nil {
		document.Findings = []analysisReportFinding{}
	}
	if document.Limitations == nil {
		document.Limitations = []string{}
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(document)
}
