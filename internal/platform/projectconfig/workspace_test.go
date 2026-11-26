package projectconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/platform/target"
	"sec/internal/project"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile("../../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, File), data, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
func ptr(s string) *string { return &s }
func read(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, File))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func open(t *testing.T, root string, approve func(string) bool) *Workspace {
	t.Helper()
	w, err := Open(root, approve)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

// All MD-139 scopes use canonical precedence, provenance and verified target
// facts. Clearing a local key restores inheritance without editing other keys.
func TestSelectionsAndCommentPreservingEdits(t *testing.T) {
	root := fixture(t)
	before := read(t, root)
	for _, op := range []Operation{
		{Variant: "linux-amd64", Profile: ptr("hosted"), Mode: ptr("noalloc")},
		{Target: "sec", Profile: ptr("hosted")},
		{Target: "sec", Variant: "linux-amd64", Profile: ptr("hosted"), Mode: ptr("noalloc")},
	} {
		t.Run(op.Target+"-"+op.Variant, func(t *testing.T) {
			edit := Apply(before, op)
			if edit.Rejected || !edit.Changed {
				t.Fatal(edit)
			}
			if !strings.Contains(edit.Text, "# The human name, machine identity and physical location are independent.") {
				t.Fatal("comment lost")
			}
			selected, errs := project.ResolvePlanSelection(edit.Text, "sec", "linux-amd64")
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
			profile, err := target.ResolveTargetProfile(definition, selected.Profile, string(selected.AllocationMode), selected.Provider)
			if err != nil {
				t.Fatal(err)
			}
			entries, diagnostics := Resolve(edit.Text)
			if len(diagnostics) > 0 {
				t.Fatal(diagnostics)
			}
			found := false
			for _, entry := range entries {
				if entry.Target == "sec" && entry.Variant == "linux-amd64" {
					found = true
					if !reflect.DeepEqual(entry.Selection, selected) || !reflect.DeepEqual(entry.Profile, profile) {
						t.Fatal(entry)
					}
				}
			}
			if !found {
				t.Fatal(entries)
			}
			again := Apply(edit.Text, op)
			if again.Rejected || again.Changed || again.Text != edit.Text {
				t.Fatal(again)
			}
			if op.Profile != nil {
				op.Profile = ptr("")
			}
			if op.Mode != nil {
				op.Mode = ptr("")
			}
			cleared := Apply(edit.Text, op)
			if cleared.Rejected {
				t.Fatal(cleared)
			}
		})
	}
	w := open(t, root, nil)
	inspection, err := w.Inspect()
	if err != nil || len(inspection.Diagnostics) > 0 {
		t.Fatal(inspection, err)
	}
	if read(t, root) != before {
		t.Fatal("read-only operations wrote")
	}
}

func TestEditRejectionsAndNoops(t *testing.T) {
	root := fixture(t)
	before := read(t, root)
	for name, op := range map[string]Operation{
		"unknown variant":     {Variant: "missing", Mode: ptr("noalloc")},
		"unknown target":      {Target: "missing", Mode: ptr("noalloc")},
		"traversal":           {Variant: "../outside", Mode: ptr("noalloc")},
		"no fields":           {Variant: "linux-amd64"},
		"no scope":            {Profile: ptr("hosted")},
		"build profile":       {Variant: "linux-amd64", Profile: ptr("debug")},
		"reclassification":    {Variant: "linux-amd64", Profile: ptr("baremetal")},
		"unknown mode":        {Variant: "linux-amd64", Mode: ptr("malloc")},
		"provider activation": {Variant: "linux-amd64", Mode: ptr("hosted"), Provider: ptr("sec.system-heap.v1")},
		"contradiction":       {Variant: "linux-amd64", Mode: ptr("noalloc"), Provider: ptr("x")},
	} {
		t.Run(name, func(t *testing.T) {
			edit := Apply(before, op)
			if !edit.Rejected || len(edit.Diagnostics) == 0 || edit.Text != before {
				t.Fatal(edit)
			}
		})
	}
	for _, text := range []string{before + "\n[variant.linux-amd64.allocation]\nunknown = 1\n", before + "\n[variant.linux-amd64]\nos = 'linux'\n", "not valid TOML = ["} {
		edit := Apply(text, Operation{Variant: "linux-amd64", Profile: ptr("hosted")})
		if !edit.Rejected || edit.Text != text {
			t.Fatal(edit)
		}
	}
	for _, op := range []Operation{{Variant: "linux-amd64", Provider: ptr("")}, {Variant: "linux-amd64", Profile: ptr("")}} {
		edit := Apply(before, op)
		if edit.Rejected || edit.Changed || edit.Text != before {
			t.Fatal(edit)
		}
	}
	quoted := before + "\n[target.sec.variant.linux-amd64]\nprofile = 'hosted' # preserve quotes\n"
	edit := Apply(quoted, Operation{Target: "sec", Variant: "linux-amd64", Profile: ptr("hosted")})
	if edit.Rejected || edit.Changed || edit.Text != quoted {
		t.Fatal(edit)
	}
}

func TestPlanApprovalAndAtomicCommit(t *testing.T) {
	root := fixture(t)
	before := read(t, root)
	allowed := ""
	w := open(t, root, func(hash string) bool { return hash == allowed })
	op := Operation{Variant: "linux-amd64", Mode: ptr("noalloc")}
	plan, diagnostics, err := w.PlanChange(op)
	if err != nil || len(diagnostics) > 0 {
		t.Fatal(diagnostics, err)
	}
	again, _, _ := w.PlanChange(op)
	a, _ := json.Marshal(plan)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("nondeterministic plan")
	}
	*op.Mode = "hosted"
	if *plan.Operation.Mode != "noalloc" {
		t.Fatal("caller aliases plan")
	}
	if _, err := w.Apply(*plan); err == nil {
		t.Fatal("unauthorized write")
	}
	if read(t, root) != before {
		t.Fatal("unauthorized change")
	}
	forged := *plan
	forged.Patch += "forged"
	allowed = forged.PlanSHA256
	if _, err := w.Apply(forged); err == nil {
		t.Fatal("forged plan")
	}
	allowed = plan.PlanSHA256
	edit, err := w.Apply(*plan)
	if err != nil || edit.Rejected || !edit.Changed {
		t.Fatal(edit, err)
	}
	if read(t, root) != Apply(before, plan.Operation).Text {
		t.Fatal("canonical edit mismatch")
	}
	info, _ := os.Stat(filepath.Join(root, File))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	repeated, err := w.Apply(*plan)
	if err != nil || !repeated.Rejected {
		t.Fatal(repeated, err)
	}
	noop, _, _ := w.PlanChange(plan.Operation)
	allowed = noop.PlanSHA256
	out, err := w.Apply(*noop)
	if err != nil || out.Rejected || out.Changed {
		t.Fatal(out, err)
	}
}

func TestPlanConfinementAndFailedWrites(t *testing.T) {
	for _, scenario := range []string{"stale", "no permission", "interrupted", "foreign", "file override", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := fixture(t)
			before := read(t, root)
			var w *Workspace
			allowed := ""
			w = open(t, root, func(hash string) bool {
				if scenario == "interrupted" {
					w.Close()
				}
				return hash == allowed
			})
			plan, _, err := w.PlanChange(Operation{Variant: "linux-amd64", Mode: ptr("noalloc")})
			if err != nil {
				t.Fatal(err)
			}
			allowed = plan.PlanSHA256
			switch scenario {
			case "stale":
				if err := os.WriteFile(filepath.Join(root, File), []byte(before+"\n# concurrent edit\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "no permission":
				if os.Geteuid() == 0 {
					t.Skip("permission enforcement requires unprivileged user")
				}
				directory := filepath.Join(root, ".sec")
				if err := os.Chmod(directory, 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(directory, 0700) })
			case "foreign":
				plan.ProjectRoot = fixture(t)
				plan.PlanSHA256 = digest(*plan)
			case "file override":
				plan.File = "../outside"
				plan.PlanSHA256 = digest(*plan)
			case "symlink":
				outside := fixture(t)
				os.Remove(filepath.Join(root, File))
				os.Symlink(filepath.Join(outside, File), filepath.Join(root, File))
				t.Cleanup(func() {
					if read(t, outside) != before {
						t.Error("outside changed")
					}
				})
			}
			out, err := w.Apply(*plan)
			if err == nil && !out.Rejected {
				t.Fatal("unsafe commit", out)
			}
			if scenario != "stale" && read(t, root) != before {
				t.Fatal("failed apply changed bytes")
			}
			entries, _ := os.ReadDir(filepath.Join(root, ".sec"))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".c-bindings-") {
					t.Fatal("leaked temporary")
				}
			}
		})
	}
}
