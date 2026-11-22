package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeWithBuildInfo(t *testing.T, source string, snapshot *BuildInfoSnapshot) (*Analyzer, []Error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "build.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if snapshot != nil {
		a.SetBuildInfo(*snapshot)
	}
	return a, a.Analyze(result.Program)
}

var testBuildInfo = BuildInfoSnapshot{
	ProjectUUID: "0198e5b8-1a2b-7c3d-8e4f-5a6b7c8d9e0f", ProjectName: "demo", TargetName: "app", VariantName: "linux-arm64",
	VersionMajor: 1, VersionMinor: 2, VersionRevision: 3, VersionBuild: 42, VersionRendered: "1.2.3.42",
	OperatingSystem: "linux", Architecture: "arm64", BuildProfile: "release", TargetProfile: "hosted",
}

// MD-143 core-library.md § 28, compiler_known_members.md § 113B: the fields
// keep their canonical types, BuildProfile and TargetProfile are distinct,
// and Current() is usable in ordinary code and semantic CTE with the plan's
// target values (here a cross-compiled linux/arm64 plan).
func TestBuildInfoCurrentInCodeAndCTE(t *testing.T) {
	a, errs := analyzeWithBuildInfo(t, `module main

let Name := BuildInfo.Current().ProjectName
let Major := BuildInfo.Current().VersionMajor
let Arch := BuildInfo.Current().Architecture
type Label string default BuildInfo.Current().VersionRendered

fn Describe() string {
    let info := BuildInfo.Current()
    let build: uint64 := info.VersionBuild
    discard build
    let user: Option[string] := info.VersionUserDefined
    discard user
    let dirty: Option[bool] := info.DirtySourceStatus
    discard dirty
    return info.BuildProfile + "/" + info.TargetProfile
}
`, &testBuildInfo)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	a.currentModule = "main"
	for name, want := range map[string]string{"Name": `"demo"`, "Major": "1", "Arch": `"arm64"`} {
		value, outcome := a.semanticCompileTimeConstant(&ast.Identifier{Value: name})
		if outcome != compileTimeEvaluated || value.Lexeme != want {
			t.Fatalf("%s = %+v (%v), want %s", name, value, outcome, want)
		}
	}
	if label := a.types["Label"].ExplicitDefault; label == nil || label.String != "1.2.3.42" {
		t.Fatalf("Label default = %+v", label)
	}
	fields := map[string]string{}
	for _, field := range a.types["BuildInfo"].Fields {
		fields[field.Name] = typeDisplayName(field.Type)
	}
	for name, want := range map[string]string{"VersionMajor": "uint64", "ProjectUUID": "string", "BuildProfile": "string", "TargetProfile": "string", "SourceRevision": "Option[string]", "DirtySourceStatus": "Option[bool]"} {
		if fields[name] != want {
			t.Fatalf("field %s = %q, want %s (%v)", name, fields[name], want, fields)
		}
	}
	if _, single := fields["Profile"]; single || len(fields) != 16 {
		t.Fatalf("fields = %v", fields)
	}
}

// § 113B(3): without a resolved Target and Variant the call is diagnosed,
// never filled with defaults; user code cannot redeclare BuildInfo.
func TestBuildInfoCurrentRequiresResolvedPlan(t *testing.T) {
	_, errs := analyzeWithBuildInfo(t, "module main\n\nfn F() string {\n    return BuildInfo.Current().ProjectName\n}\n", nil)
	if len(errs) != 1 || errs[0].ID != diagnostics.BuildInfoUnavailable {
		t.Fatalf("errors = %v", errs)
	}
	_, errs = analyzeWithBuildInfo(t, "module main\n\ntype BuildInfo struct {\n    Name: string,\n}\n", &testBuildInfo)
	if len(errs) == 0 {
		t.Fatal("a user BuildInfo declaration was accepted")
	}
}

// MD-143 § 113B(2): the Option fields are ordinary Option values in semantic
// CTE. A present VersionUserDefined is Some of its text, an absent
// SourceRevision None, a present DirtySourceStatus Some(true); CTE functions
// match them, test them with `is`, and read the whole Current() value, also
// through a module constant.
func TestBuildInfoOptionFieldsInCTE(t *testing.T) {
	user, dirty := "beta", true
	snapshot := testBuildInfo
	snapshot.VersionUserDefined, snapshot.DirtySourceStatus = &user, &dirty
	_, errs := analyzeWithBuildInfo(t, `module main

fn UserLength() int {
    return match BuildInfo.Current().VersionUserDefined {
        Some(text) where text == "beta" => 14
        Some(text) => 2
        None => 1
    }
}

fn RevisionScore() int {
    match BuildInfo.Current().SourceRevision {
        Some(text) => return 100
        None => return 20
    }
}

fn DirtyScore() int {
    let info := BuildInfo.Current()
    if info.DirtySourceStatus is Some(flag) {
        if flag {
            return 30
        }
        return 31
    }
    return 32
}

type A int range 0..UserLength()
type B int range 0..RevisionScore()
type C int range 0..DirtyScore()

let Info := BuildInfo.Current()
type D int range 0..Info.VersionMajor

fn Main() int {
    let a: A := 15
    let b: B := 21
    let c: C := 31
    let d: D := 2
    return 0
}
`, &snapshot)
	want := []string{"A 0..14", "B 0..20", "C 0..30", "D 0..1"}
	if len(errs) != len(want) {
		t.Fatalf("errors = %v", errs)
	}
	for index, contract := range want {
		if errs[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errs[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errs[index].Message, contract)
		}
	}
}
