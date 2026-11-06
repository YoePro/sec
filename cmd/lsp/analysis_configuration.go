package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
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
	return newLSPAnalyzerForInputsAtDepth(uri, program, overlay, lspAnalysisDepthWithOverlay(pathFromURI(uri), overlay))
}

// newLSPAnalyzerForInputsAtDepth binds the selected snapshot budget to the shared
// target-aware analyzer; no safety proof is disabled by a reduced depth.
// Rules: rules/compiler/compiler_analysis.md — §§60(1–5),62(1–3);
// rules/tooling/lsp.md — "Configuration", "Snapshots".
func newLSPAnalyzerForInputsAtDepth(uri string, program *ast.Program, overlay sourceOverlay, depth sema.AnalysisDepth) *sema.Analyzer {
	analyzer := newTargetLSPAnalyzer(uri, program, overlay, depth)
	analyzer.SetHardwareModuleContext(lspHardwareModuleContext(program, pathFromURI(uri)))
	analyzer.SetDiagnosticRules(lspDiagnosticRules(pathFromURI(uri)))
	return analyzer
}

// lspDiagnosticRules reads the governing project's `[diagnostics.rules]`
// policy, the same policy the compiler CLI applies, so configurable
// diagnostics are disabled or promoted identically in the editor
// (rules/tooling/diagnostics.md § 18; rules/tooling/lsp.md "Configuration").
func lspDiagnosticRules(sourcePath string) diagnostics.RulePolicy {
	if sourcePath == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(findProjectRoot(sourcePath), ".sec", "sec.toml"))
	if err != nil {
		return nil
	}
	return diagnostics.ParseRulePolicy(string(data))
}

func newTargetLSPAnalyzer(uri string, program *ast.Program, overlay sourceOverlay, depth sema.AnalysisDepth) *sema.Analyzer {
	programs := []*ast.Program{program}
	sourcePath := pathFromURI(uri)
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
	// Without a declared project target, including settings-only manifests,
	// CLI and LSP use the same host plan so scalar widths and C ABI facts
	// remain resolved. Failed declared-target selection never takes this path.
	// Rules: rules/tooling/lsp.md — Target-aware analysis; rules/platform/ffi.md — §5.
	if definition, found := platformtarget.Find(platformtarget.Host()); found && lspAllowsHostScalarPlan(sourcePath, overlay) {
		if plan, err := definition.ScalarPlan(); err == nil {
			analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, depth)
			analyzer.SetAllocationProfile(lspAllocationCapabilities(sourcePath).Profile)
			analyzer.SetPitfallSourcePriority([]string{sourcePath})
			return analyzer
		}
	}
	analyzer := sema.NewAnalyzerWithDepth(depth)
	analyzer.SetAllocationProfile(lspAllocationCapabilities(sourcePath).Profile)
	analyzer.SetPitfallSourcePriority([]string{sourcePath})
	return analyzer
}

// lspAllowsHostScalarPlan permits the CLI-default host plan for a stand-alone
// source with optional editor settings, while an explicit target selection or
// declared target/variant (including invalid or ambiguous ones) prevents guessing.
// Rules: rules/tooling/lsp.md — Diagnostics (target-aware source selection),
// Configuration, Snapshots; rules/platform/platform_model.md — §26.
func lspAllowsHostScalarPlan(sourcePath string, overlay sourceOverlay) bool {
	if sourcePath == "" {
		return true
	}
	root := normalizedSourcePath(findProjectRoot(sourcePath))
	if _, selected := overlay.Targets[root]; selected {
		return false
	}
	_, text, found := lspProjectManifest(sourcePath, overlay)
	if !found {
		_, err := os.Stat(filepath.Join(root, ".sec", "sec.toml"))
		return os.IsNotExist(err)
	}
	var settings map[string]any
	if _, err := toml.Decode(text, &settings); err != nil {
		return false
	}
	_, hasTargets := settings["target"]
	_, hasVariants := settings["variant"]
	return !hasTargets && !hasVariants
}
