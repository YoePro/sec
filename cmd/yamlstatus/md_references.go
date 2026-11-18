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
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var decisionIDPattern = regexp.MustCompile(`\bMD-[0-9]{3,}\b`)
var exactDecisionIDPattern = regexp.MustCompile(`^MD-[0-9]{3,}$`)
var blockerBeforeID = regexp.MustCompile("(?i)\\b(blocked (on|by)|await(s|ing)?|wait(s|ing)? (on|for)|pending|unresolved|undecided|depends on)\\s*[`(]*\\s*$")
var blockerAfterID = regexp.MustCompile("(?i)^\\s*[`)]*\\s*(\\(open\\)|(is (still )?(open|unresolved|undecided)|remain(s)? (open|undecided))\\b)")
var negatedBlocker = regexp.MustCompile(`(?i)\b(not|no longer)\s+(blocked (on|by)|await(s|ing)?|wait(s|ing)? (on|for))\s*$`)

type DecisionReference struct {
	ID     string `json:"id"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

type DecisionReferenceFile struct {
	File       string              `json:"file"`
	References []DecisionReference `json:"references"`
}

type DecisionReferenceReport struct {
	Query string `json:"query"`
	// Closed is a wording audit, never an authoritative decision-status change.
	PossibleStaleBlockers bool                    `json:"possible_stale_blockers"`
	Files                 []DecisionReferenceFile `json:"files"`
}

// decisionLedgerIDs reads canonical decision identities; a present decision
// remains tracked even when its status is decided-implementation-pending.
// Contract: cmd/yamlstatus/IDEAS.md §5; missing-decisions.yaml purpose.
func decisionLedgerIDs(root string) (map[string]bool, error) {
	doc, err := loadDocument(filepath.Join(root, "missing-decisions.yaml"))
	if err != nil {
		return nil, err
	}
	decisions := mappingValue(doc.Root, "decisions")
	if decisions == nil || decisions.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("missing-decisions.yaml: decisions must be a list")
	}
	ids := map[string]bool{}
	for _, decision := range decisions.Content {
		id := mappingValue(decision, "id")
		if id == nil || id.Tag != "!!str" || !exactDecisionIDPattern.MatchString(id.Value) {
			return nil, fmt.Errorf("missing-decisions.yaml:%d: invalid decision ID", decision.Line)
		}
		if ids[id.Value] {
			return nil, fmt.Errorf("missing-decisions.yaml:%d: duplicate decision ID %s", id.Line, id.Value)
		}
		ids[id.Value] = true
	}
	return ids, nil
}

// possibleStaleBlocker conservatively recognizes explicit blocker wording
// adjacent to a reference, including wrapped lines. These are review candidates,
// not proof that a missing decision was closed or its implementation finished.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func possibleStaleBlocker(before, after string) bool {
	before = strings.Join(strings.Fields(before), " ")
	after = strings.Join(strings.Fields(after), " ")
	if negatedBlocker.MatchString(strings.TrimRight(before, " `(\t")) {
		return false
	}
	return blockerBeforeID.MatchString(before) || blockerAfterID.MatchString(after)
}

// findDecisionReferences scans textual references in canonical governance,
// rules, rulebook status and all Go/Sec source files. It does not follow symlinks
// or read generated build/dependency directories. Results use portable paths
// and stable file/line/column order. Exact IDs cannot match longer IDs.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func findDecisionReferences(root, governanceDir, query string) (DecisionReferenceReport, error) {
	report := DecisionReferenceReport{Query: query, PossibleStaleBlockers: query == "closed", Files: []DecisionReferenceFile{}}
	if query != "closed" && !exactDecisionIDPattern.MatchString(query) {
		return report, fmt.Errorf("-md requires an exact MD-NNN ID or closed")
	}
	ids, err := decisionLedgerIDs(root)
	if err != nil {
		return report, err
	}
	governanceRoot, err := filepath.Abs(governanceDir)
	if err != nil {
		return report, err
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				switch entry.Name() {
				case ".git", ".cache", "node_modules", "vendor", "build", "dist":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		portable := filepath.ToSlash(relative)
		inGovernance, err := filepath.Rel(governanceRoot, path)
		if err != nil {
			return err
		}
		selectedGovernance := inGovernance != ".." && !strings.HasPrefix(inGovernance, ".."+string(filepath.Separator))
		ext := filepath.Ext(path)
		if !selectedGovernance && !strings.HasPrefix(portable, "rules/") && portable != "language-rulebook-status.md" && ext != ".go" && ext != ".sec" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Binary files in rule directories do not contain textual references.
		if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
			return nil
		}
		text := string(data)
		file := DecisionReferenceFile{File: portable}
		matches := textIDOccurrences(text, decisionIDPattern)
		for i, match := range matches {
			id := match.ID
			if query != "closed" && id != query || query == "closed" && ids[id] {
				continue
			}
			if query == "closed" {
				start, end := max(0, match.Start-160), min(len(text), match.End+160)
				if i > 0 {
					start = max(start, matches[i-1].End)
				}
				if i+1 < len(matches) {
					end = min(end, matches[i+1].Start)
				}
				if !possibleStaleBlocker(text[start:match.Start], text[match.End:end]) {
					continue
				}
			}
			file.References = append(file.References, DecisionReference{ID: id, Line: match.Line,
				Column: match.Column, Text: match.Text})
		}
		if len(file.References) > 0 {
			report.Files = append(report.Files, file)
		}
		return nil
	})
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].File < report.Files[j].File })
	return report, err
}

// printDecisionReferences groups every occurrence by file; closed-mode wording
// is explicitly advisory and never writes or reclassifies a decision.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func printDecisionReferences(out io.Writer, report DecisionReferenceReport, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	if report.PossibleStaleBlockers {
		if _, err := fmt.Fprintln(out, "Possible stale blocker references (IDs absent from missing-decisions.yaml; review wording):"); err != nil {
			return err
		}
	}
	for _, file := range report.Files {
		if _, err := fmt.Fprintln(out, file.File); err != nil {
			return err
		}
		for _, ref := range file.References {
			if _, err := fmt.Fprintf(out, "  %d:%d %s: %s\n", ref.Line, ref.Column, ref.ID, ref.Text); err != nil {
				return err
			}
		}
	}
	return nil
}
