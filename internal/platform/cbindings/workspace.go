package cbindings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"sec/internal/configedit"
)

var (
	ErrWorkspaceAuthorization = errors.New("workspace mutation requires trusted caller authorization of this exact plan")
	ErrWorkspacePlan          = errors.New("plan does not belong to this workspace and its named C-binding file")
)

// WorkspaceTool describes the closed adapter surface. Planning and diff are
// read-only even when their input describes a mutation; only apply writes.
// Rules: harness design notes §§5(1),5(5)–(7).
type WorkspaceTool struct {
	Name                  string `json:"name"`
	ReadOnly              bool   `json:"read_only"`
	RequiresAuthorization bool   `json:"requires_authorization"`
	File                  string `json:"file"`
	ValidationScope       string `json:"validation_scope"`
}

// WorkspaceTools returns detached truthful metadata, not shell commands or a
// generic read/write interface. The optional MCP adapter maps this closed surface.
// Rules: harness design notes §§2(2)–(4),5(1),5(6)–(7).
func WorkspaceTools() []WorkspaceTool {
	tools := make([]WorkspaceTool, 0, 5)
	for _, name := range []string{"c-bindings.inspect", "c-bindings.validate", "c-bindings.plan", "c-bindings.diff", "c-bindings.apply"} {
		mutates := name == "c-bindings.apply"
		tools = append(tools, WorkspaceTool{Name: name, ReadOnly: !mutates, RequiresAuthorization: mutates, File: DefaultFile, ValidationScope: "structural; no target capability proof"})
	}
	return tools
}

// WorkspaceApprovalRequest is supplied to trusted host code, never populated
// from tool input as an authorization token. It binds the exact reviewed plan.
type WorkspaceApprovalRequest struct {
	ProjectRoot string
	File        string
	PlanSHA256  string
	Operation   OperationKind
}

// WorkspaceAuthorizer is installed by the user/trusted caller when opening a
// project. Nil means read-only. A tool request or model text cannot install or
// replace it; hashes identify approved content and are not secret credentials.
// Rules: harness design notes §5(5).
type WorkspaceAuthorizer func(WorkspaceApprovalRequest) bool

// Workspace owns an OS directory handle for an explicitly selected project,
// not an ambient cwd or parent-discovered project. All config I/O uses os.Root.
// It exposes no shell, network, arbitrary path or unrestricted write operation.
// Rules: harness design notes §§3(1)–(2),5(5)–(7).
type Workspace struct {
	root      *os.Root
	identity  string
	authorize WorkspaceAuthorizer
}

// WorkspaceChangePlan binds the ordinary owning edit plan to its selected
// project. Copying it to another workspace cannot authorize that other project.
// Rules: harness design notes §§5(3),5(5)–(6).
type WorkspaceChangePlan struct {
	Format      string     `json:"format"`
	Version     int        `json:"version"`
	ProjectRoot string     `json:"project_root"`
	Change      ChangePlan `json:"change"`
	PlanSHA256  string     `json:"plan_sha256"`
}

// WorkspaceInspection is structural only. It makes no host-derived or guessed
// target/Variant capability claim; resolution remains with the canonical owner.
// Rules: harness design notes §§3(2),5(7).
type WorkspaceInspection struct {
	File        string       `json:"file"`
	SHA256      string       `json:"sha256"`
	Scope       string       `json:"scope"`
	Package     *Package     `json:"package,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// OpenWorkspace requires the explicitly named root itself to contain a valid
// Sec project manifest. It never climbs parents. Root-confined operations reject
// traversal and symlink escapes, including link replacement after planning.
// Hosts without the required Root containment guarantee fail closed.
// Rules: harness design notes §§5(5)–(7),7(2); rules/projects/projects.md §5.
func OpenWorkspace(projectRoot string, authorize WorkspaceAuthorizer) (*Workspace, error) {
	root, identity, err := configedit.OpenProjectRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	return &Workspace{root: root, identity: identity, authorize: authorize}, nil
}

// Close releases the selected root. Later I/O fails rather than falling back to
// ambient filesystem access. Rules: harness design notes §5(6).
func (w *Workspace) Close() error {
	if w == nil || w.root == nil {
		return os.ErrClosed
	}
	return w.root.Close()
}

// Inspect reads only the named configuration through the retained project root
// and returns newly parsed, detached structural facts.
// Rules: harness design notes §§3(2),5(1),5(6)–(7).
func (w *Workspace) Inspect() (WorkspaceInspection, error) {
	if w == nil || w.root == nil {
		return WorkspaceInspection{}, os.ErrClosed
	}
	data, err := w.root.ReadFile(filepath.FromSlash(DefaultFile))
	if err != nil {
		return WorkspaceInspection{}, err
	}
	pkg, diagnostics := Parse(string(data))
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	return WorkspaceInspection{File: DefaultFile, SHA256: SHA256(data), Scope: "structural", Package: pkg, Diagnostics: diagnostics}, nil
}

// Validate runs the owning structural validator, without creating a target
// capability assertion or performing a write.
// Rules: harness design notes §§3(2),5(1),5(7).
func (w *Workspace) Validate() ([]Diagnostic, error) {
	result, err := w.Inspect()
	return result.Diagnostics, err
}

// prepareChange is the rooted adapter for the shared transaction engine. The
// file allowlist is closed: caller-selected paths are never forwarded to I/O.
// Rules: harness design notes §§3(1)–(2),5(6).
func (w *Workspace) prepareChange(file string, op Operation) (*ChangePlan, []Diagnostic, error) {
	if w == nil || w.root == nil {
		return nil, nil, os.ErrClosed
	}
	if file != DefaultFile {
		return nil, nil, ErrWorkspacePlan
	}
	data, err := w.root.ReadFile(filepath.FromSlash(DefaultFile))
	if err != nil {
		return nil, nil, err
	}
	return planChangeBytes(DefaultFile, data, op)
}

// PlanChange is read-only, shares the existing typed editor, and binds the
// resulting proposal to this exact selected project. It does not authorize it.
// Rules: harness design notes §§5(2)–(7).
func (w *Workspace) PlanChange(op Operation) (*WorkspaceChangePlan, []Diagnostic, error) {
	change, diagnostics, err := w.prepareChange(DefaultFile, op)
	if err != nil || len(diagnostics) != 0 {
		return nil, diagnostics, err
	}
	plan := &WorkspaceChangePlan{Format: "sec.c-bindings-workspace-plan", Version: 1, ProjectRoot: w.identity, Change: *change}
	plan.PlanSHA256 = workspacePlanDigest(*plan)
	return plan, nil, nil
}

// validPlan checks both integrity layers and the exact project/file before any
// authorization call or input read. No tool input controls the root or policy.
// Rules: harness design notes §§5(3),5(5)–(6).
func (w *Workspace) validPlan(plan WorkspaceChangePlan) bool {
	if w == nil || w.root == nil {
		return false
	}
	return plan.Format == "sec.c-bindings-workspace-plan" && plan.Version == 1 && plan.ProjectRoot == w.identity &&
		plan.Change.File == DefaultFile && plan.Change.Format == "sec.c-bindings-change-plan" && plan.Change.Version == 1 &&
		plan.Change.PlanSHA256 == planDigest(plan.Change) && plan.PlanSHA256 == workspacePlanDigest(plan)
}

// ShowDiff presents only an intact proposal belonging to this selected project.
// It does not claim that a saved proposal's expected input is still current.
// Rules: harness design notes §§5(1)–(3),5(6).
func (w *Workspace) ShowDiff(plan WorkspaceChangePlan) (string, error) {
	if !w.validPlan(plan) {
		return "", ErrWorkspacePlan
	}
	return plan.Change.Patch, nil
}

// Apply requires a trusted host policy's explicit authorization of the exact
// project-bound digest. It then shares all existing edit/hash/preview checks
// and performs every read/temp/check/rename through the selected OS root.
// Rules: harness design notes §§2(2),3(2),5(3)–(7).
func (w *Workspace) Apply(plan WorkspaceChangePlan) (EditResult, error) {
	if !w.validPlan(plan) {
		return EditResult{}, ErrWorkspacePlan
	}
	request := WorkspaceApprovalRequest{ProjectRoot: w.identity, File: DefaultFile, PlanSHA256: plan.PlanSHA256, Operation: plan.Change.Operation.Kind}
	if w.authorize == nil || !w.authorize(request) {
		return EditResult{}, ErrWorkspaceAuthorization
	}
	read := func(file string) ([]byte, error) {
		if file != DefaultFile {
			return nil, ErrWorkspacePlan
		}
		return w.root.ReadFile(filepath.FromSlash(DefaultFile))
	}
	replace := func(file string, data []byte, expected *string) error {
		if file != DefaultFile {
			return ErrWorkspacePlan
		}
		return replaceFileUsing(w.root, filepath.FromSlash(DefaultFile), data, expected)
	}
	return applyChangeUsing(plan.Change, plan.Change.PlanSHA256, read, w.prepareChange, replace)
}

// workspacePlanDigest makes project identity part of the review/approval key.
// Rules: harness design notes §§5(3),5(5)–(6).
func workspacePlanDigest(plan WorkspaceChangePlan) string {
	plan.PlanSHA256 = ""
	data, _ := json.Marshal(plan)
	return SHA256(data)
}
