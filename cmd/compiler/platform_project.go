package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"sec/internal/platform/projectconfig"
)

const projectConfigurationUsage = `usage: sec platform project inspect --project ROOT [--json]
       sec platform project resolve --project ROOT --target TARGET --variant VARIANT [--json]
       sec platform project plan --project ROOT --operation JSON [--json]
       sec platform project diff --project ROOT --plan PLAN.json [--json]
       sec platform project apply-plan --project ROOT --plan PLAN.json --approve SHA256 [--json]

All results are JSON. Planning never writes. Approval names the exact reviewed
project-bound plan. Profile/provider facts use the canonical target resolver.
`

// decodeProjectJSON rejects unknown fields and trailing documents, matching MCP
// tool schemas; no JSON input can install policy or override the selected root.
// Rules: rules/tooling/cli.md §9; harness design notes §§3(2),5(5)-(6),7(1).
func decodeProjectJSON(text string, value any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one JSON document")
	}
	return nil
}

// runProjectConfiguration is a thin CLI adapter over the same service exposed
// by sec-mcp; it adds no profile syntax, provider catalog or validation policy.
// Rules: rules/tooling/cli.md §9; harness design notes §§2(2)-(4),3,5,6(1) step5,7(1)-(3).
func runProjectConfiguration(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, projectConfigurationUsage)
		return 1
	}
	if args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, projectConfigurationUsage)
		return 0
	}
	flags := flag.NewFlagSet("platform project", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("project", "", "explicit project root")
	target := flags.String("target", "", "declared Target")
	variant := flags.String("variant", "", "declared Variant")
	operation := flags.String("operation", "", "typed operation JSON")
	planFile := flags.String("plan", "", "reviewed plan JSON file")
	approval := flags.String("approve", "", "exact approved plan SHA256")
	flags.Bool("json", true, "emit deterministic JSON")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	if flags.NArg() != 0 || *root == "" {
		return fail(errors.New(projectConfigurationUsage))
	}
	allowed := map[string]bool{"project": true, "json": true}
	switch args[0] {
	case "inspect":
	case "resolve":
		allowed["target"], allowed["variant"] = true, true
	case "plan":
		allowed["operation"] = true
	case "diff":
		allowed["plan"] = true
	case "apply-plan":
		allowed["plan"], allowed["approve"] = true, true
	default:
		return fail(errors.New(projectConfigurationUsage))
	}
	var flagError error
	flags.Visit(func(f *flag.Flag) {
		if !allowed[f.Name] {
			flagError = fmt.Errorf("--%s is not valid for %s", f.Name, args[0])
		}
	})
	if flagError != nil {
		return fail(flagError)
	}
	workspace, err := projectconfig.Open(*root, func(hash string) bool { return *approval != "" && hash == *approval })
	if err != nil {
		return fail(err)
	}
	defer workspace.Close()
	write := func(value any, rejected bool) int {
		if err := json.NewEncoder(stdout).Encode(value); err != nil {
			return fail(err)
		}
		if rejected {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "inspect":
		out, err := workspace.Inspect()
		if err != nil {
			return fail(err)
		}
		return write(out, len(out.Diagnostics) > 0)
	case "resolve":
		out, err := workspace.Resolve(*target, *variant)
		if err != nil {
			return fail(err)
		}
		return write(out, len(out.Diagnostics) > 0)
	case "plan":
		var op projectconfig.Operation
		if err := decodeProjectJSON(*operation, &op); err != nil {
			return fail(err)
		}
		plan, diagnostics, err := workspace.PlanChange(op)
		if err != nil {
			return fail(err)
		}
		if diagnostics == nil {
			diagnostics = []projectconfig.Diagnostic{}
		}
		return write(struct {
			Plan        *projectconfig.Plan        `json:"plan,omitempty"`
			Diagnostics []projectconfig.Diagnostic `json:"diagnostics"`
		}{plan, diagnostics}, len(diagnostics) > 0)
	case "diff", "apply-plan":
		data, err := os.ReadFile(*planFile)
		if err != nil {
			return fail(err)
		}
		var plan projectconfig.Plan
		if err := decodeProjectJSON(string(data), &plan); err != nil {
			return fail(err)
		}
		if args[0] == "diff" {
			patch, err := workspace.Diff(plan)
			if err != nil {
				return fail(err)
			}
			return write(struct {
				Patch string `json:"patch"`
			}{patch}, false)
		}
		out, err := workspace.Apply(plan)
		if err != nil {
			return fail(err)
		}
		return write(out, out.Rejected)
	}
	return 1
}
