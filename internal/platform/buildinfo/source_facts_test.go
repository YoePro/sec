package buildinfo

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// Requested and available are independent requirements, and false/empty are
// present values rather than substitutions for missing facts.
// Rules: rules/library/core-library.md — §28(3); rules/projects/projects.md — §33(4).
func TestExplicitSourceFacts(t *testing.T) {
	root := t.TempDir()
	manifest, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "platform", "buildinfo", "source_facts.toml"))
	if err != nil {
		t.Fatal(err)
	}
	revision, dirty := "rev-a", true
	empty, clean := "", false
	for _, test := range []struct {
		name     string
		facts    SourceFacts
		revision *string
		dirty    *bool
	}{
		{name: "absent"},
		{name: "available-unrequested", facts: SourceFacts{Revision: &revision, DirtyStatus: &dirty}},
		{name: "requested-unavailable", facts: SourceFacts{RevisionRequested: true, DirtyStatusRequested: true}},
		{name: "revision-only", facts: SourceFacts{RevisionRequested: true, Revision: &revision, DirtyStatus: &dirty}, revision: &revision},
		{name: "dirty-only", facts: SourceFacts{DirtyStatusRequested: true, Revision: &revision, DirtyStatus: &dirty}, dirty: &dirty},
		{name: "both", facts: SourceFacts{RevisionRequested: true, DirtyStatusRequested: true, Revision: &revision, DirtyStatus: &dirty}, revision: &revision, dirty: &dirty},
		{name: "empty-and-clean", facts: SourceFacts{RevisionRequested: true, DirtyStatusRequested: true, Revision: &empty, DirtyStatus: &clean}, revision: &empty, dirty: &clean},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := Resolve(Request{ProjectRoot: root, ManifestText: string(manifest), Target: "app", Variant: "linux", Platform: platformtarget.Target{OS: "linux", Arch: "arm64"}, SourceFacts: test.facts})
			if snapshot == nil {
				t.Fatal("plan did not resolve")
			}
			if (snapshot.SourceRevision == nil) != (test.revision == nil) || (snapshot.DirtySourceStatus == nil) != (test.dirty == nil) {
				t.Fatalf("presence: %+v", snapshot)
			}
			if test.revision != nil && (*snapshot.SourceRevision != *test.revision || snapshot.SourceRevision == test.facts.Revision) {
				t.Fatal("revision value or ownership")
			}
			if test.dirty != nil && (*snapshot.DirtySourceStatus != *test.dirty || snapshot.DirtySourceStatus == test.facts.DirtyStatus) {
				t.Fatal("dirty value or ownership")
			}
		})
	}
	// Resolve using explicit inputs in an empty project root: no repository or
	// source-file metadata is available to supply the result implicitly.
	snapshot := Resolve(Request{ProjectRoot: root, ManifestText: string(manifest), Target: "app", Variant: "linux", Platform: platformtarget.Target{OS: "linux", Arch: "arm64"}, SourceFacts: SourceFacts{RevisionRequested: true, DirtyStatusRequested: true, Revision: &revision, DirtyStatus: &clean}})
	revision, clean = "changed", true
	if *snapshot.SourceRevision != "rev-a" || *snapshot.DirtySourceStatus {
		t.Fatal("snapshot aliases producer inputs")
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "platform", "buildinfo", "source_facts.sec"))
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "source_facts.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	analyzer.SetBuildInfo(*snapshot)
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	calls := 0
	ast.Visit(program, func(node any) bool {
		if call, ok := node.(*ast.CallExpression); ok {
			if info, accepted := analyzer.BuildInfoCurrentOf(call); accepted {
				calls++
				if *info.SourceRevision != "rev-a" || *info.DirtySourceStatus {
					t.Fatal("Current differs from CTE inputs")
				}
				*info.SourceRevision, *info.DirtySourceStatus = "escaped", true
			}
		}
		return true
	})
	current, _ := analyzer.BuildInfo()
	if calls != 1 || *current.SourceRevision != "rev-a" || *current.DirtySourceStatus {
		t.Fatal("Current metadata was not detached")
	}
}
