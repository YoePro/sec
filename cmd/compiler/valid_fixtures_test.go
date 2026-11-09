package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// validFixtureExceptions are *_valid.sec fixtures that are not standalone
// `sec check` programs: parser-, lexer- or CST-only inputs, fixtures read by
// their own tests in a specialized context (a prefix, expected diagnostics,
// a configured analyzer), and Sec 0.5 register fixtures that need a hardware
// target.
var validFixtureExceptions = map[string]string{
	"arrays_valid.sec":                          "read by its own Sema test",
	"cst/groups_valid.sec":                      "CST-only input",
	"errorhandling_valid.sec":                   "read by its own Sema test",
	"lexer/interpolation_balanced_valid.sec":    "lexer-only input",
	"match_valid.sec":                           "read by its own Sema test",
	"parser/documentation_attachment_valid.sec": "parser-only input",
	"parser/grammar_metadata_valid.sec":         "parser-only input",
	"parser/interpolation_parts_valid.sec":      "parser-only input",
	"parser/map_assignment_forms_valid.sec":     "parser-only input",
	"parser/try_handler_comment_valid.sec":      "parser-only input",
	"sema/assert_effects_valid.sec":             "read by its own Sema test with expected diagnostics",
	"sema/call_graph_effect_domains_valid.sec":  "read by its own Sema test with expected diagnostics",
	"sema/checked_unreachable_valid.sec":        "read by its own Sema test with expected diagnostics",
	"sema/membership/valid.sec":                 "read by its own Sema test",
	"sema/switch_partial_relations_valid.sec":   "read by its own Sema test with expected diagnostics",
	"sema/try_handler_direct_valid.sec":         "read by its own Sema test with expected diagnostics",
	"units_advanced_valid.sec":                  "read by its own unit tests",
	"units_valid.sec":                           "read by its own unit tests",
	"register3_valid.sec":                       "Sec 0.5 register fixture (hardware target)",
	"register4_valid.sec":                       "Sec 0.5 register fixture (hardware target)",
	"register5_valid.sec":                       "Sec 0.5 register fixture (hardware target)",
	"register6_valid.sec":                       "Sec 0.5 register fixture (hardware target)",
}

// TestValidFixturesPassCheck keeps every standalone valid fixture a valid
// program under `sec check`, so fixtures no test reads cannot drift away from
// the language unnoticed. A new *_valid.sec fixture is checked by default.
func TestValidFixturesPassCheck(t *testing.T) {
	root := "../../testdata"
	var fixtures []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_valid.sec") || name == "valid.sec" {
			relative, _ := filepath.Rel(root, path)
			fixtures = append(fixtures, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(fixtures)
	for _, fixture := range fixtures {
		if _, excepted := validFixtureExceptions[fixture]; excepted {
			continue
		}
		t.Run(fixture, func(t *testing.T) {
			if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(root, fixture)); code != 0 {
				t.Fatalf("valid fixture fails sec check (exit %d):\n%s", code, stderr)
			}
		})
	}
	for fixture := range validFixtureExceptions {
		found := false
		for _, candidate := range fixtures {
			found = found || candidate == fixture
		}
		if !found {
			t.Errorf("exception %s names no fixture", fixture)
		}
	}
}
