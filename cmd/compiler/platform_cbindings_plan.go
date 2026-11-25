package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"sec/internal/platform/cbindings"
)

// emitCBindingsPlan exposes the same reviewable proposal to people and scripts.
// It never writes the target or a plan file; callers choose where to save JSON.
// Rules: harness design notes §§5(1)–(5); rules/tooling/cli.md §8.
func emitCBindingsPlan(plan cbindings.ChangePlan, asJSON bool, stdout io.Writer) int {
	if asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "Plan %s\nFile: %s\nOperation: %s\nInput SHA-256: %s\nOutput SHA-256: %s\n", plan.PlanSHA256, plan.File, plan.Operation.Kind, plan.BeforeSHA256, plan.AfterSHA256)
		for _, consequence := range plan.Validation {
			fmt.Fprintln(stdout, consequence)
		}
		if plan.Changed {
			fmt.Fprint(stdout, plan.Patch)
		} else {
			fmt.Fprintf(stdout, "Unchanged (%s)\n", plan.Reason)
		}
		fmt.Fprintln(stdout, "Read-only proposal. Save --plan --json output, review it, then use apply-plan PLAN.json --approve PLAN_SHA256.")
	}
	return 0
}

// runCBindingsApplyPlan reads a saved proposal strictly and takes approval only
// from the explicit caller flag, never from JSON/model text. Shared service code
// owns revalidation, stale-input checks, the typed edit and atomic replacement.
// Rules: harness design notes §§5(2)–(7); rules/tooling/cli.md §8.
func runCBindingsApplyPlan(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		fmt.Fprintln(stderr, cBindingsUsage)
		return 1
	}
	file, approved, asJSON := args[0], "", false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if asJSON {
				fmt.Fprintln(stderr, "--json given more than once")
				return 1
			}
			asJSON = true
		case "--approve":
			if approved != "" || i+1 == len(args) {
				fmt.Fprintln(stderr, "--approve requires one plan SHA-256")
				return 1
			}
			i++
			approved = args[i]
		default:
			fmt.Fprintf(stderr, "unknown apply-plan argument %q\n", args[i])
			return 1
		}
	}
	reader, err := os.Open(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var plan cbindings.ChangePlan
	if err := decoder.Decode(&plan); err != nil {
		fmt.Fprintf(stderr, "invalid change plan: %v\n", err)
		return 1
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fmt.Fprintln(stderr, "change plan must contain exactly one JSON document")
		return 1
	}
	edit, err := cbindings.ApplyChange(plan, approved)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := cBindingsResult{Format: "sec.c-bindings-cli", Version: 1, Operation: string(plan.Operation.Kind), File: plan.File, Status: "complete", Changed: edit.Changed, Reason: edit.Reason, Diagnostics: edit.Diagnostics, BeforeSHA: &plan.BeforeSHA256, AfterSHA: &plan.AfterSHA256}
	if edit.Rejected {
		result.Status, result.AfterSHA = "rejected", nil
		emitCBindings(result, asJSON, stdout, "change plan rejected; nothing was written")
		return 2
	}
	emitCBindings(result, asJSON, stdout, "change plan applied: "+plan.File)
	return 0
}
