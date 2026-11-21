// Package cbindings models, validates and resolves target C binding
// environments: the `.sec/platform/c-bindings.toml` metadata of MD-021.
//
// Rules: rules/platform/ffi.md — § 56; rules/platform/platform_model.md — § 31;
// rules/projects/modules.md — § 27; rules/tooling/cli.md — § 8;
// rules/corrections/applied/MD-021-c-binding-environment-complete-proposal-20261120-20261121.md.
package cbindings

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"sec/internal/platform/target"
)

// DefaultFile is the portable metadata location inside a binding package root.
const DefaultFile = ".sec/platform/c-bindings.toml"

// SchemaVersion is the only supported metadata grammar version.
const SchemaVersion = 1

// Package is one parsed binding package (correction § 2).
type Package struct {
	SchemaVersion int
	ID            string
	Version       string
	SourceRoot    string
	CommonModules []string
	Environments  map[string]Environment
}

// Environment is one named compatibility environment of a package.
type Environment struct {
	Name             string
	OS               string
	Arch             string
	ABI              string
	CRuntime         string
	CRuntimeVersions string
	Modules          []string
}

// Diagnostic is a stable, machine-readable finding. IDs are stable within
// the c-bindings JSON contracts (rules/tooling/cli.md § 8(3)).
type Diagnostic struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func diagnostic(id, format string, args ...any) Diagnostic {
	return Diagnostic{ID: id, Message: fmt.Sprintf(format, args...)}
}

// rawPackage mirrors schema v1 for decoding.
type rawPackage struct {
	SchemaVersion int                       `toml:"schema_version"`
	Bindings      rawBindings               `toml:"bindings"`
	Modules       rawModules                `toml:"modules"`
	Environments  map[string]rawEnvironment `toml:"environments"`
}

type rawBindings struct {
	ID         string `toml:"id"`
	Version    string `toml:"version"`
	SourceRoot string `toml:"source_root"`
}

type rawModules struct {
	Common []string `toml:"common"`
}

type rawEnvironment struct {
	OS               string   `toml:"os"`
	Arch             string   `toml:"arch"`
	ABI              string   `toml:"abi"`
	CRuntime         string   `toml:"c_runtime"`
	CRuntimeVersions string   `toml:"c_runtime_versions"`
	Modules          []string `toml:"modules"`
}

// Parse decodes and structurally validates metadata text. A returned package
// is usable only when no diagnostic is returned. Unknown keys and unknown
// schema versions are rejected (correction § 2(8)).
// Rules: rules/platform/ffi.md — § 56(1); MD-021 correction §§ 2(2)-(8), 5(7), 8.9(1).
func Parse(text string) (*Package, []Diagnostic) {
	var raw rawPackage
	metadata, err := toml.Decode(text, &raw)
	if err != nil {
		return nil, []Diagnostic{diagnostic("c-bindings.invalid-toml", "invalid TOML: %v", err)}
	}
	var diagnostics []Diagnostic
	undecoded := metadata.Undecoded()
	keys := make([]string, 0, len(undecoded))
	for _, key := range undecoded {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)
	for _, key := range keys {
		diagnostics = append(diagnostics, diagnostic("c-bindings.unknown-key", "unknown key %q in schema version 1", key))
	}
	pkg := &Package{
		SchemaVersion: raw.SchemaVersion, ID: raw.Bindings.ID, Version: raw.Bindings.Version,
		SourceRoot: raw.Bindings.SourceRoot, CommonModules: raw.Modules.Common, Environments: map[string]Environment{},
	}
	for name, environment := range raw.Environments {
		pkg.Environments[name] = Environment{Name: name, OS: environment.OS, Arch: environment.Arch, ABI: environment.ABI,
			CRuntime: environment.CRuntime, CRuntimeVersions: environment.CRuntimeVersions, Modules: environment.Modules}
	}
	diagnostics = append(diagnostics, pkg.Validate()...)
	return pkg, diagnostics
}

// Validate checks the structural contract of schema v1: supported schema,
// required identity fields, MAJOR.MINOR.PATCH package version, safe source
// root and module paths, canonical environment selectors, parseable runtime
// constraints and no duplicate module in one list.
// Rules: MD-021 correction §§ 2(3)-(5), 5(7), 8.4, 8.6(1); rules/projects/modules.md — § 27.
func (pkg *Package) Validate() []Diagnostic {
	var diagnostics []Diagnostic
	if pkg.SchemaVersion != SchemaVersion {
		diagnostics = append(diagnostics, diagnostic("c-bindings.unsupported-schema", "schema_version %d is not supported (expected %d)", pkg.SchemaVersion, SchemaVersion))
	}
	if pkg.ID == "" {
		diagnostics = append(diagnostics, diagnostic("c-bindings.missing-field", "bindings.id is required"))
	}
	if _, err := ParsePackageVersion(pkg.Version); err != nil {
		diagnostics = append(diagnostics, diagnostic("c-bindings.invalid-version", "bindings.version: %v", err))
	}
	if err := checkRelativePath(pkg.SourceRoot); err != nil {
		diagnostics = append(diagnostics, diagnostic("c-bindings.unsafe-path", "bindings.source_root: %v", err))
	}
	diagnostics = append(diagnostics, checkModules("modules.common", pkg.CommonModules)...)
	for _, name := range pkg.EnvironmentNames() {
		environment := pkg.Environments[name]
		scope := "environments." + name
		for field, value := range map[string]string{"os": environment.OS, "arch": environment.Arch, "abi": environment.ABI, "c_runtime": environment.CRuntime} {
			if value == "" {
				diagnostics = append(diagnostics, diagnostic("c-bindings.missing-field", "%s.%s is required", scope, field))
			}
		}
		if environment.OS != "" && !registered(environment.OS, func(d target.Definition) string { return d.OS }) && target.NormalizeOS(environment.OS) == environment.OS {
			diagnostics = append(diagnostics, diagnostic("c-bindings.unknown-target", "%s.os %q is not a registered target operating system", scope, environment.OS))
		}
		if environment.Arch != "" && !registered(environment.Arch, func(d target.Definition) string { return d.Arch }) && target.NormalizeArch(environment.Arch) == environment.Arch {
			diagnostics = append(diagnostics, diagnostic("c-bindings.unknown-target", "%s.arch %q is not a registered target architecture", scope, environment.Arch))
		}
		if environment.OS != "" && target.NormalizeOS(environment.OS) != environment.OS {
			diagnostics = append(diagnostics, diagnostic("c-bindings.noncanonical-target", "%s.os %q is not canonical; write %q", scope, environment.OS, target.NormalizeOS(environment.OS)))
		}
		if environment.Arch != "" && target.NormalizeArch(environment.Arch) != environment.Arch {
			diagnostics = append(diagnostics, diagnostic("c-bindings.noncanonical-target", "%s.arch %q is not canonical; write %q", scope, environment.Arch, target.NormalizeArch(environment.Arch)))
		}
		if environment.CRuntimeVersions != "" {
			if _, err := ParseConstraint(environment.CRuntimeVersions, false); err != nil {
				diagnostics = append(diagnostics, diagnostic("c-bindings.invalid-constraint", "%s.c_runtime_versions: %v", scope, err))
			}
		}
		if environment.Modules == nil {
			diagnostics = append(diagnostics, diagnostic("c-bindings.missing-field", "%s.modules is required (an empty array is allowed)", scope))
		}
		diagnostics = append(diagnostics, checkModules(scope+".modules", environment.Modules)...)
	}
	return diagnostics
}

// registered reports whether a canonical spelling names a field of some
// registered target definition; an unregistered name is never canonical.
// Rules: MD-021 correction §§ 5(7), 8.5(4); rules/platform/platform_model.md — target registry.
func registered(value string, field func(target.Definition) string) bool {
	for _, definition := range target.Definitions() {
		if field(definition) == value {
			return true
		}
	}
	return false
}

// Resolvable reports whether the package can be selected by a plan: a
// zero-environment package is only an editing draft (correction § 2(8)).
// Rules: MD-021 correction §§ 2(8), 8.9(1).
func (pkg *Package) Resolvable() bool { return len(pkg.Environments) > 0 }

// EnvironmentNames lists environment keys in canonical sorted order.
// Rules: MD-021 correction § 8.9(4).
func (pkg *Package) EnvironmentNames() []string {
	names := make([]string, 0, len(pkg.Environments))
	for name := range pkg.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// checkModules validates one module list: safe relative paths, no duplicates.
// Rules: rules/projects/modules.md — § 27(1); MD-021 correction § 8.6(1).
func checkModules(scope string, modules []string) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[string]bool{}
	for _, module := range modules {
		if err := checkRelativePath(module); err != nil {
			diagnostics = append(diagnostics, diagnostic("c-bindings.unsafe-path", "%s %q: %v", scope, module, err))
			continue
		}
		if seen[module] {
			diagnostics = append(diagnostics, diagnostic("c-bindings.duplicate-module", "%s lists %q more than once", scope, module))
		}
		seen[module] = true
	}
	return diagnostics
}

// checkRelativePath accepts a non-empty, normalized, forward-slash relative
// path that cannot escape its root.
// Rules: MD-021 correction §§ 2(5), 8.6(1).
func checkRelativePath(value string) error {
	switch {
	case value == "":
		return fmt.Errorf("path is empty")
	case strings.Contains(value, "\\"):
		return fmt.Errorf("path %q must use '/' separators", value)
	case strings.HasPrefix(value, "/") || (len(value) > 1 && value[1] == ':'):
		return fmt.Errorf("path %q is absolute", value)
	case path.Clean(value) != value:
		return fmt.Errorf("path %q is not normalized (write %q)", value, path.Clean(value))
	case value == ".." || strings.HasPrefix(value, "../"):
		return fmt.Errorf("path %q escapes the package root", value)
	}
	return nil
}
