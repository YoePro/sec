// sec-mcp exposes Sec's optional project tooling over local MCP stdio.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
)

const usage = `usage: sec-mcp --project ROOT [--approve PLAN_SHA256 ...]

Serve C-binding and project profile/allocation inspection, resolution, planning
and diff over local
MCP stdio. Without --approve only read-only tools are available. Each --approve
allows applying exactly that reviewed project-bound plan; input JSON cannot
grant approval or change the selected root. No harness configuration file or
network listener is used. stdout is reserved for MCP; diagnostics use stderr.
`

// options are trusted launch configuration, never MCP tool input.
// Rules: harness design notes §§2(1),5(5)–(7),8.
type options struct {
	project  string
	approved approvals
}

// parseOptions fixes scope and approvals before accepting protocol messages.
// Rules: harness design notes §§5(5)–(6),8.
func parseOptions(args []string, stderr io.Writer) (options, error) {
	var opts options
	flags := flag.NewFlagSet("sec-mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	flags.StringVar(&opts.project, "project", "", "explicit Sec project root")
	flags.Var(&opts.approved, "approve", "exact reviewed project-bound plan SHA-256 (repeatable)")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() != 0 || opts.project == "" {
		return opts, fmt.Errorf("--project ROOT is required; positional arguments are not accepted")
	}
	return opts, nil
}

// run keeps protocol bytes on stdout and uses only the local stdio transport.
// Rules: harness design notes §§2(1)–(4),5(5)–(7),6(3).
func run(ctx context.Context, args []string, input io.ReadCloser, output io.WriteCloser, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "sec-mcp: %v\n", err)
		return 1
	}
	server, closeWorkspace, err := newServer(opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "sec-mcp: %v\n", err)
		return 1
	}
	defer closeWorkspace()
	if err := serve(ctx, server, input, output); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "sec-mcp: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	status := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(status)
}
