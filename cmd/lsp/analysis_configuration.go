package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sec/internal/ast"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
	"strconv"
	"strings"
)

// newLSPAnalyzer binds the active project plan and depth, prioritizing the
// requested document within the optional pitfall search budget. An active
// source target directive takes precedence over the manifest plan.
// Rules: rules/tooling/lsp.md — "Target-aware analysis", "Configuration";
// rules/analysis/pitfall_analysis.md — "Interactive analysis";
// rules/memory/allocation.md — §29(1); rules/platform/platform_model.md — source selection by target.
func newLSPAnalyzer(uri string, programs ...*ast.Program) *sema.Analyzer {
	var program *ast.Program
	if len(programs) > 0 {
		program = programs[0]
	}
	return newLSPAnalyzerForInputs(uri, program, sourceOverlay{})
}

// newLSPAnalyzerForInputs binds canonical target facts from the request snapshot.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Configuration, Snapshots;
// rules/types/types.md — int and uint, Binary floating-point types.
func newLSPAnalyzerForInputs(uri string, program *ast.Program, overlay sourceOverlay) *sema.Analyzer {
	programs := []*ast.Program{program}
	sourcePath := pathFromURI(uri)
	depth := lspAnalysisDepth(sourcePath)
	// Source selection and semantic target selection must use the same active
	// document, including unsaved directives; imported directives do not select
	// the active document's allocation profile.
	if len(programs) > 0 && programs[0] != nil {
		for _, statement := range programs[0].Statements {
			directive, ok := statement.(*ast.TargetDirective)
			if !ok || directive == nil || (directive.Token.File != "" && filepath.Clean(directive.Token.File) != filepath.Clean(sourcePath)) {
				continue
			}
			target := platformtarget.Target{OS: platformtarget.NormalizeOS(directive.OS), Arch: platformtarget.NormalizeArch(directive.Arch)}
			if definition, found := platformtarget.Find(target); found {
				if plan, err := definition.ScalarPlan(); err == nil {
					analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, depth)
					analyzer.SetPitfallSourcePriority([]string{sourcePath})
					return analyzer
				}
			}
			analyzer := sema.NewAnalyzerWithDepth(depth)
			analyzer.SetAllocationProfile("")
			analyzer.SetPitfallSourcePriority([]string{sourcePath})
			return analyzer
		}
	}
	if plan, ok := selectedLSPPlan(sourcePath, overlay); ok {
		analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, depth)
		analyzer.SetPitfallSourcePriority([]string{sourcePath})
		return analyzer
	}
	if plan, err := lspScalarPlan(sourcePath); err == nil {
		analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, depth)
		analyzer.SetPitfallSourcePriority([]string{sourcePath})
		return analyzer
	}
	analyzer := sema.NewAnalyzerWithDepth(depth)
	analyzer.SetAllocationProfile(lspAllocationCapabilities(sourcePath).Profile)
	analyzer.SetPitfallSourcePriority([]string{sourcePath})
	return analyzer
}

// lspAnalysisDepth resolves the normative project analysis setting per request.
// Rules: rules/projects/projects.md — §23; rules/tooling/lsp.md — "Configuration".
func lspAnalysisDepth(sourcePath string) sema.AnalysisDepth {
	depth := sema.AnalysisInteractive
	if sourcePath == "" {
		return depth
	}
	manifest := filepath.Join(findProjectRoot(sourcePath), ".sec", "sec.toml")
	configured, err := readLSPAnalysisDepth(manifest)
	if err == nil {
		return configured
	}
	return depth
}

// readLSPAnalysisDepth validates the existing analysis.lsp_depth manifest key.
// Rules: rules/tooling/lsp.md — "Configuration".
func readLSPAnalysisDepth(manifest string) (sema.AnalysisDepth, error) {
	file, err := os.Open(manifest)
	if err != nil {
		return "", err
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "analysis" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "lsp_depth" {
			continue
		}
		value = strings.TrimSpace(value)
		if comment := strings.Index(value, "#"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid analysis.lsp_depth in %s: %w", manifest, err)
		}
		return sema.ParseAnalysisDepth(unquoted)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sema.AnalysisInteractive, nil
}
