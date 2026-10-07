package main

import (
	"os"
	"path/filepath"

	"sec/internal/layout"
	platformtarget "sec/internal/platform/target"
)

// lspAllocationCapabilities resolves common allocation facts independently of
// scalar width. Matching allocation profiles across variants are usable even
// when their scalar plans differ; missing, mixed or invalid project metadata
// stays unknown. Standalone sources use the actual registered host target,
// rather than deriving Hosted from a platform family or convenience default.
// Rules: rules/platform/target_profiles.md — §§2,3,31-32;
// rules/memory/allocation.md — §§22,29(1); rules/tooling/lsp.md — "Target-aware analysis".
func lspAllocationCapabilities(sourcePath string) layout.AllocationCapabilities {
	unknown := layout.ResolveAllocationCapabilities("")
	if sourcePath == "" {
		return unknown
	}
	root := findProjectRoot(sourcePath)
	variants, targets, err := readLSPProjectTargets(filepath.Join(root, ".sec", "sec.toml"))
	if os.IsNotExist(err) {
		if definition, found := platformtarget.Find(platformtarget.Host()); found {
			if plan, err := definition.ScalarPlan(); err == nil {
				return plan.AllocationCapabilities()
			}
		}
		return unknown
	}
	if err != nil {
		return unknown
	}
	selected, err := selectLSPManifestTarget(root, sourcePath, targets)
	if err != nil || len(selected.variants) == 0 {
		return unknown
	}
	common := unknown
	for index, name := range selected.variants {
		variant, found := variants[name]
		if !found {
			return unknown
		}
		definition, found := platformtarget.Find(platformtarget.Target{OS: platformtarget.NormalizeOS(variant.os), Arch: platformtarget.NormalizeArch(variant.arch)})
		if !found {
			return unknown
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			return unknown
		}
		facts := plan.AllocationCapabilities()
		if index == 0 {
			common = facts
		} else if common != facts {
			return unknown
		}
	}
	return common
}
