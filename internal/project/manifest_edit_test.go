package project

import (
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§5(4–6),40–43,49(5).
func TestManifestAddMissingFieldsPreservesSyntax(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		original := strings.Join([]string{"# first comment", "[project]", "name = 'unchanged'", "", "[parameters]", "description = '''", "[target.worker]", "kind = 'fake'", "'''", "values = [", " ['not', 'a', 'header'],", "]", "", "[target.\"worker\"] # target comment", "# source comment", "source = 'cmd/worker'", "", "[target.worker.version]", "build = 27", ""}, newline)
		doc, err := ParseManifestDocument(original)
		if err != nil {
			t.Fatal(err)
		}
		proposal, err := doc.AddMissingFields([]string{"target", "worker"}, []ManifestField{{"kind", `"command"`}, {"source", `"ignored"`}, {"variants", `["native"]`}})
		if err != nil {
			t.Fatal(err)
		}
		addition := "kind = \"command\"" + newline + "variants = [\"native\"]" + newline
		if strings.Replace(proposal, addition, "", 1) != original {
			t.Fatal("changed unrelated bytes", proposal)
		}
		if strings.Count(proposal, "kind = \"command\"") != 1 || strings.Contains(proposal, "ignored") {
			t.Fatal(proposal)
		}
		updated, err := ParseManifestDocument(proposal)
		if err != nil {
			t.Fatal(err)
		}
		repeated, err := updated.AddMissingFields([]string{"target", "worker"}, []ManifestField{{"kind", `"command"`}, {"variants", `["native"]`}})
		if err != nil || repeated != proposal {
			t.Fatal("not idempotent", err)
		}
	}
}

// Rules: rules/projects/projects.md — §§17,41,49(5).
func TestManifestImplicitAndInlineTables(t *testing.T) {
	doc, err := ParseManifestDocument("[target.worker.version]\nbuild = 4\n")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := doc.AddMissingFields([]string{"target", "worker"}, []ManifestField{{"kind", `"command"`}})
	if err != nil || !strings.HasPrefix(proposal, doc.Text) {
		t.Fatal(proposal, err)
	}
	doc, err = ParseManifestDocument(`target.worker = { kind = 'command', source = 'cmd/worker', variants = ['native'] }`)
	if err != nil {
		t.Fatal(err)
	}
	same, err := doc.AddMissingFields([]string{"target", "worker"}, []ManifestField{{"kind", `"command"`}})
	if err != nil || same != doc.Text {
		t.Fatal(same, err)
	}
	quoted, err := doc.AddMissingFields([]string{"variant", "name.with.dot"}, []ManifestField{{"os", `"linux"`}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseManifestDocument(quoted)
	if err != nil {
		t.Fatal(err)
	}
	if table, err := parsed.Table("variant", "name.with.dot"); err != nil || table["os"] != "linux" {
		t.Fatal(table, err)
	}
}

// Rules: rules/projects/projects.md — §§5(2),17,49(2).
func TestTargetsUseRealTOMLDeclarations(t *testing.T) {
	text := "[parameters]\ntext = '''\n[target.fake]\nkind = 'firmware'\nsource = '.'\n'''\n[target.real]\nkind = 'library'\nsource = 'real'\n"
	targets := ParseManifestTargets(text)
	if len(targets) != 1 || targets[0].Name != "real" || targets[0].Kind != "library" {
		t.Fatal(targets)
	}
	inline := ParseManifestTargets(`target.real = { kind = 'library', source = 'real' }`)
	if len(inline) != 1 || inline[0] != targets[0] {
		t.Fatal(inline)
	}
}

// Rules: rules/projects/projects.md — §§8,49(2).
func TestVersionIgnoresUnownedMultilineTables(t *testing.T) {
	text := manifestFixture(t) + "\n[parameters]\ntext = '''\n[version]\nmajor = -1\n[target.ghost.version]\nbuild = -1\n'''\n"
	manifest, failures := ParseManifest("/project/.sec/sec.toml", text)
	if len(failures) != 0 || manifest.Facts().ProjectVersion.Build != 123 {
		t.Fatal(failures)
	}
	if _, found := manifest.TargetVersion("ghost"); found {
		t.Fatal("string forged a Target version")
	}
}
