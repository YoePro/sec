package main

import (
	"sec/internal/platform/buildinfo"
	"sec/internal/project"
	"sec/internal/sema"
)

// cliBuildInvocation is the `sec build` invocation whose resolved build
// number BuildInfo.Current() reflects; nil for `sec check` and other
// analysis-only commands, which use the stored build number.
var cliBuildInvocation *project.BuildInvocation

// cliBuildInfo resolves the BuildInfo snapshot of the plan being analyzed
// through the shared resolver; nil leaves every BuildInfo.Current() to S1258.
// Rules: rules/compiler/compiler_known_members.md — § 113B(1)-(4).
func cliBuildInfo(sourceFiles []string, target CompilerTarget) *sema.BuildInfoSnapshot {
	if len(sourceFiles) == 0 {
		return nil
	}
	request := buildinfo.Request{
		ProjectRoot: findProjectRoot(sourceFiles[0]), SourceFile: sourceFiles[0],
		Target: cliPlanTarget, Variant: cliCBindingVariant, Platform: target,
	}
	if cliPlanTarget != "" {
		request.Invocation = cliBuildInvocation
	}
	return buildinfo.Resolve(request)
}
