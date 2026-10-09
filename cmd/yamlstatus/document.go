package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is one governance fragment parsed as a yaml.Node tree. Editing works
// on the tree so untouched values keep their exact content.
type Document struct {
	Path   string
	Source []byte
	Root   *yaml.Node // the document's top-level value node
}

// loadDocument parses one fragment and rejects the YAML features governance
// does not use (aliases, multiple documents) and duplicate mapping keys, which
// ordinary decoding silently resolves by dropping one value.
func loadDocument(path string) (*Document, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDocument(path, source)
}

func parseDocument(path string, source []byte) (*Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		return nil, fmt.Errorf("%s: multiple YAML documents are not supported", path)
	}
	if len(root.Content) != 1 {
		return nil, fmt.Errorf("%s: empty YAML document", path)
	}
	document := &Document{Path: path, Source: source, Root: root.Content[0]}
	if err := checkNode(path, document.Root); err != nil {
		return nil, err
	}
	return document, nil
}

func checkNode(path string, node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("%s:%d: anchors and aliases are not supported", path, node.Line)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode {
				return fmt.Errorf("%s:%d: only scalar mapping keys are supported", path, key.Line)
			}
			if first, duplicate := seen[key.Value]; duplicate {
				return fmt.Errorf("%s:%d: duplicate key %q (first defined at line %d)", path, key.Line, key.Value, first)
			}
			seen[key.Value] = key.Line
		}
	}
	for _, child := range node.Content {
		if err := checkNode(path, child); err != nil {
			return err
		}
	}
	return nil
}

// governanceFiles lists the YAML fragments below dir in a stable order.
func governanceFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !d.IsDir() && (ext == ".yaml" || ext == ".yml") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// mappingValue returns the value node for key, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// integrations returns the integration mappings of a document: the entries of
// an `integrations` list, a top-level list, or a single top-level integration.
func (d *Document) integrations() []*yaml.Node {
	switch d.Root.Kind {
	case yaml.SequenceNode:
		return d.Root.Content
	case yaml.MappingNode:
		if list := mappingValue(d.Root, "integrations"); list != nil && list.Kind == yaml.SequenceNode {
			return list.Content
		}
		if mappingValue(d.Root, "id") != nil {
			return []*yaml.Node{d.Root}
		}
	}
	return nil
}

func integrationID(node *yaml.Node) string {
	if id := mappingValue(node, "id"); id != nil && id.Kind == yaml.ScalarNode {
		return id.Value
	}
	return ""
}

// Workspace is the set of governance fragments an edit command operates on.
type Workspace struct {
	Documents []*Document
}

func loadWorkspace(dir string) (*Workspace, error) {
	files, err := governanceFiles(dir)
	if err != nil {
		return nil, err
	}
	workspace := &Workspace{}
	for _, file := range files {
		document, err := loadDocument(file)
		if err != nil {
			return nil, err
		}
		workspace.Documents = append(workspace.Documents, document)
	}
	return workspace, nil
}

// find locates exactly one integration by ID across the workspace.
func (w *Workspace) find(id string) (*Document, *yaml.Node, error) {
	var foundDocument *Document
	var found *yaml.Node
	for _, document := range w.Documents {
		for _, integration := range document.integrations() {
			if integrationID(integration) != id {
				continue
			}
			if found != nil {
				return nil, nil, fmt.Errorf("integration ID %q is defined in %s and %s", id, foundDocument.Path, document.Path)
			}
			foundDocument, found = document, integration
		}
	}
	if found == nil {
		return nil, nil, fmt.Errorf("integration ID %q not found", id)
	}
	return foundDocument, found, nil
}
