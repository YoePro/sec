package semantic

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// MD-143 § 113B(2): runtime BuildInfo.Current() is a statically materialized
// struct.construct of the snapshot; present Option facts are Some of a
// constant, absent ones None, and the module verifies.
func TestBuildInfoCurrentMaterializesSnapshot(t *testing.T) {
	source := "module main\n\nfn Name() string {\n    let info := BuildInfo.Current()\n    return info.TargetName\n}\n\nfn Whole() BuildInfo {\n    return BuildInfo.Current()\n}\n"
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	a := sema.NewAnalyzer()
	user, dirty := "beta", true
	a.SetBuildInfo(sema.BuildInfoSnapshot{ProjectName: "demo", TargetName: "app", VariantName: "v", VersionBuild: 9,
		VersionUserDefined: &user, VersionRendered: "1.0.0.9-beta", OperatingSystem: "linux", Architecture: "arm64",
		BuildProfile: "debug", TargetProfile: "hosted", DirtySourceStatus: &dirty})
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: 15})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`!2 = struct "core::BuildInfo"`,
		`const.string "app"`, `const.string "arm64"`, `const.int 9`, `const.string "beta"`, `const.bool true`,
		"union.construct variant=#0 %", "union.construct variant=#1 :",
		"struct.extract-field",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Count(text, `struct.construct #0=`) != 2 {
		t.Fatalf("expected one materialization per call:\n%s", text)
	}
}

// MD-143 § 113B(2): a module constant of an Option field is a static whose
// initializer is the CTE variant value (Some of the snapshot text).
func TestBuildInfoOptionFieldStatic(t *testing.T) {
	source := "module main\n\nlet User := BuildInfo.Current().VersionUserDefined\n\nfn F() Option[string] {\n    return User\n}\n"
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	a := sema.NewAnalyzer()
	user := "beta"
	a.SetBuildInfo(sema.BuildInfoSnapshot{ProjectName: "demo", VersionUserDefined: &user})
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: 15})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if len(module.Statics) != 1 || module.Statics[0].Name != "main::User" {
		t.Fatalf("statics = %+v", module.Statics)
	}
	if text := Format(module); !strings.Contains(text, "beta") || !strings.Contains(text, "Some") {
		t.Fatalf("static initializer lacks Some(\"beta\"):\n%s", text)
	}
}
