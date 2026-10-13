package project

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// CounterScope identifies the visible authoritative build field.
// Rules: rules/projects/projects.md — §28(3–4),§28.1,§28.2.
type CounterScope string

const (
	TargetCounter  CounterScope = "target"
	ProjectCounter CounterScope = "project"
)

// BuildCounter contains resolved counter facts for one declared Target. A
// disabled counter uses the ordinary inherited version without advancement.
// Rules: rules/projects/projects.md — §§11,27,28.
type BuildCounter struct {
	Enabled       bool
	Scope         CounterScope
	StoredBuild   uint64
	StorageTarget string // empty for project scope; explicit Target for target scope
}

// Candidate derives a number only from the current snapshot, with no historical
// high-water mark and no implicit reset when release components change. Reading
// a candidate is not allocation, locking or committing a successful build.
// Rules: rules/projects/projects.md — §§8(6–7),27(5–9),29(1–2).
func (counter BuildCounter) Candidate() (uint64, error) {
	if counter.StoredBuild > math.MaxInt64 {
		return 0, fmt.Errorf("stored build exceeds maximum VersionNumber 9223372036854775807")
	}
	if !counter.Enabled {
		return counter.StoredBuild, nil
	}
	if counter.StoredBuild >= math.MaxInt64 {
		return 0, fmt.Errorf("automatic build candidate exceeds maximum VersionNumber 9223372036854775807")
	}
	return counter.StoredBuild + 1, nil
}

// BuildCounter returns immutable facts, independently of the display name and
// of filesystem moves. Unknown Targets receive neither counter nor version.
// Rules: rules/projects/projects.md — §§28,49(1–2).
func (manifest *Manifest) BuildCounter(target string) (BuildCounter, bool) {
	counter, found := manifest.buildCounters[target]
	return counter, found
}

// BuildNumberCandidate resolves the complete Target version and a precise edit
// from one loaded snapshot. The edit must be committed only by the owning
// build transaction after all requested Variants succeed under its counter lock.
// Rules: rules/projects/projects.md — §§3.8,11,27(8–9),28,29(1–10),31(1–2).
func (manifest *Manifest) BuildNumberCandidate(target string) (Version, *BuildNumberEdit, error) {
	counter, found := manifest.BuildCounter(target)
	if !found {
		return Version{}, nil, fmt.Errorf("unknown Target %q", target)
	}
	version, _ := manifest.TargetVersion(target)
	build, err := counter.Candidate()
	if err != nil {
		return Version{}, nil, err
	}
	version.Build = build
	if !counter.Enabled {
		return version, nil, nil
	}
	edit, err := PlanBuildNumberEdit(manifest.path, manifest.text, counter.StorageTarget, build)
	if err != nil {
		return Version{}, nil, err
	}
	return version, edit, nil
}

// LoadBuildNumberCandidate explicitly rereads the current authoritative manifest;
// manual jumps and resets become the new baseline rather than corruption. No
// cached candidate or hidden counter is consulted. The caller owns serialization.
// Rules: rules/projects/projects.md — §§8(6–7),27(4–9),29(10)(2–5).
func LoadBuildNumberCandidate(path, target string) (Version, *BuildNumberEdit, error) {
	manifest, err := LoadManifest(path)
	if err != nil {
		return Version{}, nil, err
	}
	return manifest.BuildNumberCandidate(target)
}

// resolveBuildCounters validates all declared counter tables and binds active
// counters to their explicit storage. Disabled counters may inherit a version;
// project scope forbids every explicit local build override, even an equal one.
// Rules: rules/projects/projects.md — §§28(1–4),28.1(1–3),28.2(1–4),48(1–2).
func resolveBuildCounters(text string, targets []Target, versions ManifestVersions) (map[string]BuildCounter, []ManifestError) {
	document, err := ParseManifestDocument(text)
	if err != nil {
		return nil, []ManifestError{{Message: err.Error()}}
	}
	result := map[string]BuildCounter{}
	var failures []ManifestError
	for _, target := range targets {
		version := versions.Project.Resolve(versions.TargetOverrides[target.Name])
		counter := BuildCounter{Scope: TargetCounter, StoredBuild: version.Build, StorageTarget: target.Name}
		path := []string{"target", target.Name, "build_counter"}
		table, err := document.Table(path...)
		report := func(key, message string) {
			location := append(append([]string(nil), path...), key)
			if key == "" {
				location = path
			}
			failures = append(failures, ManifestError{Line: manifestKeyLine(text, location), Message: strings.Join(location, ".") + ": " + message})
		}
		if err != nil {
			report("", "must be a TOML table")
			continue
		}
		keys := make([]string, 0, len(table))
		for key := range table {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch key {
			case "enabled":
				value, ok := table[key].(bool)
				if !ok {
					report(key, "expected a boolean (true or false)")
				} else {
					counter.Enabled = value
				}
			case "scope":
				value, ok := table[key].(string)
				if !ok || (value != string(TargetCounter) && value != string(ProjectCounter)) {
					report(key, `expected "target" or "project"`)
				} else {
					counter.Scope = CounterScope(value)
				}
			default:
				report(key, "unknown counter setting; expected enabled or scope")
			}
		}
		if counter.Scope == ProjectCounter {
			counter.StoredBuild = versions.Project.Build
			counter.StorageTarget = ""
		}
		if counter.Enabled {
			local := versions.TargetOverrides[target.Name].Build
			if counter.Scope == TargetCounter && local == nil {
				report("", "target scope requires an explicit target."+target.Name+".version.build field; inherited Project build is insufficient")
			}
			if counter.Scope == ProjectCounter && local != nil {
				report("scope", "project scope conflicts with explicit target."+target.Name+".version.build; remove the local build override or use target scope")
			}
		} else {
			// Without advancement the ordinary Target version stays authoritative.
			counter.StoredBuild = version.Build
		}
		result[target.Name] = counter
	}
	return result, failures
}
