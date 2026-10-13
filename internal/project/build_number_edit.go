package project

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// BuildNumberEdit owns an exact before/after manifest snapshot. It allocates no
// counter and does not resolve whether automatic numbering is enabled.
// Rules: rules/projects/projects.md — §§27–29,49(5).
type BuildNumberEdit struct {
	path, before, after string
}

func (edit *BuildNumberEdit) Text() string { return edit.after }

// PlanBuildNumberEdit replaces the explicit Project build (target == "") or
// Target-local build in a source snapshot. Missing local storage is never
// silently replaced by the inherited Project value. It performs no disk writes.
// Rules: rules/projects/projects.md — §§8(3),27(2–3),28.1(1–3),49(5).
func PlanBuildNumberEdit(path, text, target string, value uint64) (*BuildNumberEdit, error) {
	document, err := ParseManifestDocument(text)
	if err != nil {
		return nil, err
	}
	after, err := document.ReplaceBuildNumber(target, value)
	if err != nil {
		return nil, err
	}
	return &BuildNumberEdit{path: path, before: text, after: after}, nil
}

// Commit publishes an already chosen number through the common atomic editor.
// The caller owns counter locking and successful-build gating (§29(6–10)); a
// stale snapshot is rejected, including changes to unrelated manifest content.
// Rules: rules/projects/projects.md — §§29(7–12),49(5).
func (edit *BuildNumberEdit) Commit() error {
	_, err := ReplaceManifestSnapshot(edit.path, []byte(edit.before), []byte(edit.after))
	return err
}

// ReplaceBuildNumber changes only the scalar token belonging to the requested
// authoritative field. Parsed key components and source ranges support quoted
// and dotted keys and inline tables; strings and comments cannot impersonate
// declarations. Every other byte is preserved, including CRLF and final newline.
// Rules: rules/projects/projects.md — §§5(2,4–6),8(3),27(2–3),29(11),49(5).
func (document *ManifestDocument) ReplaceBuildNumber(target string, value uint64) (string, error) {
	if value > math.MaxInt64 {
		return "", fmt.Errorf("build exceeds maximum VersionNumber 9223372036854775807")
	}
	path := []string{"version", "build"}
	if target != "" {
		if !ValidProjectKey(target) {
			return "", fmt.Errorf("invalid Target ProjectKey %q", target)
		}
		path = []string{"target", target, "version", "build"}
	}
	// Revalidate the retained text; exported Values are not editing authority.
	validated, err := ParseManifestDocument(document.Text)
	if err != nil {
		return "", err
	}
	table, err := validated.Table(path[:len(path)-1]...)
	if err != nil {
		return "", err
	}
	if _, found := table["build"]; !found {
		return "", fmt.Errorf("missing explicit %s", strings.Join(path, "."))
	}
	if uint64(len(document.Text)) > math.MaxUint32 {
		return "", fmt.Errorf("manifest exceeds source-range capacity")
	}
	var parser unstable.Parser
	parser.Reset([]byte(document.Text))
	var section []string
	var ranges []unstable.Range
	for parser.NextExpression() {
		node := parser.Expression()
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = manifestSyntaxKey(node)
		case unstable.KeyValue:
			collectBuildNumberRanges(node, section, path, &ranges)
		}
	}
	if err := parser.Error(); err != nil {
		return "", fmt.Errorf("cannot locate build field: %w", err)
	}
	if len(ranges) != 1 {
		return "", fmt.Errorf("%s must have exactly one integer source token", strings.Join(path, "."))
	}
	span := ranges[0]
	old, err := parseVersionNumber(string(parser.Raw(span)))
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", strings.Join(path, "."), err)
	}
	if old == value {
		return document.Text, nil
	}
	start, end := int(span.Offset), int(span.Offset)+int(span.Length)
	after := document.Text[:start] + strconv.FormatUint(value, 10) + document.Text[end:]
	if _, err := ParseManifestDocument(after); err != nil {
		return "", fmt.Errorf("invalid build edit: %w", err)
	}
	return after, nil
}

// manifestSyntaxKey copies decoded components before the parser advances.
// Rules: rules/projects/projects.md — §5(2),§49(5).
func manifestSyntaxKey(node *unstable.Node) []string {
	var key []string
	iterator := node.Key()
	for iterator.Next() {
		key = append(key, string(iterator.Node().Data))
	}
	return key
}

// collectBuildNumberRanges descends only real inline-table key/value nodes.
// Arrays and strings are values, never authoritative counter declarations.
// Rules: rules/projects/projects.md — §§27(2–3),49(5).
func collectBuildNumberRanges(node *unstable.Node, parent, wanted []string, ranges *[]unstable.Range) {
	path := append(slices.Clone(parent), manifestSyntaxKey(node)...)
	value := node.Value()
	if slices.Equal(path, wanted) && value.Kind == unstable.Integer {
		*ranges = append(*ranges, value.Raw)
	}
	if value.Kind == unstable.InlineTable && len(path) < len(wanted) && slices.Equal(path, wanted[:len(path)]) {
		children := value.Children()
		for children.Next() {
			collectBuildNumberRanges(children.Node(), path, wanted, ranges)
		}
	}
}
