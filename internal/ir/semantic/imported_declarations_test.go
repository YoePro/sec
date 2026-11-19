package semantic

import (
	"strings"
	"testing"

	"sec/internal/sema"
)

// The canonical identity of a function never repeats its module qualifier,
// so an imported module's assembled name (`util.Twice`) and its own name
// (`Twice`) give one identity.
// Rules: rules/compiler/semantic_ir.md — § 6.
func TestFunctionIDDropsAssembledModuleQualifier(t *testing.T) {
	types := NewTypeTable()
	assembled := functionID(sema.Function{Name: "util.Twice", Module: "util"}, types)
	own := functionID(sema.Function{Name: "Twice", Module: "util"}, types)
	if assembled != own || assembled != "util::Twice()" {
		t.Fatalf("identities = %q and %q", assembled, own)
	}
}

// An imported declaration has no body, is not extern and names another
// defining module.
// Rules: rules/compiler/semantic_ir.md — § 5(1).
func TestVerifierImportedDeclarations(t *testing.T) {
	for name, check := range map[string]struct {
		function Function
		want     string
	}{
		"valid":        {Function{Imported: true, DefiningModule: "util"}, ""},
		"extern":       {Function{Imported: true, Extern: true, DefiningModule: "util"}, "both extern and an imported"},
		"own module":   {Function{Imported: true, DefiningModule: "main"}, "another defining module"},
		"no module":    {Function{Imported: true}, "another defining module"},
		"stray module": {Function{DefiningModule: "util", Entry: 1}, "another defining module"},
		"body":         {Function{Imported: true, DefiningModule: "util", Blocks: []*Block{{ID: 1}}}, "imported declaration has body"},
	} {
		types := NewTypeTable()
		check.function.ID = "util::Twice()"
		check.function.ReturnType = types.Intern(Type{Kind: TypeVoid, Name: "void"})
		module := &Module{Version: Version, Identity: "main", Types: types}
		err := verifyFunction(module, &check.function, map[FunctionID]*Function{check.function.ID: &check.function})
		if check.want == "" && err != nil || check.want != "" && (err == nil || !strings.Contains(err.Error(), check.want)) {
			t.Errorf("%s: %v, want %q", name, err, check.want)
		}
	}
}
