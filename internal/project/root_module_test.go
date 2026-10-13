package project

import (
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§7(7–8),14(3,7–9),53(8).
func TestRootModuleName(t *testing.T) {
	fixture := manifestFixture(t)
	for _, tc := range []struct {
		name, text, want string
		fail             bool
	}{
		{"override", fixture, "suite", false},
		{"valid human name", strings.Replace(strings.Replace(fixture, `name = "SEC Compiler Suite"`, `name = "codec"`, 1), "module = \"suite\"\n", "", 1), "codec", false},
		{"invalid human name", strings.Replace(fixture, "module = \"suite\"\n", "", 1), "", true},
		{"root command", fixture + "\n[target.root]\nkind = 'command'\nsource = '.'\n", "main", false},
		{"root firmware", fixture + "\n[target.root]\nkind = 'firmware'\nsource = '.'\n", "main", false},
		{"conflicting roots", fixture + "\n[target.root]\nkind = 'command'\nsource = '.'\n[target.rootlib]\nkind = 'library'\nsource = '.'\n", "", true},
		{"same root name", strings.Replace(fixture, `module = "suite"`, `module = "main"`, 1) + "\n[target.root]\nkind = 'command'\nsource = '.'\n[target.rootlib]\nkind = 'library'\nsource = '.'\n", "main", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest, errors := ParseManifest(filepath.Join(t.TempDir(), ".sec", "sec.toml"), tc.text)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			name, err := manifest.RootModuleName()
			if name != tc.want || (err != nil) != tc.fail {
				t.Fatal(name, err)
			}
		})
	}
}
