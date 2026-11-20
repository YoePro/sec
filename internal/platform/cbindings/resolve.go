package cbindings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"sec/internal/platform/target"
)

// Candidate is one discovered binding package and where it came from:
// "compiler" (shipped and trusted), "project" (.sec/platform/c-bindings.toml)
// or "explicit" (a --file request).
type Candidate struct {
	Package *Package
	Origin  string
}

// Selector is the optional Variant `c_bindings` table (projects.md § 19(16)).
type Selector struct {
	ID          string
	Version     string
	Environment string
}

// Request is the normalized target side of one Variant's resolution. The
// target facts come from the plan; packages never change them.
type Request struct {
	Variant         string
	OS              string
	Arch            string
	ABI             string
	CRuntime        string
	CRuntimeVersion string
	Selector        *Selector
}

// Resolution is the frozen binding identity of one plan (correction § 8.8(1)).
type Resolution struct {
	Variant         string   `json:"variant"`
	PackageID       string   `json:"id"`
	PackageVersion  string   `json:"version"`
	Environment     string   `json:"environment"`
	SchemaVersion   int      `json:"schema_version"`
	OS              string   `json:"os"`
	Arch            string   `json:"arch"`
	ABI             string   `json:"abi"`
	CRuntime        string   `json:"c_runtime"`
	CRuntimeVersion string   `json:"c_runtime_version,omitempty"`
	Modules         []string `json:"modules"`
	Origin          string   `json:"origin"`
	Fingerprint     string   `json:"fingerprint"`
}

// Resolve selects exactly one (package, version, environment) for a Variant
// following correction § 8.5: validate and deduplicate candidates, filter by
// the explicit package id and version constraint, by exact os/arch/abi/
// c_runtime and runtime-version constraints, then by an explicit
// environment key. Zero survivors is missing/incompatible, several is
// ambiguous with every candidate listed in sorted order. Nothing is chosen
// by version height, origin or discovery order, and no fact is taken from
// the compiler host.
// Rules: rules/platform/platform_model.md — § 31; rules/platform/abi.md — § 46;
// MD-021 correction §§ 8.1-8.5, 8.7, 8.8.
func Resolve(request Request, candidates []Candidate) (*Resolution, []Diagnostic) {
	request.OS, request.Arch = target.NormalizeOS(request.OS), target.NormalizeArch(request.Arch)
	var runtimeVersion *Version
	if request.CRuntimeVersion != "" {
		version, err := ParseRuntimeVersion(request.CRuntimeVersion)
		if err != nil {
			return nil, []Diagnostic{diagnostic("c-bindings.invalid-version", "variant %s c_runtime_version: %v", request.Variant, err)}
		}
		runtimeVersion = &version
	}
	var packageConstraint Constraint
	if selector := request.Selector; selector != nil {
		if selector.ID == "" {
			return nil, []Diagnostic{diagnostic("c-bindings.missing-field", "variant %s c_bindings.id is required", request.Variant)}
		}
		if selector.Version != "" {
			constraint, err := ParseConstraint(selector.Version, true)
			if err != nil {
				return nil, []Diagnostic{diagnostic("c-bindings.invalid-constraint", "variant %s c_bindings.version: %v", request.Variant, err)}
			}
			packageConstraint = constraint
		}
	}

	valid, diagnostics := validCandidates(request, candidates)
	if len(diagnostics) != 0 {
		return nil, diagnostics
	}
	if request.Selector != nil {
		var selected []Candidate
		known := false
		for _, candidate := range valid {
			if candidate.Package.ID != request.Selector.ID {
				continue
			}
			known = true
			version, _ := ParsePackageVersion(candidate.Package.Version)
			if packageConstraint == nil || packageConstraint.Allows(version) {
				selected = append(selected, candidate)
			}
		}
		if !known {
			return nil, []Diagnostic{diagnostic("c-bindings.unknown-package", "variant %s selects binding package %q, which is not available", request.Variant, request.Selector.ID)}
		}
		if len(selected) == 0 {
			return nil, []Diagnostic{diagnostic("c-bindings.version-unsatisfied", "variant %s: no version of %q satisfies %q", request.Variant, request.Selector.ID, request.Selector.Version)}
		}
		valid = selected
	}

	type match struct {
		candidate   Candidate
		environment Environment
	}
	var matches []match
	var reasons []string
	environmentFound := false
	for _, candidate := range valid {
		for _, name := range candidate.Package.EnvironmentNames() {
			environment := candidate.Package.Environments[name]
			if request.Selector != nil && request.Selector.Environment != "" && name != request.Selector.Environment {
				continue
			}
			environmentFound = true
			if reason := incompatibility(request, runtimeVersion, environment); reason != "" {
				reasons = append(reasons, fmt.Sprintf("%s@%s/%s: %s", candidate.Package.ID, candidate.Package.Version, name, reason))
				continue
			}
			matches = append(matches, match{candidate, environment})
		}
	}
	if request.Selector != nil && request.Selector.Environment != "" && !environmentFound {
		return nil, []Diagnostic{diagnostic("c-bindings.unknown-environment", "variant %s selects environment %q, which package %q does not declare", request.Variant, request.Selector.Environment, request.Selector.ID)}
	}
	switch len(matches) {
	case 0:
		sort.Strings(reasons)
		detail := ""
		if len(reasons) != 0 {
			detail = "; " + strings.Join(reasons, "; ")
		}
		return nil, []Diagnostic{diagnostic("c-bindings.no-candidate", "variant %s (%s/%s %s, c_runtime %q): no compatible C binding environment%s", request.Variant, request.OS, request.Arch, request.ABI, request.CRuntime, detail)}
	case 1:
	default:
		identities := make([]string, len(matches))
		for index, found := range matches {
			identities[index] = fmt.Sprintf("%s@%s/%s", found.candidate.Package.ID, found.candidate.Package.Version, found.environment.Name)
		}
		sort.Strings(identities)
		return nil, []Diagnostic{diagnostic("c-bindings.ambiguous", "variant %s matches several C binding environments %v; select one with c_bindings", request.Variant, identities)}
	}
	chosen := matches[0]
	modules := append(append([]string(nil), chosen.candidate.Package.CommonModules...), chosen.environment.Modules...)
	resolution := &Resolution{
		Variant: request.Variant, PackageID: chosen.candidate.Package.ID, PackageVersion: chosen.candidate.Package.Version,
		Environment: chosen.environment.Name, SchemaVersion: chosen.candidate.Package.SchemaVersion,
		OS: request.OS, Arch: request.Arch, ABI: request.ABI, CRuntime: request.CRuntime, CRuntimeVersion: request.CRuntimeVersion,
		Modules: modules, Origin: chosen.candidate.Origin,
	}
	resolution.Fingerprint = resolutionFingerprint(resolution, chosen.candidate.Package.CommonModules, chosen.environment.Modules)
	return resolution, nil
}

// validCandidates drops structurally invalid packages (an invalid package
// requested by id is an error) and rejects equal identities with different
// content; identical content is deduplicated.
// Rules: MD-021 correction § 8.2(2)-(3).
func validCandidates(request Request, candidates []Candidate) ([]Candidate, []Diagnostic) {
	var diagnostics []Diagnostic
	byIdentity := map[string]Candidate{}
	var identities []string
	for _, candidate := range candidates {
		if candidate.Package == nil {
			continue
		}
		if problems := candidate.Package.Validate(); len(problems) != 0 {
			if request.Selector != nil && request.Selector.ID == candidate.Package.ID || candidate.Origin == "explicit" {
				diagnostics = append(diagnostics, diagnostic("c-bindings.malformed-package", "requested binding package %q (%s) is invalid: %s", candidate.Package.ID, candidate.Origin, problems[0].Message))
			}
			continue
		}
		identity := candidate.Package.ID + "@" + candidate.Package.Version
		if existing, found := byIdentity[identity]; found {
			if packageFingerprint(existing.Package) != packageFingerprint(candidate.Package) {
				diagnostics = append(diagnostics, diagnostic("c-bindings.identity-conflict", "binding package %s is provided by %s and %s with different content", identity, existing.Origin, candidate.Origin))
			}
			continue
		}
		byIdentity[identity] = candidate
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	valid := make([]Candidate, 0, len(identities))
	for _, identity := range identities {
		valid = append(valid, byIdentity[identity])
	}
	return valid, diagnostics
}

// incompatibility explains why an environment does not fit the request, or
// returns "" when it fits. An unknown runtime version never satisfies a
// declared range.
// Rules: rules/platform/abi.md — § 46; MD-021 correction §§ 8.4(3), 8.5(1) steps 5-6.
func incompatibility(request Request, runtimeVersion *Version, environment Environment) string {
	switch {
	case environment.OS != request.OS:
		return fmt.Sprintf("os %s", environment.OS)
	case environment.Arch != request.Arch:
		return fmt.Sprintf("arch %s", environment.Arch)
	case environment.ABI != request.ABI:
		return fmt.Sprintf("abi %s", environment.ABI)
	case environment.CRuntime != request.CRuntime:
		return fmt.Sprintf("c_runtime %s", environment.CRuntime)
	}
	if environment.CRuntimeVersions == "" {
		return ""
	}
	if runtimeVersion == nil {
		return fmt.Sprintf("requires c_runtime_versions %q but the target runtime version is unknown", environment.CRuntimeVersions)
	}
	constraint, _ := ParseConstraint(environment.CRuntimeVersions, false)
	if !constraint.Allows(*runtimeVersion) {
		return fmt.Sprintf("c_runtime_version %s outside %q", runtimeVersion, environment.CRuntimeVersions)
	}
	return ""
}

// resolutionFingerprint hashes the normalized semantic facts of a selection;
// module lists are sorted per layer, so array order, comments and table order
// never change it. The alias surface joins it once @cAlias is collected.
// Rules: MD-021 correction § 8.8(2).
func resolutionFingerprint(resolution *Resolution, common, environment []string) string {
	parts := []string{
		"schema=" + fmt.Sprint(resolution.SchemaVersion), "id=" + resolution.PackageID, "version=" + resolution.PackageVersion,
		"environment=" + resolution.Environment, "os=" + resolution.OS, "arch=" + resolution.Arch, "abi=" + resolution.ABI,
		"c_runtime=" + resolution.CRuntime, "c_runtime_version=" + resolution.CRuntimeVersion,
		"common=" + strings.Join(sortedCopy(common), ","), "modules=" + strings.Join(sortedCopy(environment), ","),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// packageFingerprint hashes a package's complete normalized content for
// identity-conflict checks.
// Rules: MD-021 correction § 8.2(3).
func packageFingerprint(pkg *Package) string {
	parts := []string{fmt.Sprint(pkg.SchemaVersion), pkg.ID, pkg.Version, pkg.SourceRoot, strings.Join(sortedCopy(pkg.CommonModules), ",")}
	for _, name := range pkg.EnvironmentNames() {
		environment := pkg.Environments[name]
		parts = append(parts, strings.Join([]string{name, environment.OS, environment.Arch, environment.ABI, environment.CRuntime,
			environment.CRuntimeVersions, strings.Join(sortedCopy(environment.Modules), ",")}, "|"))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// sortedCopy returns a sorted copy of values.
// Rules: MD-021 correction § 8.8(2).
func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}
