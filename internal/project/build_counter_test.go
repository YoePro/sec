package project

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§11,28,28.1,28.2,49(2).
func TestManifestBuildCounterScopes(t *testing.T) {
	text := manifestFixture(t) + "\n[target.sec.build_counter]\nenabled = true\n" +
		"\n[target.library.build_counter]\nenabled = true\nscope = 'project'\n" +
		"\n[target.other]\nkind='library'\nsource='other'\n" +
		"[target.other.version]\nminor=4\n" +
		"[target.other.build_counter]\nenabled=true\nscope='project'\n"
	manifest, failures := ParseManifest("/project/.sec/sec.toml", text)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	for _, tc := range []struct {
		target string
		want   BuildCounter
		render string
	}{
		{"sec", BuildCounter{true, TargetCounter, 421, "sec"}, "1.0.3-alpha.422"},
		{"library", BuildCounter{true, ProjectCounter, 123, ""}, "1.0.3-alpha.124"},
		{"other", BuildCounter{true, ProjectCounter, 123, ""}, "1.4.3-alpha.124"},
		{"lsp", BuildCounter{false, TargetCounter, 97, "lsp"}, "0.8.2-beta.97"},
	} {
		counter, found := manifest.BuildCounter(tc.target)
		if !found || counter != tc.want {
			t.Fatal(tc.target, counter, found)
		}
		version, edit, err := manifest.BuildNumberCandidate(tc.target)
		rendered, renderErr := version.Render()
		if err != nil || renderErr != nil || rendered != tc.render || (edit != nil) != tc.want.Enabled {
			t.Fatal(tc.target, rendered, err, renderErr)
		}
		if edit != nil {
			updated, failures := ParseManifest("/project/.sec/sec.toml", edit.Text())
			if len(failures) != 0 {
				t.Fatal(failures)
			}
			stored, _ := updated.BuildCounter(tc.target)
			if stored.StoredBuild != version.Build {
				t.Fatal("edit selects different storage", stored, version)
			}
		}
		// Mutating a returned fact cannot alter the immutable snapshot.
		counter.StoredBuild = 0
		original, _ := manifest.BuildCounter(tc.target)
		if original != tc.want || manifest.Text() != text {
			t.Fatal("candidate or copied facts mutated the snapshot")
		}
	}
	if _, found := manifest.BuildCounter("ghost"); found {
		t.Fatal("phantom counter")
	}
	if _, edit, err := manifest.BuildNumberCandidate("ghost"); edit != nil || err == nil {
		t.Fatal("unknown target accepted")
	}
}

// Rules: rules/projects/projects.md — §§28(3–4),28.1(3),28.2(2),48(1–2).
func TestManifestBuildCounterValidation(t *testing.T) {
	cases := []struct{ name, declaration, message string }{
		{"missing local", "[target.library.build_counter]\nenabled=true", "requires an explicit target.library.version.build"},
		{"project conflict", "[target.sec.build_counter]\nenabled=true\nscope='project'", "conflicts with explicit target.sec.version.build"},
		{"scope", "[target.sec.build_counter]\nenabled=true\nscope='module'", `expected "target" or "project"`},
		{"scope type", "[target.sec.build_counter]\nscope=1", `expected "target" or "project"`},
		{"enabled type", "[target.sec.build_counter]\nenabled='true'", "expected a boolean"},
		{"unknown", "[target.sec.build_counter]\nreset=true", "unknown counter setting"},
		{"scalar table", "[target.sec]\nbuild_counter=true", "must be a TOML table"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := manifestFixture(t) + "\n" + tc.declaration + "\n"
			if tc.name == "scalar table" {
				text = strings.Replace(manifestFixture(t), "[target.sec]\n", "[target.sec]\nbuild_counter=true\n", 1)
			}
			manifest, failures := ParseManifest("/project/.sec/sec.toml", text)
			if manifest != nil || len(failures) == 0 || !strings.Contains(failures[0].Message, tc.message) || failures[0].Line <= 0 {
				t.Fatal(manifest, failures)
			}
		})
	}
	// Equal local values also conflict: an active project sequence must retain
	// authoritative project storage when its next candidate differs.
	text := strings.Replace(manifestFixture(t), "build = 421", "build = 123", 1) +
		"\n[target.sec.build_counter]\nenabled=true\nscope='project'\n"
	if manifest, failures := ParseManifest("/project/.sec/sec.toml", text); manifest != nil || len(failures) == 0 {
		t.Fatal("accepted local override of shared storage")
	}
	// Inactive counter configuration does not require local build storage.
	text = manifestFixture(t) + "\n[target.library.build_counter]\nenabled=false\n"
	manifest, failures := ParseManifest("/project/.sec/sec.toml", text)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	version, edit, err := manifest.BuildNumberCandidate("library")
	if err != nil || edit != nil || version.Build != 123 {
		t.Fatal(version, edit, err)
	}
	text = manifestFixture(t) + "\n[target.lsp.build_counter]\nenabled=false\nscope='project'\n"
	manifest, failures = ParseManifest("/project/.sec/sec.toml", text)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	version, edit, err = manifest.BuildNumberCandidate("lsp")
	if err != nil || edit != nil || version.Build != 97 {
		t.Fatal("inactive scope changed ordinary version inheritance", version, edit, err)
	}
}

// Rules: rules/projects/projects.md — §§5(2),28,48(2),49(5).
func TestManifestBuildCountersUseRealKeys(t *testing.T) {
	text := manifestFixture(t) + "\n[parameters]\ndescription='''\n[target.library.build_counter]\nenabled=true\n'''\n" +
		"[target.sec.build_counter]\n'enabled'=true\n\"scope\"='target'\n"
	// Use an actual inline counter under the already declared Target, without
	// redeclaring its parent table.
	text = strings.Replace(text, "[target.lsp]\n", "[target.lsp]\nbuild_counter = { enabled = true, scope = 'target' }\n", 1)
	manifest, failures := ParseManifest("/project/.sec/sec.toml", text)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	for _, name := range []string{"sec", "lsp"} {
		counter, _ := manifest.BuildCounter(name)
		if !counter.Enabled || counter.Scope != TargetCounter {
			t.Fatal(name, counter)
		}
	}
	if counter, _ := manifest.BuildCounter("library"); counter.Enabled {
		t.Fatal("string text enabled a counter")
	}
	bad := strings.Replace(text, "scope = 'target'", "scope = 'bad'", 1)
	_, failures = ParseManifest("/project/.sec/sec.toml", bad)
	if len(failures) != 1 || failures[0].Line != manifestKeyLine(bad, []string{"target", "lsp", "build_counter", "scope"}) {
		t.Fatal(failures)
	}
}

// Rules: rules/projects/projects.md — §§27(4–9),49(6).
func TestBuildCounterIgnoresLegacyHiddenState(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".sec", "state")
	if err := os.MkdirAll(state, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(state, "build-counters.toml")
	old := "this is not even TOML; legacy counter claims 9999999"
	if err := os.WriteFile(legacy, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	text := manifestFixture(t) + "\n[target.sec.build_counter]\nenabled=true\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	version, edit, err := LoadBuildNumberCandidate(path, "sec")
	if err != nil || edit == nil || version.Build != 422 {
		t.Fatal(version, edit, err)
	}
	data, err := os.ReadFile(legacy)
	if err != nil || string(data) != old {
		t.Fatal("candidate touched hidden legacy state", err)
	}
}

// Rules: rules/projects/projects.md — §§8(6–7),27(4–9),29(1–2).
func TestBuildCounterManualBaselinesAndReleaseChanges(t *testing.T) {
	for _, scope := range []CounterScope{TargetCounter, ProjectCounter} {
		t.Run(string(scope), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".sec", "sec.toml")
			if err := os.Mkdir(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			target, storage := "sec", "sec"
			if scope == ProjectCounter {
				target, storage = "library", ""
			}
			current := manifestFixture(t) + "\n[target." + target + ".build_counter]\nenabled=true\nscope='" + string(scope) + "'\n"
			if err := os.WriteFile(path, []byte(current), 0644); err != nil {
				t.Fatal(err)
			}
			old, err := LoadManifest(path)
			if err != nil {
				t.Fatal(err)
			}
			// Forward jump, backward edit and reset all redefine the next
			// candidate. Maximum stored value is valid, but cannot advance.
			for _, baseline := range []uint64{9000, 7, 0, math.MaxInt64, 12} {
				manual, err := PlanBuildNumberEdit(path, current, storage, baseline)
				if err != nil {
					t.Fatal(err)
				}
				if err := manual.Commit(); err != nil {
					t.Fatal(err)
				}
				current = manual.Text()
				for _, field := range []string{"major = 1", "minor = 0", "revision = 3"} {
					changed := strings.Replace(current, field, strings.Split(field, " = ")[0]+" = 8", 1)
					if err := os.WriteFile(path, []byte(changed), 0644); err != nil {
						t.Fatal(err)
					}
					version, edit, err := LoadBuildNumberCandidate(path, target)
					if baseline == math.MaxInt64 {
						if err == nil || edit != nil {
							t.Fatal("overflow accepted")
						}
					} else if err != nil || edit == nil || version.Build != baseline+1 {
						t.Fatal("release change reset or ignored manual baseline", version, err)
					}
					data, _ := os.ReadFile(path)
					if string(data) != changed {
						t.Fatal("reading a candidate committed it")
					}
					if err := os.WriteFile(path, []byte(current), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			counter, _ := old.BuildCounter(target)
			if (scope == TargetCounter && counter.StoredBuild != 421) || (scope == ProjectCounter && counter.StoredBuild != 123) {
				t.Fatal("external edits mutated an old snapshot")
			}
			entries, _ := os.ReadDir(filepath.Dir(path))
			if len(entries) != 1 {
				t.Fatal("created hidden counter state", entries)
			}
		})
	}
}
