package temporal_test

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/concurrency/mutex.md §13(1)-(5); cancellation.md §43(4).
func TestInstantCanonicalIdentity(t *testing.T) {
	source, err := os.ReadFile("../../../sec/core/instant.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, core := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "trusted_core"}[core], func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(fixture(t, "instant_identity.sec"), "application.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			if core {
				trusted := parser.New(lexer.NewWithFile(string(source), "sec/core/instant.sec")).Parse()
				if trusted.HasErrors {
					t.Fatal(trusted.Diagnostics)
				}
				parsed.Program.Statements = append(trusted.Program.Statements, parsed.Program.Statements...)
				parsed.Program.SourceProvenance = map[string]ast.SourceProvenance{"sec/core/instant.sec": ast.SourceCore}
			}
			analyzer := sema.NewAnalyzer()
			if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
				t.Fatal(errors)
			}
			typ := analyzer.Types()["Instant"]
			if !typ.MonotonicPoint || !typ.Intrinsic || typ.Kind != sema.StructType || len(typ.Fields) != 0 ||
				!sema.TriviallyCopyable(typ) || !sema.TriviallyDestructible(typ) || sema.IsDefaultable(typ) {
				t.Fatalf("incorrect opaque identity: %+v", typ)
			}
			if core && (!typ.Declared || typ.Module != "core") {
				t.Fatalf("core identity lost: %+v", typ)
			}
		})
	}
}

// Rules: rules/concurrency/mutex.md §13; types/temporal.md §4.
func TestInstantRejectsInventedSurface(t *testing.T) {
	for _, name := range []string{"instant_construct_invalid.sec", "instant_derived_construct_invalid.sec", "instant_derived_convert_invalid.sec", "instant_redeclare_invalid.sec", "instant_convert_invalid.sec", "instant_now_invalid.sec", "instant_wall_clock_invalid.sec", "instant_compare_invalid.sec", "instant_arithmetic_invalid.sec", "instant_default_invalid.sec"} {
		t.Run(name, func(t *testing.T) {
			if errors := analyzeSourceRaw(t, fixture(t, name)); len(errors) == 0 {
				t.Fatal("invalid Instant source accepted")
			}
		})
	}
	for _, name := range []string{"instant_backing_invalid.sec", "instant_enum_invalid.sec", "instant_interface_invalid.sec"} {
		t.Run(name, func(t *testing.T) {
			_, _, errors := analyzeTemporalCoreSource(t, fixture(t, name))
			if len(errors) == 0 {
				t.Fatal("core replaced opaque Instant identity")
			}
		})
	}
}
