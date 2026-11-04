package main

import (
	"os"
	"testing"
)

// Project fixed width and adaptive fallback reach the same shared layout as
// editor requests. A CLI request has no observed viewport; check/write/check
// uses one effective configuration and never rewrites the project manifest.
// Rules: rules/tooling/formatter.md — §4(9–23),§16(1–2),§28.
func TestFmtPreferredWidth(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/preferred_width.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		settings string
		narrow   bool
	}{
		{"line_width = 64\n", true},
		{"line_width = 120\n", false},
		{"width_mode = \"adaptive\"\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 64\n", true},
		{"width_mode = \"adaptive\"\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 120\n", false},
		{"width_mode = \"adaptive\"\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 1\n", true},
		{"", false},
	} {
		name := "preferred_width.expected.sec"
		if test.narrow {
			name = "preferred_width.narrow.expected.sec"
		}
		want, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		manifest := "[format]\n" + test.settings
		manifestPath, file := writeFmtProject(t, manifest, string(input))
		if err := runFmtCommand([]string{"--check", file}); err == nil {
			t.Fatal("noncanonical input passed check")
		}
		if got := readFile(t, file); got != string(input) {
			t.Fatal("check modified source")
		}
		if err := runFmtCommand([]string{file}); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, file); got != string(want) {
			t.Fatalf("configuration %s:\n%s\nwant:\n%s", test.settings, got, want)
		}
		if err := runFmtCommand([]string{"--check", file}); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, manifestPath); got != manifest {
			t.Fatal("formatting rewrote manifest")
		}
	}
}
