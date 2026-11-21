package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/platform/cbindingplan"
	"sec/internal/sema"
)

func writeCBindingProject(t *testing.T, variants string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".sec/sec.toml": "[project]\nname = \"demo\"\n\n" + variants,
		".sec/platform/c-bindings.toml": `schema_version = 1

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
modules = ["c/linux/amd64"]
`,
		"sec/platform/c/common/common.sec":      "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::uint)\n",
		"sec/platform/c/common/common_test.sec": "module common\n",
		"sec/platform/c/linux/amd64/amd64.sec":  "module amd64\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n",
		"app/app.sec":                           "module app\n",
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const glibcVariant = "[variant.linux-amd64]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\n"

// MD-021 §§ 8.3, 8.5-8.7: the Variant's binding environment is resolved
// before Sema, its module sources are loaded with their layers, an explicit
// but incompatible selector fails closed and an unselected one stays empty.
func TestResolveCLICBindingPlan(t *testing.T) {
	linux, _ := parseCompilerTarget("linux-amd64")
	root := writeCBindingProject(t, glibcVariant+"c_bindings = { id = \"sec.hosted-c\", version = \"=1.0.0\" }\n")
	app := filepath.Join(root, "app", "app.sec")
	plan, problems := resolveCLICBindingPlan([]string{app}, linux)
	if len(problems) != 0 || plan == nil || plan.plan.Resolution.Environment != "linux-amd64-glibc" || len(plan.programs) != 2 {
		t.Fatalf("plan = %+v, problems %v", plan, problems)
	}
	layers := map[string]sema.CBindingLayer{}
	for file, layer := range plan.surface.Files {
		layers[filepath.Base(file)] = layer
	}
	if len(layers) != 2 || layers["common.sec"] != sema.CBindingCommon || layers["amd64.sec"] != sema.CBindingEnvironment {
		t.Fatalf("layers = %v", layers)
	}
	if plan.surface.Identity != "sec.hosted-c@1.0.0/linux-amd64-glibc" {
		t.Fatalf("identity = %q", plan.surface.Identity)
	}

	musl := writeCBindingProject(t, strings.Replace(glibcVariant, "glibc", "musl", 1)+"c_bindings = { id = \"sec.hosted-c\" }\n")
	if plan, problems := resolveCLICBindingPlan([]string{filepath.Join(musl, "app", "app.sec")}, linux); plan != nil || len(problems) != 1 || !strings.Contains(problems[0], "c-bindings.no-candidate") {
		t.Fatalf("incompatible selected = %+v %v", plan, problems)
	}
	unselected := writeCBindingProject(t, strings.Replace(glibcVariant, "glibc", "musl", 1))
	if plan, problems := resolveCLICBindingPlan([]string{filepath.Join(unselected, "app", "app.sec")}, linux); plan != nil || len(problems) != 0 {
		t.Fatalf("incompatible unselected = %+v %v", plan, problems)
	}
	implicit := writeCBindingProject(t, glibcVariant)
	if plan, problems := resolveCLICBindingPlan([]string{filepath.Join(implicit, "app", "app.sec")}, linux); plan == nil || len(problems) != 0 {
		t.Fatalf("compatible unselected = %+v %v", plan, problems)
	}
	windows, _ := parseCompilerTarget("windows-amd64")
	if plan, problems := resolveCLICBindingPlan([]string{app}, windows); plan != nil || len(problems) != 0 {
		t.Fatalf("no Variant for the target = %+v %v", plan, problems)
	}
	if err := os.RemoveAll(filepath.Join(root, "sec", "platform", "c", "linux")); err != nil {
		t.Fatal(err)
	}
	if plan, problems := resolveCLICBindingPlan([]string{app}, linux); plan != nil || len(problems) != 1 || !strings.Contains(problems[0], "c-bindings.missing-module") {
		t.Fatalf("missing module = %+v %v", plan, problems)
	}
}

// projects.md §§ 19-21: without a named Variant, only a unique os/arch
// match supplies the binding environment.
func TestUniqueVariantForTarget(t *testing.T) {
	linux, _ := parseCompilerTarget("linux-amd64")
	two := glibcVariant + "\n[variant.linux-amd64-musl]\nos = \"linux\"\narch = \"amd64\"\n"
	if name := cbindingplan.UniqueVariant(glibcVariant, linux); name != "linux-amd64" {
		t.Fatalf("unique = %q", name)
	}
	if name := cbindingplan.UniqueVariant(two, linux); name != "" {
		t.Fatalf("ambiguous = %q", name)
	}
}

// MD-139 §§ 2-5: a Variant's profile/allocation selection is validated
// before analysis; a valid baseline and an absent selection pass, a
// reclassifying profile and an unknown mode are reported.
func TestValidateCLIPlanSelection(t *testing.T) {
	linux, _ := parseCompilerTarget("linux-amd64")
	definition, _ := findTargetDefinition(linux)
	for _, test := range []struct {
		extra string
		want  string
	}{
		{"", ""},
		{"profile = \"hosted\"\n", ""},
		{"profile = \"baremetal\"\n", "would reclassify"},
		{"\n[variant.linux-amd64.allocation]\nmode = \"sometimes\"\n", "sometimes"},
	} {
		root := writeCBindingProject(t, glibcVariant+test.extra)
		problems := validateCLIPlanSelection([]string{filepath.Join(root, "app", "app.sec")}, linux, definition.Definition)
		if test.want == "" && len(problems) != 0 || test.want != "" && (len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), test.want)) {
			t.Fatalf("%q: problems %v, want %q", test.extra, problems, test.want)
		}
	}
}
