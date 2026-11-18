package main

import (
	"flag"
	"fmt"
)

// auditQueryRoot shares the option policy for repository-wide read-only audits:
// one audit, the selected governance directory and optional JSON output. Even
// explicitly false presentation flags are rejected, rather than silently ignored.
// Contract: cmd/yamlstatus/IDEAS.md §§5,6,8.
func auditQueryRoot(flags *flag.FlagSet, mode, dir string) (string, error) {
	var incompatible string
	flags.Visit(func(option *flag.Flag) {
		if option.Name != "dir" && option.Name != mode && option.Name != "json" && incompatible == "" {
			incompatible = option.Name
		}
	})
	if incompatible != "" {
		return "", fmt.Errorf("-%s cannot be combined with -%s; use only -dir and -json", mode, incompatible)
	}
	return referenceRoot(dir)
}
