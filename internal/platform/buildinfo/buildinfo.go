// Package buildinfo resolves the BuildInfo.Current() snapshot of one
// CompilationPlan for the CLI and the language server alike (MD-143).
package buildinfo

import (
	"os"
	"path/filepath"

	"sec/internal/platform/cbindingplan"
	platformtarget "sec/internal/platform/target"
	"sec/internal/project"
	"sec/internal/sema"
)

// Request names the plan whose build metadata is resolved. Target and
// Variant may be empty: the Target then comes from the source's owning
// Project Target and the Variant from the unique os/arch match.
type Request struct {
	ProjectRoot, SourceFile string
	// ManifestText, when set, is the manifest snapshot to use instead of
	// reading .sec/sec.toml (the language server's unsaved overlay).
	ManifestText    string
	Target, Variant string
	Platform        platformtarget.Target
	// Invocation is the running `sec build`; nil for analysis, which uses
	// the stored build number.
	Invocation *project.BuildInvocation
}

// Resolve returns the snapshot, or nil when the plan has no concrete
// Target, Variant or [build].profile, so BuildInfo.Current() is diagnosed
// rather than filled with defaults. Values describe the selected target,
// never the host; no Git or filesystem source facts are inspected.
// Rules: rules/compiler/compiler_known_members.md — § 113B(1)-(4);
// rules/library/core-library.md — § 28; rules/projects/projects.md — § 32(8).
func Resolve(request Request) *sema.BuildInfoSnapshot {
	path := filepath.Join(request.ProjectRoot, ".sec", "sec.toml")
	text := []byte(request.ManifestText)
	if request.ManifestText == "" {
		var err error
		if text, err = os.ReadFile(path); err != nil {
			return nil
		}
	}
	manifest, problems := project.ParseManifest(path, string(text))
	if manifest == nil || len(problems) != 0 {
		return nil
	}
	targetName := request.Target
	if targetName == "" {
		absolute, err := filepath.Abs(request.SourceFile)
		if err != nil {
			return nil
		}
		owner, ok := project.TargetForSource(manifest.Targets(), manifest.Facts().ProjectRoot, absolute)
		if !ok {
			return nil
		}
		targetName = owner.Name
	}
	variant := request.Variant
	if variant == "" {
		variant = cbindingplan.UniqueVariant(string(text), request.Platform)
	}
	if variant == "" {
		return nil
	}
	version, ok := manifest.TargetVersion(targetName)
	if !ok {
		return nil
	}
	rendered, err := version.Render()
	if err != nil {
		return nil
	}
	if request.Invocation != nil {
		version, rendered = request.Invocation.Version(), request.Invocation.Rendered()
	}
	doc, err := project.ParseManifestDocument(string(text))
	if err != nil {
		return nil
	}
	build, _ := doc.Table("build")
	buildProfile, _ := build["profile"].(string)
	if buildProfile == "" {
		return nil
	}
	definition, ok := platformtarget.Find(request.Platform)
	if !ok {
		return nil
	}
	selection, errs := project.ResolvePlanSelection(string(text), targetName, variant)
	if len(errs) != 0 {
		return nil
	}
	profile, err := platformtarget.ResolveTargetProfile(definition, selection.Profile, string(selection.AllocationMode), selection.Provider)
	if err != nil {
		return nil
	}
	facts := manifest.Facts()
	snapshot := &sema.BuildInfoSnapshot{
		ProjectUUID: facts.ProjectUUID.String(), ProjectName: facts.ProjectName,
		TargetName: targetName, VariantName: variant,
		VersionMajor: version.Major, VersionMinor: version.Minor, VersionRevision: version.Revision, VersionBuild: version.Build,
		VersionRendered: rendered, OperatingSystem: request.Platform.OS, Architecture: request.Platform.Arch,
		BuildProfile: buildProfile, TargetProfile: profile.TargetProfileIdentity,
	}
	if version.HasUserDefined {
		user := version.UserDefined
		snapshot.VersionUserDefined = &user
	}
	return snapshot
}
