package formatter

import (
	"strings"
	"testing"
)

// MD-032: case and default labels are indented one configured level inside
// switch, their bodies one further level, and the closing brace aligns with
// switch, for every supported indentation width; nested switches repeat the
// hierarchy and formatting is idempotent.
//
// Rules:
//   - rules/tooling/formatter.md — § 18 "switch statements"
//   - rules/corrections/applied/md028-md032-md055-try-switch-import-comments-correction-20261012.md — §§ 3, 5.2
func TestFormatSwitchLabelsFollowConfiguredIndentation(t *testing.T) {
	input := "fn Classify(value: int, other: int) int {\nswitch value {\ncase 1:\nswitch other {\ncase 2:\nreturn 2\ndefault:\nreturn 3\n}\ndefault:\nreturn 0\n}\n}\n"
	for _, width := range []int{2, 4, 6} {
		level := func(n int) string { return strings.Repeat(" ", n*width) }
		want := "fn Classify(value: int, other: int) int {\n" +
			level(1) + "switch value {\n" +
			level(2) + "case 1:\n" +
			level(3) + "switch other {\n" +
			level(4) + "case 2:\n" +
			level(5) + "return 2\n" +
			level(4) + "default:\n" +
			level(5) + "return 3\n" +
			level(3) + "}\n" +
			level(2) + "default:\n" +
			level(3) + "return 0\n" +
			level(1) + "}\n" +
			"}\n"
		options := Options{IndentationWidth: width}
		got := Format(Source{Text: input}, options).Text
		if got != want {
			t.Fatalf("width %d switch layout:\n%s\nwant:\n%s", width, got, want)
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Fatalf("width %d switch layout is not idempotent:\n%s", width, again)
		}
	}
}

// MD-055: a heading comment that starts a blank-line-separated run of a
// grouped import declaration stays directly above that run's first import;
// imports are sorted only within their run, runs are separated by one blank
// line, an unnecessary blank line after a heading is removed, comments on a
// single import travel with it, and every ambiguous comment keeps the region
// unchanged. The layout follows the configured indentation width.
//
// Rules:
//   - rules/tooling/formatter.md — § 13 "Imports"
//   - rules/corrections/applied/md028-md032-md055-try-switch-import-comments-correction-20261012.md — §§ 4, 5.3
func TestFormatImportGroupHeadings(t *testing.T) {
	canonical := "module main\n\nimport (\n    // Network libraries\n    \"net/http\"\n    \"net/url\"\n\n    // File libraries\n    \"fs\"\n    \"io\"\n)\n"
	for name, input := range map[string]string{
		"canonical example":        canonical,
		"blank line after heading": "module main\n\nimport (\n    // Network libraries\n\n    \"net/http\"\n    \"net/url\"\n\n    // File libraries\n\n\n    \"fs\"\n    \"io\"\n)\n",
		"sorted within each run":   "module main\n\nimport (\n    // Network libraries\n    \"net/url\"\n    \"net/http\"\n\n\n    // File libraries\n    \"io\"\n    \"fs\"\n)\n",
	} {
		got := Format(Source{Text: input}, Options{}).Text
		if got != canonical {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, canonical)
		}
	}

	// Sorting never moves an import across a heading, and a comment on one
	// import travels with it inside its run.
	input := "module main\n\nimport (\n    // Tools\n    \"zeta\"\n    // the codec\n    \"codec\"\n\n    // Base\n    \"alpha\" // first\n)\n"
	want := "module main\n\nimport (\n    // Tools\n    // the codec\n    \"codec\"\n    \"zeta\"\n\n    // Base\n    \"alpha\" // first\n)\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Errorf("runs and attached comments:\n%s\nwant:\n%s", got, want)
	}

	for name, ambiguous := range map[string]string{
		"trailing group comment":     "module main\n\nimport (\n    // Network libraries\n    \"net/url\"\n    \"net/http\"\n    // more to come\n)\n",
		"comment ending a run":       "module main\n\nimport (\n    // Network libraries\n    \"net/url\"\n    \"net/http\"\n    // orphan\n\n    // File libraries\n    \"io\"\n)\n",
		"inline block comment":       "module main\n\nimport (\n    // Network libraries\n    \"net/url\" /* why */\n    \"net/http\"\n)\n",
		"headings across statements": "module main\n\nimport (\n    // Network libraries\n    \"net/url\"\n    \"net/http\"\n)\n\nimport \"io\"\n",
	} {
		if got := Format(Source{Text: ambiguous}, Options{}).Text; got != ambiguous {
			t.Errorf("%s: ambiguous import region was rewritten:\n%s", name, got)
		}
	}

	for _, width := range []int{2, 6} {
		pad := strings.Repeat(" ", width)
		want := "module main\n\nimport (\n" + pad + "// Network libraries\n" + pad + "\"net/http\"\n" + pad + "\"net/url\"\n\n" + pad + "// File libraries\n" + pad + "\"fs\"\n" + pad + "\"io\"\n)\n"
		options := Options{IndentationWidth: width}
		got := Format(Source{Text: canonical}, options).Text
		if got != want {
			t.Errorf("width %d:\n%s\nwant:\n%s", width, got, want)
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Errorf("width %d is not idempotent:\n%s", width, again)
		}
	}
}

// MD-033: without explicit groups the imports form one alphabetic sequence by
// logical path; explicit blank-line-separated groups, with or without
// headings, are sorted internally and never exchange imports. Sorting is
// purely syntactic (no module lookup, stdlib inventory or CompilationPlan)
// and idempotent.
//
// Rules:
//   - rules/tooling/formatter.md — § 13 "Imports"
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — §§ 3, 5.2
func TestFormatImportsSortWithinExplicitGroups(t *testing.T) {
	for name, test := range map[string]struct{ input, want string }{
		"one alphabetic sequence": {
			"module main\n\nimport (\n    \"str\"\n    sys \"platform/linux\"\n    \"app/server\"\n    \"github.com/example/library\"\n    \"fmt\"\n)\n",
			"module main\n\nimport (\n    \"app/server\"\n    \"fmt\"\n    \"github.com/example/library\"\n    sys \"platform/linux\"\n    \"str\"\n)\n",
		},
		"groups without headings": {
			"module main\n\nimport (\n    \"str\"\n    \"fmt\"\n\n    \"app/server\"\n    \"app/config\"\n\n    \"github.com/example/library\"\n)\n",
			"module main\n\nimport (\n    \"fmt\"\n    \"str\"\n\n    \"app/config\"\n    \"app/server\"\n\n    \"github.com/example/library\"\n)\n",
		},
		"documented groups": {
			"module main\n\nimport (\n    // Standard library\n    \"str\"\n    \"net/http\"\n    \"io\"\n    \"fmt\"\n\n    // Project modules\n    \"app/server\"\n    \"app/config\"\n\n    // External dependencies\n    \"github.com/example/library\"\n)\n",
			"module main\n\nimport (\n    // Standard library\n    \"fmt\"\n    \"io\"\n    \"net/http\"\n    \"str\"\n\n    // Project modules\n    \"app/config\"\n    \"app/server\"\n\n    // External dependencies\n    \"github.com/example/library\"\n)\n",
		},
	} {
		got := Format(Source{Text: test.input}, Options{}).Text
		if got != test.want {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, test.want)
		}
		if again := Format(Source{Text: got}, Options{}).Text; again != got {
			t.Errorf("%s is not idempotent:\n%s", name, again)
		}
	}
}

// `bit[N](value)` is formatted without spaces inside the extraction syntax.
//
// Rules:
//   - rules/tooling/formatter.md — § 17; md046-md056 correction § 8.4
func TestFormatBitExtractionCanonically(t *testing.T) {
	input := "module main\n\nfn F(v: uint32) uint8 {\n    return bit[ 8 ]( v )\n}\n"
	want := "module main\n\nfn F(v: uint32) uint8 {\n    return bit[8](v)\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatted:\n%s\nwant:\n%s", got, want)
	}
}
