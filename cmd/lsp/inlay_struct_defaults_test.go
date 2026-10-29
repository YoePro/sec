package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func structDefaultHintFixture(t *testing.T, name string) (string, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/lsp/struct_default_hints", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return uriFromPath(path), string(data)
}

// Defaults come from validated final-field sources, including explicit type,
// range, membership, enum and recursive aggregate selection. Spread and explicit
// fields never masquerade as defaults; source delimiters anchor nested hints.
// Rules: rules/types/default_values.md — A.7,A.11;
// rules/declarations/struct.md — §§6,9; rules/tooling/lsp.md — "Inlay hints".
func TestInlayHintsOmittedStructDefaults(t *testing.T) {
	uri, original := structDefaultHintFixture(t, "valid.sec")
	for _, text := range []string{original, strings.ReplaceAll(original, "\n", "\r\n")} {
		if reported := analyze(uri, text); len(reported) != 0 {
			t.Fatal(reported)
		}
		hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Defaults: true})
		cases := []struct {
			marker             string
			includes, excludes []string
		}{
			{"empty", []string{"port: 8080", "count: 1", "choice: 7", "enabled: false", `name: ""`, "mode: Mode.First", "inner: Inner { level: 0 }", "values: [0, 0, 0]"}, nil},
			{"explicit", []string{"count: 1", "choice: 7"}, []string{"port:", "enabled:"}},
			{"inner", []string{"defaults: level: 0"}, nil},
			{"outer", []string{"port: 8080", "count: 1"}, []string{"inner:"}},
			{"comments", []string{"port: 8080"}, []string{"name:"}},
			{"large", []string{"values: [0, ...]"}, nil},
			{"wide", []string{"h: 0", "…"}, []string{"i:"}},
		}
		if len(hints) != len(cases) {
			t.Fatalf("hints: %+v", hints)
		}
		for _, tc := range cases {
			close := strings.Index(text, " // "+tc.marker) - 1
			at := offsetPosition(text, close)
			matches := 0
			for _, hint := range hints {
				if hint.Position != at {
					continue
				}
				matches++
				if hint.Kind != 0 || !hint.PaddingLeft || utf8.RuneCountInString(hint.Label) > 160 {
					t.Fatal(hint)
				}
				for _, want := range tc.includes {
					if !strings.Contains(hint.Label, want) {
						t.Fatalf("%s: missing %q in %s", tc.marker, want, hint.Label)
					}
				}
				for _, unwanted := range tc.excludes {
					if strings.Contains(hint.Label, unwanted) {
						t.Fatalf("%s: unwanted %q in %s", tc.marker, unwanted, hint.Label)
					}
				}
				raw, err := json.Marshal(hint)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), `"kind"`) || strings.Contains(string(raw), `"textEdits"`) {
					t.Fatal(string(raw))
				}
				filtered := inlayHintsForSource(uri, text, lspRange{Start: at, End: at}, inlayHintSettings{Defaults: true})
				if len(filtered) != 1 || !reflect.DeepEqual(filtered[0], hint) {
					t.Fatal(filtered, hint)
				}
			}
			if matches != 1 {
				t.Fatalf("%s: %d matching hints", tc.marker, matches)
			}
		}
	}
}

// The value-preview category is independent of type/parameter/ownership hints
// and follows partial initialization/configuration updates without source edits.
// Rules: rules/tooling/lsp.md — "Inlay hints", individually configurable.
func TestInlayHintsStructDefaultSettingsAndInvalidPlans(t *testing.T) {
	uri, text := structDefaultHintFixture(t, "valid.sec")
	if !defaultInlayHintSettings().Defaults {
		t.Fatal("missing default setting")
	}
	for _, payload := range []string{`{"inlayHints":{"defaults":false}}`, `{"sec":{"inlayHints":{"defaults":false}}}`} {
		settings := inlayHintSettingsFrom(defaultInlayHintSettings(), json.RawMessage(payload))
		settings.Types, settings.Parameters, settings.Ownership = false, false, false
		if settings.Defaults {
			t.Fatal(settings)
		}
		if hints := inlayHintsForSource(uri, text, allLines(), settings); len(hints) != 0 {
			t.Fatal(hints)
		}
		settings = inlayHintSettingsFrom(settings, json.RawMessage(`{"inlayHints":{"defaults":true}}`))
		if hints := inlayHintsForSource(uri, text, allLines(), settings); len(hints) == 0 {
			t.Fatal("defaults-only setting ignored")
		}
	}
	uri, text = structDefaultHintFixture(t, "literals_invalid.sec")
	if reported := analyze(uri, text); len(reported) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	if hints := inlayHintsForSource(uri, text, allLines(), inlayHintSettings{Defaults: true}); len(hints) != 0 {
		t.Fatal(hints)
	}
}
