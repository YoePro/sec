package formatter

import "testing"

func formatImportCase(t *testing.T, input string, want string) {
	t.Helper()
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}

// Rules:
//   - rules/tooling/formatter.md — § 13(2), (3), (5), (6), (7), (8), (10); § 14(1)–(5)
func TestFormatGathersSortsAndGroupsImports(t *testing.T) {
	input := "module main\n\nimport \"net/http\"\nimport sys \"platform/linux\"\n// formatting\nimport \"fmt\"\n\nfn A() void {\n}\n\nimport z \"io\"\n\nfn B() void {\n}\n"
	want := "module main\n\nimport (\n    // formatting\n    \"fmt\"\n    z \"io\"\n    \"net/http\"\n\n    sys \"platform/linux\"\n)\n\nfn A() void {\n}\n\nfn B() void {\n}\n"
	formatImportCase(t, input, want)
}

// Rules:
//   - rules/tooling/formatter.md — § 13(1) single-import form, § 13(6) attached comments
func TestFormatKeepsSingleImportForm(t *testing.T) {
	input := "module main\n// output\nimport   \"io\"   // writer\nfn A() void {\n}\n"
	want := "module main\n\n// output\nimport \"io\" // writer\n\nfn A() void {\n}\n"
	formatImportCase(t, input, want)
}

// Rules:
//   - rules/tooling/formatter.md — § 13(5) sort by path not alias, § 13(4) never remove imports
func TestFormatSortsImportsByPathAndKeepsDuplicates(t *testing.T) {
	input := "module main\n\nimport (\n    zz \"alpha\"\n    \"zeta\" // last\n    aa \"beta\"\n    \"alpha\"\n)\n"
	want := "module main\n\nimport (\n    \"alpha\"\n    zz \"alpha\"\n    aa \"beta\"\n    \"zeta\" // last\n)\n"
	formatImportCase(t, input, want)
}

// Rules:
//   - rules/tooling/formatter.md — § 14(3) the import region precedes ordinary declarations
func TestFormatMovesImportsAboveEarlierDeclarationsAndTheirComments(t *testing.T) {
	input := "module main\n\n// helper\n@noPanic\nfn A() void {\n}\n\nimport \"io\"\nimport \"fmt\"\n"
	want := "module main\n\nimport (\n    \"fmt\"\n    \"io\"\n)\n\n// helper\n@noPanic\nfn A() void {\n}\n"
	formatImportCase(t, input, want)
}

func TestFormatLeavesUnattributableImportCommentsInPlace(t *testing.T) {
	cases := []string{
		// A comment between an import group's last item and its closer
		// belongs to no single import.
		"module main\n\nimport (\n    \"b\"\n    \"a\"\n\n    // dangling\n)\n",
		// An inline block comment is not a movable trailing line comment.
		"module main\n\nimport \"b\" /* inline */\nimport \"a\"\n",
	}
	for _, input := range cases {
		if got := Format(Source{Text: input}, Options{}).Text; got != input {
			t.Fatalf("Format() changed unattributable import comments:\n%s", got)
		}
	}
}

func TestFormatImportRegionRequiresCleanParse(t *testing.T) {
	input := "module main\n\nimport \"b\"\nimport \"a\"\n\nfn A( {\n"
	if got := formatImportRegion(input, false); got != input {
		t.Fatalf("formatImportRegion() rewrote a document with parse errors:\n%s", got)
	}
}
