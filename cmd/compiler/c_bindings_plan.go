package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/ast"
	"sec/internal/platform/cbindingplan"
	platformtarget "sec/internal/platform/target"
	"sec/internal/project"
	"sec/internal/sema"
)

// cliCBindingVariant names the Variant whose CompilationPlan is being
// analyzed. Empty means the command analyzes a source for a bare target;
// the binding environment is then taken from the one Variant whose os/arch
// equal that target, if exactly one exists.
var cliCBindingVariant string

// cliCBindingPlan is the frozen binding part of one plan: the resolved
// environment and the parsed sources of its selected modules.
type cliCBindingPlan struct {
	plan     *cbindingplan.Plan
	surface  sema.CBindingSurface
	programs []*ast.Program
}

// resolveCLICBindingPlan resolves the selected Variant's C binding
// environment before Sema and parses its module sources (MD-021
// § 8.6(5)); see cbindingplan.Resolve for the selection rules.
// Rules: rules/platform/ffi.md — § 56; MD-021 correction §§ 8.3(3)-(5), 8.5-8.7.
func resolveCLICBindingPlan(sourceFiles []string, target CompilerTarget) (*cliCBindingPlan, []string) {
	if len(sourceFiles) == 0 {
		return nil, nil
	}
	plan, problems := cbindingplan.Resolve(findProjectRoot(sourceFiles[0]), cliCBindingVariant, target)
	if plan == nil {
		return nil, problems
	}
	result := &cliCBindingPlan{plan: plan, surface: cBindingSurface(plan)}
	for _, file := range plan.Files {
		source, _ := parseSourceFileWithDiagnostics(file.Path)
		if source.Program == nil || len(source.Errors) != 0 {
			problems = append(problems, fmt.Sprintf("c-bindings.malformed-package: %s does not parse: %s", file.Path, strings.Join(source.Errors, "; ")))
			continue
		}
		result.programs = append(result.programs, source.Program)
	}
	if len(problems) != 0 {
		return nil, problems
	}
	return result, nil
}

// cBindingSurface converts a plan into the Sema binding surface.
// Rules: MD-021 correction § 8.6(2), (5).
func cBindingSurface(plan *cbindingplan.Plan) sema.CBindingSurface {
	surface := sema.CBindingSurface{Identity: plan.Identity(), Files: map[string]sema.CBindingLayer{}}
	for _, file := range plan.Files {
		layer := sema.CBindingEnvironment
		if file.Layer == cbindingplan.Common {
			layer = sema.CBindingCommon
		}
		surface.Files[file.Path] = layer
	}
	return surface
}

// applyCLICBindingPlan checks binding module identities against the
// program, then adds the binding sources to it and hands the frozen surface
// to Sema. On a collision nothing is added and the problems are returned.
// Rules: MD-021 correction §§ 8.6(1), 8.6(5).
func applyCLICBindingPlan(plan *cliCBindingPlan, program *ast.Program, analyzer *sema.Analyzer) []string {
	if plan == nil {
		return nil
	}
	modules := map[string]string{}
	for index, source := range plan.programs {
		modules[plan.plan.Files[index].Path] = cbindingplan.ModuleName(source)
	}
	if problems := cbindingplan.ModuleIdentityProblems(plan.plan, modules, program); len(problems) != 0 {
		return problems
	}
	missing := map[string]bool{}
	for _, file := range cbindingplan.MissingFiles(program, plan.plan) {
		missing[file.Path] = true
	}
	for index, source := range plan.programs {
		if missing[plan.plan.Files[index].Path] {
			program.Statements = append(program.Statements, source.Statements...)
		}
	}
	analyzer.SetCBindingSurface(plan.surface)
	return nil
}

// cliPlanTarget names the Project Target of the plan being built; empty for
// commands that analyze a source for a bare target.
var cliPlanTarget string

// validateCLIPlanSelection reads the MD-139 profile and allocation
// selections of the plan's Target–Variant pair and resolves them against
// the target's frozen profile, so configuration errors are reported before
// analysis. The resolved profile is not yet a projection source for
// allocation capabilities: every provider catalog is empty until the owner
// decides them (platform.target-profiles).
// Rules: rules/platform/target_profiles.md — § 41; rules/projects/projects.md — § 19(11)-(16);
// MD-139 correction §§ 2-5.
func validateCLIPlanSelection(sourceFiles []string, target CompilerTarget, definition platformtarget.Definition) []string {
	if len(sourceFiles) == 0 {
		return nil
	}
	manifest, err := os.ReadFile(filepath.Join(findProjectRoot(sourceFiles[0]), ".sec", "sec.toml"))
	if err != nil {
		return nil
	}
	variant := cliCBindingVariant
	if variant == "" {
		variant = cbindingplan.UniqueVariant(string(manifest), target)
	}
	if variant == "" && cliPlanTarget == "" {
		return nil
	}
	selection, errs := project.ResolvePlanSelection(string(manifest), cliPlanTarget, variant)
	var problems []string
	for _, err := range errs {
		problems = append(problems, err.Error())
	}
	if len(problems) != 0 || selection.Profile == "" && selection.AllocationMode == "" && selection.Provider == "" {
		return problems
	}
	if _, err := platformtarget.ResolveTargetProfile(definition, selection.Profile, string(selection.AllocationMode), selection.Provider); err != nil {
		problems = append(problems, fmt.Sprintf("Variant %s: %v", variant, err))
	}
	return problems
}
