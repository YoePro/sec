package semantic

import (
	"strings"
	"testing"
)

// CanonicalizeSourceFiles rewrites Module.SourceFiles and every reachable
// Location, keeps unknown paths, and rejects two paths that would share one
// identity.
// Rules: rules/compiler/semantic_ir.md — § 5(1); rules/projects/projects.md — § 34.
func TestCanonicalizeSourceFiles(t *testing.T) {
	identity := func(path string) (string, bool) {
		if strings.HasPrefix(path, "/work/app/") {
			return strings.TrimPrefix(path, "/work/app/"), true
		}
		return "", false
	}
	module := &Module{
		Identity:    "main",
		SourceFiles: []string{"/work/app/main.sec"},
		Functions: []*Function{{
			Location: Location{File: "/work/app/main.sec", Line: 3, Column: 1},
			Blocks: []*Block{{Operations: []Operation{
				{Location: Location{File: "/work/app/util/u.sec", Line: 4, Column: 2}},
				{Location: Location{File: "<generated>", Line: 1, Column: 1}},
			}}},
		}},
	}
	original, err := CanonicalizeSourceFiles(module, identity)
	if err != nil {
		t.Fatal(err)
	}
	if original["main.sec"] != "/work/app/main.sec" || original["util/u.sec"] != "/work/app/util/u.sec" {
		t.Fatalf("identity map = %v", original)
	}
	if len(module.SourceFiles) != 1 || module.SourceFiles[0] != "main.sec" {
		t.Fatalf("source files = %v", module.SourceFiles)
	}
	operations := module.Functions[0].Blocks[0].Operations
	if module.Functions[0].Location.File != "main.sec" || operations[0].Location.File != "util/u.sec" || operations[1].Location.File != "<generated>" {
		t.Fatalf("locations = %v %v %v", module.Functions[0].Location, operations[0].Location, operations[1].Location)
	}

	collision := &Module{Identity: "main", SourceFiles: []string{"/a/main.sec", "/b/main.sec"}}
	_, err = CanonicalizeSourceFiles(collision, func(path string) (string, bool) { return "main.sec", true })
	if err == nil || !strings.Contains(err.Error(), "share the source identity") {
		t.Fatalf("colliding identities = %v", err)
	}
}
