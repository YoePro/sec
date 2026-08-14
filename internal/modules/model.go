// Package modules owns compiler-wide module identities and logical import-path
// rules. It intentionally has no dependency on the parser, Sema, or a host
// filesystem so command-line and interactive consumers can share it.
package modules

import (
	"fmt"
	"strings"
)

type ModuleName string

type ModuleIdentity struct {
	ImportRootIdentity  string
	CanonicalImportPath string
}

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

func (identity ModuleIdentity) String() string {
	return identity.ImportRootIdentity + ":" + identity.CanonicalImportPath
}

type ModuleInstance struct {
	Identity          ModuleIdentity
	CompilationPlanID string
}

func (instance ModuleInstance) String() string {
	if instance.CompilationPlanID == "" {
		return instance.Identity.String()
	}
	return instance.Identity.String() + "@" + instance.CompilationPlanID
}

// ValidateImportPath checks the canonical logical spelling before any root is
// resolved. Logical paths always use slash separators and never inherit host
// filesystem normalization behavior.
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

func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
