package buildinfo

import "sec/internal/sema"

// SourceFacts distinguishes a collection request from an available fact.
// Each field is independent; nil means unavailable, while an empty revision
// or false dirty status is a present value when explicitly requested.
// This is a compiler input contract, not source or manifest syntax.
// Rules: rules/library/core-library.md — §28(3);
// rules/projects/projects.md — §33(4).
type SourceFacts struct {
	RevisionRequested, DirtyStatusRequested bool
	Revision                                *string
	DirtyStatus                             *bool
}

// apply copies only requested and available facts into the resolved snapshot.
// Rules: rules/library/core-library.md — §28(3);
// rules/compiler/compiler_known_members.md — §113B(1–4).
func (facts SourceFacts) apply(snapshot *sema.BuildInfoSnapshot) {
	if facts.RevisionRequested && facts.Revision != nil {
		revision := *facts.Revision
		snapshot.SourceRevision = &revision
	}
	if facts.DirtyStatusRequested && facts.DirtyStatus != nil {
		dirty := *facts.DirtyStatus
		snapshot.DirtySourceStatus = &dirty
	}
}
