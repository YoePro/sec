package projectconfig

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"

	"sec/internal/configedit"
)

var ErrPlan = errors.New("invalid project configuration plan or workspace")
var ErrApproval = errors.New("exact trusted startup approval is required")

// Plan is a review artifact, not authority. The digest binds the project, fixed
// file, exact bytes, typed edit, diff and canonical resolved selection/provenance.
type Plan struct {
	Format       string            `json:"format"`
	Version      int               `json:"version"`
	ProjectRoot  string            `json:"project_root"`
	File         string            `json:"file"`
	Operation    Operation         `json:"operation"`
	BeforeSHA256 string            `json:"before_sha256"`
	AfterSHA256  string            `json:"after_sha256"`
	Changed      bool              `json:"changed"`
	Patch        string            `json:"patch"`
	Resolutions  []ResolutionEntry `json:"resolutions"`
	PlanSHA256   string            `json:"plan_sha256"`
}
type Inspection struct {
	File        string            `json:"file"`
	SHA256      string            `json:"sha256"`
	Resolutions []ResolutionEntry `json:"resolutions"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

// Workspace retains the selected root and immutable caller policy; request
// data cannot select another file or install an authorizer.
type Workspace struct {
	root      *os.Root
	identity  string
	authorize func(string) bool
}

// Open uses the same OS root containment as the C-binding workspace.
// Rules: harness design notes §§3(1)-(2),5(5)-(7).
func Open(name string, authorize func(string) bool) (*Workspace, error) {
	root, identity, err := configedit.OpenProjectRoot(name)
	if err != nil {
		return nil, err
	}
	return &Workspace{root, identity, authorize}, nil
}
func (w *Workspace) Close() error { return w.root.Close() }

// Inspect resolves the current manifest through canonical schema/profile owners.
// Rules: rules/projects/projects.md §§19,25; rules/platform/target_profiles.md §41.
func (w *Workspace) Inspect() (Inspection, error) {
	data, err := w.root.ReadFile(File)
	if err != nil {
		return Inspection{}, err
	}
	resolutions, diagnostics := Resolve(string(data))
	return Inspection{File, configedit.SHA256(data), resolutions, diagnostics}, nil
}

// PlanChange prepares the exact detached proposal without granting writes.
// Rules: harness design notes §§5(2)-(7),6(1) step5.
func (w *Workspace) PlanChange(op Operation) (*Plan, []Diagnostic, error) {
	data, err := w.root.ReadFile(File)
	if err != nil {
		return nil, nil, err
	}
	edit := Apply(string(data), op)
	if edit.Rejected {
		return nil, edit.Diagnostics, nil
	}
	encoded, _ := json.Marshal(op)
	var detached Operation
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return nil, nil, err
	}
	resolutions, _ := Resolve(edit.Text)
	plan := &Plan{Format: "sec.project-config-plan", Version: 1, ProjectRoot: w.identity, File: File, Operation: detached, BeforeSHA256: configedit.SHA256(data), AfterSHA256: configedit.SHA256([]byte(edit.Text)), Changed: edit.Changed, Patch: configedit.ReviewPatch(string(data), edit.Text, "sec.toml"), Resolutions: resolutions}
	plan.PlanSHA256 = digest(*plan)
	return plan, nil, nil
}

// digest binds every public review field, excluding its own digest.
// Rules: harness design notes §5(3),(5).
func digest(plan Plan) string {
	plan.PlanSHA256 = ""
	data, _ := json.Marshal(plan)
	return configedit.SHA256(data)
}
func (w *Workspace) valid(plan Plan) bool {
	return plan.Format == "sec.project-config-plan" && plan.Version == 1 && plan.ProjectRoot == w.identity && plan.File == File && plan.PlanSHA256 == digest(plan)
}

// Diff rejects a forged or foreign review artifact without reading request paths.
// Rules: harness design notes §§5(1)-(3),(6).
func (w *Workspace) Diff(plan Plan) (string, error) {
	if !w.valid(plan) {
		return "", ErrPlan
	}
	return plan.Patch, nil
}

// Apply requires exact external approval, revalidates canonical edit and target
// facts, and commits through the same rooted atomic primitive as C-binding edits.
// Rules: harness design notes §§3,5(2)-(7),7(2)-(3).
func (w *Workspace) Apply(plan Plan) (EditResult, error) {
	if !w.valid(plan) {
		return EditResult{}, ErrPlan
	}
	if w.authorize == nil || !w.authorize(plan.PlanSHA256) {
		return EditResult{}, ErrApproval
	}
	reject := func(id, msg string) (EditResult, error) {
		return EditResult{Rejected: true, Diagnostics: []Diagnostic{{id, msg}}}, nil
	}
	data, err := w.root.ReadFile(File)
	if os.IsNotExist(err) || err == nil && configedit.SHA256(data) != plan.BeforeSHA256 {
		return reject("project-config.stale-file", "manifest changed after review; nothing written")
	}
	if err != nil {
		return EditResult{}, err
	}
	regenerated, diagnostics, err := w.PlanChange(plan.Operation)
	if err != nil {
		return EditResult{}, err
	}
	if len(diagnostics) > 0 || regenerated == nil || !reflect.DeepEqual(*regenerated, plan) {
		return reject("project-config.invalid-plan", "proposal no longer matches canonical validation; nothing written")
	}
	edit := Apply(string(data), plan.Operation)
	if edit.Rejected || configedit.SHA256([]byte(edit.Text)) != plan.AfterSHA256 {
		return reject("project-config.invalid-plan", "output no longer matches proposal; nothing written")
	}
	if !edit.Changed {
		return edit, nil
	}
	err = configedit.ReplaceFileInRoot(w.root, File, []byte(edit.Text), &plan.BeforeSHA256)
	if errors.Is(err, configedit.ErrStaleFile) {
		return reject("project-config.stale-file", "manifest changed during apply; nothing written")
	}
	return edit, err
}
