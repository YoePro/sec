package sema

import (
	"fmt"
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Whole BuildInfo constants retain all sixteen fields from one explicit plan
// snapshot, including Some("")/Some(false) as distinct from absent facts. The
// resolved static tree, producer inputs and public queries have separate storage.
// Rules: rules/library/core-library.md — §28(1)-(4);
// rules/compiler/compiler_known_members.md — §113B(1)-(4);
// rules/declarations/static.md — §§15-16.
func TestBuildInfoWholeStaticInitializers(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/build_info/static.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for mask := 0; mask < 8; mask++ {
			t.Run(fmt.Sprintf("%d/options-%d", width, mask), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "build-info-static.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				snapshot := testBuildInfo
				snapshot.VersionMajor = 9223372036854775807
				snapshot.OperatingSystem, snapshot.Architecture = "cross-os", "cross-arch"
				snapshot.ProjectName = "räksmörgås\n\"Sec\""
				user, revision, dirty := "", "rév\n\"123\"", false
				if mask&1 != 0 {
					snapshot.VersionUserDefined = &user
				}
				if mask&2 != 0 {
					snapshot.SourceRevision = &revision
				}
				if mask&4 != 0 {
					snapshot.DirtySourceStatus = &dirty
				}
				a.SetBuildInfo(snapshot)
				user, revision, dirty = "producer mutation", "producer mutation", true
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				statics := a.ResolvedStaticDeclarations()
				if len(statics) != 1 || statics[0].Name != "Info" || !statics[0].Initializer.Aggregate || len(statics[0].Initializer.Components) != 16 {
					t.Fatal("missing complete static snapshot")
				}
				info := statics[0]
				strings := map[string]string{
					"ProjectUUID": snapshot.ProjectUUID, "ProjectName": snapshot.ProjectName, "TargetName": snapshot.TargetName, "VariantName": snapshot.VariantName,
					"VersionRendered": snapshot.VersionRendered, "OperatingSystem": "cross-os", "Architecture": "cross-arch", "BuildProfile": snapshot.BuildProfile, "TargetProfile": snapshot.TargetProfile,
				}
				numbers := map[string]uint64{"VersionMajor": snapshot.VersionMajor, "VersionMinor": snapshot.VersionMinor, "VersionRevision": snapshot.VersionRevision, "VersionBuild": snapshot.VersionBuild}
				for i, field := range info.Type.Fields {
					value := info.Initializer.Components[i]
					if want, ok := strings[field.Name]; ok {
						if value.Scalar == nil || value.Scalar.String != want {
							t.Fatalf("lost string field %s", field.Name)
						}
						value.Scalar.String = "query mutation"
					} else if want, ok := numbers[field.Name]; ok {
						if value.Integer == nil || !value.Integer.IsUint64() || value.Integer.Uint64() != want {
							t.Fatalf("lost uint64 field %s", field.Name)
						}
						value.Integer.SetUint64(99)
					} else {
						bit := map[string]int{"VersionUserDefined": 1, "SourceRevision": 2, "DirtySourceStatus": 4}[field.Name]
						variant := value.Variant
						if variant == nil {
							t.Fatalf("lost Option field %s", field.Name)
						}
						if mask&bit == 0 {
							if variant.Name != "None" || variant.Payload != nil {
								t.Fatalf("invented optional metadata %s", field.Name)
							}
						} else {
							if variant.Name != "Some" || variant.Payload == nil {
								t.Fatalf("lost present metadata %s", field.Name)
							}
							if field.Name == "DirtySourceStatus" {
								if variant.Payload.Bool == nil || *variant.Payload.Bool {
									t.Fatal("Some(false) became absent/true")
								}
							} else {
								want := ""
								if field.Name == "SourceRevision" {
									want = "rév\n\"123\""
								}
								if variant.Payload.Scalar == nil || variant.Payload.Scalar.String != want {
									t.Fatal("Some string changed")
								}
							}
						}
						variant.Name = "query mutation"
					}
				}
				again, ok := a.ResolvedStaticOf(info.Declaration.Name)
				if !ok || again.Initializer.Components[0].Scalar.String != snapshot.ProjectUUID || again.Initializer.Components[4].Integer.Uint64() != snapshot.VersionMajor || again.Initializer.Components[8].Variant.Name == "query mutation" {
					t.Fatal("query mutated frozen static")
				}
			})
		}
	}
}
