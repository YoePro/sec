package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// subcommands lists the editing commands; everything else keeps the original
// statistics/selection behavior of runCLI.
var subcommands = map[string]func(args []string, out, errOut io.Writer) error{
	"check":  runCheck,
	"fmt":    runFmt,
	"set":    runSet,
	"add":    runAdd,
	"remove": runRemove,
	"move":   runMove,
	"verify": runVerify,
	"repath": runRepath,
	"new":    runNew,
}

const editUsage = `Editing commands (flags precede positional arguments; nothing is written without -write):
  yamlstatus check
  yamlstatus fmt [-check] [-diff] [-fix-types] [-write] [file ...]
  yamlstatus set    [-write] ID KEY VALUE
  yamlstatus add    [-write] [-at N] ID KEY TEXT
  yamlstatus remove [-write] ID KEY N|TEXT
  yamlstatus move   [-write] ID FROM N TO
  yamlstatus verify [-write] ID COMMAND RESULT
  yamlstatus repath [-write] OLD=NEW ...
  yamlstatus new    [-write] [-status S] [-summary TEXT] FILE ID
All commands accept -dir (default governance).`

type editFlags struct {
	set   *flag.FlagSet
	dir   *string
	write *bool
}

func newEditFlags(name string, errOut io.Writer) editFlags {
	set := flag.NewFlagSet("yamlstatus "+name, flag.ContinueOnError)
	set.SetOutput(errOut)
	set.Usage = func() { fmt.Fprintln(errOut, editUsage) }
	return editFlags{set: set, dir: set.String("dir", "governance", "Governance directory"), write: set.Bool("write", false, "Write the change instead of printing a diff")}
}

func (f editFlags) parse(args []string, positional int) ([]string, error) {
	if err := f.set.Parse(args); err != nil {
		return nil, err
	}
	if positional >= 0 && f.set.NArg() != positional {
		return nil, fmt.Errorf("expected %d arguments, got %d\n%s", positional, f.set.NArg(), editUsage)
	}
	return f.set.Args(), nil
}

// ---------------------------------------------------------------------------
// fmt

func runFmt(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("fmt", errOut)
	check := flags.set.Bool("check", false, "Exit nonzero when a fragment is not canonical")
	showDiff := flags.set.Bool("diff", false, "Print the full diff of every reformatted fragment")
	fixTypes := flags.set.Bool("fix-types", false, "Rewrite fixable type hazards")
	files, err := flags.parse(args, -1)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if files, err = governanceFiles(*flags.dir); err != nil {
			return err
		}
	}
	var nonCanonical []string
	unresolved := 0
	for _, file := range files {
		document, err := loadDocument(file)
		if err != nil {
			return err
		}
		hazards := findHazards(document)
		for _, hazard := range hazards {
			if *fixTypes && hazard.Fixable {
				hazard.fix()
				continue
			}
			unresolved++
			printHazards(out, []Hazard{hazard})
		}
		rendered, err := render(document.Root)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if err := verifyRendered(document.Root, rendered); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if bytes.Equal(rendered, document.Source) {
			continue
		}
		nonCanonical = append(nonCanonical, file)
		if *showDiff {
			fmt.Fprint(out, unifiedDiff(file, document.Source, rendered))
		}
		if *flags.write {
			if err := writeChecked(file, document.Root, rendered); err != nil {
				return err
			}
			fmt.Fprintf(out, "formatted %s\n", file)
		} else if !*showDiff {
			fmt.Fprintf(out, "not canonical: %s (%d changed lines)\n", file, changedLines(document.Source, rendered))
		}
	}
	fmt.Fprintf(out, "%d of %d fragments not canonical; %d type hazards reported\n", len(nonCanonical), len(files), unresolved)
	if *check && !*flags.write && len(nonCanonical) > 0 {
		return fmt.Errorf("%d fragments are not canonical; run yamlstatus fmt -write", len(nonCanonical))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Single-integration edits

// editIntegration loads the workspace, locates id, requires its fragment to be
// canonical, applies change, and shows or writes the result.
func editIntegration(flags editFlags, id string, out io.Writer, change func(*Workspace, *yaml.Node) error) error {
	workspace, err := loadWorkspace(*flags.dir)
	if err != nil {
		return err
	}
	document, integration, err := workspace.find(id)
	if err != nil {
		return err
	}
	if err := requireCanonical(document); err != nil {
		return err
	}
	if err := change(workspace, integration); err != nil {
		return err
	}
	return commit(document, *flags.write, out)
}

func requireCanonical(document *Document) error {
	rendered, err := render(document.Root)
	if err != nil {
		return fmt.Errorf("%s: %w", document.Path, err)
	}
	if !bytes.Equal(rendered, document.Source) {
		return fmt.Errorf("%s is not canonical; run `yamlstatus fmt -write %s` first so the edit diff contains only the edit", document.Path, document.Path)
	}
	return nil
}

func commit(document *Document, write bool, out io.Writer) error {
	rendered, err := render(document.Root)
	if err != nil {
		return fmt.Errorf("%s: %w", document.Path, err)
	}
	if err := verifyRendered(document.Root, rendered); err != nil {
		return fmt.Errorf("%s: %w", document.Path, err)
	}
	if bytes.Equal(rendered, document.Source) {
		return fmt.Errorf("%s: the edit changes nothing", document.Path)
	}
	if err := rejectNewProblems(document); err != nil {
		return err
	}
	fmt.Fprint(out, unifiedDiff(document.Path, document.Source, rendered))
	if !write {
		fmt.Fprintln(out, "(dry run; pass -write to apply)")
		return nil
	}
	if err := writeChecked(document.Path, document.Root, rendered); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %s\n", document.Path)
	return nil
}

// rejectNewProblems refuses an edit that introduces a structural problem
// (see check.go). Problems the fragment already had do not block the edit.
func rejectNewProblems(document *Document) error {
	var before []Problem
	if original, err := parseDocument(document.Path, document.Source); err == nil {
		before = checkDocument(original, nil)
	}
	added := newProblems(before, checkDocument(document, nil))
	if len(added) == 0 {
		return nil
	}
	lines := make([]string, len(added))
	for i, problem := range added {
		lines[i] = problem.String()
	}
	return fmt.Errorf("the edit would introduce governance problems; nothing was written:\n%s", strings.Join(lines, "\n"))
}

// writeChecked writes rendered text and reloads it, so a written fragment is
// always parseable, free of duplicate keys and equal to the intended values.
func writeChecked(path string, root *yaml.Node, rendered []byte) error {
	info, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, rendered, mode); err != nil {
		return err
	}
	reloaded, err := loadDocument(path)
	if err != nil {
		return fmt.Errorf("written fragment failed validation: %w", err)
	}
	return verifyRendered(root, reloaded.Source)
}

func runSet(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("set", errOut)
	positional, err := flags.parse(args, 3)
	if err != nil {
		return err
	}
	id, key, value := positional[0], positional[1], positional[2]
	if key == "id" {
		return fmt.Errorf("the integration id cannot be changed with set")
	}
	return editIntegration(flags, id, out, func(workspace *Workspace, integration *yaml.Node) error {
		if key == "status" {
			if err := validateStatus(*flags.dir, value); err != nil {
				return err
			}
		}
		if existing := mappingValue(integration, key); existing != nil {
			if existing.Kind != yaml.ScalarNode {
				return fmt.Errorf("%s.%s is a %s, not a scalar; use add/remove/move", id, key, kindName(existing))
			}
			*existing = *stringNode(value)
			return nil
		}
		integration.Content = append(integration.Content, stringNode(key), stringNode(value))
		return nil
	})
}

func runAdd(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("add", errOut)
	at := flags.set.Int("at", 0, "Insert at this 1-based position instead of appending")
	positional, err := flags.parse(args, 3)
	if err != nil {
		return err
	}
	id, key, text := positional[0], positional[1], positional[2]
	return editIntegration(flags, id, out, func(_ *Workspace, integration *yaml.Node) error {
		list, err := listField(integration, id, key, true)
		if err != nil {
			return err
		}
		for _, item := range list.Content {
			if item.Kind == yaml.ScalarNode && item.Value == text {
				return fmt.Errorf("%s.%s already contains this item", id, key)
			}
		}
		return insertItem(list, stringNode(text), *at, id, key)
	})
}

func runRemove(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("remove", errOut)
	positional, err := flags.parse(args, 3)
	if err != nil {
		return err
	}
	id, key, selector := positional[0], positional[1], positional[2]
	return editIntegration(flags, id, out, func(_ *Workspace, integration *yaml.Node) error {
		list, err := listField(integration, id, key, false)
		if err != nil {
			return err
		}
		index, err := selectItem(list, selector, id, key)
		if err != nil {
			return err
		}
		list.Content = append(list.Content[:index], list.Content[index+1:]...)
		return nil
	})
}

func runMove(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("move", errOut)
	positional, err := flags.parse(args, 4)
	if err != nil {
		return err
	}
	id, from, selector, to := positional[0], positional[1], positional[2], positional[3]
	if from == to {
		return fmt.Errorf("source and destination are both %q", from)
	}
	return editIntegration(flags, id, out, func(_ *Workspace, integration *yaml.Node) error {
		source, err := listField(integration, id, from, false)
		if err != nil {
			return err
		}
		index, err := selectItem(source, selector, id, from)
		if err != nil {
			return err
		}
		destination, err := listField(integration, id, to, true)
		if err != nil {
			return err
		}
		item := source.Content[index]
		source.Content = append(source.Content[:index], source.Content[index+1:]...)
		destination.Content = append(destination.Content, item)
		return nil
	})
}

func runVerify(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("verify", errOut)
	positional, err := flags.parse(args, 3)
	if err != nil {
		return err
	}
	id, command, result := positional[0], positional[1], positional[2]
	return editIntegration(flags, id, out, func(_ *Workspace, integration *yaml.Node) error {
		list, err := listField(integration, id, "verification", true)
		if err != nil {
			return err
		}
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			stringNode("command"), stringNode(command), stringNode("result"), stringNode(result),
		}}
		return insertItem(list, entry, 1, id, "verification")
	})
}

// ---------------------------------------------------------------------------
// Workspace-wide edits

func runRepath(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("repath", errOut)
	pairs, err := flags.parse(args, -1)
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("repath needs at least one OLD=NEW pair")
	}
	mapping := map[string]string{}
	for _, pair := range pairs {
		old, replacement, ok := strings.Cut(pair, "=")
		if !ok || old == "" || replacement == "" || old == replacement {
			return fmt.Errorf("invalid pair %q; expected OLD=NEW", pair)
		}
		mapping[old] = replacement
	}
	workspace, err := loadWorkspace(*flags.dir)
	if err != nil {
		return err
	}
	touchedIDs := map[string]bool{}
	var touched []*Document
	for _, document := range workspace.Documents {
		changed := false
		for _, integration := range document.integrations() {
			if repathNode(integration, mapping) {
				changed = true
				touchedIDs[integrationID(integration)] = true
			}
		}
		if changed {
			touched = append(touched, document)
		}
	}
	if len(touched) == 0 {
		return fmt.Errorf("no governance list contains any of the old paths")
	}
	for _, document := range touched {
		original, _ := loadDocument(document.Path)
		if err := requireCanonical(original); err != nil {
			return err
		}
	}
	for _, document := range touched {
		if err := commit(document, *flags.write, out); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(touchedIDs))
	for id := range touchedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintf(out, "integrations touched (add a verification item to each): %s\n", strings.Join(ids, ", "))
	return nil
}

// repathNode rewrites list items equal to an old path. When the new path is
// already in the same list, the old item is dropped instead of duplicated.
func repathNode(node *yaml.Node, mapping map[string]string) bool {
	changed := false
	if node.Kind == yaml.SequenceNode {
		present := map[string]bool{}
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				present[item.Value] = true
			}
		}
		kept := node.Content[:0]
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				if replacement, ok := mapping[item.Value]; ok {
					changed = true
					if present[replacement] {
						continue
					}
					item.Value = replacement
					present[replacement] = true
				}
			}
			kept = append(kept, item)
		}
		node.Content = kept
	}
	for _, child := range node.Content {
		if repathNode(child, mapping) {
			changed = true
		}
	}
	return changed
}

func runNew(args []string, out, errOut io.Writer) error {
	flags := newEditFlags("new", errOut)
	status := flags.set.String("status", "planned", "Initial status")
	summary := flags.set.String("summary", "", "Initial summary (required)")
	positional, err := flags.parse(args, 2)
	if err != nil {
		return err
	}
	file, id := positional[0], positional[1]
	if *summary == "" {
		return fmt.Errorf("new requires -summary")
	}
	if err := validateStatus(*flags.dir, *status); err != nil {
		return err
	}
	if !filepath.IsAbs(file) && !strings.HasPrefix(filepath.Clean(file), filepath.Clean(*flags.dir)+string(filepath.Separator)) {
		file = filepath.Join(*flags.dir, file)
	}
	workspace, err := loadWorkspace(*flags.dir)
	if err != nil {
		return err
	}
	if _, _, err := workspace.find(id); err == nil {
		return fmt.Errorf("integration ID %q already exists", id)
	}
	var document *Document
	for _, candidate := range workspace.Documents {
		if filepath.Clean(candidate.Path) == filepath.Clean(file) {
			document = candidate
		}
	}
	if document == nil {
		return fmt.Errorf("fragment %s not found under %s", file, *flags.dir)
	}
	if err := requireCanonical(document); err != nil {
		return err
	}
	list := mappingValue(document.Root, "integrations")
	if list == nil || list.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s has no integrations list", document.Path)
	}
	summaryNode := stringNode(*summary)
	summaryNode.Style = yaml.FoldedStyle
	list.Content = append(list.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		stringNode("id"), stringNode(id),
		stringNode("status"), stringNode(*status),
		stringNode("summary"), summaryNode,
		stringNode("rules"), emptyList(),
		stringNode("implemented"), emptyList(),
		stringNode("remaining"), emptyList(),
	}})
	return commit(document, *flags.write, out)
}

// ---------------------------------------------------------------------------
// Helpers

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func emptyList() *yaml.Node { return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"} }

func kindName(node *yaml.Node) string {
	switch node.Kind {
	case yaml.SequenceNode:
		return "list"
	case yaml.MappingNode:
		return "mapping"
	}
	return "scalar"
}

// listField returns the list stored under key, creating an empty list at the
// end of the integration when create is set.
func listField(integration *yaml.Node, id, key string, create bool) (*yaml.Node, error) {
	list := mappingValue(integration, key)
	if list == nil {
		if !create {
			return nil, fmt.Errorf("%s has no %s list", id, key)
		}
		list = emptyList()
		integration.Content = append(integration.Content, stringNode(key), list)
	}
	if list.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s.%s is a %s, not a list", id, key, kindName(list))
	}
	return list, nil
}

func insertItem(list, item *yaml.Node, at int, id, key string) error {
	if at == 0 {
		list.Content = append(list.Content, item)
		return nil
	}
	if at < 1 || at > len(list.Content)+1 {
		return fmt.Errorf("-at %d is outside %s.%s (1..%d)", at, id, key, len(list.Content)+1)
	}
	list.Content = append(list.Content[:at-1], append([]*yaml.Node{item}, list.Content[at-1:]...)...)
	return nil
}

// selectItem resolves a 1-based position or the exact text of one item.
func selectItem(list *yaml.Node, selector, id, key string) (int, error) {
	if position, err := strconv.Atoi(selector); err == nil {
		if position < 1 || position > len(list.Content) {
			return 0, fmt.Errorf("%s.%s has %d items; position %d does not exist", id, key, len(list.Content), position)
		}
		return position - 1, nil
	}
	found := -1
	for index, item := range list.Content {
		if item.Kind == yaml.ScalarNode && item.Value == selector {
			if found >= 0 {
				return 0, fmt.Errorf("%s.%s contains this text more than once; select by position", id, key)
			}
			found = index
		}
	}
	if found < 0 {
		return 0, fmt.Errorf("%s.%s has no item with exactly this text", id, key)
	}
	return found, nil
}

// validateStatus checks a status against status_values in index.yaml when the
// governance directory has one.
func validateStatus(dir, status string) error {
	index, err := loadDocument(filepath.Join(dir, "index.yaml"))
	if err != nil {
		return nil
	}
	values := mappingValue(index.Root, "status_values")
	if values == nil || values.Kind != yaml.MappingNode {
		return nil
	}
	var allowed []string
	for i := 0; i+1 < len(values.Content); i += 2 {
		if values.Content[i].Value == status {
			return nil
		}
		allowed = append(allowed, values.Content[i].Value)
	}
	return fmt.Errorf("status %q is not one of %s", status, strings.Join(allowed, ", "))
}

// ---------------------------------------------------------------------------
// Diff

func splitLines(text []byte) []string {
	trimmed := strings.TrimSuffix(string(text), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func changedLines(old, new []byte) int {
	count := 0
	for _, op := range diffOps(splitLines(old), splitLines(new)) {
		if op.kind != ' ' {
			count++
		}
	}
	return count
}

type diffOp struct {
	kind byte // ' ', '-', '+'
	text string
}

// diffOps computes a line diff through a longest common subsequence of the
// region between the common prefix and suffix.
func diffOps(a, b []string) []diffOp {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	var ops []diffOp
	for _, line := range a[:prefix] {
		ops = append(ops, diffOp{' ', line})
	}
	x, y := a[prefix:len(a)-suffix], b[prefix:len(b)-suffix]
	table := make([][]int32, len(x)+1)
	for i := range table {
		table[i] = make([]int32, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < len(x) && j < len(y) {
		switch {
		case x[i] == y[j]:
			ops = append(ops, diffOp{' ', x[i]})
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, diffOp{'-', x[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', y[j]})
			j++
		}
	}
	for ; i < len(x); i++ {
		ops = append(ops, diffOp{'-', x[i]})
	}
	for ; j < len(y); j++ {
		ops = append(ops, diffOp{'+', y[j]})
	}
	for _, line := range a[len(a)-suffix:] {
		ops = append(ops, diffOp{' ', line})
	}
	return ops
}

// unifiedDiff prints hunks with three lines of context.
func unifiedDiff(path string, old, new []byte) string {
	ops := diffOps(splitLines(old), splitLines(new))
	const context = 3
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", path, path)
	oldLine, newLine := make([]int, len(ops)+1), make([]int, len(ops)+1)
	for k, op := range ops {
		oldLine[k+1], newLine[k+1] = oldLine[k], newLine[k]
		if op.kind != '+' {
			oldLine[k+1]++
		}
		if op.kind != '-' {
			newLine[k+1]++
		}
	}
	for k := 0; k < len(ops); {
		if ops[k].kind == ' ' {
			k++
			continue
		}
		start := k - context
		if start < 0 {
			start = 0
		}
		end := k
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := end
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run == len(ops) || run-end > 2*context {
				end += context
				if end > len(ops) {
					end = len(ops)
				}
				break
			}
			end = run
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldLine[start]+1, oldLine[end]-oldLine[start], newLine[start]+1, newLine[end]-newLine[start])
		for _, op := range ops[start:end] {
			b.WriteByte(op.kind)
			b.WriteString(op.text)
			b.WriteByte('\n')
		}
		k = end
	}
	return b.String()
}
