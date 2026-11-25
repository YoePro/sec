package cbindings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// workspaceFixture supplies only caller-selected local files; project identity
// and C-binding validation remain with their owning services.
// Rules: harness design notes §§3(1)–(2),5(6).
func workspaceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	manifest, err := os.ReadFile("../../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, filepath.Join(root, ".sec", "sec.toml"), string(manifest))
	file := filepath.Join(root, filepath.FromSlash(DefaultFile))
	writeWorkspaceFile(t, file, editFixture)
	return root, file
}

func writeWorkspaceFile(t *testing.T, file, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// Metadata describes all and only the closed service surface. Read-only
// inspect/validate/plan/diff never writes or invokes the authorizer.
// Rules: harness design notes §§5(1),5(5)–(7),7(2)–(3).
func TestWorkspaceMetadataAndReadOnly(t *testing.T) {
	tools := WorkspaceTools()
	want := []string{"c-bindings.inspect", "c-bindings.validate", "c-bindings.plan", "c-bindings.diff", "c-bindings.apply"}
	if len(tools) != len(want) {
		t.Fatal(tools)
	}
	for i, tool := range tools {
		write := tool.Name == "c-bindings.apply"
		if tool.Name != want[i] || tool.ReadOnly == write || tool.RequiresAuthorization != write || tool.File != DefaultFile || !strings.Contains(tool.ValidationScope, "no target capability proof") {
			t.Fatal(tool)
		}
	}
	tools[0].Name = "shell"
	if WorkspaceTools()[0].Name != "c-bindings.inspect" {
		t.Fatal("shared metadata mutated")
	}
	root, file := workspaceFixture(t)
	called := false
	w, err := OpenWorkspace(root, func(WorkspaceApprovalRequest) bool { called = true; return false })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	before, _ := os.Stat(file)
	inspection, err := w.Inspect()
	if err != nil || inspection.Scope != "structural" || inspection.Package == nil || len(inspection.Diagnostics) != 0 || inspection.File != DefaultFile {
		t.Fatal(inspection, err)
	}
	inspection.Package.ID = "changed"
	next, err := w.Inspect()
	if err != nil || next.Package.ID == "changed" {
		t.Fatal("inspection aliases cached state", err)
	}
	if ds, err := w.Validate(); err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	plan, ds, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	diff, err := w.ShowDiff(*plan)
	if err != nil || diff != plan.Change.Patch {
		t.Fatal(diff, err)
	}
	after, _ := os.Stat(file)
	data, _ := os.ReadFile(file)
	if called || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || string(data) != editFixture {
		t.Fatal("read-only tools changed state")
	}
	// Even a caller-supplied correct digest is not authorization from host code.
	if _, err := w.Apply(*plan); !errors.Is(err, ErrWorkspaceAuthorization) {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("trusted policy not consulted")
	}
	deniedBytes, err := os.ReadFile(file)
	if err != nil || string(deniedBytes) != editFixture {
		t.Fatal("denied operation wrote target", err)
	}
}

// Approval exists outside model/tool data. Only the exact project-bound plan
// admitted by the trusted caller can commit, with ordinary comment preservation.
// Rules: harness design notes §§5(2)–(7),7(2)–(3).
func TestWorkspaceTrustedAuthorization(t *testing.T) {
	root, file := workspaceFixture(t)
	var approved WorkspaceApprovalRequest
	w, err := OpenWorkspace(root, func(request WorkspaceApprovalRequest) bool { return request == approved })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	plan, ds, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	if _, err := w.Apply(*plan); err != ErrWorkspaceAuthorization {
		t.Fatal(err)
	}
	approved = WorkspaceApprovalRequest{ProjectRoot: root, File: DefaultFile, PlanSHA256: plan.PlanSHA256, Operation: OpPackageSet}
	raw, _ := json.Marshal(plan)
	var restored WorkspaceChangePlan
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	result, err := w.Apply(restored)
	if err != nil || result.Rejected || !result.Changed {
		t.Fatal(result, err)
	}
	data, _ := os.ReadFile(file)
	expected := Apply(editFixture, plan.Change.Operation)
	if string(data) != expected.Text {
		t.Fatal("workspace forked edit semantics")
	}
	stale, err := w.Apply(restored)
	if err != nil || !stale.Rejected || stale.Diagnostics[0].ID != "c-bindings.stale-file" {
		t.Fatal(stale, err)
	}
	// Read-only is a host policy, not merely a request annotation.
	ro, err := OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	noop, _, err := ro.PlanChange(plan.Change.Operation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ro.Apply(*noop); err != ErrWorkspaceAuthorization {
		t.Fatal(err)
	}
}

// Unknown roots are never inferred from cwd/parents; malformed manifests and
// manifest links outside the selected project are rejected before a service opens.
// Rules: harness design notes §§5(6),7(2); projects.md §5.
func TestWorkspaceRequiresExplicitProject(t *testing.T) {
	root, _ := workspaceFixture(t)
	sub := filepath.Join(root, "sub")
	os.Mkdir(sub, 0700)
	invalid := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(invalid, ".sec", "sec.toml"), "[project]\nname = [")
	for _, path := range []string{"", t.TempDir(), sub, invalid, filepath.Join(root, "missing")} {
		w, err := OpenWorkspace(path, nil)
		if err == nil {
			w.Close()
			t.Fatalf("opened invalid project %q", path)
		}
	}
	escape := t.TempDir()
	os.Mkdir(filepath.Join(escape, ".sec"), 0700)
	if err := os.Symlink(filepath.Join(root, ".sec", "sec.toml"), filepath.Join(escape, ".sec", "sec.toml")); err != nil {
		t.Skip(err)
	}
	if w, err := OpenWorkspace(escape, nil); err == nil {
		w.Close()
		t.Fatal("followed manifest outside selected root")
	}
}

// Integrity and the file allowlist are checked before authorization. A valid
// digest does not permit absolute/traversal/other named files or another project.
// Rules: harness design notes §§5(3),5(5)–(6),7(2).
func TestWorkspaceRejectsForeignAndForgedPlans(t *testing.T) {
	root, file := workspaceFixture(t)
	other, otherFile := workspaceFixture(t)
	calls := 0
	w, err := OpenWorkspace(root, func(WorkspaceApprovalRequest) bool { calls++; return true })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	original, _, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil {
		t.Fatal(err)
	}
	// Even internally rehashed forged plans cannot open arbitrary paths.
	for _, path := range []string{"../outside.toml", file, otherFile, ".sec/sec.toml", ".sec/platform/../sec.toml", ".sec\\platform\\c-bindings.toml"} {
		plan := *original
		plan.Change.File = path
		plan.Change.PlanSHA256 = planDigest(plan.Change)
		plan.PlanSHA256 = workspacePlanDigest(plan)
		if _, err := w.Apply(plan); err != ErrWorkspacePlan {
			t.Fatal(path, err)
		}
	}
	bad := *original
	bad.Change.Patch = "forged"
	if _, err := w.ShowDiff(bad); err != ErrWorkspacePlan {
		t.Fatal(err)
	}
	if _, err := w.Apply(bad); err != ErrWorkspacePlan {
		t.Fatal(err)
	}
	second, err := OpenWorkspace(other, func(WorkspaceApprovalRequest) bool { calls++; return true })
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := second.Apply(*original); err != ErrWorkspacePlan {
		t.Fatal("cross-project plan admitted", err)
	}
	if calls != 0 {
		t.Fatal("untrusted plan reached authorization callback")
	}
	for _, path := range []string{file, otherFile} {
		data, _ := os.ReadFile(path)
		if string(data) != editFixture {
			t.Fatal("rejected request changed file")
		}
	}
}

// Rooted reads and final mutation cannot escape through a symlink installed
// either before inspection or after review/authorization. External bytes stay intact.
// Rules: harness design notes §§5(4)–(7),7(2).
func TestWorkspaceSymlinkEscape(t *testing.T) {
	for _, phase := range []string{"before-open", "after-plan", "during-authorization", "parent-after-plan"} {
		t.Run(phase, func(t *testing.T) {
			root, file := workspaceFixture(t)
			external := filepath.Join(t.TempDir(), "c-bindings.toml")
			writeWorkspaceFile(t, external, editFixture)
			install := func() {
				t.Helper()
				if phase == "parent-after-plan" {
					parent := filepath.Dir(file)
					if err := os.Rename(parent, parent+"-original"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Dir(external), parent); err != nil {
						t.Skip(err)
					}
				} else {
					if err := os.Remove(file); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(external, file); err != nil {
						t.Skip(err)
					}
				}
			}
			if phase == "before-open" {
				install()
			}
			w, err := OpenWorkspace(root, func(WorkspaceApprovalRequest) bool {
				if phase == "during-authorization" {
					install()
				}
				return true
			})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if phase == "before-open" {
				if _, err := w.Inspect(); err == nil {
					t.Fatal("inspection escaped")
				}
				if _, _, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")}); err == nil {
					t.Fatal("planning escaped")
				}
			} else {
				plan, _, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")})
				if err != nil {
					t.Fatal(err)
				}
				if phase != "during-authorization" {
					install()
				}
				result, err := w.Apply(*plan)
				if err == nil && !result.Rejected {
					t.Fatal("apply escaped", result)
				}
			}
			data, err := os.ReadFile(external)
			if err != nil || string(data) != editFixture {
				t.Fatal("external file modified", err)
			}
		})
	}
}

// Closing never falls back to normal paths; concurrent independently reviewed
// applications still use the same transaction engine's one-commit rule.
// Rules: harness design notes §§5(3)–(6),7(2).
func TestWorkspaceCloseAndConcurrentApply(t *testing.T) {
	var zero Workspace
	if _, err := zero.Inspect(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("zero workspace fell back", err)
	}
	if _, _, err := zero.PlanChange(Operation{}); !errors.Is(err, os.ErrClosed) {
		t.Fatal("zero workspace planned", err)
	}
	if _, err := zero.Apply(WorkspaceChangePlan{}); err != ErrWorkspacePlan {
		t.Fatal(err)
	}

	root, _ := workspaceFixture(t)
	w, err := OpenWorkspace(root, func(WorkspaceApprovalRequest) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	plan, _, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	changed := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := w.Apply(*plan)
			if err != nil {
				t.Error(err)
			}
			changed <- result.Changed && !result.Rejected
		}()
	}
	wg.Wait()
	close(changed)
	count := 0
	for yes := range changed {
		if yes {
			count++
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Inspect(); err == nil {
		t.Fatal("closed workspace read through ambient filesystem")
	}
	if _, _, err := w.PlanChange(plan.Change.Operation); err == nil {
		t.Fatal("closed workspace planned")
	}
	if _, err := w.Apply(*plan); err == nil {
		t.Fatal("closed workspace applied")
	}
}

// Invalid operations/TOML remain owning validation failures, never assertions
// that an unavailable platform/provider became usable because a plan exists.
// Rules: harness design notes §§3(2),5(7),7(2)–(3).
func TestWorkspaceKeepsCanonicalValidation(t *testing.T) {
	root, file := workspaceFixture(t)
	w, err := OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, op := range []Operation{{Kind: OpModuleAdd, Common: true, Module: "../escape"}, {Kind: OpEnvironmentSet, Environment: "linux-amd64-glibc", Arch: text("made-up-architecture")}, {Kind: "run-shell"}} {
		plan, ds, err := w.PlanChange(op)
		expected := Apply(editFixture, op)
		if err != nil || plan != nil || !expected.Rejected || !reflect.DeepEqual(ds, expected.Diagnostics) {
			t.Fatal(plan, ds, err)
		}
	}
	writeWorkspaceFile(t, file, "schema_version = [")
	ds, err := w.Validate()
	if err != nil || len(ds) == 0 {
		t.Fatal(ds, err)
	}
	if plan, ds, err := w.PlanChange(Operation{Kind: OpPackageSet, Version: text("2.0.0")}); err != nil || plan != nil || len(ds) == 0 {
		t.Fatal(plan, ds, err)
	}
}
