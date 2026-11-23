package sema

import "testing"

// Configured metadata must remain one immutable plan input, including when
// semantic CTE creates a fresh analyzer from recorded configuration.
// Rules: rules/compiler/compiler_known_members.md — §113B(1–4);
// rules/library/core-library.md — §28(3).
func TestBuildInfoSnapshotOwnership(t *testing.T) {
	user, revision, dirty := "beta", "revision-a", false
	snapshot := testBuildInfo
	snapshot.VersionUserDefined, snapshot.SourceRevision, snapshot.DirtySourceStatus = &user, &revision, &dirty
	a := NewAnalyzer()
	a.SetBuildInfo(snapshot)
	user, revision, dirty = "changed", "revision-b", true
	assertFrozen := func(analyzer *Analyzer) {
		t.Helper()
		got, ok := analyzer.BuildInfo()
		if !ok || *got.VersionUserDefined != "beta" || *got.SourceRevision != "revision-a" || *got.DirtySourceStatus {
			t.Fatalf("metadata changed after configuration: %+v", got)
		}
	}
	assertFrozen(a)
	exposed, _ := a.BuildInfo()
	*exposed.VersionUserDefined, *exposed.SourceRevision, *exposed.DirtySourceStatus = "escape", "escape", true
	assertFrozen(a)
	assertFrozen(a.freshConfiguredAnalyzer())
}
