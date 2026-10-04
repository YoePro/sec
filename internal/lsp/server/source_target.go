package server

import (
	"sec/internal/ast"
	platformtarget "sec/internal/platform/target"
)

// ProgramTarget returns the target a source file selects with its
// `#target(os: ..., arch: ...)` directive, normalized through the shared
// target registry. A file without a directive applies to every target.
func ProgramTarget(program *ast.Program) (platformtarget.Target, bool) {
	if program == nil {
		return platformtarget.Target{}, false
	}
	for _, statement := range program.Statements {
		if directive, ok := statement.(*ast.TargetDirective); ok && directive != nil {
			return platformtarget.Target{
				OS:   platformtarget.NormalizeOS(directive.OS),
				Arch: platformtarget.NormalizeArch(directive.Arch),
			}, true
		}
	}
	return platformtarget.Target{}, false
}

// ProgramMatchesTarget reports whether a source file belongs to the
// compilation for active: a file without a target directive always does, and
// a directed file does when its OS matches and its architecture matches or is
// `any`. This is the same selection the compiler applies, so files written
// for different platforms never meet in one analysis.
//
// Rules:
//   - rules/platform/platform_model.md — source selection by target
//   - rules/projects/modules.md — "Source directory and module membership"
func ProgramMatchesTarget(program *ast.Program, active platformtarget.Target) bool {
	fileTarget, directed := ProgramTarget(program)
	if !directed || active.OS == "" {
		return true
	}
	return fileTarget.OS == active.OS && (fileTarget.Arch == "any" || fileTarget.Arch == "" || active.Arch == "" || fileTarget.Arch == active.Arch)
}
