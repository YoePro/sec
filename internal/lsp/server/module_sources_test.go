package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
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

func TestAssembleModuleRetainsTypesInIncompleteSibling(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "active.sec")
	activeData, err := os.ReadFile("../../../testdata/module_siblings/active.sec")
	if err != nil {
		t.Fatal(err)
	}
	siblingData, err := os.ReadFile("../../../testdata/module_siblings/sibling.sec")
	if err != nil {
		t.Fatal(err)
	}
	siblingText := strings.Replace(string(siblingData), "{ return Status(200) }", "", 1)
	overlay := SourceOverlay{NormalizeSourcePath(filepath.Join(filepath.Dir(activePath), "sibling.sec")): siblingText}
	active := parser.New(lexer.NewWithFile(string(activeData), activePath)).Parse().Program
	AssembleModule(active, activePath, overlay)
	a := sema.NewAnalyzer()
	for _, err := range a.Analyze(active) {
		if err.File == activePath {
			t.Fatalf("active document lost sibling type: %+v", err)
		}
	}
	found := false
	for _, s := range active.Statements {
		if typ, ok := s.(*ast.TypeDeclStatement); ok && typ.Name.Value == "Status" {
			found = true
		}
	}
	if !found {
		t.Fatal("sibling type discarded because its method was a stub")
	}
}
