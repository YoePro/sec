package main

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Hazard is a value whose YAML type differs from what the governance field
// means. Formatting never changes it; Fixable hazards are rewritten only with
// an explicit -fix-types request, the others need a human decision.
type Hazard struct {
	File        string
	Line        int
	Integration string
	Key         string
	Message     string
	Fixable     bool
	fix         func()
	node        *yaml.Node // the node fix rewrites, when Fixable
}

// dateKeys hold calendar dates; versionKeys hold version strings.
var (
	dateKeys        = map[string]bool{"integrated": true, "audited": true, "revised": true, "updated": true, "created": true, "reviewed": true}
	versionKeys     = map[string]bool{"document_revision": true, "revision": true}
	checklistFields = map[string]bool{"implemented": true, "remaining": true, "partial": true, "required_tests": true, "unimplemented": true}
)

func findHazards(document *Document) []Hazard {
	var hazards []Hazard
	var walk func(node *yaml.Node, integration string)
	walk = func(node *yaml.Node, integration string) {
		if node.Kind == yaml.MappingNode {
			if id := integrationID(node); id != "" {
				integration = id
			}
			for i := 0; i+1 < len(node.Content); i += 2 {
				key, value := node.Content[i].Value, node.Content[i+1]
				hazards = append(hazards, scalarHazards(document.Path, integration, key, value)...)
				if checklistFields[key] && value.Kind == yaml.SequenceNode {
					hazards = append(hazards, checklistHazards(document.Path, integration, key, value)...)
				}
			}
		}
		for _, child := range node.Content {
			walk(child, integration)
		}
	}
	walk(document.Root, "")
	return hazards
}

func scalarHazards(file, integration, key string, value *yaml.Node) []Hazard {
	if value.Kind != yaml.ScalarNode {
		return nil
	}
	hazard := Hazard{File: file, Line: value.Line, Integration: integration, Key: key}
	tag := value.ShortTag()
	switch {
	case dateKeys[key] && (tag == "!!bool" || tag == "!!null"):
		hazard.Message = fmt.Sprintf("%s holds %s, not a date; decide the real date by hand", key, describeScalar(value))
	case dateKeys[key] && tag == "!!timestamp":
		hazard.Message = fmt.Sprintf("unquoted %s %s decodes as a timestamp; it is written as a string with -fix-types", key, value.Value)
		hazard.Fixable, hazard.fix = true, func() { makeString(value) }
	case versionKeys[key] && (tag == "!!float" || tag == "!!int"):
		hazard.Message = fmt.Sprintf("unquoted %s %s decodes as a number; it is written as the string %q with -fix-types", key, value.Value, value.Value)
		hazard.Fixable, hazard.fix = true, func() { makeString(value) }
	default:
		return nil
	}
	return []Hazard{hazard}
}

// checklistHazards finds plain checklist items that YAML parsed as one-pair
// mappings because their text contains ": ".
func checklistHazards(file, integration, key string, list *yaml.Node) []Hazard {
	var hazards []Hazard
	for _, item := range list.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			continue
		}
		name, value := item.Content[0], item.Content[1]
		if value.Kind != yaml.ScalarNode || value.ShortTag() != "!!str" || name.Style != 0 || value.Style != 0 {
			continue
		}
		text := name.Value + ": " + value.Value
		target := item
		hazards = append(hazards, Hazard{
			File: file, Line: item.Line, Integration: integration, Key: key,
			Message: fmt.Sprintf("%s item %q contains \": \" and parses as a mapping; it is written as one string with -fix-types", key, abbreviate(text, 60)),
			Fixable: true,
			node:    item,
			fix:     func() { *target = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: text, Line: target.Line} },
		})
	}
	return hazards
}

func makeString(node *yaml.Node) {
	node.Tag = "!!str"
	node.Style = 0
}

func describeScalar(node *yaml.Node) string {
	if node.Value == "" {
		return "null"
	}
	return node.Value
}

func abbreviate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

func printHazards(out io.Writer, hazards []Hazard) {
	for _, hazard := range hazards {
		kind := "needs decision"
		if hazard.Fixable {
			kind = "fixable"
		}
		where := hazard.Integration
		if where == "" {
			where = "(fragment)"
		}
		fmt.Fprintf(out, "%s:%d: %s [%s] %s\n", hazard.File, hazard.Line, where, kind, hazard.Message)
	}
}
