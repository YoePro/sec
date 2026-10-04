package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sec/internal/ast"
	lspserver "sec/internal/lsp/server"
	"sort"
	"strconv"
	"strings"

	"sec/internal/layout"
	platformtarget "sec/internal/platform/target"
)

type lspManifestVariant struct {
	os   string
	arch string
}

type lspManifestTarget struct {
	source   string
	variants []string
}

// lspScalarPlan resolves the active source's project target through the same
// compiler-owned registry used by sec check/build. Until target switching is
// exposed by the protocol, resolution is deliberately limited to an
// unambiguous logical target and scalar plan; ambiguous multi-variant projects
// retain the target-independent fallback instead of guessing.
//
// Rules:
//   - rules/tooling/lsp.md — "Target-aware analysis" and A.20
//   - rules/projects/projects.md — "Targets", "Variants", and "Compilation plans and target lowering"
//   - rules/types/types.md — "int and uint" and "Binary floating-point types"
//   - rules/corrections/applied/correction5-20260823.md — selected target plan must reach Sema
func lspScalarPlan(sourcePath string) (layout.ResolvedScalarPlan, error) {
	if sourcePath == "" {
		return layout.ResolvedScalarPlan{}, fmt.Errorf("source path is empty")
	}
	root := findProjectRoot(sourcePath)
	manifest := filepath.Join(root, ".sec", "sec.toml")
	variants, targets, err := readLSPProjectTargets(manifest)
	if err != nil {
		return layout.ResolvedScalarPlan{}, err
	}
	target, err := selectLSPManifestTarget(root, sourcePath, targets)
	if err != nil {
		return layout.ResolvedScalarPlan{}, err
	}
	if len(target.variants) == 0 {
		return layout.ResolvedScalarPlan{}, fmt.Errorf("selected project target has no variants")
	}

	var selected layout.ResolvedScalarPlan
	for index, name := range target.variants {
		variant, ok := variants[name]
		if !ok {
			return layout.ResolvedScalarPlan{}, fmt.Errorf("project target references unknown variant %q", name)
		}
		definition, ok := platformtarget.Find(platformtarget.Target{OS: platformtarget.NormalizeOS(variant.os), Arch: platformtarget.NormalizeArch(variant.arch)})
		if !ok {
			return layout.ResolvedScalarPlan{}, fmt.Errorf("variant %q has unsupported target %s-%s", name, variant.os, variant.arch)
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			return layout.ResolvedScalarPlan{}, err
		}
		if index == 0 {
			selected = plan
			continue
		}
		if plan != selected {
			return layout.ResolvedScalarPlan{}, fmt.Errorf("project target has multiple active scalar plans")
		}
	}
	return selected, nil
}

func selectLSPManifestTarget(root string, sourcePath string, targets map[string]lspManifestTarget) (lspManifestTarget, error) {
	relative, err := filepath.Rel(root, sourcePath)
	if err != nil {
		return lspManifestTarget{}, err
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	matches := make([]lspManifestTarget, 0, 1)
	for _, name := range names {
		target := targets[name]
		if strings.TrimSpace(target.source) == "" {
			continue
		}
		source := filepath.ToSlash(filepath.Clean(target.source))
		if source != "." && relative != source && !strings.HasPrefix(relative, source+"/") {
			continue
		}
		matches = append(matches, target)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 && len(names) == 1 {
		target := targets[names[0]]
		if strings.TrimSpace(target.source) != "" {
			return target, nil
		}
	}
	return lspManifestTarget{}, fmt.Errorf("source does not select one unambiguous project target")
}

func readLSPProjectTargets(manifest string) (map[string]lspManifestVariant, map[string]lspManifestTarget, error) {
	file, err := os.Open(manifest)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	variants := map[string]lspManifestVariant{}
	targets := map[string]lspManifestTarget{}
	sectionKind, sectionName := "", ""
	lines, err := lspManifestLogicalLines(file)
	if err != nil {
		return nil, nil, err
	}
	for _, line := range lines {
		line = strings.TrimSpace(stripLSPManifestComment(line))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionKind, sectionName = lspManifestSection(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || sectionName == "" {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch sectionKind {
		case "variant":
			variant := variants[sectionName]
			switch key {
			case "os":
				variant.os, err = lspManifestString(value)
			case "arch":
				variant.arch, err = lspManifestString(value)
			}
			variants[sectionName] = variant
		case "target":
			target := targets[sectionName]
			switch key {
			case "source":
				target.source, err = lspManifestString(value)
			case "variants":
				target.variants, err = lspManifestStringArray(value)
			}
			targets[sectionName] = target
		}
		if err != nil {
			return nil, nil, fmt.Errorf("invalid %s in %s: %w", key, manifest, err)
		}
	}
	if len(targets) == 0 {
		return nil, nil, fmt.Errorf("project manifest defines no targets")
	}
	return variants, targets, nil
}

func lspManifestLogicalLines(reader io.Reader) ([]string, error) {
	lines := []string{}
	scanner := bufio.NewScanner(reader)
	pending := ""
	arrayDepth := 0
	for scanner.Scan() {
		line := scanner.Text()
		if pending != "" {
			pending += " " + strings.TrimSpace(line)
		} else {
			pending = line
		}
		arrayDepth += strings.Count(stripLSPManifestComment(line), "[") - strings.Count(stripLSPManifestComment(line), "]")
		if arrayDepth > 0 && strings.Contains(pending, "=") {
			continue
		}
		lines = append(lines, pending)
		pending = ""
		arrayDepth = 0
	}
	if pending != "" {
		lines = append(lines, pending)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func lspManifestSection(section string) (string, string) {
	kind, name, ok := strings.Cut(strings.TrimSpace(section), ".")
	if !ok || (kind != "variant" && kind != "target") {
		return "", ""
	}
	name = strings.TrimSpace(name)
	if unquoted, err := strconv.Unquote(name); err == nil {
		name = unquoted
	}
	return kind, name
}

func lspManifestString(value string) (string, error) {
	return strconv.Unquote(strings.TrimSpace(value))
}

func lspManifestStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, fmt.Errorf("expected string array")
	}
	value = strings.TrimSpace(value[1 : len(value)-1])
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		item, err := strconv.Unquote(part)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func stripLSPManifestComment(line string) string {
	inString := false
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if char == '#' && !inString {
			return line[:index]
		}
	}
	return line
}

// lspActiveTarget is the target whose sources one analysis of sourcePath
// sees: the active document's own `#target` when it has one, so a
// platform-specific file is analyzed for its platform; otherwise the project
// target when all of its variants share one OS and architecture; otherwise the
// host target, as `sec check` uses by default.
//
// Rules:
//   - rules/platform/platform_model.md — source selection by target
//   - rules/tooling/lsp.md — "Shared compiler workspace" (target variants)
func lspActiveTarget(program *ast.Program, sourcePath string) platformtarget.Target {
	if target, directed := lspserver.ProgramTarget(program); directed {
		return target
	}
	if sourcePath != "" {
		root := findProjectRoot(sourcePath)
		variants, targets, err := readLSPProjectTargets(filepath.Join(root, ".sec", "sec.toml"))
		if err == nil {
			if selected, err := selectLSPManifestTarget(root, sourcePath, targets); err == nil {
				var common platformtarget.Target
				for index, name := range selected.variants {
					variant, ok := variants[name]
					if !ok {
						common = platformtarget.Target{}
						break
					}
					current := platformtarget.Target{OS: platformtarget.NormalizeOS(variant.os), Arch: platformtarget.NormalizeArch(variant.arch)}
					if index == 0 {
						common = current
					} else if current != common {
						common = platformtarget.Target{}
						break
					}
				}
				if common.OS != "" {
					return common
				}
			}
		}
	}
	return platformtarget.Host()
}
