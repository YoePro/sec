// Package cbindingplan resolves the C binding environment of one Variant's
// CompilationPlan and lists the selected binding module sources with their
// semantic layer. CLI, LSP and a future tooling harness share it, so every
// surface selects the same environment (MD-021 correction §§ 8.6(5), 8.7).
//
// Rules: rules/platform/ffi.md — § 56; rules/platform/platform_model.md — § 31;
// rules/corrections/applied/MD-021-c-binding-environment-complete-proposal-20261120-20261121.md — §§ 8.3, 8.5-8.7.
package cbindingplan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/platform/cbindings"
	"sec/internal/platform/target"
	"sec/internal/project"
)

// Layer is the semantic layer of a selected binding module (§ 8.6(2)).
type Layer uint8

const (
	Common Layer = iota + 1
	Environment
)

// ModuleFile is one production source of a selected binding module.
type ModuleFile struct {
	Path  string
	Layer Layer
}

// Plan is the frozen binding part of one CompilationPlan.
type Plan struct {
	Variant    string
	Resolution *cbindings.Resolution
	Files      []ModuleFile
}

// Identity names the selected package@version/environment.
func (plan *Plan) Identity() string {
	return plan.Resolution.PackageID + "@" + plan.Resolution.PackageVersion + "/" + plan.Resolution.Environment
}

// Resolve selects the binding environment of the named Variant of the
// project at root, or of the only Variant matching platform when variant is
// empty. A Variant that names c_bindings needs a resolvable environment and
// otherwise returns problems (fail closed); a Variant without a selector
// gets the project's unique compatible environment or no plan, so its
// binding names stay unresolved. No fact is taken from the host.
// Rules: MD-021 correction §§ 8.3(3)-(5), 8.5, 8.7(1)-(3).
func Resolve(root, variant string, platform target.Target) (*Plan, []string) {
	manifest, err := os.ReadFile(filepath.Join(root, ".sec", "sec.toml"))
	if err != nil {
		return nil, nil
	}
	if variant == "" {
		if variant = UniqueVariant(string(manifest), platform); variant == "" {
			return nil, nil
		}
	}
	selectors, errs := project.ReadVariantCBindings(string(manifest), variant)
	if len(errs) != 0 {
		problems := make([]string, len(errs))
		for index, err := range errs {
			problems[index] = err.Error()
		}
		return nil, problems
	}
	fail := func(diagnostics []cbindings.Diagnostic) (*Plan, []string) {
		if !selectors.Selected {
			return nil, nil
		}
		problems := make([]string, len(diagnostics))
		for index, d := range diagnostics {
			problems[index] = fmt.Sprintf("Variant %s: %s: %s", variant, d.ID, d.Message)
		}
		return nil, problems
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cbindings.DefaultFile)))
	if err != nil {
		return fail([]cbindings.Diagnostic{{ID: "c-bindings.unknown-package", Message: fmt.Sprintf("binding package %q is not available: %s is missing and no compiler-distributed packages exist yet", selectors.BindingsID, cbindings.DefaultFile)}})
	}
	pkg, diagnostics := cbindings.Parse(string(data))
	if len(diagnostics) != 0 {
		return fail(diagnostics)
	}
	request := cbindings.Request{Variant: variant, OS: selectors.OS, Arch: selectors.Arch, ABI: selectors.ABI, CRuntime: selectors.CRuntime, CRuntimeVersion: selectors.CRuntimeVersion}
	if selectors.Selected {
		request.Selector = &cbindings.Selector{ID: selectors.BindingsID, Version: selectors.BindingsVersion, Environment: selectors.BindingsEnvironment}
	}
	resolution, diagnostics := cbindings.Resolve(request, []cbindings.Candidate{{Package: pkg, Origin: "project"}})
	if len(diagnostics) != 0 || resolution == nil {
		return fail(diagnostics)
	}
	files, problems := moduleFiles(root, pkg, resolution)
	if len(problems) != 0 {
		return nil, problems
	}
	return &Plan{Variant: variant, Resolution: resolution, Files: files}, nil
}

// UniqueVariant returns the only Variant whose os/arch equal platform, or
// "" when none or several do.
// Rules: rules/projects/projects.md — §§ 19-21; MD-021 correction § 8.7(2).
func UniqueVariant(manifest string, platform target.Target) string {
	doc, err := project.ParseManifestDocument(manifest)
	if err != nil {
		return ""
	}
	variants, err := doc.Table("variant")
	if err != nil {
		return ""
	}
	found := ""
	for name, value := range variants {
		table, ok := value.(map[string]any)
		if !ok {
			continue
		}
		osName, _ := table["os"].(string)
		arch, _ := table["arch"].(string)
		if candidate, ok := target.Parse(osName + "-" + arch); ok && candidate == platform {
			if found != "" {
				return ""
			}
			found = name
		}
	}
	return found
}

// moduleFiles lists every production source of the selected module
// directories under bindings.source_root with its layer. A missing
// directory, a symbolic-link escape from the package root or a file reached
// through two modules rejects the environment.
// Rules: MD-021 correction §§ 2(5), 8.6(1)-(2); rules/projects/modules.md — § 27.
func moduleFiles(root string, pkg *cbindings.Package, resolution *cbindings.Resolution) ([]ModuleFile, []string) {
	common := map[string]bool{}
	for _, module := range pkg.CommonModules {
		common[module] = true
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, []string{fmt.Sprintf("c-bindings.missing-module: cannot resolve the package root: %v", err)}
	}
	var files []ModuleFile
	var problems []string
	seen := map[string]bool{}
	for _, module := range resolution.Modules {
		layer := Environment
		if common[module] {
			layer = Common
		}
		directory := filepath.Join(root, filepath.FromSlash(pkg.SourceRoot), filepath.FromSlash(module))
		real, err := filepath.EvalSymlinks(directory)
		if err != nil {
			problems = append(problems, fmt.Sprintf("c-bindings.missing-module: module %q of environment %s does not exist at %s", module, resolution.Environment, directory))
			continue
		}
		if relative, err := filepath.Rel(realRoot, real); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			problems = append(problems, fmt.Sprintf("c-bindings.unsafe-path: module %q escapes the package root", module))
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(directory, "*.sec"))
		sort.Strings(matches)
		for _, file := range matches {
			if info, err := os.Stat(file); err != nil || info.IsDir() || strings.HasSuffix(file, "_test.sec") {
				continue
			}
			absolute, _ := filepath.Abs(file)
			if seen[absolute] {
				problems = append(problems, fmt.Sprintf("c-bindings.duplicate-module: %s is selected by more than one module", file))
				continue
			}
			seen[absolute] = true
			files = append(files, ModuleFile{Path: absolute, Layer: layer})
		}
	}
	return files, problems
}

// MissingFiles lists the plan's sources whose statements are not yet part
// of program, so a program analyzed more than once never receives the same
// binding declarations twice.
// Rules: MD-021 correction § 8.6(2) (same-layer duplicates are errors).
func MissingFiles(program *ast.Program, plan *Plan) []ModuleFile {
	present := map[string]bool{}
	if program != nil {
		for _, statement := range program.Statements {
			if module, ok := statement.(*ast.ModuleStatement); ok && module != nil {
				present[filepath.Clean(module.Token.File)] = true
			}
		}
	}
	var missing []ModuleFile
	for _, file := range plan.Files {
		if !present[filepath.Clean(file.Path)] {
			missing = append(missing, file)
		}
	}
	return missing
}
