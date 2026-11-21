package cbindings

import (
	"strings"
	"testing"
)

const editFixture = `# Binding package for hosted targets.
schema_version = 1

[bindings]
id = "sec.hosted-c" # stable identity
version = "1.0.0"
source_root = "sec/platform"

[modules]
common = [
    "c/common", # shared declarations
]

# glibc on amd64
[environments.linux-amd64-glibc]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_versions = ">=2.31, <3.0"
modules = [
    "c/linux/glibc/common",
    "c/linux/glibc/amd64"
]

# UCRT on Windows
[environments.windows-amd64-ucrt]
os = "windows"
arch = "amd64"
abi = "msvc"
c_runtime = "ucrt"
modules = ["c/windows/ucrt"]
`

func text(value string) *string { return &value }

func mustApply(t *testing.T, input string, op Operation) EditResult {
	t.Helper()
	result := Apply(input, op)
	if result.Rejected {
		t.Fatalf("%s rejected: %v", op.Kind, result.Diagnostics)
	}
	return result
}

// MD-021 §§ 4(3)-(4), 5(3): every mutation is a minimal edit that keeps
// every comment and unrelated line byte for byte.
func TestApplyMutationsPreserveComments(t *testing.T) {
	cases := []struct {
		name    string
		op      Operation
		want    []string // substrings of the result
		missing []string // substrings that must be gone
	}{
		{"package set", Operation{Kind: OpPackageSet, Version: text("1.1.0")}, []string{`version = "1.1.0"`, `id = "sec.hosted-c" # stable identity`}, []string{`"1.0.0"`}},
		{"environment add", Operation{Kind: OpEnvironmentAdd, Environment: "linux-amd64-musl", OS: text("linux"), Arch: text("amd64"), ABI: text("gnu"), CRuntime: text("musl")},
			[]string{"modules = [\"c/windows/ucrt\"]\n\n[environments.linux-amd64-musl]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"musl\"\nmodules = []\n"}, nil},
		{"environment set", Operation{Kind: OpEnvironmentSet, Environment: "windows-amd64-ucrt", CRuntimeVersions: text(">=10.0")},
			[]string{"c_runtime = \"ucrt\"\nmodules = [\"c/windows/ucrt\"]\nc_runtime_versions = \">=10.0\"\n"}, nil},
		{"clear runtime versions", Operation{Kind: OpEnvironmentSet, Environment: "linux-amd64-glibc", ClearCRuntimeVersions: true},
			[]string{"c_runtime = \"glibc\"\nmodules = ["}, []string{"c_runtime_versions"}},
		{"environment remove", Operation{Kind: OpEnvironmentRemove, Environment: "linux-amd64-glibc"},
			[]string{"]\n\n# UCRT on Windows\n[environments.windows-amd64-ucrt]"}, []string{"glibc", "# glibc on amd64"}},
		{"module add multi-line", Operation{Kind: OpModuleAdd, Environment: "linux-amd64-glibc", Module: "c/linux/extra"},
			[]string{"    \"c/linux/glibc/amd64\",\n    \"c/linux/extra\"\n]"}, nil},
		{"module add trailing comma", Operation{Kind: OpModuleAdd, Common: true, Module: "c/more"},
			[]string{"    \"c/common\", # shared declarations\n    \"c/more\",\n]"}, nil},
		{"module add single-line", Operation{Kind: OpModuleAdd, Environment: "windows-amd64-ucrt", Module: "c/windows/extra"},
			[]string{`modules = ["c/windows/ucrt", "c/windows/extra"]`}, nil},
		{"module remove last", Operation{Kind: OpModuleRemove, Environment: "linux-amd64-glibc", Module: "c/linux/glibc/amd64"},
			[]string{"modules = [\n    \"c/linux/glibc/common\"\n]"}, nil},
		{"module remove with comment", Operation{Kind: OpModuleRemove, Common: true, Module: "c/common"},
			[]string{"common = [\n]"}, []string{"shared declarations"}},
		{"module remove single-line", Operation{Kind: OpModuleRemove, Environment: "windows-amd64-ucrt", Module: "c/windows/ucrt"},
			[]string{"modules = []"}, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := mustApply(t, editFixture, test.op)
			if !result.Changed {
				t.Fatalf("not changed")
			}
			for _, want := range test.want {
				if !strings.Contains(result.Text, want) {
					t.Fatalf("missing %q in\n%s", want, result.Text)
				}
			}
			for _, gone := range test.missing {
				if strings.Contains(result.Text, gone) {
					t.Fatalf("still has %q in\n%s", gone, result.Text)
				}
			}
			for _, comment := range []string{"# Binding package for hosted targets.", "# stable identity", "# UCRT on Windows"} {
				if !strings.Contains(result.Text, comment) {
					t.Fatalf("lost comment %q", comment)
				}
			}
			if again := Apply(result.Text, test.op); again.Rejected || again.Changed {
				if test.op.Kind != OpEnvironmentSet && test.op.Kind != OpPackageSet {
					t.Fatalf("repeat not a no-op: %+v", again)
				}
			}
		})
	}
}

// MD-021 § 5(4): identical repeats are no-ops with a machine-readable reason;
// a differing environment add is a conflict; unknown targets are rejected.
func TestApplyNoOpsAndRejections(t *testing.T) {
	noops := []struct {
		op     Operation
		reason string
	}{
		{Operation{Kind: OpModuleAdd, Common: true, Module: "c/common"}, ReasonAlreadyPresent},
		{Operation{Kind: OpModuleRemove, Common: true, Module: "c/absent"}, ReasonNotPresent},
		{Operation{Kind: OpEnvironmentRemove, Environment: "absent"}, ReasonNotPresent},
		{Operation{Kind: OpEnvironmentAdd, Environment: "windows-amd64-ucrt", OS: text("windows"), Arch: text("amd64"), ABI: text("msvc"), CRuntime: text("ucrt")}, ReasonAlreadyPresent},
		{Operation{Kind: OpEnvironmentSet, Environment: "windows-amd64-ucrt", OS: text("windows")}, ReasonUnchanged},
		{Operation{Kind: OpPackageSet, ID: text("sec.hosted-c")}, ReasonUnchanged},
	}
	for _, test := range noops {
		result := Apply(editFixture, test.op)
		if result.Rejected || result.Changed || result.Reason != test.reason || result.Text != editFixture {
			t.Fatalf("%s: got %+v, want no-op %s", test.op.Kind, result, test.reason)
		}
	}
	rejections := []struct {
		op Operation
		id string
	}{
		{Operation{Kind: OpEnvironmentAdd, Environment: "windows-amd64-ucrt", OS: text("windows"), Arch: text("arm64"), ABI: text("msvc"), CRuntime: text("ucrt")}, "c-bindings.environment-conflict"},
		{Operation{Kind: OpEnvironmentSet, Environment: "absent", OS: text("linux")}, "c-bindings.unknown-environment"},
		{Operation{Kind: OpModuleAdd, Environment: "absent", Module: "c/x"}, "c-bindings.unknown-environment"},
		{Operation{Kind: OpModuleAdd, Common: true, Module: "../escape"}, "c-bindings.unsafe-path"},
		{Operation{Kind: OpEnvironmentAdd, Environment: "Bad Name", OS: text("linux"), Arch: text("amd64"), ABI: text("gnu"), CRuntime: text("glibc")}, "c-bindings.invalid-environment-name"},
		{Operation{Kind: OpEnvironmentAdd, Environment: "partial", OS: text("linux")}, "c-bindings.missing-field"},
		{Operation{Kind: OpEnvironmentSet, Environment: "windows-amd64-ucrt", CRuntimeVersions: text("^1.0")}, "c-bindings.invalid-constraint"},
		{Operation{Kind: OpPackageSet, Version: text("v2")}, "c-bindings.invalid-version"},
	}
	for _, test := range rejections {
		result := Apply(editFixture, test.op)
		if !result.Rejected || result.Changed || result.Text != editFixture || len(result.Diagnostics) == 0 || result.Diagnostics[0].ID != test.id {
			t.Fatalf("%s: got %+v, want rejection %s", test.op.Kind, result, test.id)
		}
	}
	if result := Apply("schema_version = [", Operation{Kind: OpEnvironmentRemove, Environment: "x"}); !result.Rejected || result.Diagnostics[0].ID != "c-bindings.invalid-toml" {
		t.Fatalf("invalid TOML: %+v", result)
	}
}

// MD-021 § 5(3): layouts the minimal editor cannot change safely are
// rejected instead of being rewritten, and the input is returned untouched.
func TestApplyRejectsUnsupportedLayouts(t *testing.T) {
	dotted := "schema_version = 1\nbindings.id = \"x\"\nbindings.version = \"1.0.0\"\nbindings.source_root = \"sec\"\nenvironments.e = { os = \"linux\", arch = \"amd64\", abi = \"gnu\", c_runtime = \"glibc\", modules = [] }\n"
	if result := Apply(dotted, Operation{Kind: OpEnvironmentRemove, Environment: "e"}); !result.Rejected || result.Diagnostics[0].ID != "c-bindings.unsupported-edit" || result.Text != dotted {
		t.Fatalf("inline environment remove: %+v", result)
	}
	if result := mustApply(t, dotted, Operation{Kind: OpPackageSet, Version: text("2.0.0")}); !strings.Contains(result.Text, `bindings.version = "2.0.0"`) {
		t.Fatalf("dotted value replaced in place:\n%s", result.Text)
	}
}

// MD-021 § 5(3): a missing [modules] table is created before the first
// environment, and CRLF files keep CRLF.
func TestApplyCreatesTablesAndKeepsLineEndings(t *testing.T) {
	input := strings.ReplaceAll("schema_version = 1\n\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"sec\"\n\n# env\n[environments.e]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\nmodules = []\n", "\n", "\r\n")
	result := mustApply(t, input, Operation{Kind: OpModuleAdd, Common: true, Module: "c/common"})
	if !strings.Contains(result.Text, "source_root = \"sec\"\r\n\r\n[modules]\r\ncommon = [\"c/common\"]\r\n\r\n# env\r\n[environments.e]") {
		t.Fatalf("modules table:\n%q", result.Text)
	}
	if strings.Contains(strings.ReplaceAll(result.Text, "\r\n", ""), "\n") {
		t.Fatalf("bare LF introduced: %q", result.Text)
	}
	removed := mustApply(t, result.Text, Operation{Kind: OpEnvironmentRemove, Environment: "e"})
	if strings.Contains(removed.Text, "environments") || strings.Contains(removed.Text, "# env") || !strings.HasSuffix(removed.Text, "common = [\"c/common\"]\r\n") {
		t.Fatalf("remove last table: %q", removed.Text)
	}
	draft := "schema_version = 1\n\n[bindings]\nid = \"x\"\nversion = \"1.0.0\"\nsource_root = \"sec\"\n"
	added := mustApply(t, draft, Operation{Kind: OpEnvironmentAdd, Environment: "e", OS: text("linux"), Arch: text("amd64"), ABI: text("gnu"), CRuntime: text("glibc"), CRuntimeVersions: text(">=2.31")})
	if pkg, diagnostics := Parse(added.Text); len(diagnostics) != 0 || !pkg.Resolvable() || pkg.Environments["e"].CRuntimeVersions != ">=2.31" {
		t.Fatalf("draft add: %v\n%s", diagnostics, added.Text)
	}
}
