// Package modules owns compiler-wide module identities and logical import-path
// rules. It intentionally has no dependency on the parser, Sema, or a host
// filesystem so command-line and interactive consumers can share it.
package modules

import (
	"fmt"
	"strings"
)

// ModuleName is the short identifier declared by a Sec source module. Per
// rules/projects/modules.md it is not a globally unique module identity.
type ModuleName string

// ModuleIdentity is the root-qualified logical identity defined by
// rules/projects/modules.md. It is independent of host filesystem spelling.
type ModuleIdentity struct {
	ImportRootIdentity  string
	CanonicalImportPath string
}

// NewModuleIdentity validates a canonical logical import path and combines it
// with its resolved import-root identity as required by
// rules/projects/modules.md.
func NewModuleIdentity(importRootIdentity, importPath string) (ModuleIdentity, error) {
	if importRootIdentity == "" {
		return ModuleIdentity{}, fmt.Errorf("module import-root identity is empty")
	}
	if err := ValidateImportPath(importPath); err != nil {
		return ModuleIdentity{}, err
	}
	return ModuleIdentity{
		ImportRootIdentity:  importRootIdentity,
		CanonicalImportPath: importPath,
	}, nil
}

// String returns the human-readable root-qualified module identity.
func (identity ModuleIdentity) String() string {
	return identity.ImportRootIdentity + ":" + identity.CanonicalImportPath
}

// ModuleInstance identifies one logical module within a concrete compilation
// plan, following the ModuleIdentity/ModuleInstance split in
// rules/projects/modules.md.
type ModuleInstance struct {
	Identity          ModuleIdentity
	CompilationPlanID string
}

// String returns the logical identity with its optional compilation-plan
// discriminator.
func (instance ModuleInstance) String() string {
	if instance.CompilationPlanID == "" {
		return instance.Identity.String()
	}
	return instance.Identity.String() + "@" + instance.CompilationPlanID
}

// CrossModuleDeclarationIdentity is the canonical semantic owner identity from
// rules/projects/modules.md. The short source ModuleName and any file-local
// import alias are intentionally absent: neither is globally unique and neither
// changes which logical module owns a declaration.
type CrossModuleDeclarationIdentity struct {
	Module      ModuleIdentity
	Declaration string
}

// NewCrossModuleDeclarationIdentity validates and constructs the canonical
// module-owned declaration identity required by rules/projects/modules.md. It
// deliberately accepts no source qualifier or import alias because neither
// changes semantic ownership.
func NewCrossModuleDeclarationIdentity(module ModuleIdentity, declaration string) (CrossModuleDeclarationIdentity, error) {
	if module.ImportRootIdentity == "" {
		return CrossModuleDeclarationIdentity{}, fmt.Errorf("cross-module declaration identity has an empty import-root identity")
	}
	if err := ValidateImportPath(module.CanonicalImportPath); err != nil {
		return CrossModuleDeclarationIdentity{}, fmt.Errorf("cross-module declaration identity has an invalid module path: %w", err)
	}
	if declaration == "" {
		return CrossModuleDeclarationIdentity{}, fmt.Errorf("cross-module declaration identity has an empty declaration identity")
	}
	if strings.ContainsRune(declaration, '\x00') {
		return CrossModuleDeclarationIdentity{}, fmt.Errorf("declaration identity contains NUL")
	}
	return CrossModuleDeclarationIdentity{Module: module, Declaration: declaration}, nil
}

// String returns a readable identity for diagnostics and debugging. Stable
// persistence uses StableKey instead.
func (identity CrossModuleDeclarationIdentity) String() string {
	return identity.Module.String() + "::" + identity.Declaration
}

// StableKey encodes the rules/projects/modules.md identity without delimiter
// ambiguity, even when an internal declaration identity uses punctuation. It
// may be retained by Semantic IR, caches, ABI planning, and linking without
// allowing those layers to redefine semantic ownership.
func (identity CrossModuleDeclarationIdentity) StableKey() string {
	return stableIdentityComponent(identity.Module.ImportRootIdentity) +
		stableIdentityComponent(identity.Module.CanonicalImportPath) +
		stableIdentityComponent(identity.Declaration)
}

// stableIdentityComponent length-prefixes one opaque identity component.
func stableIdentityComponent(value string) string {
	return fmt.Sprintf("%d:%s", len(value), value)
}

// ValidateImportPath checks the rules/projects/modules.md canonical logical
// spelling before any root is resolved. Logical paths always use slash
// separators and never inherit host filesystem normalization behavior.
func ValidateImportPath(path string) error {
	if path == "" {
		return fmt.Errorf("import path is empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("import path contains NUL")
	}
	if strings.Contains(path, "\\") {
		return fmt.Errorf("import path must use '/' separators")
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("import path must not be absolute")
	}
	if len(path) >= 3 && isASCIILetter(path[0]) && path[1] == ':' && path[2] == '/' {
		return fmt.Errorf("import path must not be an absolute host path")
	}
	if strings.HasSuffix(path, "/") {
		return fmt.Errorf("import path must not end with '/'")
	}
	for _, component := range strings.Split(path, "/") {
		switch component {
		case "":
			return fmt.Errorf("import path contains an empty component")
		case ".", "..":
			return fmt.Errorf("import path must not contain %q traversal components", component)
		}
	}
	return nil
}

// isASCIILetter recognizes drive-letter prefixes without applying host locale
// or Unicode filesystem rules.
func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
