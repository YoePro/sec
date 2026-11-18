package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"sec/internal/diagnostics"
)

var diagnosticIDPattern = regexp.MustCompile(`\b[LPSABF][0-9]{4}\b`)
var exactDiagnosticIDPattern = regexp.MustCompile(`^[LPSABF][0-9]{4}$`)

type DiagnosticMention struct {
	File   string `json:"file"`
	ID     string `json:"id"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

type DiagnosticAuditEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TopicFamily string `json:"topic_family"`
	Retired     bool   `json:"retired"`
}

type DiagnosticNamespaceCandidate struct {
	Namespace            string `json:"namespace"`
	Registered           int    `json:"registered"`
	Retired              int    `json:"retired"`
	NextID               string `json:"next_id"`
	AppendSpaceExhausted bool   `json:"append_space_exhausted"`
}

type DiagnosticAudit struct {
	Registered              int                            `json:"registered"`
	UnregisteredMentions    []DiagnosticMention            `json:"unregistered_mentions"`
	UnmentionedInGovernance []DiagnosticAuditEntry         `json:"unmentioned_in_governance"`
	Candidates              []DiagnosticNamespaceCandidate `json:"namespace_candidates"`
	AllocationPolicy        string                         `json:"allocation_policy"`
}

// auditDiagnosticRegistry compares exact textual ID occurrences with the
// canonical active AND retired registry. Topic families are metadata, not ID
// namespaces or semantic numeric ranges. Candidates append after each namespace's
// highest registered number, skipping prose mentions; gaps and retired IDs are
// never recycled. This is a local suggestion, not an ID reservation/allocation.
// Contract: cmd/yamlstatus/IDEAS.md §8;
// rules/tooling/diagnostics.md §§3(9–10),5(2–10),6(5),(8–10).
func auditDiagnosticRegistry(root, dir string, definitions []diagnostics.Definition) (DiagnosticAudit, error) {
	report := DiagnosticAudit{Registered: len(definitions), UnregisteredMentions: []DiagnosticMention{}, UnmentionedInGovernance: []DiagnosticAuditEntry{}, Candidates: []DiagnosticNamespaceCandidate{},
		AllocationPolicy: "Append after the highest registered number in each namespace, skipping active/retired IDs and exact governance/rulebook mentions; an unused namespace starts at 0000. Suggestions are not reservations; topic families and numeric ranges do not determine semantics."}
	registry := map[string]diagnostics.Definition{}
	mentioned, reserved := map[string]bool{}, map[string]bool{}
	counts, retired, highest := map[byte]int{}, map[byte]int{}, map[byte]int{}
	for _, ns := range "ABFLPS" {
		highest[byte(ns)] = -1
	}
	for _, definition := range definitions {
		if !exactDiagnosticIDPattern.MatchString(definition.ID) {
			return report, fmt.Errorf("invalid diagnostic registry ID %q", definition.ID)
		}
		if _, duplicate := registry[definition.ID]; duplicate {
			return report, fmt.Errorf("duplicate diagnostic registry ID %s", definition.ID)
		}
		registry[definition.ID] = definition
		reserved[definition.ID] = true
		number, _ := strconv.Atoi(definition.ID[1:])
		ns := definition.ID[0]
		highest[ns] = max(highest[ns], number)
		counts[ns]++
		if definition.Retired {
			retired[ns]++
		}
	}
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
	consume := func(file string, data []byte, governance bool) error {
		if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
			return fmt.Errorf("%s: invalid UTF-8 or NUL in diagnostic audit text", file)
		}
		for _, reference := range textIDOccurrences(string(data), diagnosticIDPattern) {
			reserved[reference.ID] = true
			if governance {
				mentioned[reference.ID] = true
			}
			if _, registered := registry[reference.ID]; !registered {
				report.UnregisteredMentions = append(report.UnregisteredMentions, DiagnosticMention{File: file, ID: reference.ID, Line: reference.Line, Column: reference.Column, Text: reference.Text})
			}
		}
		return nil
	}
	for _, doc := range workspace.Documents {
		file, err := filepath.Rel(root, doc.Path)
		if err != nil {
			return report, err
		}
		if err := consume(filepath.ToSlash(file), doc.Source, true); err != nil {
			return report, err
		}
	}
	rules := filepath.Join(root, "rules")
	if _, err := os.Lstat(rules); err == nil {
		err = filepath.WalkDir(rules, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !entry.Type().IsRegular() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".md", ".yaml", ".yml":
			default:
				return nil
			}
			file, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return consume(filepath.ToSlash(file), data, false)
		})
		if err != nil {
			return report, err
		}
	} else if !os.IsNotExist(err) {
		return report, err
	}
	for id, definition := range registry {
		if !mentioned[id] {
			report.UnmentionedInGovernance = append(report.UnmentionedInGovernance, DiagnosticAuditEntry{ID: definition.ID, Name: definition.Name, TopicFamily: definition.Family, Retired: definition.Retired})
		}
	}
	sort.Slice(report.UnmentionedInGovernance, func(i, j int) bool {
		return report.UnmentionedInGovernance[i].ID < report.UnmentionedInGovernance[j].ID
	})
	sort.Slice(report.UnregisteredMentions, func(i, j int) bool {
		l, r := report.UnregisteredMentions[i], report.UnregisteredMentions[j]
		if l.File != r.File {
			return l.File < r.File
		}
		if l.Line != r.Line {
			return l.Line < r.Line
		}
		return l.Column < r.Column
	})
	for _, ns := range "ABFLPS" {
		candidate := DiagnosticNamespaceCandidate{Namespace: string(ns), Registered: counts[byte(ns)], Retired: retired[byte(ns)]}
		for number := highest[byte(ns)] + 1; number <= 9999; number++ {
			id := fmt.Sprintf("%c%04d", ns, number)
			if !reserved[id] {
				candidate.NextID = id
				break
			}
		}
		candidate.AppendSpaceExhausted = candidate.NextID == ""
		report.Candidates = append(report.Candidates, candidate)
	}
	return report, nil
}

// printDiagnosticAudit reports documentation/registry mismatches for review.
// Source examples may intentionally mention unregistered IDs; no registry or
// source is edited, and candidates must be allocated through the registry.
// Contract: cmd/yamlstatus/IDEAS.md §8; rules/tooling/diagnostics.md §5(8–10).
func printDiagnosticAudit(out io.Writer, report DiagnosticAudit, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%d registered diagnostics (including retired)\n", report.Registered)
	for _, mention := range report.UnregisteredMentions {
		fmt.Fprintf(&text, "%s:%d:%d unregistered mention %s: %s\n", mention.File, mention.Line, mention.Column, mention.ID, mention.Text)
	}
	for _, definition := range report.UnmentionedInGovernance {
		fmt.Fprintf(&text, "not mentioned in governance: %s %s (topic %s, retired=%t)\n", definition.ID, definition.Name, definition.TopicFamily, definition.Retired)
	}
	fmt.Fprintln(&text, report.AllocationPolicy)
	for _, candidate := range report.Candidates {
		if candidate.AppendSpaceExhausted {
			fmt.Fprintf(&text, "%s: no unused candidate above the highest registered ID\n", candidate.Namespace)
		} else {
			fmt.Fprintf(&text, "%s: next candidate %s (%d registered, %d retired)\n", candidate.Namespace, candidate.NextID, candidate.Registered, candidate.Retired)
		}
	}
	_, err := io.WriteString(out, text.String())
	return err
}
