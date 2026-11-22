package main

import (
	"path/filepath"

	"sec/internal/platform/buildinfo"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// applyLSPBuildInfo gives the editor the CLI's BuildInfo snapshot for the
// analysis plan's target, read from the overlay manifest. Editor analysis
// has no build invocation, so the stored build number is used and never
// reserved or incremented; an unresolved plan leaves BuildInfo.Current()
// to S1258 exactly as in `sec check`.
// Rules: rules/compiler/compiler_known_members.md — § 113B(3)-(4);
// rules/tooling/lsp.md — "Target-aware analysis".
func applyLSPBuildInfo(analyzer *sema.Analyzer, sourcePath string, overlay sourceOverlay) {
	targetOS, targetArch := analyzer.PlanTarget()
	if sourcePath == "" || targetOS == "" || targetArch == "" {
		return
	}
	manifestPath, text, ok := lspProjectManifest(sourcePath, overlay)
	if !ok {
		return
	}
	snapshot := buildinfo.Resolve(buildinfo.Request{
		ProjectRoot: filepath.Dir(filepath.Dir(manifestPath)), SourceFile: normalizedSourcePath(sourcePath),
		ManifestText: text, Platform: platformtarget.Target{OS: targetOS, Arch: targetArch},
	})
	if snapshot != nil {
		analyzer.SetBuildInfo(*snapshot)
	}
}
