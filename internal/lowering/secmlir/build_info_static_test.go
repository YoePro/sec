package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Whole BuildInfo statics emit the existing schema-13 mixed aggregate constant:
// target strings and both Option states remain literal data, not runtime calls.
// Malformed constants publish no output.
// Rules: rules/compiler/compiler_known_members.md — §113B(1)-(4);
// rules/library/core-library.md — §28; rules/compiler/semantic_ir.md — §28.
func TestBuildInfoWholeStaticEmission(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/build_info/static.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/present-%t", width, present), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "build-info-static.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(testPlan(width))
				user, revision, dirty := "", "source-revision", false
				snapshot := sema.BuildInfoSnapshot{ProjectName: "demo", TargetName: "app", VariantName: "cross", VersionBuild: 42, OperatingSystem: "cross-os", Architecture: "cross-arch"}
				if present {
					snapshot.VersionUserDefined, snapshot.SourceRevision, snapshot.DirtySourceStatus = &user, &revision, &dirty
				}
				analyzer.SetBuildInfo(snapshot)
				if errors := analyzer.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				module, err := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 19})
				if err != nil {
					t.Fatal(err)
				}
				options := EmitOptions{DialectVersion: 13}
				output, err := EmitWithOptions(module, testPlan(width), options)
				if err != nil {
					t.Fatal(err)
				}
				again, err := EmitWithOptions(module, testPlan(width), options)
				if err != nil || !bytes.Equal(output, again) {
					t.Fatal("nondeterministic snapshot emission", err)
				}
				text := string(output)
				for _, want := range []string{`"sec.static.global"`, `sec.static_initializer = [`, `{string = "demo"}`, `{string = "cross-os"}`, `{string = "cross-arch"}`, `"42"`} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %s", want)
					}
				}
				if strings.Count(text, `"sec.static.global"`) != 1 || strings.Contains(text, `"sec.call"`) || strings.Contains(text, `"sec.struct.construct"`) {
					t.Fatal("runtime metadata construction")
				}
				if present {
					for _, want := range []string{`{payload = {string = ""}, variant = "Some"}`, `{payload = "false", variant = "Some"}`} {
						if !strings.Contains(text, want) {
							t.Fatalf("missing %s", want)
						}
					}
				} else if strings.Count(text, `{variant = "None"}`) != 3 {
					t.Fatal("missing absent metadata")
				}
				module.Statics[0].Initializer.Components = module.Statics[0].Initializer.Components[:15]
				if output, err := EmitWithOptions(module, testPlan(width), options); err == nil || len(output) != 0 {
					t.Fatal("emitted incomplete BuildInfo")
				}
			})
		}
	}
}
