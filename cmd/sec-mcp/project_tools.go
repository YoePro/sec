package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindings"
	"sec/internal/platform/projectconfig"
)

type projectOperationArguments struct {
	Operation projectconfig.Operation `json:"operation"`
}
type projectPlanArguments struct {
	Plan projectconfig.Plan `json:"plan"`
}
type projectSelectionArguments struct {
	Target  string `json:"target"`
	Variant string `json:"variant"`
}
type projectPlanOutput struct {
	Plan        *projectconfig.Plan        `json:"plan,omitempty"`
	Diagnostics []projectconfig.Diagnostic `json:"diagnostics"`
}

// registerProjectTools exposes the same confined project service used by the
// CLI. Fields select existing scopes, never arbitrary files or startup policy.
// Rules: harness design notes §§2(2),3,5,6(1) step5,7.
func registerProjectTools(server *mcp.Server, workspace *projectconfig.Workspace, enableApply bool) {
	tool := func(name, title, description string, write bool) *mcp.Tool {
		return descriptor(cbindings.WorkspaceTool{Name: name, ReadOnly: !write}, title, description)
	}
	mcp.AddTool(server, tool("project.inspect", "Inspect project selections", "Read and validate all declared target-profile/allocation selections with canonical scope provenance.", false),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArguments) (*mcp.CallToolResult, projectconfig.Inspection, error) {
			if err := ctx.Err(); err != nil {
				return nil, projectconfig.Inspection{}, err
			}
			out, err := workspace.Inspect()
			return &mcp.CallToolResult{IsError: len(out.Diagnostics) > 0}, out, err
		})
	mcp.AddTool(server, tool("project.resolve", "Resolve a target and variant", "Read the canonical selected profile, allocation facts and C-binding plan/fingerprint. Unavailable providers remain errors.", false),
		func(ctx context.Context, _ *mcp.CallToolRequest, in projectSelectionArguments) (*mcp.CallToolResult, projectconfig.Resolution, error) {
			if err := ctx.Err(); err != nil {
				return nil, projectconfig.Resolution{}, err
			}
			out, err := workspace.Resolve(in.Target, in.Variant)
			return &mcp.CallToolResult{IsError: len(out.Diagnostics) > 0}, out, err
		})
	mcp.AddTool(server, tool("project.plan", "Plan profile/allocation selections", "Prepare a read-only manifest proposal for an existing Target/Variant scope. Empty strings clear local keys; nil fields preserve them. Provider names cannot create capability.", false),
		func(ctx context.Context, _ *mcp.CallToolRequest, in projectOperationArguments) (*mcp.CallToolResult, projectPlanOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, projectPlanOutput{}, err
			}
			plan, diagnostics, err := workspace.PlanChange(in.Operation)
			if diagnostics == nil {
				diagnostics = []projectconfig.Diagnostic{}
			}
			return &mcp.CallToolResult{IsError: len(diagnostics) > 0}, projectPlanOutput{plan, diagnostics}, err
		})
	mcp.AddTool(server, tool("project.diff", "Review project selection diff", "Show only an intact saved plan belonging to the selected project; no writes or approval.", false),
		func(ctx context.Context, _ *mcp.CallToolRequest, in projectPlanArguments) (*mcp.CallToolResult, diffOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, diffOutput{}, err
			}
			patch, err := workspace.Diff(in.Plan)
			return nil, diffOutput{patch}, err
		})
	if enableApply {
		mcp.AddTool(server, tool("project.apply", "Apply approved project selections", "Apply an intact project-bound proposal only when its exact hash was approved at launch. Stale inputs fail without writes.", true),
			func(ctx context.Context, _ *mcp.CallToolRequest, in projectPlanArguments) (*mcp.CallToolResult, projectconfig.EditResult, error) {
				if err := ctx.Err(); err != nil {
					return nil, projectconfig.EditResult{}, err
				}
				out, err := workspace.Apply(in.Plan)
				return &mcp.CallToolResult{IsError: out.Rejected}, out, err
			})
	}
}
