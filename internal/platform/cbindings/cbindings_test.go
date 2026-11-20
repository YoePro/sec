package cbindings

import (
	"strings"
	"testing"
)

const hostedPackage = `
schema_version = 1

[bindings]
id = "sec.hosted-c"
version = "1.0.0"
source_root = "sec/platform"

[modules]
common = ["c/common"]

[environments.linux-amd64-glibc]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_versions = ">=2.31, <3.0"
modules = ["c/linux/glibc/common", "c/linux/glibc/amd64"]

[environments.linux-arm64-glibc]
os = "linux"
arch = "arm64"
abi = "gnu"
c_runtime = "glibc"
modules = ["c/linux/glibc/common", "c/linux/glibc/arm64"]

[environments.linux-amd64-musl]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "musl"
c_runtime_versions = ">=1.2, <2.0"
modules = ["c/linux/musl/common", "c/linux/musl/amd64"]

[environments.windows-amd64-ucrt]
os = "windows"
arch = "amd64"
abi = "msvc"
c_runtime = "ucrt"
modules = ["c/windows/ucrt/common", "c/windows/ucrt/amd64"]
`

func mustParse(t *testing.T, text string) *Package {
	t.Helper()
	pkg, diagnostics := Parse(text)
	if len(diagnostics) != 0 {
		t.Fatalf("parse: %v", diagnostics)
	}
	return pkg
}

func hasDiagnostic(diagnostics []Diagnostic, id string) bool {
	for _, d := range diagnostics {
		if d.ID == id {
			return true
		}
	}
	return false
}

// MD-021 § 2, § 5(7), § 8.9(1): structural validation.
func TestParseAndValidate(t *testing.T) {
	pkg := mustParse(t, hostedPackage)
	if !pkg.Resolvable() || len(pkg.Environments) != 4 || pkg.CommonModules[0] != "c/common" {
		t.Fatalf("package = %+v", pkg)
	}
	draft := mustParse(t, "schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"0.1.0\"\nsource_root = \"sec/platform\"\n")
	if draft.Resolvable() {
		t.Fatal("a zero-environment draft is resolvable")
	}
	for name, check := range map[string]struct{ text, id string }{
		"schema":         {"schema_version = 2\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n", "c-bindings.unsupported-schema"},
		"unknown key":    {"schema_version = 1\nextends = \"y\"\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n", "c-bindings.unknown-key"},
		"version":        {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0\"\nsource_root = \"s\"\n", "c-bindings.invalid-version"},
		"escape":         {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"../s\"\n", "c-bindings.unsafe-path"},
		"absolute":       {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n[modules]\ncommon = [\"/etc\"]\n", "c-bindings.unsafe-path"},
		"duplicate":      {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n[modules]\ncommon = [\"a\", \"a\"]\n", "c-bindings.duplicate-module"},
		"noncanonical":   {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n[environments.e]\nos = \"darwin\"\narch = \"arm64\"\nabi = \"gnu\"\nc_runtime = \"libc\"\nmodules = []\n", "c-bindings.noncanonical-target"},
		"constraint":     {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n[environments.e]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\nc_runtime_versions = \"^2.31\"\nmodules = []\n", "c-bindings.invalid-constraint"},
		"missing fields": {"schema_version = 1\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"s\"\n[environments.e]\nos = \"linux\"\n", "c-bindings.missing-field"},
		"invalid toml":   {"schema_version = \n", "c-bindings.invalid-toml"},
	} {
		if _, diagnostics := Parse(check.text); !hasDiagnostic(diagnostics, check.id) {
			t.Errorf("%s: %v, want %s", name, diagnostics, check.id)
		}
	}
}

// MD-021 § 8.4: versions and comparator conjunctions.
func TestVersionConstraints(t *testing.T) {
	constraint, err := ParseConstraint(">=2.31, <3.0", false)
	if err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]bool{"2.31": true, "2.39": true, "2.30.9": false, "3.0": false} {
		version, _ := ParseRuntimeVersion(text)
		if constraint.Allows(version) != want {
			t.Errorf("%s in %v = %v", text, constraint, !want)
		}
	}
	for _, bad := range []string{"", "~1.0.0", "1.0.0", ">=1.0.0 || <2.0.0", ">=1.x.0"} {
		if _, err := ParseConstraint(bad, true); err == nil {
			t.Errorf("constraint %q accepted", bad)
		}
	}
	if _, err := ParsePackageVersion("1.0"); err == nil {
		t.Error("package version 1.0 accepted")
	}
}

func linuxRequest(runtime, version string) Request {
	return Request{Variant: "linux-amd64", OS: "linux", Arch: "amd64", ABI: "gnu", CRuntime: runtime, CRuntimeVersion: version}
}

// MD-021 § 8.5, § 9(2): glibc versus musl, arm64 versus amd64, Windows UCRT
// and runtime-version handling.
func TestResolveSelectsOneEnvironment(t *testing.T) {
	candidates := []Candidate{{mustParse(t, hostedPackage), "compiler"}}
	for name, check := range map[string]struct {
		request     Request
		environment string
	}{
		"glibc":   {linuxRequest("glibc", "2.39"), "linux-amd64-glibc"},
		"musl":    {linuxRequest("musl", "1.2.5"), "linux-amd64-musl"},
		"arm64":   {Request{Variant: "linux-arm64", OS: "linux", Arch: "arm64", ABI: "gnu", CRuntime: "glibc"}, "linux-arm64-glibc"},
		"windows": {Request{Variant: "windows-amd64", OS: "windows", Arch: "amd64", ABI: "msvc", CRuntime: "ucrt"}, "windows-amd64-ucrt"},
		"alias":   {Request{Variant: "arm", OS: "linux", Arch: "arm64", ABI: "gnu", CRuntime: "glibc"}, "linux-arm64-glibc"},
	} {
		resolution, diagnostics := Resolve(check.request, candidates)
		if len(diagnostics) != 0 || resolution.Environment != check.environment {
			t.Errorf("%s: %+v %v", name, resolution, diagnostics)
		}
	}
	glibc, _ := Resolve(linuxRequest("glibc", "2.39"), candidates)
	if strings.Join(glibc.Modules, ",") != "c/common,c/linux/glibc/common,c/linux/glibc/amd64" || glibc.PackageVersion != "1.0.0" || len(glibc.Fingerprint) != 64 {
		t.Errorf("glibc resolution = %+v", glibc)
	}
	for name, check := range map[string]struct {
		request Request
		id      string
	}{
		"unknown runtime version": {linuxRequest("glibc", ""), "c-bindings.no-candidate"},
		"runtime out of range":    {linuxRequest("glibc", "2.17"), "c-bindings.no-candidate"},
		"no runtime family":       {linuxRequest("bionic", "1.0"), "c-bindings.no-candidate"},
	} {
		if _, diagnostics := Resolve(check.request, candidates); !hasDiagnostic(diagnostics, check.id) {
			t.Errorf("%s: %v, want %s", name, diagnostics, check.id)
		}
	}
}

// MD-021 § 8.2(3), § 8.3, § 8.5(2): explicit selection, versions,
// ambiguity and identity conflicts.
func TestResolveSelectionAndConflicts(t *testing.T) {
	first := mustParse(t, hostedPackage)
	second := mustParse(t, strings.Replace(hostedPackage, `version = "1.0.0"`, `version = "1.1.0"`, 1))
	both := []Candidate{{first, "compiler"}, {second, "compiler"}}
	request := linuxRequest("glibc", "2.39")
	if _, diagnostics := Resolve(request, both); !hasDiagnostic(diagnostics, "c-bindings.ambiguous") {
		t.Fatalf("two versions without a selector: %v", diagnostics)
	}
	request.Selector = &Selector{ID: "sec.hosted-c", Version: "=1.1.0"}
	if resolution, diagnostics := Resolve(request, both); len(diagnostics) != 0 || resolution.PackageVersion != "1.1.0" {
		t.Fatalf("exact version: %+v %v", resolution, diagnostics)
	}
	for name, check := range map[string]struct {
		selector Selector
		id       string
	}{
		"unknown package":     {Selector{ID: "other"}, "c-bindings.unknown-package"},
		"unsatisfied version": {Selector{ID: "sec.hosted-c", Version: ">=2.0.0"}, "c-bindings.version-unsatisfied"},
		"unknown environment": {Selector{ID: "sec.hosted-c", Version: "=1.0.0", Environment: "macos"}, "c-bindings.unknown-environment"},
		"incompatible env":    {Selector{ID: "sec.hosted-c", Version: "=1.0.0", Environment: "windows-amd64-ucrt"}, "c-bindings.no-candidate"},
	} {
		selector := check.selector
		request.Selector = &selector
		if _, diagnostics := Resolve(request, both); !hasDiagnostic(diagnostics, check.id) {
			t.Errorf("%s: %v, want %s", name, diagnostics, check.id)
		}
	}
	request.Selector = nil
	changed := mustParse(t, strings.Replace(hostedPackage, `common = ["c/common"]`, `common = ["c/common2"]`, 1))
	if _, diagnostics := Resolve(request, []Candidate{{first, "compiler"}, {changed, "project"}}); !hasDiagnostic(diagnostics, "c-bindings.identity-conflict") {
		t.Fatalf("project package with the compiler identity and other content: %v", diagnostics)
	}
	if resolution, diagnostics := Resolve(request, []Candidate{{first, "compiler"}, {mustParse(t, hostedPackage), "project"}}); len(diagnostics) != 0 || resolution.Origin != "compiler" {
		t.Fatalf("identical duplicate is deduplicated: %+v %v", resolution, diagnostics)
	}
}

// MD-021 § 8.8(2): the fingerprint ignores module order and comments.
func TestResolutionFingerprintIsOrderIndependent(t *testing.T) {
	reordered := strings.Replace(hostedPackage, `modules = ["c/linux/glibc/common", "c/linux/glibc/amd64"]`, "# comment\nmodules = [\"c/linux/glibc/amd64\", \"c/linux/glibc/common\"]", 1)
	request := linuxRequest("glibc", "2.39")
	a, _ := Resolve(request, []Candidate{{mustParse(t, hostedPackage), "compiler"}})
	b, _ := Resolve(request, []Candidate{{mustParse(t, reordered), "compiler"}})
	if a == nil || b == nil || a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprints differ: %v %v", a, b)
	}
	other, _ := Resolve(linuxRequest("glibc", "2.31"), []Candidate{{mustParse(t, hostedPackage), "compiler"}})
	if other.Fingerprint == a.Fingerprint {
		t.Fatal("a different runtime version kept the fingerprint")
	}
}
