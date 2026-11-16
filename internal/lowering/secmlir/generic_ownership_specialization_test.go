package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// Generic specializations with an ownership-sensitive argument resolve the
// template's symbolic ownership, cleanup and destruction facts for their
// concrete argument (GOS-1..7): a move-only T registers, cancels, discards,
// replaces and destroys with the concrete plan, while T = int keeps trivial
// copies and no registrations. Free functions, impl[T] receivers, generic
// structs and unions, Option/Result payloads, try, defer and loops are covered,
// and the module passes the schema-13 Place, reference-guard, cleanup,
// destruction-plan and ownership verifiers on 32- and 64-bit targets.
// Rules: rules/compiler/monomorphization.md — §§ 11.2, 11.4, 12.3;
// rules/memory/destruction.md — §§ 6.1, 9.2, 13; rules/memory/copy_move.md — §§ 3–5.
func TestSchema13GenericOwnershipSpecialization(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/generic_ownership_specialization.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "generic.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"generic.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(id string) string {
				start := strings.Index(text, `func "main::`+id)
				if start < 0 {
					t.Fatalf("missing specialization %s", id)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			const box = `plan="destroy:main:struct:Box"`
			for id, wants := range map[string][]string{
				"Keep[T=main::Box](main::Box)":                             {"cleanup.track-owned %0", box, "reason=transferred"},
				"Throw[T=main::Box](main::Box)":                            {"own.discard %0 " + box + " origin=explicit", "reason=discarded"},
				"Statement[T=main::Box](main::Box)":                        {"origin=implicit-call-result"},
				"Reassign[T=main::Box](main::Box,main::Box)":               {"own.replace-place", "cleanup=#"},
				"Unwrap[T=main::Box](Option<main::Box>,main::Box)":         {`plan="destroy::union:Option[main:struct:Box]"`, "variant=#0[move]"},
				"Cell.Set[T=main::Box](main::Box)":                         {"own.replace-place %3, %5 " + box},
				"Open[T=main::Box](main::Cell<main::Box>)":                 {"own.move-from-place", `plan="destroy:main:struct:Cell[main:struct:Box]"`},
				"TakeLeft[T=main::Box](main::Either<main::Box>,main::Box)": {`plan="destroy:main:union:Either[main:struct:Box]"`},
				"Through[T=main::Box](main::Box)":                          {"exit=error-propagation", "result.unwrap-ok"},
				"Deferred[T=main::Box](main::Box)":                         {"cleanup.defer-register", "Peek[T=main::Box](ref main::Box)"},
			} {
				part := body(id)
				for _, want := range wants {
					if !strings.Contains(part, want) {
						t.Errorf("%s lacks %q:\n%s", id, want, part)
					}
				}
			}
			// A trivial argument needs none of the template's obligations.
			for _, id := range []string{"Keep[T=int](int)", "Throw[T=int](int)", "Reassign[T=int](int,int)", "Unwrap[T=int](Option<int>,int)", "Cell.Set[T=int](int)", "Deferred[T=int](int)"} {
				if part := body(id); strings.Contains(part, "cleanup.track-owned") || strings.Contains(part, "own.replace-place") || strings.Contains(part, "own.discard") {
					t.Errorf("%s keeps an ownership obligation for a trivial argument:\n%s", id, part)
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
