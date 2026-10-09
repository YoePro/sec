package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const messyFragment = `schema_version: 1
area: core
integrations:
    - id: core.one
      status: partial
      revised: '2026-10-07'
      summary: >-
        Short prose that is folded
        across lines.
      implemented:
        - "plain item"
        - "needs: quoting"
        - '` + "`code`" + ` starts with a backtick'
      remaining:
          - first
          - second
    - id: core.two
      status: planned
      document_revision: "2.0"
      remaining: []
`

const canonicalFragment = "schema_version: 1\narea: core\nintegrations:\n" +
	"  - id: core.one\n" +
	"    status: partial\n" +
	"    revised: \"2026-10-07\"\n" +
	"    summary: >-\n" +
	"      Short prose that is folded across lines.\n" +
	"    implemented:\n" +
	"      - plain item\n" +
	"      - \"needs: quoting\"\n" +
	"      - \"`code` starts with a backtick\"\n" +
	"    remaining:\n" +
	"      - first\n" +
	"      - second\n" +
	"\n" +
	"  - id: core.two\n" +
	"    status: planned\n" +
	"    document_revision: \"2.0\"\n" +
	"    remaining: []\n"

func governanceFixture(t *testing.T, fragments map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "index.yaml", "schema_version: 1\nstatus_values:\n  implemented: done\n  partial: some\n  planned: none\n")
	for name, content := range fragments {
		writeFixture(t, dir, name, content)
	}
	return dir
}

func runCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := runCLI(args, &out, &errOut)
	return out.String() + errOut.String(), err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRenderProducesCanonicalLayoutWithoutChangingValues(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": messyFragment})
	document, err := loadDocument(filepath.Join(dir, "core.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := render(document.Root)
	if err != nil {
		t.Fatal(err)
	}
	if string(rendered) != canonicalFragment {
		t.Fatalf("canonical output:\n%s\nwant:\n%s", rendered, canonicalFragment)
	}
	if err := verifyRendered(document.Root, rendered); err != nil {
		t.Fatal(err)
	}
	// Rendering canonical text again is the identity.
	writeFixture(t, dir, "again.yaml", string(rendered))
	again, err := loadDocument(filepath.Join(dir, "again.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := render(again.Root)
	if !bytes.Equal(second, rendered) {
		t.Fatalf("render is not idempotent:\n%s", second)
	}
}

func TestFoldedProseIsRewrappedAtLineWidth(t *testing.T) {
	words := strings.Repeat("word ", 40)
	dir := governanceFixture(t, map[string]string{"core.yaml": "integrations:\n  - id: core.one\n    summary: >-\n      " + strings.TrimSpace(words) + "\n"})
	document, _ := loadDocument(filepath.Join(dir, "core.yaml"))
	rendered, err := render(document.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(rendered), "\n") {
		if len(line) > lineWidth {
			t.Fatalf("folded line longer than %d: %q", lineWidth, line)
		}
	}
	if err := verifyRendered(document.Root, rendered); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsDuplicateKeys(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": "integrations:\n  - id: core.one\n    verification: []\n    verification: []\n"})
	if _, err := loadDocument(filepath.Join(dir, "core.yaml")); err == nil || !strings.Contains(err.Error(), `duplicate key "verification"`) {
		t.Fatalf("duplicate key error = %v", err)
	}
}

func TestTypeHazardsAreReportedAndFixedOnlyOnRequest(t *testing.T) {
	fragment := "integrations:\n" +
		"  - id: core.one\n" +
		"    integrated: null\n" +
		"    audited: true\n" +
		"    revised: 2026-10-07\n" +
		"    document_revision: 1.0\n" +
		"    remaining:\n" +
		"      - route more paths: match arms and switches\n"
	dir := governanceFixture(t, map[string]string{"core.yaml": fragment})
	document, _ := loadDocument(filepath.Join(dir, "core.yaml"))
	hazards := findHazards(document)
	if len(hazards) != 5 {
		t.Fatalf("hazards = %+v", hazards)
	}
	fixable := 0
	for _, hazard := range hazards {
		if hazard.Fixable {
			fixable++
		}
	}
	if fixable != 3 {
		t.Fatalf("fixable hazards = %d, want 3 (timestamp, version, mapping item)", fixable)
	}

	path := filepath.Join(dir, "core.yaml")
	if _, err := runCommand(t, "fmt", "-dir", dir, "-write", path); err != nil {
		t.Fatal(err)
	}
	text := readFile(t, path)
	if !strings.Contains(text, "revised: 2026-10-07\n") || !strings.Contains(text, "document_revision: 1.0\n") || !strings.Contains(text, "- route more paths: match arms") {
		t.Fatalf("plain fmt changed a value's type:\n%s", text)
	}

	if _, err := runCommand(t, "fmt", "-dir", dir, "-fix-types", "-write", path); err != nil {
		t.Fatal(err)
	}
	text = readFile(t, path)
	for _, want := range []string{`revised: "2026-10-07"`, `document_revision: "1.0"`, `- "route more paths: match arms and switches"`, "integrated: null", "audited: true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("fixed fragment lacks %q:\n%s", want, text)
		}
	}
}

func TestFmtCheckAndWrite(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": messyFragment})
	path := filepath.Join(dir, "core.yaml")
	if _, err := runCommand(t, "fmt", "-dir", dir, "-check", path); err == nil {
		t.Fatal("fmt -check accepted a non-canonical fragment")
	}
	if readFile(t, path) != messyFragment {
		t.Fatal("fmt without -write changed the file")
	}
	if _, err := runCommand(t, "fmt", "-dir", dir, "-write", path); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != canonicalFragment {
		t.Fatalf("written fragment:\n%s", readFile(t, path))
	}
	if _, err := runCommand(t, "fmt", "-dir", dir, "-check", path); err != nil {
		t.Fatalf("canonical fragment failed -check: %v", err)
	}
}

func TestEditCommandsChangeOnlyTheNamedData(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": canonicalFragment})
	path := filepath.Join(dir, "core.yaml")

	out, err := runCommand(t, "set", "-dir", dir, "core.one", "status", "implemented")
	if err != nil || !strings.Contains(out, "-    status: partial") || !strings.Contains(out, "+    status: implemented") || !strings.Contains(out, "dry run") {
		t.Fatalf("set dry run = %v\n%s", err, out)
	}
	if readFile(t, path) != canonicalFragment {
		t.Fatal("dry run wrote the file")
	}

	steps := [][]string{
		{"set", "-write", "core.one", "status", "implemented"},
		{"add", "-write", "core.one", "remaining", "third: with colon"},
		{"add", "-write", "-at", "1", "core.one", "remaining", "zeroth"},
		{"move", "-write", "core.one", "remaining", "first", "implemented"},
		{"remove", "-write", "core.one", "remaining", "2"},
		{"verify", "-write", "core.one", "go test ./...", "passed"},
		{"add", "-write", "core.two", "tests", "internal/old_test.go"},
	}
	for _, step := range steps {
		args := append([]string{step[0], "-dir", dir}, step[1:]...)
		if out, err := runCommand(t, args...); err != nil {
			t.Fatalf("%v: %v\n%s", step, err, out)
		}
	}
	want := "schema_version: 1\narea: core\nintegrations:\n" +
		"  - id: core.one\n" +
		"    status: implemented\n" +
		"    revised: \"2026-10-07\"\n" +
		"    summary: >-\n" +
		"      Short prose that is folded across lines.\n" +
		"    implemented:\n" +
		"      - plain item\n" +
		"      - \"needs: quoting\"\n" +
		"      - \"`code` starts with a backtick\"\n" +
		"      - first\n" +
		"    remaining:\n" +
		"      - zeroth\n" +
		"      - \"third: with colon\"\n" +
		"    verification:\n" +
		"      - command: go test ./...\n" +
		"        result: passed\n" +
		"\n" +
		"  - id: core.two\n" +
		"    status: planned\n" +
		"    document_revision: \"2.0\"\n" +
		"    remaining: []\n" +
		"    tests:\n" +
		"      - internal/old_test.go\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("after edits:\n%s\nwant:\n%s", got, want)
	}

	out, err = runCommand(t, "repath", "-dir", dir, "-write", "internal/old_test.go=internal/new_test.go")
	if err != nil || !strings.Contains(out, "integrations touched (add a verification item to each): core.two") {
		t.Fatalf("repath = %v\n%s", err, out)
	}
	if !strings.Contains(readFile(t, path), "      - internal/new_test.go\n") {
		t.Fatal("repath did not rewrite the path")
	}

	out, err = runCommand(t, "new", "-dir", dir, "-write", "-summary", "A new integration.", "core.yaml", "core.three")
	if err != nil || !strings.Contains(readFile(t, path), "  - id: core.three\n    status: planned\n    summary: >-\n      A new integration.\n    rules: []\n") {
		t.Fatalf("new = %v\n%s\n%s", err, out, readFile(t, path))
	}
}

func TestRepathDropsOldPathWhenNewIsAlreadyListed(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": "integrations:\n  - id: core.one\n    code:\n      - a.go\n      - b.go\n"})
	path := filepath.Join(dir, "core.yaml")
	if _, err := runCommand(t, "repath", "-dir", dir, "-write", "a.go=b.go"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "integrations:\n  - id: core.one\n    code:\n      - b.go\n" {
		t.Fatalf("repath result:\n%s", got)
	}
}

func TestEditCommandsFailClosed(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": canonicalFragment, "messy.yaml": "integrations:\n    - id: messy.one\n      remaining: [a]\n"})
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"set", "missing.id", "status", "planned"}, `integration ID "missing.id" not found`},
		{[]string{"set", "core.one", "status", "finished"}, `status "finished" is not one of`},
		{[]string{"set", "core.one", "remaining", "x"}, "is a list, not a scalar"},
		{[]string{"add", "core.one", "remaining", "first"}, "already contains this item"},
		{[]string{"remove", "core.one", "remaining", "9"}, "position 9 does not exist"},
		{[]string{"remove", "core.one", "remaining", "absent"}, "no item with exactly this text"},
		{[]string{"move", "core.one", "partial", "1", "implemented"}, "has no partial list"},
		{[]string{"add", "messy.one", "remaining", "b"}, "is not canonical; run `yamlstatus fmt -write"},
		{[]string{"new", "-summary", "x", "core.yaml", "core.one"}, "already exists"},
		{[]string{"repath", "absent.go=other.go"}, "no governance list contains"},
	}
	for _, test := range cases {
		args := append([]string{test.args[0], "-dir", dir}, test.args[1:]...)
		if _, err := runCommand(t, args...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%v: error = %v, want %q", test.args, err, test.want)
		}
	}
	if readFile(t, filepath.Join(dir, "core.yaml")) != canonicalFragment {
		t.Fatal("a failing command changed the fragment")
	}
}

func TestUnifiedDiffSeparatesDistantChanges(t *testing.T) {
	old := []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\n")
	new := []byte("A\nb\nc\nd\ne\nf\ng\nh\ni\nj\nK\n")
	diff := unifiedDiff("x.yaml", old, new)
	if strings.Count(diff, "@@ ") != 2 || !strings.Contains(diff, "@@ -1,4 +1,4 @@") || !strings.Contains(diff, "@@ -8,4 +8,4 @@") {
		t.Fatalf("diff:\n%s", diff)
	}
}
