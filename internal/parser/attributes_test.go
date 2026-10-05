package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// One general parser reads every consecutive attribute — comments between
// them included — with positional and named arguments, and attaches the set
// in source order to the following declaration; specialized fields are
// derived from the generic attributes.
//
// Rules:
//   - rules/foundations/attributes.md — "General syntax", "Positional and named arguments", "Attribute attachment",
//     "Comments and whitespace", "Attribute order", "Parser representation"
func TestGeneralAttributeParserAttachesMixedSets(t *testing.T) {
	p := New(lexer.New(`module main

@noPanic
// a comment inside the set
@target(os: "linux", arch: "amd64",)
@noAlloc
fn Work() void {
}

@link_name("c_write")
@noAlloc
extern "C" fn write(fd: int32) int32

@address(0x40000000)
let mut Device: uint32

type Reader struct {}

impl Reader {
    @noPanic
    fn Read() int {
        return 1
    }
}
`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	names := func(attributes []*ast.Attribute) string {
		result := []string{}
		for _, attribute := range attributes {
			result = append(result, attribute.Name.Value)
		}
		return strings.Join(result, ",")
	}
	work := program.Statements[1].(*ast.FunctionDeclaration)
	if got := names(work.Attributes); got != "noPanic,target,noAlloc" {
		t.Fatalf("Work attributes = %s", got)
	}
	target := work.Attributes[1]
	if len(target.Arguments) != 2 || target.Arguments[0].Name.Value != "os" || target.Arguments[1].Name.Value != "arch" {
		t.Fatalf("@target arguments = %+v", target.Arguments)
	}
	if value, ok := target.Arguments[0].Value.(*ast.StringLiteral); !ok || value.Value != "linux" {
		t.Fatalf("@target os value = %#v", target.Arguments[0].Value)
	}
	write := program.Statements[2].(*ast.FunctionDeclaration)
	if !write.Extern || write.LinkName != "c_write" || names(write.Attributes) != "link_name,noAlloc" {
		t.Fatalf("extern = %+v", write)
	}
	device := program.Statements[3].(*ast.LetStatement)
	if device.Address == nil || device.AddressToken.Type != lexer.AT || names(device.Attributes) != "address" {
		t.Fatalf("addressed let = %+v", device)
	}
	impl := program.Statements[5].(*ast.ImplStatement)
	if got := names(impl.Members[0].(*ast.FunctionDeclaration).Attributes); got != "noPanic" {
		t.Fatalf("method attributes = %s", got)
	}
}

// Validation is uniform across the closed attribute set: argument form,
// duplicate attributes and argument names with both locations, unknown and
// missing named arguments, allowed targets, the top-level-only attachment
// level, and unknown attributes inside a set.
//
// Rules:
//   - rules/foundations/attributes.md — "Positional and named arguments", "Duplicate attributes", "Duplicate arguments",
//     "Allowed attachment level", "Allowed targets for verified effects", "Closed attribute set"
func TestGeneralAttributeParserValidation(t *testing.T) {
	tests := []struct {
		name, source, id, message string
	}{
		{"duplicate attribute", "@noPanic\n@noAlloc\n@noPanic\nfn F() void {}\n", diagnostics.AttributeDuplicate, "duplicate attribute @noPanic at 3:1; first declared at 1:1"},
		{"duplicate argument", "@target(os: \"linux\", os: \"windows\")\nfn F() void {}\n", diagnostics.AttributeDuplicateArgument, "duplicate argument os in @target at 1:22; first given at 1:9"},
		{"unknown argument name", "@target(rack: \"a\")\nfn F() void {}\n", diagnostics.AttributeInvalidArgument, "@target has no argument named rack; expected os, arch, cpu, device, board at 1:9"},
		{"positional for named", "@interrupt(3)\nfn F() void {}\n", diagnostics.AttributeInvalidArgument, "@interrupt takes only named arguments (vector) at 1:12"},
		{"missing required", "@interrupt()\nfn F() void {}\n", diagnostics.AttributeInvalidArgument, "@interrupt requires the named argument vector at 1:2"},
		{"missing positional", "@address\nlet mut D: uint32\n", diagnostics.AttributeInvalidArgument, "@address requires exactly one positional argument at 1:2"},
		{"argument on argument-free", "@isr(fast)\nfn F() void {}\n", diagnostics.AttributeInvalidArgument, "@isr does not take arguments at 1:5"},
		{"non-string link name", "@link_name(42)\nextern \"C\" fn f() int32\n", diagnostics.AttributeInvalidArgument, "@link_name requires a string literal at 1:12"},
		{"empty link name", "@link_name(\"\")\nextern \"C\" fn f() int32\n", diagnostics.AttributeInvalidArgument, "@link_name requires a non-empty symbol name at 1:12"},
		{"link name on Sec function", "@link_name(\"x\")\nfn F() void {}\n", diagnostics.AttributeNotAllowedOnTarget, "@link_name may only annotate an extern declaration at 2:1"},
		{"address on function", "@address(16)\nfn F() void {}\n", diagnostics.AttributeNotAllowedOnTarget, "@address may only annotate a let declaration at 2:1"},
		{"effect on type", "@noBlock\ntype T int\n", diagnostics.AttributeNotAllowedOnTarget, "@noBlock may only annotate a function or method at 2:1"},
		{"local attachment", "fn F() void {\n    @noPanic\n    let x := 1\n}\n", diagnostics.AttributeNotAllowedOnTarget, "@noPanic may only annotate a top-level declaration at 2:5"},
		{"unknown inside a set", "@noPanic\n@audit\nfn F() void {}\n", diagnostics.UnknownAttribute, "unknown attribute @audit at 2:1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := New(lexer.New(test.source))
			p.ParseProgram()
			found := false
			for _, diagnostic := range p.Diagnostics() {
				if diagnostic.ID == test.id && diagnostic.Message == test.message {
					found = true
				}
			}
			if !found {
				t.Fatalf("diagnostics = %+v, want %s %q", p.Diagnostics(), test.id, test.message)
			}
		})
	}

	p := New(lexer.New("@noPanic\n@audit\nfn F() void {}\n"))
	program := p.ParseProgram()
	if fn, ok := program.Statements[0].(*ast.FunctionDeclaration); !ok || len(fn.Attributes) != 1 || fn.Attributes[0].Name.Value != "noPanic" {
		t.Fatalf("known attributes around an unknown one were not attached: %#v", program.Statements)
	}
}

// Repeating a single-valued attribute with a different value is a conflict
// rather than a duplicate, and a statement @target contradicting the file's
// #target can never be active.
//
// Rules:
//   - rules/foundations/attributes.md — "Conflicting attributes", "Duplicate attributes", "#target compatibility form"
func TestAttributeConflicts(t *testing.T) {
	tests := []struct {
		name, source, id, message string
	}{
		{"two addresses", "@address(0x40000000)\n@address(0x50000000)\nlet mut D: uint32\n", diagnostics.AttributeConflict, "conflicting @address attributes at 2:1 and 1:1; a declaration has exactly one absolute address"},
		{"same address twice", "@address(0x40000000)\n@address(0x40000000)\nlet mut D: uint32\n", diagnostics.AttributeDuplicate, "duplicate attribute @address at 2:1; first declared at 1:1"},
		{"two vectors", "@interrupt(vector: 3)\n@interrupt(vector: 4)\nfn H() void {}\n", diagnostics.AttributeConflict, "conflicting @interrupt attributes at 2:1 and 1:1; a handler binds exactly one interrupt vector"},
		{"contradicts #target", "#target(os: \"linux\", arch: \"amd64\")\nmodule main\n@target(arch: \"arm64\")\nfn F() void {}\n", diagnostics.AttributeConflict, "@target(arch: \"arm64\") at 3:9 contradicts the file's #target(arch: \"amd64\") at 1:1; the declaration could never be active"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := New(lexer.New(test.source))
			p.ParseProgram()
			found := false
			for _, diagnostic := range p.Diagnostics() {
				if diagnostic.ID == test.id && diagnostic.Message == test.message {
					found = true
				}
			}
			if !found {
				t.Fatalf("diagnostics = %+v, want %s %q", p.Diagnostics(), test.id, test.message)
			}
		})
	}
	for _, source := range []string{
		"#target(os: \"linux\", arch: \"amd64\")\nmodule main\n@target(os: \"linux\", device: \"ctrl\")\nfn F() void {}\n",
	} {
		p := New(lexer.New(source))
		p.ParseProgram()
		if len(p.Diagnostics()) != 0 {
			t.Errorf("%q diagnostics = %+v", source, p.Diagnostics())
		}
	}
}
