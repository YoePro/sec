package project

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/modules"
)

func writeIdentityFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSourceModuleResolverIdentities checks the resolver-owned identities of
// Project, root-module, compiler-provided and Project-less sources: the
// Project UUID root with the Project-relative directory, the empty path of a
// root module, the platform/std/core roots and the anonymous entry root.
// Rules: rules/projects/modules.md — §§2.2,8; rules/projects/projects.md — §§13(9),14,16(3).
func TestSourceModuleResolverIdentities(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	writeIdentityFile(t, filepath.Join(app, ".sec", "sec.toml"), "[project]\nname = \"app\"\nuuid = \"e3856637-43bf-4166-bf83-07edd6e03c5d\"\n")
	compiler := filepath.Join(base, "compiler")
	loose := filepath.Join(base, "loose")
	resolver := NewSourceModuleResolver(compiler, loose)
	project := "project:e3856637-43bf-4166-bf83-07edd6e03c5d"
	for _, test := range []struct {
		file string
		want modules.ModuleIdentity
	}{
		{filepath.Join(app, "drivers", "gpio", "gpio.sec"), modules.ModuleIdentity{ImportRootIdentity: project, CanonicalImportPath: "drivers/gpio"}},
		{filepath.Join(app, "main.sec"), modules.ModuleIdentity{ImportRootIdentity: project}},
		{filepath.Join(compiler, "sec", "platform", "linux", "file.sec"), modules.ModuleIdentity{ImportRootIdentity: PlatformImportRoot, CanonicalImportPath: "platform/linux"}},
		{filepath.Join(compiler, "sec", "stdlib", "net", "http", "client.sec"), modules.ModuleIdentity{ImportRootIdentity: StdlibImportRoot, CanonicalImportPath: "net/http"}},
		{filepath.Join(compiler, "sec", "core", "string.sec"), modules.ModuleIdentity{ImportRootIdentity: CoreImportRoot}},
		{filepath.Join(loose, "board", "pins.sec"), modules.ModuleIdentity{ImportRootIdentity: AnonymousImportRoot, CanonicalImportPath: "board"}},
		{filepath.Join(loose, "main.sec"), modules.ModuleIdentity{ImportRootIdentity: AnonymousImportRoot}},
	} {
		got, ok := resolver.ModuleIdentity(test.file)
		if !ok || got != test.want {
			t.Errorf("%s: got %v (%v), want %v", test.file, got, ok, test.want)
		}
	}
	if got, ok := resolver.ModuleIdentity(filepath.Join(base, "elsewhere", "x.sec")); ok {
		t.Fatalf("source outside every root resolved to %v", got)
	}
}

// A manifest that does not load gives no Project root; the source is not
// silently moved into another root.
func TestSourceModuleResolverRejectsInvalidManifest(t *testing.T) {
	base := t.TempDir()
	writeIdentityFile(t, filepath.Join(base, ".sec", "sec.toml"), "[project]\nname = \"broken\"\n")
	resolver := NewSourceModuleResolver("", filepath.Join(base, "other"))
	if got, ok := resolver.ModuleIdentity(filepath.Join(base, "src", "a.sec")); ok {
		t.Fatalf("invalid manifest resolved to %v", got)
	}
}
