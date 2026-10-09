package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Problem is a structural defect in the ledger. Unlike a Hazard it is never
// rewritten automatically; `check` exits nonzero while any problem remains.
type Problem struct {
	File        string
	Line        int
	Integration string
	Message     string
}

func (p Problem) String() string {
	where := p.Integration
	if where == "" {
		where = "(fragment)"
	}
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", p.File, p.Line, where, p.Message)
	}
	return fmt.Sprintf("%s: %s: %s", p.File, where, p.Message)
}

// fragmentHeader is the header every fragment carries (governance/README.md,
// "Fragment contract").
var fragmentHeader = []string{"schema_version", "area", "purpose", "integrations"}

// swallowedItem matches a list marker inside an item's text. A list item
// written at a shallower indentation than its siblings continues the previous
// plain scalar instead of starting a new item, so the item disappears into
// the text of the one above it.
var swallowedItem = regexp.MustCompile(`\S {2,}- \S`)

func runCheck(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("yamlstatus check", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Usage = func() { fmt.Fprintln(errOut, editUsage) }
	dir := flags.String("dir", "governance", "Governance directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("check takes no positional arguments\n%s", editUsage)
	}
	problems, files, err := checkWorkspace(*dir)
	if err != nil {
		return err
	}
	for _, problem := range problems {
		fmt.Fprintln(out, problem)
	}
	fmt.Fprintf(out, "%d problems in %d fragments\n", len(problems), files)
	if len(problems) > 0 {
		return fmt.Errorf("governance check failed with %d problems", len(problems))
	}
	return nil
}

// checkWorkspace validates every fragment below dir and the relations between
// them: unique integration IDs and agreement with the fragment list in
// index.yaml. A fragment that does not load is reported and skipped, so one
// broken file does not hide the problems of the others.
func checkWorkspace(dir string) ([]Problem, int, error) {
	files, err := governanceFiles(dir)
	if err != nil {
		return nil, 0, err
	}
	index := loadIndex(dir)
	var problems []Problem
	firstID := map[string]Problem{}
	for _, file := range files {
		document, err := loadDocument(file)
		if err != nil {
			// loadDocument errors already name the file, sometimes with a line.
			problems = append(problems, Problem{File: file, Message: "does not load: " + strings.TrimPrefix(strings.TrimPrefix(err.Error(), file), ": ")})
			continue
		}
		if isIndex(dir, file) {
			continue
		}
		if index.fragments != nil && !index.fragments[relativeFragment(dir, file)] {
			problems = append(problems, Problem{File: file, Message: "fragment is not listed in index.yaml"})
		}
		problems = append(problems, checkDocument(document, index.statuses)...)
		for _, integration := range document.integrations() {
			id := integrationID(integration)
			if id == "" {
				continue
			}
			here := Problem{File: file, Line: integration.Line, Integration: id}
			if first, seen := firstID[id]; seen {
				here.Message = fmt.Sprintf("integration ID is also defined at %s:%d", first.File, first.Line)
				problems = append(problems, here)
				continue
			}
			firstID[id] = here
		}
	}
	for _, listed := range sortedSet(index.fragments) {
		if !containsFragment(dir, files, listed) {
			problems = append(problems, Problem{File: filepath.Join(dir, "index.yaml"), Message: fmt.Sprintf("listed fragment %s does not exist", listed)})
		}
	}
	return problems, len(files), nil
}

// checkDocument validates one fragment on its own. statuses may be nil when no
// index is available; the status vocabulary is then not checked.
func checkDocument(document *Document, statuses map[string]bool) []Problem {
	var problems []Problem
	report := func(line int, integration, format string, args ...interface{}) {
		problems = append(problems, Problem{File: document.Path, Line: line, Integration: integration, Message: fmt.Sprintf(format, args...)})
	}
	root := document.Root
	if root.Kind != yaml.MappingNode {
		report(root.Line, "", "a fragment must be a mapping with %s", strings.Join(fragmentHeader, ", "))
		return problems
	}
	for _, key := range fragmentHeader {
		if mappingValue(root, key) == nil {
			report(root.Line, "", "fragment header has no %s", key)
		}
	}
	list := mappingValue(root, "integrations")
	if list == nil {
		return problems
	}
	if list.Kind != yaml.SequenceNode {
		report(list.Line, "", "integrations is a %s, not a list", kindName(list))
		return problems
	}
	for _, integration := range list.Content {
		problems = append(problems, checkIntegration(document.Path, integration, statuses)...)
	}
	return problems
}

func checkIntegration(file string, integration *yaml.Node, statuses map[string]bool) []Problem {
	var problems []Problem
	id := integrationID(integration)
	report := func(line int, format string, args ...interface{}) {
		problems = append(problems, Problem{File: file, Line: line, Integration: id, Message: fmt.Sprintf(format, args...)})
	}
	if integration.Kind != yaml.MappingNode {
		report(integration.Line, "an integration must be a mapping, not a %s", kindName(integration))
		return problems
	}
	if node := mappingValue(integration, "id"); node == nil || node.Kind != yaml.ScalarNode || node.Value == "" {
		report(integration.Line, "integration has no id")
	}
	switch status := mappingValue(integration, "status"); {
	case status == nil:
		report(integration.Line, "integration has no status")
	case status.Kind != yaml.ScalarNode:
		report(status.Line, "status is a %s, not a scalar", kindName(status))
	case statuses != nil && !statuses[status.Value]:
		report(status.Line, "status %q is not one of %s", status.Value, strings.Join(sortedSet(statuses), ", "))
	}
	for i := 0; i+1 < len(integration.Content); i += 2 {
		key, value := integration.Content[i].Value, integration.Content[i+1]
		switch {
		case key == "verification":
			problems = append(problems, checkVerification(file, id, value)...)
		case checklistFields[key]:
			problems = append(problems, checkChecklist(file, id, key, value)...)
		}
	}
	return problems
}

func checkVerification(file, id string, list *yaml.Node) []Problem {
	if list.Kind != yaml.SequenceNode {
		return []Problem{{File: file, Line: list.Line, Integration: id, Message: fmt.Sprintf("verification is a %s, not a list", kindName(list))}}
	}
	var problems []Problem
	for position, item := range list.Content {
		command, result := mappingValue(item, "command"), mappingValue(item, "result")
		if item.Kind == yaml.MappingNode && len(item.Content) == 4 && nonEmptyScalar(command) && nonEmptyScalar(result) {
			continue
		}
		problems = append(problems, Problem{File: file, Line: item.Line, Integration: id,
			Message: fmt.Sprintf("verification item %d must be a mapping with exactly a command and a result, got %s", position+1, describeItem(item))})
	}
	return problems
}

// checkChecklist accepts plain strings and the two keyed item forms the ledger
// uses: {id, description} and, in required_tests, {id, expectation}.
func checkChecklist(file, id, key string, list *yaml.Node) []Problem {
	if list.Kind != yaml.SequenceNode {
		return []Problem{{File: file, Line: list.Line, Integration: id, Message: fmt.Sprintf("%s is a %s, not a list", key, kindName(list))}}
	}
	var problems []Problem
	report := func(item *yaml.Node, position int, format string, args ...interface{}) {
		problems = append(problems, Problem{File: file, Line: item.Line, Integration: id,
			Message: fmt.Sprintf("%s item %d ", key, position+1) + fmt.Sprintf(format, args...)})
	}
	fixable := map[*yaml.Node]bool{}
	for _, hazard := range checklistHazards(file, id, key, list) {
		fixable[hazard.node] = true
	}
	for position, item := range list.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			if item.ShortTag() != "!!str" {
				report(item, position, "is the %s %s, not text", strings.TrimPrefix(item.ShortTag(), "!!"), describeScalar(item))
			} else if swallowedItem.MatchString(item.Value) {
				report(item, position, "contains a list marker (%q); a following item was probably indented too shallowly and merged into this one", abbreviate(swallowedItem.FindString(item.Value)+"…", 12))
			}
		case yaml.MappingNode:
			if keyedItem(item) {
				continue
			}
			if len(item.Content) == 2 {
				remedy := "quote it by hand"
				if fixable[item] {
					remedy = "quote it (yamlstatus fmt -fix-types)"
				}
				report(item, position, "%q parses as a one-pair mapping because its text contains \": \"; %s", abbreviate(item.Content[0].Value, 50), remedy)
				if text := item.Content[0].Value + ": " + item.Content[1].Value; swallowedItem.MatchString(text) {
					report(item, position, "contains a list marker (%q); a following item was probably indented too shallowly and merged into this one", abbreviate(swallowedItem.FindString(text)+"…", 12))
				}
				continue
			}
			report(item, position, "is a mapping with keys %s; expected text, {id, description} or {id, expectation}", strings.Join(mappingKeys(item), ", "))
		default:
			report(item, position, "is a %s; expected text", kindName(item))
		}
	}
	return problems
}

func keyedItem(item *yaml.Node) bool {
	if len(item.Content) != 4 || !nonEmptyScalar(mappingValue(item, "id")) {
		return false
	}
	return nonEmptyScalar(mappingValue(item, "description")) || nonEmptyScalar(mappingValue(item, "expectation"))
}

func nonEmptyScalar(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && node.ShortTag() == "!!str" && strings.TrimSpace(node.Value) != ""
}

func mappingKeys(mapping *yaml.Node) []string {
	var keys []string
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

func describeItem(item *yaml.Node) string {
	switch item.Kind {
	case yaml.MappingNode:
		return "keys " + strings.Join(mappingKeys(item), ", ")
	case yaml.ScalarNode:
		return fmt.Sprintf("the text %q", abbreviate(item.Value, 50))
	}
	return "a " + kindName(item)
}

// newProblems returns the problems in after that before does not have.
// Problems are compared without line numbers, because an edit shifts lines.
func newProblems(before, after []Problem) []Problem {
	existing := map[string]int{}
	for _, problem := range before {
		existing[problem.Integration+"\x00"+problem.Message]++
	}
	var added []Problem
	for _, problem := range after {
		key := problem.Integration + "\x00" + problem.Message
		if existing[key] > 0 {
			existing[key]--
			continue
		}
		added = append(added, problem)
	}
	return added
}

// ---------------------------------------------------------------------------
// index.yaml

type governanceIndex struct {
	statuses  map[string]bool // nil when index.yaml has no status_values
	fragments map[string]bool // nil when index.yaml has no fragments list
}

func loadIndex(dir string) governanceIndex {
	var index governanceIndex
	document, err := loadDocument(filepath.Join(dir, "index.yaml"))
	if err != nil {
		return index
	}
	if values := mappingValue(document.Root, "status_values"); values != nil && values.Kind == yaml.MappingNode {
		index.statuses = map[string]bool{}
		for _, key := range mappingKeys(values) {
			index.statuses[key] = true
		}
	}
	if list := mappingValue(document.Root, "fragments"); list != nil && list.Kind == yaml.SequenceNode {
		index.fragments = map[string]bool{}
		for _, entry := range list.Content {
			if path := mappingValue(entry, "path"); path != nil && path.Kind == yaml.ScalarNode {
				index.fragments[filepath.ToSlash(filepath.Clean(path.Value))] = true
			}
		}
	}
	return index
}

func isIndex(dir, file string) bool { return relativeFragment(dir, file) == "index.yaml" }

func relativeFragment(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(rel)
}

func containsFragment(dir string, files []string, fragment string) bool {
	for _, file := range files {
		if relativeFragment(dir, file) == fragment {
			return true
		}
	}
	return false
}

func sortedSet(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
