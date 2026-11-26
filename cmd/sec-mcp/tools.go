package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindings"
)

type noArguments struct{}
type planArguments struct {
	Operation cbindings.Operation `json:"operation" jsonschema:"One existing typed C-binding package/environment/module edit."`
}
type savedPlanArguments struct {
	Plan cbindings.WorkspaceChangePlan `json:"plan" jsonschema:"The intact project-bound plan returned by c-bindings.plan."`
}
type validationOutput struct {
	Scope       string                 `json:"scope"`
	Diagnostics []cbindings.Diagnostic `json:"diagnostics"`
}
type planOutput struct {
	Plan        *cbindings.WorkspaceChangePlan `json:"plan,omitempty"`
	Diagnostics []cbindings.Diagnostic         `json:"diagnostics"`
}
type diffOutput struct {
	Patch string `json:"patch"`
}
type applyOutput struct {
	Changed     bool                   `json:"changed"`
	Reason      string                 `json:"reason,omitempty"`
	Diagnostics []cbindings.Diagnostic `json:"diagnostics"`
}

// descriptor projects the shared closed tool catalogue into MCP annotations.
// Hints describe behavior; the service independently enforces authorization.
// Rules: harness design notes §§2(2),5(1),5(5)–(7).
func descriptor(meta cbindings.WorkspaceTool, title, description string) *mcp.Tool {
	destructive, openWorld := !meta.ReadOnly, false
	return &mcp.Tool{Name: meta.Name, Title: title, Description: description, Annotations: &mcp.ToolAnnotations{
		ReadOnlyHint: meta.ReadOnly, DestructiveHint: &destructive, OpenWorldHint: &openWorld,
		IdempotentHint: meta.ReadOnly,
	}}
}

// registerTools exposes read-only operations first and enables apply only when
// the trusted launcher supplied exact approvals. Inputs contain no root override,
// authorization setting, shell command, URL or unrestricted output path.
// Rules: harness design notes §§2(1)–(4),3(2),5(1)–(7),6(1).
func registerTools(server *mcp.Server, workspace *cbindings.Workspace, enableApply bool) {
	metadata := map[string]cbindings.WorkspaceTool{}
	for _, meta := range cbindings.WorkspaceTools() {
		metadata[meta.Name] = meta
	}
	tool := func(name, title, description string) *mcp.Tool { return descriptor(metadata[name], title, description) }
	mcp.AddTool(server, tool("c-bindings.inspect", "Inspect C bindings", "Read the selected project's named C-binding configuration and structural diagnostics."),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArguments) (*mcp.CallToolResult, cbindings.WorkspaceInspection, error) {
			if err := ctx.Err(); err != nil {
				return nil, cbindings.WorkspaceInspection{}, err
			}
			out, err := workspace.Inspect()
			return &mcp.CallToolResult{IsError: len(out.Diagnostics) > 0}, out, err
		})
	mcp.AddTool(server, tool("c-bindings.validate", "Validate C bindings", "Validate the existing configuration structurally; this does not establish selected target support."),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArguments) (*mcp.CallToolResult, validationOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, validationOutput{}, err
			}
			diagnostics, err := workspace.Validate()
			return &mcp.CallToolResult{IsError: len(diagnostics) > 0}, validationOutput{Scope: "structural", Diagnostics: diagnostics}, err
		})
	mcp.AddTool(server, tool("c-bindings.plan", "Plan a C-binding edit", "Prepare a read-only typed change plan with hashes, patch and validation consequences. Planning never grants approval."),
		func(ctx context.Context, _ *mcp.CallToolRequest, in planArguments) (*mcp.CallToolResult, planOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, planOutput{}, err
			}
			plan, diagnostics, err := workspace.PlanChange(in.Operation)
			if diagnostics == nil {
				diagnostics = []cbindings.Diagnostic{}
			}
			return &mcp.CallToolResult{IsError: len(diagnostics) > 0}, planOutput{Plan: plan, Diagnostics: diagnostics}, err
		})
	mcp.AddTool(server, tool("c-bindings.diff", "Review a C-binding diff", "Show the intact saved plan's patch without writing or granting approval."),
		func(ctx context.Context, _ *mcp.CallToolRequest, in savedPlanArguments) (*mcp.CallToolResult, diffOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, diffOutput{}, err
			}
			patch, err := workspace.ShowDiff(in.Plan)
			return nil, diffOutput{Patch: patch}, err
		})
	if enableApply {
		mcp.AddTool(server, tool("c-bindings.apply", "Apply an approved C-binding plan", "Apply only an intact exact plan preapproved by trusted launch arguments. Changed inputs are rejected; tool input cannot authorize edits."),
			func(ctx context.Context, _ *mcp.CallToolRequest, in savedPlanArguments) (*mcp.CallToolResult, applyOutput, error) {
				if err := ctx.Err(); err != nil {
					return nil, applyOutput{}, err
				}
				result, err := workspace.Apply(in.Plan)
				if result.Diagnostics == nil {
					result.Diagnostics = []cbindings.Diagnostic{}
				}
				return &mcp.CallToolResult{IsError: result.Rejected}, applyOutput{Changed: result.Changed, Reason: result.Reason, Diagnostics: result.Diagnostics}, err
			})
	}
}
