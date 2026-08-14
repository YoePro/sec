package server

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestAssembleModuleUsesSameModuleOverlay(t *testing.T) {
	dir := t.TempDir()
	activePath := filepath.Join(dir, "active.sec")
	siblingPath := filepath.Join(dir, "sibling.sec")
	newSiblingPath := filepath.Join(dir, "new.sec")
	foreignPath := filepath.Join(dir, "foreign.sec")
	if err := os.WriteFile(activePath, []byte("module sample\n\nfn Active() int { return 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siblingPath, []byte("module sample\n\nfn Disk() int { return 2 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreignPath, []byte("module foreign\n\nfn Foreign() int { return 3 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	activeData, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatal(err)
	}
	active := parser.New(lexer.NewWithFile(string(activeData), activePath)).Parse().Program
	overlay := SourceOverlay{
		NormalizeSourcePath(siblingPath):    "module sample\n\nfn Overlay() int { return 4 }\n",
		NormalizeSourcePath(newSiblingPath): "module sample\n\nfn Unsaved() int { return 5 }\n",
	}

	AssembleModule(active, activePath, overlay)
	functions := map[string]bool{}
	for _, statement := range active.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name != nil {
			functions[function.Name.Value] = true
		}
	}
	if !functions["Active"] || !functions["Overlay"] || !functions["Unsaved"] {
		t.Fatalf("combined module omitted active or overlay declarations: %+v", functions)
	}
	if functions["Disk"] || functions["Foreign"] {
		t.Fatalf("combined module included stale or foreign declarations: %+v", functions)
	}
}
