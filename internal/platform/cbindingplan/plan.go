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
	"io/fs"
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
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil
	}
	defer handle.Close()
	// Ordinary source analysis also supports files without a project. Only
	// explicitly opened workspaces require the strict manifest precondition.
	manifest, err := handle.ReadFile(filepath.FromSlash(".sec/sec.toml"))
	if err != nil {
		return nil, nil
	}
	return ResolveSnapshotInRoot(handle, root, variant, platform, manifest)
}

// ResolveInRoot shares CLI/LSP binding selection while confining every read and
// selected module lookup to the retained project root, including symlink swaps.
// Rules: MD-021 correction §§8.3,8.5–8.7; harness design notes §§5(6)–(7),7(3).
func ResolveInRoot(handle *os.Root, root, variant string, platform target.Target) (*Plan, []string) {
	manifest, err := handle.ReadFile(filepath.FromSlash(".sec/sec.toml"))
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read project manifest: %v", err)}
	}
	return ResolveSnapshotInRoot(handle, root, variant, platform, manifest)
}

// ResolveSnapshotInRoot uses one manifest snapshot for profile and binding
// selection, preserving consistency when the source file changes concurrently.
// Rules: MD-021 correction §8.7; rules/projects/projects.md §49(1)-(3).
func ResolveSnapshotInRoot(handle *os.Root, root, variant string, platform target.Target, manifest []byte) (*Plan, []string) {

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
	data, err := handle.ReadFile(filepath.FromSlash(cbindings.DefaultFile))
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
	files, problems := moduleFilesInRoot(handle, root, pkg, resolution)
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
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer handle.Close()
	return moduleFilesInRoot(handle, root, pkg, resolution)
}

// moduleFilesInRoot lists selected production sources using the root's FS and
// checks each source through the root handle before exposing its path.
// Rules: MD-021 correction §8.6(1)-(2); harness design notes §5(6).
func moduleFilesInRoot(handle *os.Root, root string, pkg *cbindings.Package, resolution *cbindings.Resolution) ([]ModuleFile, []string) {
	common := map[string]bool{}
	for _, module := range pkg.CommonModules {
		common[module] = true
	}
	var files []ModuleFile
	var problems []string
	seen := map[string]bool{}
	for _, module := range resolution.Modules {
		layer := Environment
		if common[module] {
			layer = Common
		}
		directory := filepath.ToSlash(filepath.Join(pkg.SourceRoot, module))
		entries, err := fs.ReadDir(handle.FS(), directory)
		if err != nil {
			problems = append(problems, fmt.Sprintf("c-bindings.missing-module: module %q of environment %s cannot be read within the package root: %v", module, resolution.Environment, err))
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sec") || strings.HasSuffix(entry.Name(), "_test.sec") {
				continue
			}
			relative := directory + "/" + entry.Name()
			opened, err := handle.Open(relative)
			if err != nil {
				problems = append(problems, fmt.Sprintf("c-bindings.unsafe-path: %s: %v", relative, err))
				continue
			}
			info, err := opened.Stat()
			opened.Close()
			if err != nil || !info.Mode().IsRegular() {
				problems = append(problems, fmt.Sprintf("c-bindings.unsafe-path: %s is not a regular source", relative))
				continue
			}
			absolute, _ := filepath.Abs(filepath.Join(root, filepath.FromSlash(relative)))
			if seen[absolute] {
				problems = append(problems, fmt.Sprintf("c-bindings.duplicate-module: %s is selected by more than one module", absolute))
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

// ModuleName returns the module a parsed source declares, or "".
// Rules: rules/projects/modules.md — § 27.
func ModuleName(program *ast.Program) string {
	if program != nil {
		for _, statement := range program.Statements {
			if module, ok := statement.(*ast.ModuleStatement); ok && module != nil {
				return module.Path
			}
		}
	}
	return ""
}

// ModuleIdentityProblems rejects selected binding modules whose module
// identity collides: two selected module directories declaring the same
// module, or a binding module named like a module of the program from
// another directory. bindingModules maps each plan file to the module its
// source declares. Binding modules join the program by name, so a collision
// would merge unrelated declarations.
// Rules: MD-021 correction § 8.6(1); rules/projects/modules.md — § 27.
func ModuleIdentityProblems(plan *Plan, bindingModules map[string]string, program *ast.Program) []string {
	directories := map[string]string{}
	bindingFiles := map[string]bool{}
	var problems []string
	for _, file := range plan.Files {
		bindingFiles[filepath.Clean(file.Path)] = true
		name, directory := bindingModules[file.Path], filepath.Dir(file.Path)
		if name == "" {
			continue
		}
		if previous, seen := directories[name]; seen && previous != directory {
			problems = append(problems, fmt.Sprintf("c-bindings.module-collision: module %q is declared by selected binding directories %s and %s", name, previous, directory))
			continue
		}
		directories[name] = directory
	}
	if program != nil {
		reported := map[string]bool{}
		for _, statement := range program.Statements {
			module, ok := statement.(*ast.ModuleStatement)
			if !ok || module == nil || bindingFiles[filepath.Clean(module.Token.File)] {
				continue
			}
			directory, binding := directories[module.Path]
			if binding && filepath.Dir(module.Token.File) != directory && !reported[module.Path] {
				reported[module.Path] = true
				problems = append(problems, fmt.Sprintf("c-bindings.module-collision: binding module %q (%s) has the same identity as the program module declared in %s", module.Path, directory, module.Token.File))
			}
		}
	}
	sort.Strings(problems)
	return problems
}
