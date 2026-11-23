package sema

// cloneBuildInfoSnapshot detaches optional values so configuring an analyzer,
// replaying its CTE inputs or exposing metadata cannot mutate the plan snapshot.
// Rules: rules/compiler/compiler_known_members.md — §113B(1–4);
// rules/library/core-library.md — §28(3).
func cloneBuildInfoSnapshot(snapshot BuildInfoSnapshot) BuildInfoSnapshot {
	if snapshot.VersionUserDefined != nil {
		value := *snapshot.VersionUserDefined
		snapshot.VersionUserDefined = &value
	}
	if snapshot.SourceRevision != nil {
		value := *snapshot.SourceRevision
		snapshot.SourceRevision = &value
	}
	if snapshot.DirtySourceStatus != nil {
		value := *snapshot.DirtySourceStatus
		snapshot.DirtySourceStatus = &value
	}
	return snapshot
}
