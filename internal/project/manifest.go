package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Facts separates stable identity, display name, location and structured version.
// Returned values are copies of one validated manifest snapshot.
// Rules: rules/projects/projects.md — §§3.2–3.3,8,49(1–3).
type Facts struct {
	ProjectUUID    ProjectUUID
	ProjectName    string
	ProjectRoot    string
	ProjectVersion Version
}

// Manifest is a read-only snapshot for compiler and tooling consumers. It owns
// the original text for precise future edits and resolved Target versions.
// Target/profile/variant completeness remains owned by their separate validators.
// Rules: rules/projects/projects.md — §§5(4–6),11,49(1–3),(5).
type Manifest struct {
	path           string
	text           string
	facts          Facts
	identity       Identity
	targets        []Target
	targetVersions map[string]Version
	buildCounters  map[string]BuildCounter
}

// Facts returns resolved project fields without I/O, string parsing or mutable
// aliases into the snapshot.
// Rules: rules/projects/projects.md — §49(1–3).
func (manifest *Manifest) Facts() Facts { return manifest.facts }
func (manifest *Manifest) Path() string { return manifest.path }
func (manifest *Manifest) Text() string { return manifest.text }

// Identity returns the parsed root-module override and located name warnings
// alongside identity. The warning slice is copied to preserve snapshot ownership.
// Rules: rules/projects/projects.md — §7,§49(1–2).
func (manifest *Manifest) Identity() Identity {
	result := manifest.identity
	result.Warnings = append([]ManifestError(nil), result.Warnings...)
	return result
}

// Targets returns the represented source/kind facts without rereading the TOML.
// This accessor does not claim complete Target validation.
// Rules: rules/projects/projects.md — §17,§49(2).
func (manifest *Manifest) Targets() []Target { return append([]Target(nil), manifest.targets...) }

// TargetVersion returns the inherited, resolved structured version of a declared
// Target. Unknown names do not acquire a default version or phantom identity.
// Rules: rules/projects/projects.md — §11(4–14),§49(2–3).
func (manifest *Manifest) TargetVersion(name string) (Version, bool) {
	version, ok := manifest.targetVersions[name]
	return version, ok
}

// ParseManifest resolves identity, version and counter storage from one snapshot.
// Invalid input publishes no resolved project. The root comes from the canonical
// manifest path, independently of display name or UUID; it is absolute and clean.
// Rules: rules/projects/projects.md — §§5(1),7–11,28,49(1–3).
func ParseManifest(path, text string) (*Manifest, []ManifestError) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, []ManifestError{{Message: "cannot resolve manifest path: " + err.Error()}}
	}
	absolute = filepath.Clean(absolute)
	if filepath.Base(absolute) != "sec.toml" || filepath.Base(filepath.Dir(absolute)) != ".sec" {
		return nil, []ManifestError{{Message: "Project manifest must be located at .sec/sec.toml"}}
	}
	identity, identityErrors := ParseManifestIdentity(text)
	versions, versionErrors := ParseManifestVersions(text)
	diagnostics := append(identityErrors, versionErrors...)
	targets := ParseManifestTargets(text)
	var counters map[string]BuildCounter
	if len(diagnostics) == 0 {
		var counterErrors []ManifestError
		counters, counterErrors = resolveBuildCounters(text, targets, versions)
		diagnostics = append(diagnostics, counterErrors...)
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	if len(diagnostics) != 0 {
		return nil, diagnostics
	}
	manifest := &Manifest{
		path: absolute, text: text, identity: identity,
		facts:   Facts{ProjectUUID: identity.UUID, ProjectName: identity.Name, ProjectRoot: filepath.Dir(filepath.Dir(absolute)), ProjectVersion: versions.Project},
		targets: targets, targetVersions: map[string]Version{}, buildCounters: counters,
	}
	for _, target := range manifest.targets {
		manifest.targetVersions[target.Name] = versions.Project.Resolve(versions.TargetOverrides[target.Name])
	}
	return manifest, nil
}

// ManifestErrors keeps all located validation failures from a single read.
// Rules: rules/projects/projects.md — §48(2).
type ManifestErrors struct {
	Path        string
	Diagnostics []ManifestError
}

func (failure *ManifestErrors) Error() string {
	if len(failure.Diagnostics) == 0 {
		return failure.Path + ": invalid Project manifest"
	}
	first := failure.Diagnostics[0]
	if first.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", failure.Path, first.Line, first.Message)
	}
	return failure.Path + ": " + first.Message
}

// LoadManifest reads once and resolves the project facts used by builds/tools.
// No cache hides subsequent edits; callers retain a snapshot for one operation
// and explicitly load another when the manifest changes.
// Rules: rules/projects/projects.md — §§43(1),49(1–3).
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	manifest, diagnostics := ParseManifest(path, string(data))
	if len(diagnostics) != 0 {
		return nil, &ManifestErrors{Path: path, Diagnostics: diagnostics}
	}
	return manifest, nil
}
