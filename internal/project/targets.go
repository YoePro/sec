package project

import (
	"bufio"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Target is the part of a `[target.<name>]` table that source analysis needs:
// its kind (command, firmware, library, or test) and its source location.
//
// Rules:
//   - rules/projects/projects.md — § 17 "Targets"
type Target struct {
	Name   string
	Kind   string
	Source string
}

// ParseManifestTargets reads the kind and source of every `[target.<name>]`
// table of a manifest. Nested tables such as `[target.<name>.version]` and
// all other tables are skipped; malformed values are left empty so the
// caller falls back to its default.
func ParseManifestTargets(text string) []Target {
	targets := map[string]*Target{}
	current := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			current = ""
			header := strings.TrimSpace(strings.Trim(line, "[]"))
			if !strings.HasPrefix(header, "target.") {
				continue
			}
			name := strings.TrimPrefix(header, "target.")
			if unquoted, err := strconv.Unquote(name); err == nil {
				name = unquoted
			} else if strings.Contains(name, ".") {
				continue
			}
			if targets[name] == nil {
				targets[name] = &Target{Name: name}
			}
			current = name
			continue
		}
		if current == "" {
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		value, err := parseTOMLString(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		switch strings.TrimSpace(key) {
		case "kind":
			targets[current].Kind = value
		case "source":
			targets[current].Source = value
		}
	}
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Target, 0, len(names))
	for _, name := range names {
		result = append(result, *targets[name])
	}
	return result
}

// TargetForSource returns the Target whose source is the given source file's
// module directory, or the file itself, under the project root.
//
// Rules:
//   - rules/projects/projects.md — § 17 "Targets", § 14 project-root source files
func TargetForSource(targets []Target, projectRoot string, sourcePath string) (Target, bool) {
	file := filepath.Clean(sourcePath)
	directory := filepath.Dir(file)
	for _, target := range targets {
		if target.Source == "" {
			continue
		}
		location := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(target.Source)))
		if location == file || location == directory {
			return target, true
		}
	}
	return Target{}, false
}
