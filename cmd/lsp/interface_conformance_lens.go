package main

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

type codeLensParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type lspCommand struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type codeLens struct {
	Range   lspRange    `json:"range"`
	Command *lspCommand `json:"command,omitempty"`
}

// showLocationsCommand is the client command that lists locations; the VS Code
// extension implements it with the references peek view.
const showLocationsCommand = "sec.showLocations"

// interfaceTargetResult is the conformance of one interface on one target.
type interfaceTargetResult struct {
	target       string
	implementers map[string]location
	failures     []interfaceConformanceFailure
}

type interfaceConformanceFailure struct {
	member   string
	message  string
	location location
}

// interfaceConformanceSummary is the cross-target conformance of one
// interface declared in the active document.
type interfaceConformanceSummary struct {
	name    string
	rng     lspRange
	targets []interfaceTargetResult
}

var interfaceConformanceCache = struct {
	sync.Mutex
	key     string
	summary []interfaceConformanceSummary
}{}

// interfaceConformanceCodeLenses answers textDocument/codeLens with one lens
// per interface declared in the document, summarizing whether its
// implementations conform on every target the module's platform files select.
//
// Rules:
//   - rules/declarations/interfaces.md — 6 "Conformance requirements", 12 "Diagnostics"
//   - rules/platform/platform_model.md — source selection by target
//   - rules/tooling/lsp.md — "Code lens"
func interfaceConformanceCodeLenses(uri string, text string, overlay sourceOverlay) []codeLens {
	lenses := []codeLens{}
	for _, summary := range crossTargetInterfaceConformance(uri, text, overlay) {
		lenses = append(lenses, codeLens{Range: summary.rng, Command: interfaceConformanceCommand(uri, summary)})
	}
	return lenses
}

func interfaceConformanceCommand(uri string, summary interfaceConformanceSummary) *lspCommand {
	if len(summary.targets) == 0 {
		return &lspCommand{Title: "no platform implementation is selected for this module", Command: ""}
	}
	implementers := map[string]bool{}
	allImplementations := []location{}
	failingTargets := []string{}
	failureLocations := []location{}
	missingTargets := []string{}
	for _, result := range summary.targets {
		names := make([]string, 0, len(result.implementers))
		for name := range result.implementers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			implementers[name] = true
			allImplementations = append(allImplementations, result.implementers[name])
		}
		if len(result.implementers) == 0 {
			missingTargets = append(missingTargets, result.target)
			continue
		}
		if len(result.failures) > 0 {
			members := []string{}
			seen := map[string]bool{}
			for _, failure := range result.failures {
				if !seen[failure.member] {
					seen[failure.member] = true
					members = append(members, failure.member)
				}
				failureLocations = append(failureLocations, failure.location)
			}
			failingTargets = append(failingTargets, result.target+": "+strings.Join(members, ", "))
		}
	}
	names := make([]string, 0, len(implementers))
	for name := range implementers {
		names = append(names, name)
	}
	sort.Strings(names)
	total := len(summary.targets)
	conforming := total - len(failingTargets) - len(missingTargets)
	position := summary.rng.Start
	if len(failingTargets) == 0 && len(missingTargets) == 0 {
		return &lspCommand{
			Title:     fmt.Sprintf("✓ %s conforms on %d target%s", strings.Join(names, ", "), total, plural(total)),
			Command:   showLocationsCommand,
			Arguments: []any{uri, position, allImplementations},
		}
	}
	parts := []string{}
	if len(failingTargets) > 0 {
		parts = append(parts, strings.Join(failingTargets, "; "))
	}
	if len(missingTargets) > 0 {
		parts = append(parts, "no implementation on "+strings.Join(missingTargets, ", "))
	}
	locations := failureLocations
	if len(locations) == 0 {
		locations = allImplementations
	}
	return &lspCommand{
		Title:     fmt.Sprintf("✗ conforms on %d of %d targets — %s", conforming, total, strings.Join(parts, " — ")),
		Command:   showLocationsCommand,
		Arguments: []any{uri, position, locations},
	}
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// crossTargetInterfaceConformance analyzes the active document's module once
// per target selected by the module's `#target` files, with that target's
// scalar plan, and records for every interface declared in the document which
// types implement it on that target and which conformance diagnostics (S1106,
// S1107) they produce. Results are cached until a module source changes.
func crossTargetInterfaceConformance(uri string, text string, overlay sourceOverlay) (summaries []interfaceConformanceSummary) {
	summaries = []interfaceConformanceSummary{}
	defer func() {
		if recover() != nil {
			summaries = []interfaceConformanceSummary{}
		}
	}()
	path := pathFromURI(uri)
	active := parseProgramForLSP(uri, text)
	if active == nil || path == "" {
		return summaries
	}
	interfaces := []*ast.InterfaceDeclaration{}
	for _, statement := range active.Statements {
		if declaration, ok := statement.(*ast.InterfaceDeclaration); ok && declaration != nil && declaration.Name != nil {
			interfaces = append(interfaces, declaration)
		}
	}
	if len(interfaces) == 0 {
		return summaries
	}
	module := lspserver.ProgramModule(active)
	targets, key := moduleTargets(path, module, text, overlay, active)
	interfaceConformanceCache.Lock()
	if interfaceConformanceCache.key == key && key != "" {
		cached := interfaceConformanceCache.summary
		interfaceConformanceCache.Unlock()
		return cached
	}
	interfaceConformanceCache.Unlock()

	for _, declaration := range interfaces {
		summaries = append(summaries, interfaceConformanceSummary{name: declaration.Name.Value, rng: tokenRange(text, declaration.Name.Token)})
	}
	for _, target := range targets {
		program := parseProgramForLSP(uri, text)
		if program == nil {
			continue
		}
		lspserver.AssembleModuleForTarget(program, path, overlay, target)
		resolveCoreSources(program, path, overlay)
		resolveSourceImportsForTarget(program, map[string]bool{}, path, target, overlay)
		analyzer := newLSPAnalyzer(uri, program)
		if definition, ok := platformtarget.Find(target); ok {
			if plan, err := definition.ScalarPlan(); err == nil {
				analyzer = sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
			}
		}
		errors := analyzer.Analyze(program)
		types := analyzer.Types()
		for index := range summaries {
			result := interfaceTargetResult{target: target.String(), implementers: map[string]location{}}
			for name, typ := range types {
				for _, implemented := range typ.Implements {
					if implemented.Name != summaries[index].name || typ.Kind == sema.InterfaceType {
						continue
					}
					if definition, ok := analyzer.ResolvedTypeDeclarationLocation(typ); ok && definition.File != "" {
						result.implementers[name] = location{URI: uriFromPath(definition.File), Range: lspRange{Start: position{Line: definition.Line - 1, Character: definition.Column - 1}, End: position{Line: definition.Line - 1, Character: definition.Column - 1 + len(name)}}}
					} else {
						result.implementers[name] = location{URI: uri, Range: summaries[index].rng}
					}
				}
			}
			for _, err := range errors {
				if err.ID != diagnostics.InterfaceMemberMissing && err.ID != diagnostics.InterfaceMemberIncompatible {
					continue
				}
				if !strings.Contains(err.Message, " interface "+summaries[index].name) && !strings.Contains(err.Message, " implements "+summaries[index].name+" ") {
					continue
				}
				result.failures = append(result.failures, interfaceConformanceFailure{
					member:   conformanceFailureMember(err.Message),
					message:  err.Message,
					location: location{URI: uriFromPath(err.File), Range: lspRange{Start: position{Line: err.Line - 1, Character: err.Column - 1}, End: position{Line: err.Line - 1, Character: err.Column - 1}}},
				})
			}
			summaries[index].targets = append(summaries[index].targets, result)
		}
	}
	interfaceConformanceCache.Lock()
	interfaceConformanceCache.key = key
	interfaceConformanceCache.summary = summaries
	interfaceConformanceCache.Unlock()
	return summaries
}

// conformanceFailureMember extracts the member name from an S1106/S1107
// message ("... method Open does not match ...", "... missing method Open").
func conformanceFailureMember(message string) string {
	fields := strings.Fields(message)
	for index := 0; index+1 < len(fields); index++ {
		switch fields[index] {
		case "method", "property", "event":
			return strings.TrimSuffix(fields[index+1], ":")
		}
	}
	return "member"
}

// moduleTargets lists the distinct targets selected by `#target` directives
// of the module's source files (the active document's own directive limits
// the set to that target), sorted, and a cache key over the module sources.
func moduleTargets(path string, module string, text string, overlay sourceOverlay, active *ast.Program) ([]platformtarget.Target, string) {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(path + "\x00" + text))
	if target, directed := lspserver.ProgramTarget(active); directed {
		return []platformtarget.Target{target}, strconv.FormatUint(hash.Sum64(), 16)
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.sec"))
	for overlayPath := range overlay {
		if filepath.Dir(overlayPath) == filepath.Dir(normalizedSourcePath(path)) && filepath.Ext(overlayPath) == ".sec" {
			matches = append(matches, overlayPath)
		}
	}
	seenPath := map[string]bool{}
	seen := map[string]bool{}
	targets := []platformtarget.Target{}
	sort.Strings(matches)
	for _, match := range matches {
		normalized := normalizedSourcePath(match)
		if seenPath[normalized] || normalized == normalizedSourcePath(path) {
			continue
		}
		seenPath[normalized] = true
		program, ok := lspserver.ParseSource(normalized, overlay)
		if !ok || lspserver.ProgramModule(program) != module {
			continue
		}
		if data, err := lspserver.ReadSource(normalized, overlay); err == nil {
			_, _ = hash.Write([]byte(normalized + "\x00"))
			_, _ = hash.Write(data)
		}
		if target, directed := lspserver.ProgramTarget(program); directed && !seen[target.String()] {
			seen[target.String()] = true
			targets = append(targets, target)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
	if len(targets) == 0 {
		// A module without platform files is analyzed for the active target.
		targets = append(targets, lspActiveTarget(active, path))
	}
	return targets, strconv.FormatUint(hash.Sum64(), 16)
}
