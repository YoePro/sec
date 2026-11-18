package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const appliedCorrectionPrefix = "rules/corrections/applied/"

var appliedCorrectionStatus = regexp.MustCompile(`(?i)^applied\b`)
var correctionMarkdownLink = regexp.MustCompile(`\]\(\s*(?:<([^>]+)>|([^\s)]+))`)

type CorrectionLinkIssue struct {
	PathReference
	Reason string `json:"reason"`
}

type PendingCorrection struct {
	File   string `json:"file"`
	Status string `json:"status"`
}

type CorrectionAudit struct {
	CheckedLinks        int                   `json:"checked_links"`
	ArchivedFiles       int                   `json:"archived_files"`
	BrokenLinks         []CorrectionLinkIssue `json:"broken_links"`
	UnreferencedApplied []string              `json:"unreferenced_applied"`
	Pending             []PendingCorrection   `json:"pending"`
	AwaitingArchive     []PendingCorrection   `json:"awaiting_archive"`
}

// correctionStatus reads the explicit leading Markdown/YAML status metadata.
// Archive location indicates normative synchronization, not implementation
// completion: statuses such as decided-implementation-pending are preserved.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func correctionStatus(text string) string {
	for index, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if index == 0 && line == "---" {
			continue // YAML document/front-matter marker, not a section break.
		}
		if strings.HasPrefix(line, "##") || strings.TrimSpace(line) == "---" || strings.HasPrefix(strings.TrimSpace(line), "```") {
			break
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			continue // Nested YAML/example status is not document metadata.
		}
		line = strings.TrimSpace(strings.ReplaceAll(line, "**", ""))
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if strings.HasPrefix(strings.ToLower(line), "status:") {
			return strings.Trim(strings.TrimSpace(line[len("status:"):]), "\"'")
		}
	}
	return "unspecified"
}

// mentionsCorrection accepts exact repository-relative path mentions rather
// than similarly prefixed archive names. A link fragment or prose punctuation
// may follow the path. A correction cannot establish its own rulebook linkage.
// Contract: cmd/yamlstatus/IDEAS.md §6.
func mentionsCorrection(text, path string) bool {
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], path)
		if index < 0 {
			return false
		}
		index += offset
		end := index + len(path)
		before := index == 0 || strings.ContainsRune(" \t\r\n\"'`<([{:=", rune(text[index-1]))
		after := end == len(text) || strings.ContainsRune(" \t\r\n\"'`>)]},;:#?", rune(text[end]))
		if !after && text[end] == '.' {
			after = end+1 == len(text) || strings.ContainsRune(" \t\r\n", rune(text[end+1]))
		}
		if before && after {
			return true
		}
		offset = end
	}
	return false
}

// correctionRulebookLinks resolves local Markdown link destinations relative
// to their owning rulebook. Encoded spaces, angle destinations and fragments
// cannot make a real archive connection appear disconnected. Remote links and
// paths outside the archive never establish local correction provenance.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func correctionRulebookLinks(text, file string) []string {
	var links []string
	for _, match := range correctionMarkdownLink.FindAllStringSubmatch(text, -1) {
		target := match[1]
		if target == "" {
			target = match[2]
		}
		parsed, err := url.Parse(target)
		if err != nil || parsed.IsAbs() || parsed.Host != "" || strings.HasPrefix(parsed.Path, "/") {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(file), filepath.FromSlash(parsed.Path))))
		if strings.HasPrefix(path, appliedCorrectionPrefix) {
			links = append(links, path)
		}
	}
	return links
}

// auditCorrections reads canonical governance nodes and rulebooks, checks every
// corrections_applied path against regular files in the archive, and reports
// disconnected archive entries and unarchived candidates. It does not follow
// symlinks, move files, or decide implementation/MD completion. Findings are
// audit data; malformed input and filesystem failures are command errors.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func auditCorrections(root, dir string) (CorrectionAudit, error) {
	report := CorrectionAudit{BrokenLinks: []CorrectionLinkIssue{}, UnreferencedApplied: []string{}, Pending: []PendingCorrection{}, AwaitingArchive: []PendingCorrection{}}
	root, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return report, err
	}
	workspace, err := loadWorkspace(dir)
	if err != nil {
		return report, err
	}
	archive := map[string]bool{}
	var referenceTexts []string
	var links []PathReference
	for _, doc := range workspace.Documents {
		file, err := filepath.Rel(root, doc.Path)
		if err != nil {
			return report, err
		}
		for _, integration := range doc.integrations() {
			status := ""
			if node := mappingValue(integration, "status"); node != nil {
				status = node.Value
			}
			// Every scalar in an integration may contain a documentary link.
			var visit func(*yaml.Node) error
			visit = func(node *yaml.Node) error {
				if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
					referenceTexts = append(referenceTexts, node.Value)
				}
				if node.Kind == yaml.MappingNode {
					for i := 0; i+1 < len(node.Content); i += 2 {
						if node.Content[i].Value != "corrections_applied" {
							continue
						}
						list := node.Content[i+1]
						if list.Kind != yaml.SequenceNode {
							return fmt.Errorf("%s:%d: corrections_applied must be a list", file, list.Line)
						}
						for _, path := range list.Content {
							if path.Kind != yaml.ScalarNode || path.Tag != "!!str" || strings.TrimSpace(path.Value) == "" {
								return fmt.Errorf("%s:%d: corrections_applied requires non-empty paths", file, path.Line)
							}
							links = append(links, PathReference{File: filepath.ToSlash(file), Line: path.Line, ID: integrationID(integration), Status: status, Field: "corrections_applied", Path: path.Value})
						}
					}
				}
				for _, child := range node.Content {
					if err := visit(child); err != nil {
						return err
					}
				}
				return nil
			}
			if err := visit(integration); err != nil {
				return report, err
			}
		}
	}
	// Scan only rulebooks and correction candidates, not unrelated repository
	// sources or build artifacts. Root candidates need a correction filename
	// or first Markdown heading; ordinary proposals are not silently classified.
	inspect := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, appliedCorrectionPrefix) {
			archive[rel] = true
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		inCorrections := strings.HasPrefix(rel, "rules/corrections/")
		if strings.HasPrefix(rel, "rules/") && !inCorrections {
			referenceTexts = append(referenceTexts, text)
			referenceTexts = append(referenceTexts, correctionRulebookLinks(text, rel)...)
			return nil
		}
		if strings.EqualFold(entry.Name(), "README.md") {
			return nil
		}
		first, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
		candidate := inCorrections || strings.Contains(strings.ToLower(entry.Name()), "correction") || strings.HasPrefix(first, "#") && strings.Contains(strings.ToLower(first), "correction")
		if candidate {
			pending := PendingCorrection{File: rel, Status: correctionStatus(text)}
			if appliedCorrectionStatus.MatchString(pending.Status) {
				report.AwaitingArchive = append(report.AwaitingArchive, pending)
			} else {
				report.Pending = append(report.Pending, pending)
			}
		}
		return nil
	}
	rules := filepath.Join(root, "rules")
	if _, err := os.Lstat(rules); err == nil {
		if err := filepath.WalkDir(rules, inspect); err != nil {
			return report, err
		}
	} else if !os.IsNotExist(err) {
		return report, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return report, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			if err := inspect(filepath.Join(root, entry.Name()), entry, nil); err != nil {
				return report, err
			}
		}
	}
	for _, ref := range links {
		report.CheckedLinks++
		path := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(ref.Path, "\\", "/")))
		reason := ""
		if !strings.HasPrefix(path, appliedCorrectionPrefix) {
			reason = "path is outside rules/corrections/applied/"
		} else if _, exists := archive[path]; !exists {
			reason = "missing or non-regular archived file"
		}
		if reason != "" {
			report.BrokenLinks = append(report.BrokenLinks, CorrectionLinkIssue{PathReference: ref, Reason: reason})
		} else {
			archive[path] = false // An exact listed link establishes the connection.
		}
	}
	for path, unreferenced := range archive {
		if unreferenced {
			connected := false
			for _, text := range referenceTexts {
				if mentionsCorrection(text, path) {
					connected = true
					break
				}
			}
			if !connected {
				report.UnreferencedApplied = append(report.UnreferencedApplied, path)
			}
		}
	}
	report.ArchivedFiles = len(archive)
	sort.Strings(report.UnreferencedApplied)
	sort.Slice(report.Pending, func(i, j int) bool { return report.Pending[i].File < report.Pending[j].File })
	sort.Slice(report.AwaitingArchive, func(i, j int) bool { return report.AwaitingArchive[i].File < report.AwaitingArchive[j].File })
	return report, nil
}

// printCorrectionAudit emits portable, deterministic findings for review;
// normative application and runtime implementation remain separate concepts.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func printCorrectionAudit(out io.Writer, report CorrectionAudit, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%d applied links checked; %d archived files\n", report.CheckedLinks, report.ArchivedFiles)
	for _, issue := range report.BrokenLinks {
		fmt.Fprintf(&text, "%s:%d %s: %s (%s)\n", issue.File, issue.Line, issue.ID, issue.Path, issue.Reason)
	}
	for _, path := range report.UnreferencedApplied {
		fmt.Fprintf(&text, "unreferenced archive: %s\n", path)
	}
	for _, pending := range report.Pending {
		fmt.Fprintf(&text, "pending: %s [%s]\n", pending.File, pending.Status)
	}
	for _, pending := range report.AwaitingArchive {
		fmt.Fprintf(&text, "applied outside archive: %s [%s]\n", pending.File, pending.Status)
	}
	_, err := io.WriteString(out, text.String())
	return err
}
