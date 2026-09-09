package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestUnterminatedSelectRetainsBranches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []ast.SelectBranchKind
	}{
		{"unterminated_select", []ast.SelectBranchKind{ast.SelectOperationBranch, ast.SelectTimeoutBranch, ast.SelectDefaultBranch}},
		{"unterminated_empty_select", nil},
		{"unterminated_select_branch", []ast.SelectBranchKind{ast.SelectOperationBranch}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "../../testdata/parser/" + tc.name + "_invalid.sec"
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.NewWithFile(string(source), path)).Parse()
			if !result.HasErrors || result.Fatal {
				t.Fatalf("expected recoverable syntax error: %+v", result)
			}
			found := false
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, "unterminated select ") && d.Primary.Type == lexer.EOF {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing select EOF diagnostic: %+v", result.Diagnostics)
			}
			if len(result.Program.Statements) != 2 {
				t.Fatal("lost enclosing function")
			}
			fn, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
			if !ok || fn.Body == nil || len(fn.Body.Statements) != 1 {
				t.Fatalf("lost function body: %#v", result.Program.Statements[1])
			}
			stmt, ok := fn.Body.Statements[0].(*ast.SelectStatement)
			if !ok {
				t.Fatalf("lost select: %T", fn.Body.Statements[0])
			}
			if len(stmt.Branches) != len(tc.kinds) {
				t.Fatalf("got %d branches, want %d", len(stmt.Branches), len(tc.kinds))
			}
			for i, branch := range stmt.Branches {
				if branch.Kind != tc.kinds[i] || branch.Body == nil || len(branch.Body.Statements) != 1 {
					t.Fatalf("lost branch kind/body: %+v", branch)
				}
				ret, ok := branch.Body.Statements[0].(*ast.ReturnStatement)
				if !ok {
					t.Fatalf("lost return: %T", branch.Body.Statements[0])
				}
				value, ok := ret.Value.(*ast.IntegerLiteral)
				if !ok || value.Value != int64((i+1)*10) {
					t.Fatalf("wrong return/order: %#v", ret.Value)
				}
				if branch.Kind == ast.SelectOperationBranch {
					if branch.Binding == nil || branch.Binding.Value != "value" || branch.Value == nil || branch.Value.String() != "Receive()" {
						t.Fatalf("lost operation/binding: %+v", branch)
					}
				}
				if branch.Kind == ast.SelectTimeoutBranch && branch.Value == nil {
					t.Fatal("lost timeout")
				}
			}
		})
	}
}

func TestMalformedSelectHeaderMakesProgress(t *testing.T) {
	path := "../../testdata/parser/select_nonprogress_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable error: %+v", result)
	}
	if len(result.Program.Statements) != 3 {
		t.Fatal("lost following declaration")
	}
	later, ok := result.Program.Statements[2].(*ast.FunctionDeclaration)
	if !ok || later.Name.Value != "Later" {
		t.Fatalf("lost Later: %#v", result.Program.Statements[2])
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	stmt, ok := fn.Body.Statements[0].(*ast.SelectStatement)
	if !ok || len(stmt.Branches) != 2 {
		t.Fatalf("lost partial/valid branches: %#v", fn.Body.Statements)
	}
	if stmt.Branches[0].Body != nil || stmt.Branches[1].Body == nil {
		t.Fatal("invented partial body or lost valid body")
	}
	ret := stmt.Branches[1].Body.Statements[0].(*ast.ReturnStatement)
	if ret.Value.(*ast.IntegerLiteral).Value != 30 {
		t.Fatal("lost later branch return")
	}
	if len(result.Recovery) == 0 {
		t.Fatal("missing skipped-token recovery record")
	}
}
