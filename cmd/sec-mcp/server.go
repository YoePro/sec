package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindings"
	"sec/internal/platform/projectconfig"
)

// approvals is populated exclusively by repeated trusted launch flags.
// Rules: harness design notes §5(5).
type approvals map[string]bool

func (a *approvals) String() string { return "" }

// Set accepts only exact plan digests; no wildcard or blanket grant exists.
// Rules: harness design notes §§5(3),5(5).
func (a *approvals) Set(value string) error {
	digest, err := hex.DecodeString(value)
	if err != nil || len(digest) != 32 {
		return fmt.Errorf("--approve requires exactly one 64-digit hexadecimal plan SHA-256")
	}
	if *a == nil {
		*a = approvals{}
	}
	(*a)[strings.ToLower(value)] = true
	return nil
}

// newServer binds the SDK transport to the existing confined service. Startup
// approvals are copied so later caller mutation cannot change server policy.
// Rules: harness design notes §§2(1)–(4),3(2),5(1),5(5)–(7).
func newServer(opts options, stderr io.Writer) (*mcp.Server, func() error, error) {
	allowed := approvals{}
	for digest, yes := range opts.approved {
		if yes {
			allowed[digest] = true
		}
	}
	var authorize cbindings.WorkspaceAuthorizer
	if len(allowed) > 0 {
		authorize = func(request cbindings.WorkspaceApprovalRequest) bool { return allowed[request.PlanSHA256] }
	}
	workspace, err := cbindings.OpenWorkspace(opts.project, authorize)
	if err != nil {
		return nil, nil, err
	}
	projectWorkspace, err := projectconfig.Open(opts.project, func(hash string) bool { return allowed[hash] })
	if err != nil {
		workspace.Close()
		return nil, nil, err
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "sec-mcp", Version: "0.1.1"}, &mcp.ServerOptions{
		Logger:       slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Instructions: "Inspect, resolve and plan C-binding and project profile/allocation selections within the launch-selected Sec project. Structural validation alone is not proof of target capability; project.resolve returns canonical selected facts. Apply is available only for exact plans preapproved by the trusted launcher.",
	})
	registerTools(server, workspace, len(allowed) > 0)
	registerProjectTools(server, projectWorkspace, len(allowed) > 0)
	return server, func() error { return errors.Join(workspace.Close(), projectWorkspace.Close()) }, nil
}

// serve delegates framing, initialization and request/schema handling to the
// official SDK. There is no HTTP transport, subprocess compiler or AI runtime.
// Rules: harness design notes §§2(1)–(4),6(2)–(3),7(4).
func serve(ctx context.Context, server *mcp.Server, input io.ReadCloser, output io.WriteCloser) error {
	return server.Run(ctx, &mcp.IOTransport{Reader: input, Writer: output})
}
