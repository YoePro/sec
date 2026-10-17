package formatconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/formatter"
)

// MD-036/MD-054: the `[format]` table of .sec/sec.toml supplies project
// settings; an absent table or omitted key uses the formatter.md § 4(5)
// default (language_corrections = false); explicit invocation options win,
// and an option that was not supplied never shadows the project setting.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(5), § 4(17)–(23)
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — §§ 4, 5.3
func TestResolveAppliesOverridesManifestAndDefaults(t *testing.T) {
	two, four := 2, 4
	enabled := true
	for name, test := range map[string]struct {
		manifest  string
		overrides Overrides
		want      formatter.Options
	}{
		"no project":        {"", Overrides{}, formatter.Options{IndentationWidth: 4, VerticalStyle: formatter.VerticalStructured}},
		"absent table":      {"[analysis]\nlsp_depth = \"standard\"\n", Overrides{}, formatter.Options{IndentationWidth: 4, VerticalStyle: formatter.VerticalStructured}},
		"partial table":     {"[format]\nindentation_width = 2\n", Overrides{}, formatter.Options{IndentationWidth: 2, VerticalStyle: formatter.VerticalStructured}},
		"complete table":    {"[format]\nformat_version = 1\nindentation_width = 6\nvertical_style = \"compact\"\nwidth_mode = \"adaptive\"\nline_width = 100\nadaptive_min_width = 80\nadaptive_max_width = 160\nadaptive_fallback_width = 100\nlanguage_corrections = true # opt in\n", Overrides{}, formatter.Options{IndentationWidth: 6, VerticalStyle: formatter.VerticalCompact, Fix: true}},
		"cli wins":          {"[format]\nindentation_width = 4\nlanguage_corrections = false\n", Overrides{IndentationWidth: &two, LanguageCorrections: &enabled}, formatter.Options{IndentationWidth: 2, VerticalStyle: formatter.VerticalStructured, Fix: true}},
		"omitted cli":       {"[format]\nindentation_width = 2\n", Overrides{LanguageCorrections: &enabled}, formatter.Options{IndentationWidth: 2, VerticalStyle: formatter.VerticalStructured, Fix: true}},
		"cli over default":  {"", Overrides{IndentationWidth: &four}, formatter.Options{IndentationWidth: 4, VerticalStyle: formatter.VerticalStructured}},
		"alignment padding": {"[format]\nmax_alignment_padding = 32\n", Overrides{}, formatter.Options{IndentationWidth: 4, VerticalStyle: formatter.VerticalStructured, MaxAlignmentPadding: 32}},
	} {
		path := ""
		if test.manifest != "" {
			path = "sec.toml"
		}
		settings, err := Resolve(path, test.manifest, test.overrides)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if settings.Options != test.want {
			t.Errorf("%s: options = %+v, want %+v", name, settings.Options, test.want)
		}
	}
	if Defaults().Options.Fix {
		t.Fatal("language_corrections must default to false")
	}
}

func TestResolveRejectsInvalidConfiguration(t *testing.T) {
	three := 3
	for name, test := range map[string]struct {
		manifest  string
		overrides Overrides
		want      string
	}{
		"unsupported version": {"[format]\nformat_version = 2\n", Overrides{}, "unsupported format_version 2"},
		"unknown field":       {"[format]\nindent = 4\n", Overrides{}, "unknown format field indent"},
		"wrong type":          {"[format]\nindentation_width = \"4\"\n", Overrides{}, "indentation_width must be a non-negative integer"},
		"bad boolean":         {"[format]\nlanguage_corrections = yes\n", Overrides{}, "language_corrections must be true or false"},
		"duplicate field":     {"[format]\nline_width = 100\nline_width = 120\n", Overrides{}, "duplicate format field line_width"},
		"second table":        {"[format]\n[format]\n", Overrides{}, "at most one [format] table"},
		"bad width":           {"[format]\nindentation_width = 5\n", Overrides{}, "indentation_width must be 2, 4, or 6"},
		"bad style":           {"[format]\nvertical_style = \"dense\"\n", Overrides{}, "vertical_style"},
		"bad width mode":      {"[format]\nwidth_mode = \"auto\"\n", Overrides{}, "width_mode"},
		"inverted range":      {"[format]\nadaptive_min_width = 200\nadaptive_max_width = 80\n", Overrides{}, "adaptive_min_width 200 exceeds"},
		"bad cli width":       {"", Overrides{IndentationWidth: &three}, "indentation_width must be 2, 4, or 6"},
		"zero padding":        {"[format]\nmax_alignment_padding = 0\n", Overrides{}, "max_alignment_padding must be 1..1024"},
		"huge padding":        {"[format]\nmax_alignment_padding = 5000\n", Overrides{}, "max_alignment_padding must be 1..1024"},
		"padding type":        {"[format]\nmax_alignment_padding = \"wide\"\n", Overrides{}, "max_alignment_padding must be a non-negative integer"},
	} {
		path := ""
		if test.manifest != "" {
			path = "sec.toml"
		}
		if _, err := Resolve(path, test.manifest, test.overrides); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: err = %v, want %q", name, err, test.want)
		}
	}
}

// A file belongs to the project of the nearest ancestor holding
// .sec/sec.toml; a stand-alone file uses the defaults.
func TestForSourceUsesTheOwningProject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[format]\nindentation_width = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(nested, "main.sec")
	if err := os.WriteFile(source, []byte("module main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := ForSource(source, Overrides{})
	if err != nil || settings.Options.IndentationWidth != 2 || settings.Manifest != filepath.Join(root, ".sec", "sec.toml") {
		t.Fatalf("project settings = %+v, %v", settings, err)
	}
	standalone := filepath.Join(t.TempDir(), "alone.sec")
	if err := os.WriteFile(standalone, []byte("module main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err = ForSource(standalone, Overrides{})
	if err != nil || settings.Options != Defaults().Options || settings.Manifest != "" {
		t.Fatalf("stand-alone settings = %+v, %v", settings, err)
	}
}
