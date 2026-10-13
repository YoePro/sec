package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/formatconfig"
	"sec/internal/project"
)

// TestInitDocumentsFormattingDefaults validates the emitted configuration for
// both scaffolds using the same resolver consumed by CLI and LSP formatting.
// Rules: rules/tooling/formatter.md — §4(5–17),§4(20–21); rules/projects/projects.md — §37.
func TestInitDocumentsFormattingDefaults(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		dir := t.TempDir()
		options := initCommandOptions{ProjectDir: dir, ProjectName: "Example", Profile: "server", Target: CompilerTarget{OS: "linux", Arch: "amd64"}, Minimal: minimal}
		if err := initProject(options); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, ".sec", "sec.toml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		config := string(data)
		parsed, errors := project.ParseManifestFormat(config)
		if len(errors) != 0 || !parsed.Present || parsed.FormatVersion == nil || parsed.VerticalStyle == nil || parsed.IndentationWidth == nil || parsed.WidthMode == nil || parsed.LineWidth == nil || parsed.AdaptiveMinWidth == nil || parsed.AdaptiveMaxWidth == nil || parsed.AdaptiveFallbackWidth == nil || parsed.LanguageCorrections == nil {
			t.Fatal(parsed, errors)
		}
		settings, err := formatconfig.Resolve(path, config, formatconfig.Overrides{})
		defaults := formatconfig.Defaults()
		settings.Manifest = ""
		defaults.Manifest = ""
		if err != nil || !reflect.DeepEqual(settings, defaults) {
			t.Fatal(settings, defaults, err)
		}
		for _, comment := range []string{"Supported version: 1", "\"structured\"", "\"compact\"", "Allowed values: 2, 4, 6", "\"fixed\"", "\"adaptive\"", "not a hard limit", "minimum <= fallback <= maximum", "true enables", "Default: false"} {
			if !strings.Contains(config, comment) {
				t.Fatal("missing documented option", comment)
			}
		}
	}
}
