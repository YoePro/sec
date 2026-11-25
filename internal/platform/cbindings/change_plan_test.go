package cbindings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A proposal and its serialization are deterministic, detached and read-only;
// application preserves unrelated bytes and permissions after exact approval.
// Rules: harness design notes §§5(2)–(7),7(2)–(3); MD-021 correction §5.
func TestChangePlanReviewAndApply(t *testing.T) {
	file := filepath.Join(t.TempDir(), "c-bindings.toml")
	if err := os.WriteFile(file, []byte(editFixture), 0600); err != nil {
		t.Fatal(err)
	}
	version := "2.0.0"
	plan, ds, err := PlanChange(file, Operation{Kind: OpPackageSet, Version: &version})
	if err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	if !plan.Changed || len(plan.Validation) == 0 || len(plan.PlanSHA256) != 64 || !strings.Contains(plan.Patch, "-version = \"1.0.0\"") || !strings.Contains(plan.Patch, "+version = \"2.0.0\"") || strings.Contains(plan.Patch, "-# Binding package") {
		t.Fatal(plan)
	}
	again, _, _ := PlanChange(file, plan.Operation)
	a, _ := json.Marshal(plan)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("nondeterministic plan")
	}
	version = "3.0.0"
	current, _ := os.ReadFile(file)
	if string(current) != editFixture || *plan.Operation.Version != "2.0.0" {
		t.Fatal("planning mutated input or retained caller pointer")
	}
	var decoded ChangePlan
	if err := json.Unmarshal(a, &decoded); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyChange(decoded, decoded.PlanSHA256)
	if err != nil || result.Rejected || !result.Changed {
		t.Fatal(result, err)
	}
	current, _ = os.ReadFile(file)
	expected := Apply(editFixture, plan.Operation)
	if string(current) != expected.Text || SHA256(current) != plan.AfterSHA256 {
		t.Fatal(string(current))
	}
	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	duplicate, err := ApplyChange(decoded, decoded.PlanSHA256)
	if err != nil || !duplicate.Rejected || duplicate.Diagnostics[0].ID != "c-bindings.stale-file" {
		t.Fatal(duplicate, err)
	}
	noop, _, err := PlanChange(file, plan.Operation)
	if err != nil || noop.Changed || noop.Patch != "" {
		t.Fatal(noop, err)
	}
	before, _ := os.Stat(file)
	result, err = ApplyChange(*noop, noop.PlanSHA256)
	after, _ := os.Stat(file)
	if err != nil || result.Rejected || result.Changed || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("no-op rewrote file", result, err)
	}
}

// A saved/modified proposal never confers authorization or bypasses the owning
// engine; even a newly fingerprinted forged output must match a fresh plan.
// Rules: harness design notes §§5(3)–(7),7(2).
func TestChangePlanRejectsWithoutWriting(t *testing.T) {
	for _, kind := range []string{"no-approval", "wrong-approval", "operation", "patch", "output", "validation", "version", "rehashed-forgery", "stale", "deleted"} {
		t.Run(kind, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "c-bindings.toml")
			os.WriteFile(file, []byte(editFixture), 0600)
			plan, _, err := PlanChange(file, Operation{Kind: OpPackageSet, Version: text("2.0.0")})
			if err != nil {
				t.Fatal(err)
			}
			approval := plan.PlanSHA256
			switch kind {
			case "no-approval":
				approval = ""
			case "wrong-approval":
				approval = strings.Repeat("0", 64)
			case "operation":
				plan.Operation.Version = text("3.0.0")
			case "patch":
				plan.Patch = "forged preview"
			case "output":
				plan.AfterSHA256 = strings.Repeat("0", 64)
			case "validation":
				plan.Validation = nil
			case "version":
				plan.Version = 99
			case "rehashed-forgery":
				plan.Patch = "forged preview"
				plan.PlanSHA256 = planDigest(*plan)
				approval = plan.PlanSHA256
			case "stale":
				os.WriteFile(file, []byte(editFixture+"# concurrent edit\n"), 0600)
			case "deleted":
				os.Remove(file)
			}
			before, _ := os.ReadFile(file)
			result, err := ApplyChange(*plan, approval)
			if err != nil || !result.Rejected || result.Changed || len(result.Diagnostics) != 1 {
				t.Fatal(result, err)
			}
			after, readErr := os.ReadFile(file)
			if string(before) != string(after) || kind == "deleted" && !os.IsNotExist(readErr) {
				t.Fatal("rejection wrote target")
			}
		})
	}
}

// Invalid TOML/operations are rejected before a proposal exists. Simultaneous
// approvals of the same changed input have one commit and one stale rejection.
// Rules: harness design notes §§5(3)–(4),7(2)–(3).
func TestChangePlanValidationAndConcurrentCommit(t *testing.T) {
	file := filepath.Join(t.TempDir(), "c-bindings.toml")
	os.WriteFile(file, []byte("schema_version = ["), 0600)
	plan, ds, err := PlanChange(file, Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil || plan != nil || len(ds) == 0 {
		t.Fatal(plan, ds, err)
	}
	os.WriteFile(file, []byte(editFixture), 0600)
	plan, ds, err = PlanChange(file, Operation{Kind: OpModuleAdd, Common: true, Module: "../escape"})
	if err != nil || plan != nil || len(ds) == 0 {
		t.Fatal(plan, ds, err)
	}
	plan, ds, err = PlanChange(file, Operation{Kind: OpPackageSet, Version: text("2.0.0")})
	if err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := ApplyChange(*plan, plan.PlanSHA256)
			if err != nil {
				t.Error(err)
			}
			results <- result.Changed && !result.Rejected
		}()
	}
	wg.Wait()
	close(results)
	committed := 0
	for ok := range results {
		if ok {
			committed++
		}
	}
	if committed != 1 {
		t.Fatalf("committed %d times", committed)
	}
	// Creation uses absence as its expectation, never overwrites an existing file.
	if err := ReplaceFileAtomically(file, []byte("wrong"), nil); err != ErrStaleFile {
		t.Fatal(err)
	}
}

// Preview hunks faithfully represent newline-only and empty-file changes.
// Rules: harness design notes §5(3).
func TestChangePatchBoundaries(t *testing.T) {
	for _, pair := range [][2]string{{"a", "a\n"}, {"a\n", "a"}, {"", "a\n"}, {"a\n", ""}} {
		patch := changePatch(pair[0], pair[1])
		if !strings.Contains(patch, "@@") || (pair[0] == "a" || pair[1] == "a") && !strings.Contains(patch, "\\ No newline at end of file") {
			t.Fatal(patch)
		}
	}
	if changePatch("same", "same") != "" {
		t.Fatal("no-op patch")
	}
}

// A failed final expectation removes its prepared temporary and preserves all
// original bytes; creation with an absence expectation cannot replace a file.
// Rules: MD-021 correction §5(2)–(4); harness design notes §5(4).
func TestChangePlanAtomicFailureCleansTemporary(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "c-bindings.toml")
	if err := os.WriteFile(file, []byte(editFixture), 0600); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []*string{text(strings.Repeat("0", 64)), nil} {
		if err := ReplaceFileAtomically(file, []byte("replacement"), expected); err != ErrStaleFile {
			t.Fatal(err)
		}
		data, err := os.ReadFile(file)
		if err != nil || string(data) != editFixture {
			t.Fatal("failed commit overwrote target", err)
		}
		names, err := os.ReadDir(directory)
		if err != nil || len(names) != 1 || names[0].Name() != "c-bindings.toml" {
			t.Fatal("temporary leaked", names, err)
		}
	}
}
