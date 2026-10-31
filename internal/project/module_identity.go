package project

import (
	"os"
	"path/filepath"
	"strings"

	"sec/internal/modules"
)

// Compiler-owned import-root identities. Project roots use projectRootPrefix
// plus the stable Project UUID; a source outside any Project uses the
// compilation-local anonymous root.
// Rules: rules/projects/modules.md — §§2.2,8,19; rules/projects/projects.md — §§13(9),16(3).
const (
	PlatformImportRoot  = "platform"
	StdlibImportRoot    = "std"
	CoreImportRoot      = "core"
	RuntimeImportRoot   = "runtime"
	AnonymousImportRoot = "anonymous"
	projectRootPrefix   = "project:"
)

// SourceModuleResolver derives the resolver-owned ModuleIdentity of a source
// file from its directory: compiler-provided sources below <compiler>/sec use
// the platform, std, core and runtime roots; a Project source uses its
// Project UUID root and its Project-relative directory; any other source uses
// the anonymous root relative to the entry directory. Files directly in a
// root form its root module (empty path). Identity never comes from the
// source's short module name.
type SourceModuleResolver struct {
	compilerRoot string
	entryRoot    string
	projects     map[string]projectRoot
}

type projectRoot struct {
	dir, identity string
	found         bool
}

// NewSourceModuleResolver takes the directory holding the compiler's `sec/`
// source tree ("" when unavailable) and the entry directory used as the
// anonymous root for sources outside a Project.
func NewSourceModuleResolver(compilerRoot, entryRoot string) *SourceModuleResolver {
	resolver := &SourceModuleResolver{projects: map[string]projectRoot{}}
	if compilerRoot != "" {
		if absolute, err := filepath.Abs(compilerRoot); err == nil {
			resolver.compilerRoot = filepath.Join(absolute, "sec")
		}
	}
	if absolute, err := filepath.Abs(entryRoot); err == nil {
		resolver.entryRoot = absolute
	}
	return resolver
}

// ModuleIdentity returns the identity of the module containing file, or
// false when the file belongs to no resolvable root (fail closed).
func (resolver *SourceModuleResolver) ModuleIdentity(file string) (modules.ModuleIdentity, bool) {
	absolute, err := filepath.Abs(file)
	if err != nil {
		return modules.ModuleIdentity{}, false
	}
	dir := filepath.Dir(absolute)
	if resolver.compilerRoot != "" {
		if relative, ok := relativeDir(resolver.compilerRoot, dir); ok && relative != "" {
			parts := strings.SplitN(relative, "/", 2)
			rest := ""
			if len(parts) == 2 {
				rest = parts[1]
			}
			switch parts[0] {
			case "platform":
				return identityFor(PlatformImportRoot, relative)
			case "stdlib":
				return identityFor(StdlibImportRoot, rest)
			case "core":
				return identityFor(CoreImportRoot, rest)
			case "runtime":
				return identityFor(RuntimeImportRoot, rest)
			}
		}
	}
	if root := resolver.project(dir); root.found {
		if root.identity == "" {
			return modules.ModuleIdentity{}, false
		}
		if relative, ok := relativeDir(root.dir, dir); ok {
			return identityFor(root.identity, relative)
		}
		return modules.ModuleIdentity{}, false
	}
	if resolver.entryRoot == "" {
		return modules.ModuleIdentity{}, false
	}
	relative, ok := relativeDir(resolver.entryRoot, dir)
	if !ok {
		return modules.ModuleIdentity{}, false
	}
	return identityFor(AnonymousImportRoot, relative)
}

// project finds and caches the nearest Project manifest above dir. Only its
// [project] identity is needed; a manifest whose identity section is invalid
// yields no Project root (and the source no other root), while its
// diagnostics stay with the manifest validators.
func (resolver *SourceModuleResolver) project(dir string) projectRoot {
	if root, ok := resolver.projects[dir]; ok {
		return root
	}
	root := projectRoot{}
	for current := dir; ; current = filepath.Dir(current) {
		path := filepath.Join(current, ".sec", "sec.toml")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			root.found = true
			if data, err := os.ReadFile(path); err == nil {
				if identity, errors := ParseManifestIdentity(string(data)); len(errors) == 0 {
					root.dir, root.identity = current, projectRootPrefix+identity.UUID.String()
				}
			}
			break
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	resolver.projects[dir] = root
	return root
}

// relativeDir returns dir relative to root in slash form ("" for root itself).
func relativeDir(root, dir string) (string, bool) {
	relative, err := filepath.Rel(root, dir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	if relative == "." {
		return "", true
	}
	return filepath.ToSlash(relative), true
}

func identityFor(root, path string) (modules.ModuleIdentity, bool) {
	if path == "" {
		identity, err := modules.NewRootModuleIdentity(root)
		return identity, err == nil
	}
	identity, err := modules.NewModuleIdentity(root, path)
	return identity, err == nil
}
