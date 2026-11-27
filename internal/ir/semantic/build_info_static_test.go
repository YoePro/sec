package semantic

import (
	"bytes"
	"fmt"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Whole BuildInfo static values retain one selected-plan snapshot, including
// string/optional fields and source identity, without startup inspection or
// runtime construction. Serialization retains the complete mixed value tree.
// Rules: rules/compiler/compiler_known_members.md — §113B;
// rules/library/core-library.md — §28; rules/declarations/static.md — §§15-16.
func TestBuildInfoWholeStatic(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/present-%t", width, present), func(t *testing.T) {
				program := sliceBoundaryProgram(t, "semantic_ir/build_info/static.sec")
				analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				user, revision, dirty := "", "source-revision", false
				snapshot := sema.BuildInfoSnapshot{ProjectName: "demo", TargetName: "app", VariantName: "cross", VersionBuild: 42, OperatingSystem: "cross-os", Architecture: "cross-arch"}
				if present {
					snapshot.VersionUserDefined, snapshot.SourceRevision, snapshot.DirtySourceStatus = &user, &revision, &dirty
				}
				analyzer.SetBuildInfo(snapshot)
				if errors := analyzer.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 19})
				if err != nil {
					t.Fatal(err)
				}
				if err := Verify(module); err != nil {
					t.Fatal(err)
				}
				if len(module.Statics) != 1 || !module.Statics[0].Initializer.Aggregate || len(module.Statics[0].Initializer.Components) != 16 {
					t.Fatal("missing whole snapshot")
				}
				original, err := Serialize(module)
				if err != nil {
					t.Fatal(err)
				}
				restored, err := Deserialize(original)
				if err != nil {
					t.Fatal(err)
				}
				again, err := Serialize(restored)
				if err != nil || !bytes.Equal(original, again) {
					t.Fatal("snapshot changed across transport", err)
				}
				value := restored.Statics[0].Initializer
				if value.Components[7].Integer.String() != "42" || *value.Components[11].String != "cross-arch" {
					t.Fatal("wrong target snapshot")
				}
				for _, index := range []int{8, 14, 15} {
					variant := value.Components[index].Variant
					want := "None"
					if present {
						want = "Some"
					}
					if variant == nil || variant.Name != want {
						t.Fatal("lost Option state", index)
					}
				}
				loads := 0
				for _, fn := range restored.Functions {
					for _, block := range fn.Blocks {
						for _, op := range block.Operations {
							if op.Kind == OpDirectCall || op.Kind == OpStructConstruct {
								t.Fatal("reconstructed static at runtime", op.Kind)
							}
							if op.Kind == OpStaticLoad {
								loads++
							}
						}
					}
				}
				if loads != 4 {
					t.Fatalf("expected four ordinary reads of one static, got %d", loads)
				}
				restored.Statics[0].Initializer.Components = restored.Statics[0].Initializer.Components[:15]
				if _, err := Serialize(restored); err == nil {
					t.Fatal("serialized incomplete BuildInfo")
				}
			})
		}
	}
}
