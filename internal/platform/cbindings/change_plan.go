package cbindings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sec/internal/configedit"
)

// ChangePlan is a reviewable proposal, never authorization. PlanSHA256 binds
// the named file, typed operation, input/output hashes, patch and validation.
// The trusted caller supplies approval separately, after presenting this plan.
// Rules: rules/corrections/applied/sec-harness-design-notes-0.1.100-2026-11-20-20261121.md §§5(2)–(7),6(1).
type ChangePlan struct {
	Format       string    `json:"format"`
	Version      int       `json:"version"`
	File         string    `json:"file"`
	Operation    Operation `json:"operation"`
	BeforeSHA256 string    `json:"before_sha256"`
	AfterSHA256  string    `json:"after_sha256"`
	Changed      bool      `json:"changed"`
	Reason       string    `json:"reason,omitempty"`
	Patch        string    `json:"patch"`
	Validation   []string  `json:"validation"`
	PlanSHA256   string    `json:"plan_sha256"`
}

// PlanChange reads one explicitly named existing file and runs the owning edit
// engine without writing. Structural validation does not claim target/Variant
// resolution or that an unavailable platform capability became supported.
// Rules: harness design notes §§3(2),5(2)–(7); MD-021 correction §5.
func PlanChange(file string, op Operation) (*ChangePlan, []Diagnostic, error) {
	canonical, err := filepath.Abs(file)
	if err != nil {
		return nil, nil, err
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(canonical)
	if err != nil {
		return nil, nil, err
	}
	return planChangeBytes(canonical, data, op)
}

// planChangeBytes constructs the same validated proposal for explicit CLI files
// and root-confined workspace files, without another parser or edit policy.
// Rules: harness design notes §§2(2),3(2),5(3)–(7).
func planChangeBytes(file string, data []byte, op Operation) (*ChangePlan, []Diagnostic, error) {
	edit := Apply(string(data), op)
	if edit.Rejected {
		return nil, edit.Diagnostics, nil
	}
	// Detach request pointers: editing a caller's operation after planning
	// must not change what the reviewer saw.
	encoded, _ := json.Marshal(op)
	var detached Operation
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return nil, nil, err
	}
	plan := &ChangePlan{Format: "sec.c-bindings-change-plan", Version: 1,
		File: file, Operation: detached, BeforeSHA256: SHA256(data),
		AfterSHA256: SHA256([]byte(edit.Text)), Changed: edit.Changed, Reason: edit.Reason,
		Patch: changePatch(string(data), edit.Text),
		Validation: []string{"C-binding TOML schema and edited model validated by cbindings.Apply.",
			"Unrelated comments and formatting retained by the owning edit engine.",
			"After a changed commit, recompute C-binding selection and fingerprints; prior resolved binding facts must not be reused.",
			"Selected Target/Variant and referenced module files require c-bindings validate --variant NAME; this plan proves no target capability."},
	}
	plan.PlanSHA256 = planDigest(*plan)
	return plan, nil, nil
}

// ApplyChange requires explicit approval of the exact plan fingerprint. It
// re-reads the input, regenerates/validates the same typed edit and patch, and
// checks the input again immediately before replacing. A rejected/stale plan
// never overwrites the target; an approved no-op performs no write.
// Rules: harness design notes §§5(2)–(7),7(2)–(3); MD-021 correction §5(2)–(4).
func ApplyChange(plan ChangePlan, approvedSHA256 string) (EditResult, error) {
	return applyChangeUsing(plan, approvedSHA256, os.ReadFile, PlanChange, ReplaceFileAtomically)
}

// applyChangeUsing keeps review validation, stale detection and edit semantics
// identical for ordinary explicit-file and confined-root I/O adapters.
// Rules: harness design notes §§2(2),5(2)–(7).
func applyChangeUsing(plan ChangePlan, approvedSHA256 string,
	readFile func(string) ([]byte, error),
	prepare func(string, Operation) (*ChangePlan, []Diagnostic, error),
	replace func(string, []byte, *string) error) (EditResult, error) {
	reject := func(id, message string) (EditResult, error) {
		return EditResult{Rejected: true, Diagnostics: []Diagnostic{{ID: id, Message: message}}}, nil
	}
	if approvedSHA256 == "" || approvedSHA256 != plan.PlanSHA256 {
		return reject("c-bindings.plan-not-approved", "explicit caller approval of this plan_sha256 is required; planning is read-only")
	}
	if plan.Format != "sec.c-bindings-change-plan" || plan.Version != 1 || planDigest(plan) != plan.PlanSHA256 {
		return reject("c-bindings.invalid-plan", "change plan is inconsistent or uses an unsupported format/version; prepare and review a new plan")
	}
	current, err := readFile(plan.File)
	if os.IsNotExist(err) || err == nil && SHA256(current) != plan.BeforeSHA256 {
		return reject("c-bindings.stale-file", "input changed after planning; nothing was written; prepare and review a new plan")
	}
	if err != nil {
		return EditResult{}, err
	}
	regenerated, diagnostics, err := prepare(plan.File, plan.Operation)
	if err != nil {
		return EditResult{}, err
	}
	if len(diagnostics) != 0 || regenerated == nil || !reflect.DeepEqual(*regenerated, plan) {
		return reject("c-bindings.invalid-plan", "plan differs from the canonical typed edit and validation; nothing was written")
	}
	edit := Apply(string(current), plan.Operation)
	if edit.Rejected || SHA256([]byte(edit.Text)) != plan.AfterSHA256 {
		return reject("c-bindings.invalid-plan", "proposed output no longer matches the approved edit; nothing was written")
	}
	if !edit.Changed {
		return edit, nil
	}
	err = replace(plan.File, []byte(edit.Text), &plan.BeforeSHA256)
	if err == ErrStaleFile {
		return reject("c-bindings.stale-file", "input changed while applying; nothing was written")
	}
	return edit, err
}

// planDigest binds every reviewable field while excluding its own digest.
// Rules: harness design notes §§5(3),5(5).
func planDigest(plan ChangePlan) string {
	plan.PlanSHA256 = ""
	data, _ := json.Marshal(plan)
	return SHA256(data)
}

// changePatch keeps the established C-binding patch labels and hunk format.
// Rules: harness design notes §5(2)-(4).
func changePatch(before, after string) string {
	return configedit.ReviewPatch(before, after, "c-bindings.toml")
}
