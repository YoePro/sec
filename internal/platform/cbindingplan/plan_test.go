package cbindingplan

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func moduleProgram(file, name string) *ast.Program {
	return &ast.Program{Statements: []ast.Statement{&ast.ModuleStatement{Token: lexer.Token{File: file}, Path: name}}}
}

// MD-021 § 8.6(1): selected binding modules keep distinct module identities,
// both among themselves and against the program's own modules; files of one
// module directory may share its name.
func TestModuleIdentityProblems(t *testing.T) {
	plan := &Plan{Files: []ModuleFile{
		{Path: "/pkg/sec/platform/c/common/a.sec", Layer: Common},
		{Path: "/pkg/sec/platform/c/common/b.sec", Layer: Common},
		{Path: "/pkg/sec/platform/c/linux/amd64.sec", Layer: Environment},
	}}
	modules := map[string]string{plan.Files[0].Path: "common", plan.Files[1].Path: "common", plan.Files[2].Path: "amd64"}
	if problems := ModuleIdentityProblems(plan, modules, moduleProgram("/pkg/app/app.sec", "app")); len(problems) != 0 {
		t.Fatalf("distinct identities: %v", problems)
	}
	program := moduleProgram("/pkg/app/app.sec", "common")
	if problems := ModuleIdentityProblems(plan, modules, program); len(problems) != 1 || !strings.Contains(problems[0], `binding module "common"`) {
		t.Fatalf("program collision: %v", problems)
	}
	duplicate := &Plan{Files: append(append([]ModuleFile{}, plan.Files...), ModuleFile{Path: "/pkg/sec/platform/c/other/x.sec", Layer: Environment})}
	modules[duplicate.Files[3].Path] = "amd64"
	if problems := ModuleIdentityProblems(duplicate, modules, nil); len(problems) != 1 || !strings.Contains(problems[0], `module "amd64" is declared by selected binding directories`) {
		t.Fatalf("binding collision: %v", problems)
	}
	// A program that already contains the binding files is not a collision.
	merged := &ast.Program{Statements: append(moduleProgram(plan.Files[0].Path, "common").Statements, moduleProgram("/pkg/app/app.sec", "app").Statements...)}
	if problems := ModuleIdentityProblems(plan, modules, merged); len(problems) != 0 {
		t.Fatalf("binding statements in program: %v", problems)
	}
}
