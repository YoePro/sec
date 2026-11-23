package main

import (
	"crypto/sha256"
	"fmt"
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
	if snapshot := resolveLSPBuildInfo(sourcePath, overlay, targetOS, targetArch); snapshot != nil {
		analyzer.SetBuildInfo(*snapshot)
	}
}

// resolveLSPBuildInfo resolves the same observable inputs for analysis and cache
// validation. Missing or ambiguous metadata remains unavailable, never a host
// fallback. No source metadata collection or build-number increment occurs.
// Rules: rules/compiler/compiler_known_members.md — §113B(1–4);
// rules/projects/projects.md — §§33–34.
func resolveLSPBuildInfo(sourcePath string, overlay sourceOverlay, targetOS, targetArch string) *sema.BuildInfoSnapshot {
	if sourcePath == "" || targetOS == "" || targetArch == "" {
		return nil
	}
	manifestPath, text, ok := lspProjectManifest(sourcePath, overlay)
	if !ok {
		return nil
	}
	return buildinfo.Resolve(buildinfo.Request{
		ProjectRoot: filepath.Dir(filepath.Dir(manifestPath)), SourceFile: normalizedSourcePath(sourcePath),
		ManifestText: text, Platform: platformtarget.Target{OS: targetOS, Arch: targetArch},
	})
}

// lspBuildInfoIdentity fingerprints complete observable metadata. Unavailable
// metadata also depends on the unresolved manifest input, so repairing a plan
// invalidates a cached analyzer even when its former target was unresolved.
// Rules: rules/compiler/compiler_known_members.md — §113B(3–4);
// rules/compiler/incremental_compilation.md — §7.
func lspBuildInfoIdentity(snapshot *sema.BuildInfoSnapshot, sourcePath string, overlay sourceOverlay) string {
	if snapshot == nil {
		_, text, found := lspProjectManifest(sourcePath, overlay)
		return fmt.Sprintf("unavailable:%t:%x", found, sha256.Sum256([]byte(text)))
	}
	return snapshot.CacheIdentity()
}
