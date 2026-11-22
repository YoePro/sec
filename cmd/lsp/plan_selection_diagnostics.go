package main

import (
	"regexp"
	"sort"
	"strings"

	platformtarget "sec/internal/platform/target"
	"sec/internal/project"
)

// planSelectionDiagnostics reports the MD-139 profile and allocation
// selection errors of every declared Variant and Target-Variant pair on the
// manifest, matching the CLI's pre-analysis validation: invalid tables and
// keys from project.ResolvePlanSelection, then a profile, mode or provider
// the selected target cannot provide (ResolveTargetProfile). A diagnostic
// is placed on the table header that owns the selection.
// Rules: rules/platform/target_profiles.md — § 41; rules/projects/projects.md — § 19(11)-(16);
// rules/tooling/lsp.md — shared diagnostics.
func planSelectionDiagnostics(text string) []diagnostic {
	doc, err := project.ParseManifestDocument(text)
	if err != nil {
		return nil
	}
	variants, _ := doc.Table("variant")
	targets, _ := doc.Table("target")
	type pair struct{ target, variant string }
	var pairs []pair
	for name := range variants {
		pairs = append(pairs, pair{"", name})
	}
	for target, value := range targets {
		table, _ := value.(map[string]any)
		if inner, ok := table["variant"].(map[string]any); ok {
			for name := range inner {
				pairs = append(pairs, pair{target, name})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].target+"\x00"+pairs[i].variant < pairs[j].target+"\x00"+pairs[j].variant
	})
	seen := map[string]bool{}
	var result []diagnostic
	report := func(header, message string) {
		if seen[message] {
			return
		}
		seen[message] = true
		line := manifestHeaderLine(text, header, strings.Contains(message, ".allocation"))
		result = append(result, diagnostic{Range: lspRange{Start: position{Line: line}, End: position{Line: line, Character: len(strings.Split(text, "\n")[line])}},
			Severity: 1, Code: "manifest.plan-selection", Source: "sec", Message: message})
	}
	for _, p := range pairs {
		header := "variant." + p.variant
		if p.target != "" {
			header = "target." + p.target + ".variant." + p.variant
		}
		selection, errs := project.ResolvePlanSelection(text, p.target, p.variant)
		for _, err := range errs {
			report(header, err.Error())
		}
		if len(errs) != 0 || selection.Profile == "" && selection.AllocationMode == "" && selection.Provider == "" {
			continue
		}
		table, _ := variants[p.variant].(map[string]any)
		osName, _ := table["os"].(string)
		arch, _ := table["arch"].(string)
		platform, ok := platformtarget.Parse(osName + "-" + arch)
		if !ok {
			continue
		}
		definition, found := platformtarget.Find(platform)
		if !found {
			continue
		}
		if _, err := platformtarget.ResolveTargetProfile(definition, selection.Profile, string(selection.AllocationMode), selection.Provider); err != nil {
			report(header, "Variant "+p.variant+": "+err.Error())
		}
	}
	return result
}

// manifestHeaderLine finds the 0-based line of the `[header]` table, or of
// `[header.allocation]` first for an allocation error, or 0 when the table is
// written another way.
func manifestHeaderLine(text, header string, allocation bool) int {
	lines := strings.Split(text, "\n")
	find := func(name string) (int, bool) {
		pattern := regexp.MustCompile(`^\s*\[\s*` + regexp.QuoteMeta(name) + `\s*\]`)
		for index, line := range lines {
			if pattern.MatchString(line) {
				return index, true
			}
		}
		return 0, false
	}
	if allocation {
		if line, found := find(header + ".allocation"); found {
			return line
		}
	}
	line, _ := find(header)
	return line
}
