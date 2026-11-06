package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// The editor consumes the same panic effects, stable cleanup identities, source
// links and practical help as compilation, including diagnosed ISR contexts.
// Rules: rules/errors/panic.md — §28(5–6); rules/tooling/lsp.md — Shared diagnostic model.
func TestPanicMentorLSPDiagnostics(t *testing.T) {
	for _, name := range []string{"panic_mentor_invalid", "isr_panic_paths_invalid", "panic_mentor_valid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/sema/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			counts := map[string]int{}
			for _, d := range analyze(uri, string(data)) {
				if d.Severity != 1 {
					continue
				}
				if name == "panic_mentor_valid" {
					t.Fatal(d)
				}
				if d.Code != diagnostics.NoPanicViolation && d.Code != diagnostics.InvalidDeferredCleanup && d.Code != diagnostics.TryPropagationIncompatible && d.Code != diagnostics.TryResidualUnpropagatable {
					continue
				}
				counts[d.Code]++
				if !strings.Contains(d.Message, "help:") {
					t.Fatalf("missing mentor help: %+v", d)
				}
				if d.Code == diagnostics.NoPanicViolation {
					if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.URI != uri {
						t.Fatalf("lost root/cause navigation: %+v", d)
					}
					if name == "isr_panic_paths_invalid" && (!strings.Contains(d.Message, "unsafe does not waive") || strings.Contains(d.Message, "remove @noPanic")) {
						t.Fatalf("inappropriate ISR repair: %+v", d)
					}
					if strings.Contains(d.Message, "panic path runs through") && strings.Contains(d.Message, "remove @noPanic") {
						t.Fatalf("inappropriate cleanup repair: %+v", d)
					}
				}
			}
			if name != "panic_mentor_valid" && counts[diagnostics.NoPanicViolation] != 7 {
				t.Fatal("missing panic diagnostics", counts)
			}
			if name == "panic_mentor_invalid" && (counts[diagnostics.InvalidDeferredCleanup] != 8 || counts[diagnostics.TryPropagationIncompatible] != 2 || counts[diagnostics.TryResidualUnpropagatable] != 2) {
				t.Fatal("missing cleanup diagnostics", counts)
			}
		})
	}
}
